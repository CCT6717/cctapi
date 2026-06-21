# Free Pool 配置指南

**适用范围**：cctapi 项目 `fallback/free_pool.go` 及关联模块
**文件状态**：稳定版，对应代码 v0.2.x
**最后更新**：2026-06-21

---

## 概述

Free Pool 是 cct/free 三层网关的底层实现，自动管理上游免费提供商的 channel（渠道）和 deployment（部署点），无需手动创建。

工作流程：

```
cct/free 请求
  → fallback router: IsVirtualModel("cct/free") == true
  → relayWithFallback()
  → GetDeploymentsForVirtualModel("cct/free")
      → pools: ["free"]  → 筛选 pool="free" 的 deployment
      → SyncFreePool 已自动创建这些 deployment
  → 按 strategy + capability + health + quota 排序筛选
  → 依次尝试 deployment，成功则 sticky pin
```

---

## 配置结构

### `free_providers`

在 `data/fallback.json` 的顶层字段：

```json
{
  "enabled": true,
  "free_providers": {
    "openrouter": {
      "enabled": true,
      "keys": ["sk-or-v1-REPLACE_ME"],
      "limits_override": {
        "rpm_limit": 10,
        "rpd_limit": 500
      }
    },
    "groq": {
      "enabled": true,
      "keys": ["gsk_REPLACE_GROQ_KEY"],
      "models": ["mixtral-8x7b-32768", "llama-3.3-70b-versatile"],
      "default_rpm": 30,
      "default_tpm": 6000,
      "limits_override": {
        "rpm_limit": 20
      }
    }
  }
}
```

每个 provider 可配字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `enabled` | bool | 是否启用该 provider |
| `keys` | string[] | API key 列表（支持多 key 做多个 channel/deployment） |
| `models` | string[] | 覆盖默认模型列表，留空用 builtin 默认值。第一个模型为 deployment 的 real_model |
| `default_rpm` | int | 覆盖内置默认 RPM，为 0 时使用内置值 |
| `default_rpd` | int | 同上 |
| `default_tpm` | int | 同上 |
| `default_tpd` | int | 同上 |
| `limits_override` | object | 对最终限额再做一次覆盖（见下节） |

### `limits_override` 的 nil vs 0 语义

`limits_override` 的四个字段都是 `*int`（指针类型），这是理解限额合并的关键：

- **字段不存在或为 null** → 不覆盖，沿用合并后的默认值
- **字段值为 0** → 明确设为无限制（unlimited）
- **字段值为正数** → 覆盖为该数值
- **字段值为负数** → 被 `ValidateFreeProviderLimits` 拒绝，config reload 失败

合并优先级（从低到高）：

```
内置默认 ← default_* 覆盖 ← limits_override 覆盖
```

示例：OpenRouter 内置默认 `rpm=20`，如果配置 `default_rpm=10` 且 `limits_override.rpm_limit=0`，最终 RPM = 0（无限制）。

### 如何理解"限额为 0 表示无限制"

在 `PassQuotaCheck` 中，`dep.RPMLimit == 0` 时跳过 RPM 检查。因此 0 不是"零配额"而是"不限制"。想要实际限制，必须设正数。

---

## Provider Registry

内置 provider 定义在 `BuiltinFreeProviders`（`free_pool.go`）：

| Provider | ChannelType | BaseURL | 默认模型 | Context |
|----------|-------------|---------|----------|---------|
| `openrouter` | OpenRouter (24) | `https://openrouter.ai/api` | `["openrouter/free"]` | 128K |
| `groq` | Groq | `https://api.groq.com/openai` | `["mixtral-8x7b-32768", "llama-3.3-70b-versatile", "llama-3.1-8b-instant"]` | 32K |

Groq 默认路由到 models[0]（mixtral）。若需切换模型，在配置中显式写 `models`。

---

## Key Hash 命名原理

`SafeKeyHash(key)` 用 SHA256 取前 4 字节（8 个 hex 字符）作为 key 的摘要标识：

```go
func SafeKeyHash(key string) string {
    h := sha256.Sum256([]byte(key))
    return hex.EncodeToString(h[:4])
}
```

用于自动生成的 channel name 和 deployment ID：

```
channel name:  [CCT Auto] openrouter-a1b2c3d4
deployment ID: free:openrouter-a1b2c3d4
```

**旧格式兼容**：在 hash 方案之前，命名使用整数索引（`openrouter-0`, `openrouter-1`）。`isAutoDeploymentSuffix` 同时识别两种格式，迁移过程中旧 deployment 不会被误删。

---

## cct/free 路由链路

完整请求链路：

```
客户端请求 model="cct/free"
  → middleware/distributor.go:
      IsVirtualModel("cct/free") → true，进入 fallback 模式
  → controller/relay.go: relayWithFallback()
  → fallback.GetDeploymentsForVirtualModel("cct/free")
      → pools: ["free"] → 筛选所有池 deployment
  → 按策略排序 + 能力过滤 + 健康过滤 + 限额预检 + 并发槽
  → 选择第一个可用的 deployment
  → 改写 model 为 dep.RealModel（如 "openrouter/free"）
  → 转发到对应 channel（type=24 OpenRouter）
  → 成功后记录 sticky deployment
```

关键日志标记：

| 日志关键词 | 含义 |
|-----------|------|
| `[fallback] virtual model cct/free matched deployment` | Distributor 阶段选中 |
| `[fallback] sticky routing:` | 使用 sticky pin |
| `[fallback] strategy-based start deployment` | 首次启动的 deployment |
| `[free_pool]` | SyncFreePool 操作 |
| `[health][debug] skip ping for free deployment` | 健康检查跳过 free pool |

---

## SyncFreePool 自动同步

`SyncFreePool` 在以下时机执行：

1. **config reload**（`ReloadConfig`）：在 `validateConfigData` 之前调用
2. **手动触发**：`POST /api/fallback/free-pool/sync`

同步逻辑（7 步）：

1. 扫描 DB 中所有 `[CCT Auto]%` 前缀的 channel
2. 计算期望的 channel/deployment（根据 `free_providers` 配置）
3. 创建/更新 channel（key 变动时自动更新，以配置为准）
4. 将已删除的 auto channel **disable**（不删除，保留为 ManuallyDisabled）
5. 将 auto deployment 写入 `cfg.Deployments`
6. 移除 config 中已 stale 的 auto deployment
7. 保留用户手动创建的 `free:*` deployment（通过 `IsAutoDeploymentID` 鉴别）

**重要**：SyncFreePool 不会删除 channel，只会 disable。旧 channel 在 One API 后台仍可见，状态为 `manually_disabled`。

---

## Free Deployment 健康检查策略

`checkOneDeployment` 中有一段特殊逻辑：

```go
if dep.QuotaMode == "free" {
    logger.SysLogf("[health][debug] skip ping for free deployment %s", deploymentID)
    setHealthStatus(deploymentID, HealthUnknown)
    return
}
```

Free pool 的 deployment 的 `QuotaMode` 固定为 `"free"`。健康检查会跳过真实 ping，直接标记为 `unknown`。因为：

- Free 上游通常有极低的 RPM/RPD，每次 ping 消耗真实配额
- `IsDeploymentHealthy` 允许 `unknown` 状态通过（只拦截 `invalid` / `error`）
- 部署的实际可用性由 fallback 循环中的错误处理保障（失败 → cooldown → 移除）

---

## Channel/Deployment 命名约定

### Channel Name

```
[CCT Auto] {provider}-{key_hash}
```

示例：`[CCT Auto] openrouter-a1b2c3d4`

### Deployment ID

```
free:{provider}-{key_hash}
```

示例：`free:openrouter-a1b2c3d4`

### 旧格式向后兼容

迁移前的命名：

```
channel name:  [CCT Auto] openrouter-0
deployment ID: free:openrouter-0
```

`isAutoDeploymentSuffix` 同时识别整数索引和 8 字符 hex，因此：

- 旧 deployment 不会被识别为 stale 而被删除
- 新 key 用 hash 命名，新旧共存没有问题
- 迁移完成后可手动清理旧 channel（在 One API 后台操作）

---

## usage model_name 修复说明

请求 `cct/free` 时，usage log 中的 `model_name` 和 `real_model_name` 按以下规则记录（见 `relay/controller/helper.go`）：

```go
// model_name: 优先写入虚拟模型名（cct/free）
logModelName := textRequest.Model
if vm := ctx.Value(ctxkey.FallbackVirtualModel); vm != nil {
    logModelName = vmStr
}
// real_model_name: 写入上游真实模型名（如 openrouter/free）
realModelName := textRequest.Model
if rm := ctx.Value(ctxkey.FallbackRealModel); rm != nil {
    realModelName = rmStr
}
```

即：

| 字段 | cct/free 请求 | cct/high 请求 |
|------|-------------|--------------|
| `model_name` | `cct/free` | `cct/high` |
| `real_model_name` | `openrouter/free`（上游实际模型） | `claude-sonnet-4-20250514`（上游实际模型） |

注意：只有走 `relayWithFallback` 路径的请求才有此区分。不走 fallback 的传统请求 `model_name` 和 `real_model_name` 一致。

---

## 完整配置示例

参见 `data/fallback.json.example`。核心要点：

- `deployments` 中 `pool: "free"` 的 deployment 可以是手动创建的（如 `cct/gemini`），也可以是由 `SyncFreePool` 自动生成的
- 手动创建 + 自动生成的 deployment 在同一个 pool 中混合工作，优先级由 `priority` 决定
- `cct/free` 的 virtual model 配置为 `pools: ["free"]`，`strategy: "free_first"`（对 free pool 来说实际等价于 priority 排序）

---

## FAQ

**Q: 添加新 key 后需要重启吗？**
A: 不需要。编辑 `fallback.json` → `POST /api/fallback/config/reload` 即可。SyncFreePool 在 reload 过程中自动执行。

**Q: 一个 provider 可以配多个 key 吗？**
A: 可以。每个 key 生成独立的 channel 和 deployment，轮流使用。这在 OpenRouter 等限流较严的上游特别有用。

**Q: 为什么 health check 显示 free deployment 为 unknown？**
A: 这是设计如此。Free deployment 有独立的不健康检测机制（错误 → cooldown → 不参与排序），不需要消耗额外配额去 ping。
