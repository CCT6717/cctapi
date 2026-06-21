# Fallback 真实场景测试清单

本文档用于实际验证 CCT API 的虚拟模型 fallback 行为。默认服务地址按本地开发环境写为 `http://localhost:3007`。

## 准备

需要准备：

- 一个可调用 `/v1/chat/completions` 的用户 API Token。
- 一个管理员 Token，用于调用 `/api/fallback/*` 管理接口。
- 一个虚拟模型名，例如 `high/auto`、`low/auto` 或 `all/auto`。
- 该虚拟模型下的部署 ID 列表，例如 `doubao-code,openrouter-new-free,openrouter-old`。

PowerShell 环境变量示例：

```powershell
$env:CCT_API_BASE_URL = "http://localhost:3007"
$env:CCT_API_TOKEN = "sk-user-token"
$env:CCT_ADMIN_TOKEN = "sk-admin-token"
$env:CCT_API_MODEL = "all/auto"
$env:CCT_PRIMARY_DEPLOYMENT = "doubao-code"
$env:CCT_FALLBACK_DEPLOYMENTS = "doubao-18,doubao-16,openrouter-new-free,openrouter-old,openrouter-new,all-auto-model-7"
```

## 自动化脚本

安全基础测试，不会改部署状态：

```powershell
cd D:\project\cctapi
powershell -ExecutionPolicy Bypass -File scripts/fallback-smoke.ps1
```

覆盖内容：

- 非流式请求。
- 流式请求。
- `/metrics` 是否有 fallback 指标。
- 基础指标增量展示。

故障场景测试，会临时冷却部署，并在结束时恢复：

```powershell
cd D:\project\cctapi
powershell -ExecutionPolicy Bypass -File scripts/fallback-smoke.ps1 -RunFaultScenarios
```

覆盖内容：

- 主部署被冷却后，请求应切到后备部署。
- 所有部署都被冷却后，请求应失败。
- 恢复所有部署后，请求应重新成功。

注意：`CCT_FALLBACK_DEPLOYMENTS` 必须包含当前虚拟模型除主部署外的所有部署，否则“全部失败”场景可能不会真的覆盖所有候选部署。

## 手工核对清单

### 1. 非流式请求

运行基础脚本后确认：

- 脚本输出 `Non-stream request passed.`。
- `/metrics` 中 `fallback_requests_total` 增加。
- `/api/fallback/logs?limit=20` 中没有异常切换，除非上游确实失败。

### 2. 流式请求

运行基础脚本后确认：

- 脚本输出 `Stream request passed.`。
- 返回内容包含 SSE `data:`。
- 若流中断，上游错误应触发 fallback，而不是让请求长期卡住。

### 3. 429 或上游失败

推荐先用脚本的故障场景模拟：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/fallback-smoke.ps1 -RunFaultScenarios
```

通过标准：

- 主部署冷却后，请求仍成功。
- `/api/fallback/logs?limit=20` 能看到切换记录。
- `/api/fallback/alert/history?limit=20` 能看到 cooldown 或相关告警。

真实 429 验证方式：

- 使用一个容易触发上游限速的部署。
- 连续请求直到上游返回 429。
- 确认该部署进入冷却或失败状态。
- 下一次请求应立即使用其他可用部署。

### 4. 额度 95% 阈值

这个场景需要真实 token 用量，脚本不能直接伪造。

推荐验证方式：

1. 在部署配置里选择一个受控额度部署。
2. 临时把 `daily_limit_tokens` 调低，例如 `1000`。
3. 保持 `soft_limit_ratio` 为 `0.95`，或按当前配置确认软阈值。
4. 多次请求让 `used_tokens / daily_limit_tokens` 接近或超过软阈值。
5. 再发起请求。

通过标准：

- 达到软阈值后，不继续长期使用该部署。
- 请求应切到同一虚拟模型下的其他健康部署。
- 达到 hard limit 后，该部署应被跳过。
- `/fallback/status` 和 `/api/fallback/states` 中能看到用量变化。

测试结束后恢复原始额度配置，并重新加载 fallback 配置。

### 5. 全部失败

脚本会通过批量冷却模拟：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/fallback-smoke.ps1 -RunFaultScenarios
```

通过标准：

- 当该虚拟模型下全部部署都不可用，请求必须失败。
- 返回错误应明确表示没有可用部署或上游均失败。
- `fallback_failed_total` 应增加。
- 告警历史里应有 all failed 或 cooldown 相关记录。

### 6. 恢复后请求

脚本会在 `finally` 中恢复部署状态。也可以手动恢复：

```powershell
Invoke-WebRequest -Method POST `
  -Uri "http://localhost:3007/api/fallback/deployments/<deployment-id>/recover" `
  -Headers @{ Authorization = "Bearer $env:CCT_ADMIN_TOKEN" } `
  -UseBasicParsing
```

通过标准：

- 恢复后非流式请求成功。
- 恢复后部署不再显示 cooldown 或 exhausted。
- 后续请求可重新选择恢复后的部署。

## 验证入口

常用页面：

- `http://localhost:3007/fallback/status`
- `http://localhost:3007/fallback/scores`

常用接口：

```powershell
Invoke-WebRequest http://localhost:3007/metrics -UseBasicParsing
Invoke-WebRequest http://localhost:3007/api/fallback/states -Headers @{ Authorization = "Bearer $env:CCT_ADMIN_TOKEN" } -UseBasicParsing
Invoke-WebRequest http://localhost:3007/api/fallback/logs?limit=20 -Headers @{ Authorization = "Bearer $env:CCT_ADMIN_TOKEN" } -UseBasicParsing
Invoke-WebRequest http://localhost:3007/api/fallback/alert/history?limit=20 -Headers @{ Authorization = "Bearer $env:CCT_ADMIN_TOKEN" } -UseBasicParsing
```

## 测试结论模板

```text
测试日期：
服务地址：
虚拟模型：
部署列表：

非流式：通过 / 失败，备注：
流式：通过 / 失败，备注：
429/上游失败切换：通过 / 失败，备注：
额度 95% 阈值切换：通过 / 失败，备注：
全部失败：通过 / 失败，备注：
恢复后请求：通过 / 失败，备注：

发现的问题：
是否适合长期使用：
```
