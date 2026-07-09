# OpenRouter/auto 稳定性验收（最终提交版）

该文档用于部署后在新机器执行一次标准化验收并直接提交工单/PR 备注。

## 1. 执行命令（可重跑）

```powershell
$env:CCT_API_BASE_URL = "http://localhost:3008"
$env:CCT_API_TOKEN = "replace-with-user-token"
$env:CCT_ADMIN_TOKEN = "replace-with-admin-token"

powershell -ExecutionPolicy Bypass -File scripts/fallback-openrouter-auto-smoke.ps1 -OutputJson
```

## 2. 执行环境

- 服务地址：`http://localhost:3008`
- 检查模型：`openrouter/auto`
- 参考验收清单：
  - [`docs/openrouter-auto-acceptance-checklist.md`](./openrouter-auto-acceptance-checklist.md)
  - [`docs/openrouter-auto-stability-submission.md`](./openrouter-auto-stability-submission.md)

## 3. 需满足的通过标准（全部必过）

- [ ] `free_provider_catalog` 包含 `openrouter` 且 `enabled=true`
- [ ] `POST /api/fallback/config/reload` 成功
- [ ] `POST /api/fallback/free-pool/sync` 成功
- [ ] `GET /api/fallback/deployments/runtime-status` 包含 `free:openrouter-*` 运行时
- [ ] 非流 `POST /v1/chat/completions`（`model=openrouter/auto`）返回 `choices`
- [ ] 流式请求返回 SSE（`data:`）
- [ ] `usage` 查询返回成功并可见计数更新
- [ ] `fallback_requests_total` 有正向增量
- [ ] `GET /fallback/free-pool` 可见 openrouter/auto 路由入口（或页面复核通过）

## 4. JSON 结果快填（脚本 `-OutputJson`）

```text
{
  "pass": true/false,
  "baseUrl": "http://localhost:3008",
  "model": "openrouter/auto",
  "deploymentId": "...",
  "usageRowsBefore": 0,
  "usageRowsAfter": 0,
  "runtimeRows": 0,
  "usageRequestCount": 0,
  "usageSuccessCount": 0,
  "fallbackRequestsDelta": 0,
  "pageContainsOpenRouterAuto": true/false
}
```

## 5. 提交文本（直接粘贴）

- 执行时间：
- 执行机器：
- 操作人：
- 执行结果：`PASS` / `FAIL`
- 非流流量：`pass/fail`
- 流式流量：`pass/fail`
- 关键指标：
  - deploymentId:
  - runtimeRows:
  - usageRowsBefore:
  - usageRowsAfter:
  - usageRequestCount:
  - usageSuccessCount:
  - fallbackRequestsDelta:
  - pageContainsOpenRouterAuto:
- 备注：

## 6. 失败时快速排查（建议）

- 检查 token 与对应用户/管理员权限
- 检查数据库 `fallback_*` 渠道/provider 配置是否恢复到位
- 检查 `CCT_API_TOKEN` 与 `CCT_ADMIN_TOKEN` 使用机器一致性
- 重启服务后再执行一次并记录对比（建议 `idempotence`）
