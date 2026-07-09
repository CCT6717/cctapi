# OpenRouter/auto 可复跑验收一体化

## 目标
- 验证数据库/配置恢复后的 `openrouter/auto` 可用性（含 free-pool 同步和实打流量）。
- 用于环境重建、发布前后和回归验证。

## 一键复跑命令

```powershell
$env:CCT_API_BASE_URL = "http://127.0.0.1:3008"
$env:CCT_API_TOKEN = "replace-with-user-token"
$env:CCT_ADMIN_TOKEN = "replace-with-admin-token"

powershell -ExecutionPolicy Bypass -File scripts/fallback-openrouter-auto-smoke.ps1 -OutputJson
```

## 复跑前置

- 服务可达：`curl.exe -I http://127.0.0.1:3008/`
- 服务页面可达：`curl.exe -I http://127.0.0.1:3008/fallback/free-pool`
- 用户令牌与管理员令牌有效
- `free_provider` 中已配置可用的 `openrouter`

## 强制验收项

- `free_provider_catalog` 中有 `openrouter` 且为 `enabled`
- `POST /api/fallback/config/reload` 成功
- `POST /api/fallback/free-pool/sync` 成功
- `GET /api/fallback/deployments/runtime-status` 存在 `free:openrouter-` 前缀运行时记录（至少 1 条）
- `POST /v1/chat/completions`（`model=openrouter/auto`）非流式返回 200 且有 `choices`
- `POST /v1/chat/completions`（`model=openrouter/auto`）流式返回 `data:` SSE
- `/api/fallback/free-pool/usage?provider=openrouter` 查询成功且 provider 行存在
- `/metrics` 中 `fallback_requests_total` 有正向增长
- `/fallback/free-pool` 页面可读并出现 OpenRouter / auto 标记

上述均通过时，脚本返回码应为 `0`，`-OutputJson` 输出包含 `pass=true`。

## `-OutputJson` 关键字段

- `pass`
- `baseUrl`
- `model`
- `deploymentId`
- `usageRowsBefore`
- `usageRowsAfter`
- `runtimeRows`
- `usageRequestCount`
- `usageSuccessCount`
- `fallbackRequestsDelta`
- `pageContainsOpenRouterAuto`

## 失败排查

- 若运行时有记录但请求失败：优先检查 `fallback_*` 表在目标机器上的恢复一致性
- 若流量测试失败：先确认 `CCT_API_TOKEN`/`CCT_ADMIN_TOKEN` 与 openrouter key 生效
- 关键事件后建议再次执行一次 `POST /api/fallback/free-pool/sync` 并重跑

## 最终提交块（可直接粘贴）

```text
Execution time: <YYYY-MM-DD HH:mm:ss>
Host: <machine-name>
Operator: <operator>
Branch/commit: <branch> / <commit>
Result: PASS / FAIL
Non-stream: pass / fail
Stream: pass / fail
pass: true / false
deploymentId: <value>
runtimeRows: <number>
usageRowsBefore: <number>
usageRowsAfter: <number>
usageRequestCount: <number>
usageSuccessCount: <number>
fallbackRequestsDelta: <number>
pageContainsOpenRouterAuto: true / false
Remarks: <short notes>
```

附证据建议：
- 原始 `-OutputJson` 输出
- `curl.exe -I http://127.0.0.1:3008/`
- `curl.exe -I http://127.0.0.1:3008/fallback/free-pool`

