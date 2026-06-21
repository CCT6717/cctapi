# Native Free Pool — 安全审查报告

- 审查日期: 2026-06-21
- 审查范围: free_pool.go / config.go (FreeProvider 相关) / free_pool_test.go / relay.go L385-392 / fallback.json.example
- 审查方式: 只读代码审查, 未执行任何写操作

---

## 1. API Key 安全性

### 1.1 SafeKeyHash 可逆性

**分析**: `SafeKeyHash` 使用 SHA256 完整哈希后取前 4 字节 (32 bits), 输出 8 字符 hex 串。SHA256 是单向哈希, 不可逆推原始 key。碰撞概率: 生日攻击阈值约 2^16=65536 个 key 才达 50% 碰撞 —— 远大于 free pool 实际用量 (<100), 可忽略。

**特别说明**: 此 hash 仅用于 channel name 和 deployment ID 的区分标识, 不用于认证/鉴权, 即使 32 bits 空间可暴力枚举, 攻击者也只获得 hash->key 映射表, 不会直接泄露原始 key 值。

**Verdict: PASS**

### 1.2 日志/输出中是否泄漏完整 API key

遍历 free_pool.go 中所有 `logger.*` 调用:

| 行号 | 日志内容 | 包含 key? |
|------|----------|-----------|
| 154 | `provider %q, skipping` | 否 |
| 251 | `failed to update channel %s` | 仅 channel name |
| 255 | `auto channel %s (id=%d) key updated` | 仅 channel name, 提到 key updated 但未打印 key 值 |
| 270 | `failed to insert channel %s` | 仅 channel name |
| 293 | `disabled removed auto channel %s (id=%d)` | 仅 channel name/ID |
| 305 | `skipping deployment %s with no channel` | 仅 deployment ID |
| 321 | `removed stale auto deployment %s` | 仅 deployment ID |
| 382 | `switched to channel id=%d name=%s model=%s` | 仅 channel name |

同样在 DB 写入时 (L179) `Key` 字段被存到 `model.Channel.Key`, 这是正常的凭据持久化, 属于预期行为。DB 中 key 的存储安全属于 one-api 整体 DB 安全范畴, 不在本次审查范围内。

**Verdict: PASS** — 无任何地方 log 或 expose 完整 API key。

### 1.3 示例文件中是否存在真实 key

`data/fallback.json.example`:
- L138: `"keys": ["sk-or-v1-your-key-here"]` — 明显占位符
- L149: `"keys": ["gsk_your_groq_key"]` — 明显占位符
- 所有 channel_id 均为 0

**Verdict: PASS**

---

## 2. SQL 注入 / ORM 安全性

### 2.1 所有 DB 查询使用 parameterized query

遍历 free_pool.go 中的 DB 操作:

| 行号 | 操作 | 方式 | 安全? |
|------|------|------|-------|
| 132 | SELECT name LIKE | `model.DB.Where("name LIKE ?", autoChannelPrefix+"%")` | **PASS** — GORM `?` 参数化 |
| 246 | UPDATE key/models/type | `model.DB.Model(existingCh).Updates(map[string]interface{}{...})` | **PASS** — map-based safe update |
| 259 | UPDATE status | `model.UpdateChannelStatusById(existingCh.Id, ...)` | **PASS** — 底层也是参数化 |
| 269 | INSERT channel | `d.ch.Insert()` | **PASS** — GORM Insert |
| 292 | UPDATE status | `model.UpdateChannelStatusById(existingCh.Id, ...)` | **PASS** |
| 446 | SELECT name LIKE | `model.DB.Where("name LIKE ?", autoChannelPrefix+"%")` | **PASS** — 同 L132 |

### 2.2 LIKE 查询中 autoChannelPrefix 的 SQL 通配符风险

`autoChannelPrefix = "[CCT Auto] "` 包含 `[` 和 `]` 字符。在不同数据库中的行为:

- **MySQL / SQLite**: `[` 和 `]` 在 LIKE 中是**普通字符**, 无特殊含义。`%` 是通配符, 但这是预期行为 (匹配 `[CCT Auto]` 开头的所有 channel)。
- **SQL Server**: `[` 会开始一个字符类通配符 (如 `[ABC]%`)。但 one-api 的数据库后端是 SQLite/MySQL, 不存在此问题。

**Verdict: PASS** — 无 SQL 注入向量。

---

## 3. 配置注入

### 3.1 limits_override *int 指针 nil dereference

`ApplyLimitsOverride` (free_pool.go L347-364):
- 第一行检查 `override == nil` 并 early return
- 每个字段前 `!= nil` 检查

`ValidateFreeProviderLimits` (free_pool.go L367-384):
- 第一行检查 `limits == nil` 并 return nil
- 每个字段前 `!= nil` 检查

**Verdict: PASS** — 无 nil dereference 风险。

### 3.2 ValidateFreeProviderLimits 覆盖率

函数检查 4 个字段:
- `RPMLimit` — 检查 `< 0`
- `RPDLimit` — 检查 `< 0`
- `TPMLimit` — 检查 `< 0`
- `TPDLimit` — 检查 `< 0`

对应 `FreeProviderLimits` 结构体所有 4 个字段 (config.go L14-19), **全覆盖**。

测试文件 (free_pool_test.go L367-406) 覆盖: nil / 合法值 / 各字段负值。

**Verdict: PASS**

### 3.3 验证-执行顺序问题 (WARN)

`ReloadConfig` (config.go L486-496) 中, `SyncFreePool` 在 `validateConfigData` 之前执行:

```go
// Step 3: Sync free pool BEFORE validation
if err := SyncFreePool(newCfg); err != nil { ... }

// Step 4: Validate the new config before swapping
if err := validateConfigData(newCfg); err != nil { ... }
```

这意味着如果配置包含未验证的 limits_override (例如负值), `SyncFreePool` 会用它们生成 deployment 配置。如果验证随后失败, `newCfg` 被丢弃, 全局 config 不变。**然而 DB 中的 channel 操作 (创建/更新) 已经执行**。

实际影响:
- Channel 表中不存储 limit 值, 只有 name/type/key/models/status
- 错误限制仅存在于被丢弃的 `newCfg` 的 deployment 配置中, 不持久化
- 配置源是本地文件 (操作者控制), 攻击面有限

**Verdict: WARN (L1)** — 顺序不合理但实际影响极低。建议将 `validateConfigData` 移到 `SyncFreePool` 之前, 确保任何 DB 操作前配置已通过语义验证。

---

## 4. FROZEN_SCOPE 合规

| 底线 | 状态 |
|------|------|
| `data/fallback.json` (生产配置) | 未触达 — 所有操作在内存 `Config` 结构体和 DB 上 |
| `one-api.exe` / `one-api.db` | 未触达 — free_pool.go 仅通过 GORM 查询/写入 Channel 表 |
| 旧 disabled channel 删除 | **未执行删除** — 使用 `ChannelStatusManuallyDisabled` (值=2) 标记禁用, 非硬删除 |
| Groq 启用操作 | 不受影响 — free_pool 仅按 `cfg.FreeProviders` 配置操作, 不独立启用 Groq |

**Verdict: PASS** — 全部范围内, 无违规。

---

## 5. context.WithValue 传播 (relay.go L385-392)

```go
newCtx := context.WithValue(ctx, ctxkey.FallbackVirtualModel, virtualModel)
newCtx = context.WithValue(newCtx, ctxkey.FallbackDeploymentID, dep.ID)
newCtx = context.WithValue(newCtx, ctxkey.FallbackRealModel, dep.RealModel)
newCtx = context.WithValue(newCtx, ctxkey.FallbackChannelID, dep.ChannelID)
newCtx = context.WithValue(newCtx, ctxkey.FallbackDeploymentIndex, i)
newCtx = context.WithValue(newCtx, ctxkey.FallbackAttemptCount, attempts)
c.Request = c.Request.WithContext(newCtx)
```

**发现**:
- 使用标准 `context.WithValue` 链, 不可变 context 模式正确
- Context key 使用 `common/ctxkey` 包中的 string 常量 (非 typed key)。string key 在不同包间可能碰撞, 但 ctxkey 包名本身已提供命名空间隔离, 且 key 值前缀统一 (`fallback_`), 碰撞概率极低
- 传递的值: 字符串 (model/ID) 和整数 (channel ID/index/attempt), 均为请求路由元数据, 不含敏感信息
- 链接到 `c.Request.Context()` — 标准 HTTP 请求作用域模式, 下游 `postConsumeQuota` 等通过 `ctx.Value()` 读取

**Verdict: PASS**

---

## 6. 其他发现

### 6.1 SafeKeyHash 与 key trim 不一致 (WARN, L1)

在 SyncFreePool (free_pool.go) 循环中:

```go
// L171 — hash 使用原始 key
keyHash := SafeKeyHash(key)
// L168 — trim 检查
if strings.TrimSpace(key) == "" { continue }
// L179 — 存储时 trim
Key: strings.TrimSpace(key),
```

如果配置中 key 包含前导/尾随空格 (如 `"  sk-or-v1-xxx  "`):
- `SafeKeyHash("  sk-or-v1-xxx  ")` != `SafeKeyHash("sk-or-v1-xxx")`
- Channel name 使用带空格的 hash, 但存储的 key 是 trim 后的
- 重启重载配置后, hash 重新计算 (再次使用原始带空格 key), 行为一致

实际风险: 极低。仅当 JSON 配置手动编辑产生意外空格时才可能触发, 且只影响 channel name 的可读性, 不影响功能。

**建议**: 将 L171 改为 `keyHash := SafeKeyHash(strings.TrimSpace(key))`, 使 hash 基于 trim 后的 key 计算。

### 6.2 LIKE 注释误导 (INFO)

free_pool.go L130 注释:
```go
// Note: escape [ and ] in LIKE — SQL treats [] as character class wildcard
```

这条注释不完全准确: `[]` 字符类通配符是 SQL Server 特性, MySQL/SQLite 的 LIKE 不支持此语法。参数化查询也让此问题无关 (即使 `[` 被解释, 也已在参数值中而非 SQL 字面量中)。

**建议**: 更新注释说明参数化查询已使其安全。

---

## 审查摘要

| 维度 | Verdict | 严重等级 |
|------|---------|----------|
| 1.1 SafeKeyHash 可逆性 | PASS | — |
| 1.2 API key 日志泄漏 | PASS | — |
| 1.3 示例文件真实 key | PASS | — |
| 2.1 SQL 参数化查询 | PASS | — |
| 2.2 LIKE 通配符风险 | PASS | — |
| 3.1 nil dereference | PASS | — |
| 3.2 Validate 全覆盖 | PASS | — |
| 3.3 验证-执行顺序 | **WARN** | L1 |
| 4. FROZEN_SCOPE | PASS | — |
| 5. context.WithValue | PASS | — |
| 6.1 SafeKeyHash/trim 不一致 | **WARN** | L1 |
| 6.2 注释误导 | INFO | — |

**总体结论**: **PASS 带 2 个 L1 建议项**。代码在 API key 安全性、SQL 注入防护、nil dereference 防护方面做得充分。两个 L1 建议不影响实际运行安全, 但值得修复以提升代码健壮性和防御深度。
