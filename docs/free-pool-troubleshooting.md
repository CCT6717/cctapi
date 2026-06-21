# Free Pool 故障排查

**适用范围**：cct/free 路由相关的常见问题诊断
**文件状态**：稳定版，对应代码 v0.2.x
**最后更新**：2026-06-21

---

## FAQ

### 1. cct/free 返回 "no available deployment"

**现象**：
```json
{
  "error": {
    "message": "No available deployments for virtual model cct/free: no enabled deployments found for virtual model: cct/free",
    "code": "no_deployments"
  }
}
```

**可能原因及排查步骤**：

**a) free_providers 未配置或未启用**
```
POST /api/fallback/config/reload
```
检查日志是否有 `[free_pool]` 相关信息。如果完全没有，表示 `free_providers` 段被跳过。

验证方式：
```bash
GET /api/fallback/virtual-models
```
查看 `cct/free` 的 `pools` 是 `["free"]`。

**b) SyncFreePool 未生成任何 channel**
```bash
GET /api/fallback/free-pool/sync
# 检查日志：
# - 如果 log 中没有任何 [free_pool] created/updated 信息，说明 free_providers 配置未解析
# - 检查 keys 数组是否为空或 key 是否为空字符串
```

**c) Auto channel 未绑定到 deployment**
```bash
GET /api/fallback/deployments/runtime-status
```
检查 `free:*` deployment 的 `enabled` 字段。若 `channel_id == 0` 则 deployment 未绑定 channel——这通常意味着 SyncFreePool 创建 channel 失败。

**d) 所有自动 deployment 因 channel 不存在被跳过**
```
[free_pool] skipping deployment free:openrouter-xxxxxxxx with no channel
```
日志中出现这条意味着 `SyncFreePool` 创建了 channel 但 write to DB 失败。检查 DB 连接和 channel 表的写入权限。

**快速排查命令**：
```bash
# 检查 auto channel 是否存在
curl -s -H "Authorization: Bearer admin-token" https://your-api/api/channel/ | jq '.data[] | select(.name | startswith("[CCT Auto]"))'

# 检查 deployment 状态
curl -s -H "Authorization: Bearer admin-token" https://your-api/api/fallback/deployments/runtime-status | jq '.data[] | select(.pool == "free")'

# 手动触发同步
curl -s -X POST -H "Authorization: Bearer admin-token" https://your-api/api/fallback/free-pool/sync

# 手动触发健康检查
curl -s -X POST -H "Authorization: Bearer admin-token" https://your-api/api/fallback/deployments/free:openrouter-xxxxxxxx/health-check
```

---

### 2. OpenRouter 返回 429 Rate Limited

**现象**：日志中出现：
```
[fallback] deployment free:openrouter-xxxxxxxx failed (attempt 1/1, 200ms): 429 rate limited
```
然后请求 fallback 到下一个 deployment，或全部失败。

**原因分析**：
- OpenRouter Free 有严格的 RPM/RPD 限制（默认 20 RPM / 200 RPD）
- 多用户同时请求时容易触发
- Free deployment 在 `checkOneDeployment` 中被跳过 health ping，所以不会提前检测到限流

**解决方案**：

**a) 降低 limits_override**
```json
{
  "openrouter": {
    "enabled": true,
    "keys": ["sk-or-v1-REPLACE_ME"],
    "limits_override": {
      "rpm_limit": 10,
      "rpd_limit": 200
    }
  }
}
```
减小 RPM/RPD 后 reload，`PassQuotaCheck` 会在发送请求前拦截超出限额的请求。

**b) 添加多个 key**
```json
{
  "keys": ["sk-or-v1-KEY1", "sk-or-v1-KEY2", "sk-or-v1-KEY3"]
}
```
每个 key 生成独立 channel + deployment，fallback 循环会依次尝试。

**c) 查看当前限流状态**
```bash
GET /api/fallback/deployments/runtime-status
```
检查 `rate_limit_score` 和 `minute_requests` 字段，查看是否接近 `rpm_limit`。

**d) 手动恢复被封 deployment**
```bash
POST /api/fallback/deployments/free:openrouter-xxxxxxxx/clear-cooldown
POST /api/fallback/deployments/free:openrouter-xxxxxxxx/recover
```

---

### 3. Key Invalid / Expired

**现象**：
```
[fallback] deployment free:openrouter-xxxxxxxx failed (attempt 1/1, 500ms): 401 unauthorized
```
health 状态变为 `invalid`：
```
GET /api/fallback/deployments/runtime-status
→ "health": "invalid"
```

**处理步骤**：

1. 在 provider 网站检查 API key 状态（是否过期、超限）
2. 在 `data/fallback.json` 中更新 key
3. Reload 配置
4. SyncFreePool 会自动检测 key 变化并更新 channel：
   ```
   [free_pool] auto channel [CCT Auto] openrouter-xxxxxxxx (id=42) key updated
   ```
5. 手动清除 invalid 状态：
   ```bash
   POST /api/fallback/deployments/free:openrouter-xxxxxxxx/recover
   # 或强制触发健康检查
   POST /api/fallback/deployments/free:openrouter-xxxxxxxx/health-check
   ```

**注意**：health check 对 free deployment 仅返回 `unknown`（不实际 ping 上游）。invalid 状态通常是 fallback 循环中记录的。手动 recover 或等待 config reload 后自动恢复。

---

### 4. Runtime-Status Health=Unknown

**现象**：
```json
GET /api/fallback/deployments/runtime-status
→ "health": "unknown"
```

**这通常不是问题**。Free deployment 不会参与健康检查 ping（`checkOneDeployment` 中 `QuotaMode == "free"` 时跳过），初始状态为 `unknown`。

`unknown` 不等于不可用——`IsDeploymentHealthy` 的判定规则：

| 状态 | 是否允许路由 |
|------|------------|
| `healthy` | 是 |
| `unknown` | **是** |
| `rate_limited` | 是 |
| `invalid` | **否** |
| `error` | **否** |

只有 `invalid` 或 `error` 才禁止路由。`unknown` 和 `healthy` 等效。

如果希望看到 `healthy`，可以手动触发：

```bash
POST /api/fallback/deployments/free:openrouter-xxxxxxxx/health-check
# 返回 "health": "unknown"（因为代码强制跳过）
```

这是设计如此，无法通过手动触发改变。

---

### 5. Sticky Routing 绑定旧 Deployment（openrouter-0 → openrouter-hash 迁移）

**场景**：从旧命名方案（整数索引）迁移到新方案（key hash）后，旧 deployment 仍然停留在 sticky 缓存中。

**现象**：
```
[fallback] sticky routing: virtual model cct/free pinned to deployment free:openrouter-0
[fallback] sticky active for cct/free -> free:openrouter-0
```

但 `free:openrouter-0` 可能已被 SyncFreePool 移除（因为不再有对应 channel），导致请求始终失败。

**处理**：

1. **清除 sticky**（不影响未来的路由决策）：
   ```bash
   # sticky 是内存状态，restart 服务自动清空
   systemctl restart cctapi
   ```

2. **或者在迁移时先清除旧 key 再添加新 key**：
   ```
   # 编辑 fallback.json，先删旧 key，再添新 key
   # 这样旧 deployment 被 disable，新 deployment 创建
   # sticky 记录在旧 deployment 不可用时 fallback 到新的
   ```

3. **迁移过程中的过渡建议**：新旧 key 同时保留一段时间：
   ```json
   {
     "keys": [
       "sk-or-v1-OLD_KEY",   // 旧命名 openrouter-0
       "sk-or-v1-NEW_KEY"    // 新命名 openrouter-a1b2c3d4
     ]
   }
   ```
   两个 deployment 共存，旧命名不会被清理。确认稳定后再移除旧 key。

---

### 6. SyncFreePool 未执行或未生效

**排查**：

**a) Config reload 日志检查**
```
[config] configuration reloaded successfully from data/fallback.json
```
如果没有这条日志，说明 reload 失败。检查错误日志：
```
[config] failed to parse config file ...
[config] validation failed for ... keeping old config: ...
```

**b) 手动触发同步**
```bash
POST /api/fallback/free-pool/sync
# 检查响应状态和日志
```

**c) 检查 auto channel 是否已 disable**
如果 channel 被意外 disable（如 One API 后台手动操作），SyncFreePool 会尝试 re-enable：
```go
if existingCh.Status != model.ChannelStatusEnabled {
    model.UpdateChannelStatusById(existingCh.Id, model.ChannelStatusEnabled)
}
```

查看日志确认：
```
[free_pool] disabled removed auto channel [CCT Auto] openrouter-xxxxxxxx (id=42)
```

**d) 验证 deployment 是否已写入 config**
```bash
GET /api/fallback/sort/order/cct/free
```
返回的 `order` 数组包含所有可用的 free deployment。

---

### 7. Usage Log 中 model_name 为什么还是上游模型

**现象**：请求 `model=cct/free`，但在 usage log 中看到 `model_name="openrouter/free"` 而非 `cct/free`。

**已修复状态**：此问题已通过 `relay/controller/helper.go` 的 `postConsumeQuota` 函数修复。

修复后的记录规则：

| 字段 | 值 | 说明 |
|------|-----|------|
| `model_name` | `cct/free` | 虚拟模型名，用于用户侧统计分析 |
| `real_model_name` | `openrouter/free` | 上游实际模型名，用于排查上游问题 |

**如果你仍看到上游模型名**，可能是：

1. **请求不经过 fallback 路径**
   检查是否直接请求了真实模型名而非 `cct/free`。只有 `IsVirtualModel` 为 true 时走 `relayWithFallback`。

2. **使用旧版 binary**
   检查 binary 编译时间。`postConsumeQuota` 中的修复要求代码包含：
   ```go
   logModelName := textRequest.Model
   if vm := ctx.Value(ctxkey.FallbackVirtualModel); vm != nil { ... }
   ```
   这是 `controller/relay.go` 中 `relayWithFallback` 设置到 `context.Context` 的值。

3. **日志查询翻页时只看到旧日志**
   检查日志的时间范围。修复前的记录可能仍然存在。
