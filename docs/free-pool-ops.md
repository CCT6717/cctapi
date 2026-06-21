# Free Pool 运维手册

**适用范围**：cctapi 项目，从零配置到日常运维的全流程操作
**文件状态**：稳定版，对应代码 v0.2.x
**最后更新**：2026-06-21

---

## 1. 生产上线步骤

### 前置条件

- [ ] One API 服务已运行，DB 已初始化
- [ ] 已在 `free_providers` 中填写了有效 API key
- [ ] `data/fallback.json` 的 `enabled: true`

### 上线流程

1. **编辑配置文件**
   ```bash
   vim data/fallback.json
   ```
   确保 `free_providers` 段已正确填写（参考 `free-pool.md` 的配置结构）。

2. **检查验证**
   ```json
   // POST /api/fallback/config/reload
   // 响应 200: { "message": "configuration reloaded successfully" }
   ```

   若失败，检查日志中 `[free_pool]` 和 `[config]` 相关错误。

3. **验证 channel 自动创建**
   ```bash
   # 在 One API 后台检查 channel 列表
   # 应出现 [CCT Auto] openrouter-xxxxxxxx 等 auto channel
   # 或通过 API 查询：
   GET /api/channel/?page=1&page_size=100
   ```

4. **验证 deployment 自动创建**
   ```bash
   GET /api/fallback/deployments/runtime-status
   ```
   返回的数据应包含 `free:openrouter-xxxxxxxx` 等 deployment，`enabled: true`。

5. **发送测试请求**
   ```bash
   curl https://your-api/v1/chat/completions \
     -H "Authorization: Bearer sk-your-token" \
     -d '{"model": "cct/free", "messages": [{"role": "user", "content": "hello"}]}'
   ```

   预期返回正常响应。日志中应有类似内容：
   ```
   [fallback] virtual model cct/free matched deployment free:openrouter-xxxxxxxx channel 42 real model openrouter/free
   [fallback] attempt 1/... virtual model cct/free deployment free:openrouter-xxxxxxxx
   [fallback] deployment free:openrouter-xxxxxxxx succeeded in 1234ms
   ```

---

## 2. 回滚步骤

回滚分三级：**Binary → DB → Config**，按需执行。

### 2.1 Binary 回滚

回退到旧版可执行文件：

```bash
# 停止服务
systemctl stop cctapi
# 恢复旧版本
cp /backup/cctapi-YYYYMMDD /usr/local/bin/cctapi
# 重启
systemctl start cctapi
```

### 2.2 DB 回滚（channel）

如果 SyncFreePool 创建了错误的 channel 需要清除：

```bash
# 方式一：逐个 disable（推荐，保留审计轨迹）
# 在 One API 后台手动将 auto channel 设为 disabled

# 方式二：批量通过 API
# 先用 dry-run 查看哪些是 stale
POST /api/fallback/free-pool/cleanup/dry-run

# 然后手动在 One API 后台操作
```

**不删除 channel 的理由**：One API 的 quota 日志和 usage 数据可能引用 channel ID，硬删除会导致数据完整性断裂。一律 disable 保留。

### 2.3 Config 回滚

恢复旧版 `fallback.json`：

```bash
cp /backup/fallback.json.YYYYMMDD data/fallback.json
POST /api/fallback/config/reload
```

验证恢复：
```bash
GET /api/fallback/deployments/runtime-status
```

---

## 3. 热加载 Config Reload

修改 `data/fallback.json` 后无需重启服务：

```bash
POST /api/fallback/config/reload
# 可选参数 path= 指定其他路径
```

reload 过程（`ReloadConfig` 实现）：

1. 读取文件 → 解析 JSON → 标准化
2. **SyncFreePool** — 同步 auto channel/deployment
3. **validateConfigData** — 校验语义正确性
4. **原子替换** — 写锁切换 config 指针

**如果 reload 失败（如 JSON 格式错误或校验不通过）**，旧 config 保持不变，请求继续正常服务。无需担心 reload 导致服务中断。

**常见失败原因**：
- JSON 语法错误（缺少逗号、括号不匹配）
- `limits_override` 含负数
- Free provider name 不属于 BuiltinFreeProviders
- Enable 的 deployment 缺少有效 channel_id
- Virtual model 的 pool 中没有 enabled deployment

---

## 4. Stale Cleanup Dry-Run

`POST /api/fallback/free-pool/cleanup/dry-run` 用于查看哪些 auto 资源已不再被配置引用，但不执行任何修改。

响应示例：

```json
{
  "success": true,
  "data": {
    "will_delete": false,
    "stale_channels": [
      {"name": "[CCT Auto] openrouter-abc12345", "id": 42, "reason": "not found in current free_providers config"}
    ],
    "stale_deployments": [
      {"name": "free:openrouter-abc12345", "id": 42, "reason": "not found in current free_providers config"}
    ]
  }
}
```

**用途**：
- 迁移新旧命名格式后检查残留
- 删除 provider 前确认哪些 channel 会被 disable
- CI/CD 变更前做预检查

**为什么叫 dry-run**：此端点永远不做写操作，即使出错也能返回部分结果：

```json
{
  "success": false,
  "message": "database not initialized",
  "data": { "stale_channels": [], "stale_deployments": [], "will_delete": false }
}
```

---

## 5. 旧 Channel 保留策略

SyncFreePool 对已不再需要的 auto channel **只 disable，不 delete**：

```go
model.UpdateChannelStatusById(existingCh.Id, model.ChannelStatusManuallyDisabled)
logger.SysLog(fmt.Sprintf("[free_pool] disabled removed auto channel %s (id=%d)", ...))
```

理由：

| 原因 | 说明 |
|------|------|
| 审计完整性 | Quota log 引用 channel ID，删除后历史数据不完整 |
| 回滚安全 | Disable 的 channel 可一键 re-enable |
| 追溯价值 | 运维排查时可查看到"这个 channel 曾经存在过" |

**清理已 disable 的 channel**：确认不再需要后，在 One API 后台手动删除。

---

## 6. 观察日志关键信息

### Free Pool 日志前缀

| 日志前缀 | 所在文件 | 典型场景 |
|----------|---------|----------|
| `[free_pool]` | `free_pool.go` | channel 创建/更新/disable |
| `[config]` | `config.go` | config reload 成功/失败 |
| `[fallback]` | `relay.go` | 请求路由、部署切换、成功/失败 |
| `[health]` | `health.go` | 健康检查（free 跳过 ping） |

### 关键日志片段解读

**SyncFreePool 执行成功**：
```
[free_pool] auto channel [CCT Auto] openrouter-a1b2c3d4 (id=42) key updated
[free_pool] disabled removed auto channel [CCT Auto] openrouter-old (id=41)
[free_pool] removed stale auto deployment free:openrouter-old (channel no longer active)
```

**成功路由到 free deployment**：
```
[fallback] strategy-based start deployment for cct/free: free:openrouter-a1b2c3d4
[fallback] attempt 1/2 virtual model cct/free deployment free:openrouter-a1b2c3d4 channel 42 real model openrouter/free
[fallback] deployment free:openrouter-a1b2c3d4 succeeded in 2345ms
```

**Sticky routing 生效**：
```
[fallback] sticky routing: virtual model cct/free pinned to deployment free:openrouter-a1b2c3d4
[fallback] sticky active for cct/free -> free:openrouter-a1b2c3d4
```

**所有部署失败**：
```
[fallback] deployment free:openrouter-a1b2c3d4 failed (attempt 1/2, 1000ms): ...
[fallback] deployment free:groq-b2c3d4e5 failed (attempt 2/2, 500ms): ...
[fallback] all 2 deployments failed for virtual model cct/free
```

---

## 7. 查看 Runtime-Status 确认 Migration

`GET /api/fallback/deployments/runtime-status` 返回所有 deployment 的当前状态。关键字段：

| 字段 | 说明 |
|------|------|
| `deployment_id` | 如 `free:openrouter-a1b2c3d4` |
| `enabled` | 是否启用 |
| `pool` | 应为 `free` |
| `health` | free deployment 通常为 `unknown` |
| `minute_requests` | 当前分钟请求数 |
| `day_requests` | 当日请求数 |
| `rate_limit_score` | 速率限制分数 |
| `last_error` | 最近一次错误信息 |

**Migration 确认**：新旧命名迁移后，检查 `deployment_id` 是否全部为新格式（`free:openrouter-{hash}`），旧格式（`free:openrouter-{数字}`）是否仍然存在。旧格式不会被自动删除（向后兼容），需要手动确认是否清理。

---

## 8. 辅助运维 API

| 端点 | 方法 | 用途 |
|------|------|------|
| `/api/fallback/free-pool/sync` | POST | 手动触发 SyncFreePool |
| `/api/fallback/config/reload` | POST | 热加载配置 |
| `/api/fallback/deployments/runtime-status` | GET | 查看所有 deployment 状态 |
| `/api/fallback/deployments/:id/health-check` | POST | 手动触发单个 deployment 健康检查 |
| `/api/fallback/states` | GET | 查看所有 virtual model 的 deployment 状态 |
| `/api/fallback/summary` | GET | 近 1 小时部署切换统计 |
| `/api/fallback/logs` | GET | 查看近期的 switch events |
| `/api/fallback/virtual-models` | GET | 查看 virtual model 配置 |
| `/api/fallback/free-pool/cleanup/dry-run` | POST | Dry-run 检查 stale 资源 |

所有端点需要 `admin` 权限（`middleware.AdminAuth()`）。
