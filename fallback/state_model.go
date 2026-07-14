// Package fallback 三级运行状态模型 — 统一运行状态文档（Phase 2）
//
// 本文件将 cctapi 的回退路由系统中的三级状态边界和记账契约正式化为代码文档。
// 它不引入任何编译时逻辑，仅作为状态模型的权威参考。后续重构或新增状态
// 字段时，须同步更新本注释。
//
// ============================================================================
// 一、三级状态概述
// ============================================================================
//
// cctapi 的回退路由系统使用三层状态管理，每层有不同的生命周期、存储位置和
// 典型事件。理解各层的边界是维护回退逻辑的关键。
//
// ┌─────────────────────────────────────────────────────────────────────────┐
// │  层级                │  管理内容                          │  存储位置      │
// ├─────────────────────────────────────────────────────────────────────────┤
// │  模型级              │  单个真实模型的进程级内存状态（冷却和429计数）      │  内存          │
// │  (Model Level)       │  仅 Kilo 供应商有模型级轮换          │                │
// ├─────────────────────────────────────────────────────────────────────────┤
// │  部署/供应商级        │  失败数、限流分、配额、冷却、sticky   │  内存 + 持久化 │
// │  (Deployment Level)  │                                    │                │
// ├─────────────────────────────────────────────────────────────────────────┤
// │  渠道级              │  渠道启用、认证和自动禁用             │  数据库        │
// │  (Channel Level)     │                                    │                │
// └─────────────────────────────────────────────────────────────────────────┘
//
// ============================================================================
// 二、各层详细说明
// ============================================================================
//
// 2.1 模型级（Model Level）
// ──────────────────────────
// 管理内容：
//   - 单个真实模型（real model）的进程级内存状态，由多个请求共享，进程重启后清空
//   - 连续 429 计数（consecutive429Count）
//   - 成功/失败计数
//   - 最后尝试时间、最后成功时间
//
// 存储位置：
//   - 进程级内存状态，由多个请求共享，进程重启后清空
//   - 代码位置：fallback/free_provider_model_runtime.go
//
// 涉及类型：
//   - freeProviderModelRuntimeEntry（内部运行时条目）
//   - FreeProviderModelRuntimeEntrySnapshot（快照）
//   - FreeProviderModelRuntimeSummary（汇总）
//
// 典型事件：
//   - MarkFreeProviderModelRateLimited：Kilo 模型 429 时记录模型级冷却
//   - RecordFreeProviderModelSuccess：Kilo 模型成功时清除冷却并计数
//
// 关键约束：
//   - 只有 Kilo 供应商支持模型级轮换
//   - 模型级状态独立于部署级状态：模型 429 不触发部署级冷却或 RateLimitScore
//   - 部署恢复时通过 ResetFreeProviderModelRuntime 重置模型级状态
//
// 2.2 部署/供应商级（Deployment/Provider Level）
// ──────────────────────────────────────────────
// 管理内容：
//   A. 持久化状态（DeploymentState）：
//      - 日配额使用量（UsedPromptTokens / UsedCompletionTokens / UsedTotalTokens）
//      - 请求计数（RequestCount）
//      - 成功计数（SuccessCount）
//      - 错误计数（ErrorCount）
//      - 配额耗尽截止时间（ExhaustedUntil）
//      - 冷却截止时间（CooldownUntil）
//      - 最后错误码和错误消息（LastErrorCode / LastErrorMessage）
//
//   B. 持久化冷却状态（DeploymentCooldownState）：
//      - 独立的冷却表，用于路由检查时的冷却判断
//      - 冷却原因（Reason）
//      - 冷却截止时间（CooldownUntil）
//
//   C. 内存运行时状态（DeploymentRuntimeState）：
//      - RPM（分钟请求数）/ RPD（日请求数）
//      - TPM（分钟 Token 数）/ TPD（日 Token 数）
//      - 运行时成功计数（SuccessCount）
//      - 运行时失败计数（FailureCount）
//      - 限流惩罚分（RateLimitScore，上限 10）
//      - 最后错误和最后错误时间
//
//   D. 粘性路由（stickyDep）：
//      - 虚拟模型到部署的映射（virtualModel -> deploymentID）
//      - 用于保持同一虚拟模型的请求优先路由到同一部署
//
//   E. 健康状态（HealthStatus）：
//      - healthy / rate_limited / invalid / error / unknown
//      - 通过后台健康检查探针定期更新
//
// 存储位置：
//   - DeploymentState：数据库持久化（表 deployment_states）
//   - DeploymentCooldownState：数据库持久化（表 deployment_cooldown_states）
//   - DeploymentRuntimeState：内存（运行时映射 runtimeStates）
//   - stickyDep：内存（运行时映射）
//   - HealthStatus：内存（运行时映射 globalHealth.status）
//
// 涉及类型和文件：
//   - DeploymentState              → fallback/state.go
//   - DeploymentCooldownState      → fallback/state.go
//   - DeploymentPersistentStateSnapshot → fallback/state.go
//   - DeploymentCooldownSnapshot   → fallback/state.go
//   - stickyDep（映射）            → fallback/state.go
//   - DeploymentRuntimeState       → fallback/quota.go
//   - RuntimeStateSnapshot         → fallback/quota.go
//   - HealthStatus                 → fallback/health.go
//
// 典型事件：
//   - RecordDeploymentUsage：记录 Token 使用量
//   - RecordDeploymentSuccess：记录成功请求
//   - RecordDeploymentError：记录错误并增加 ErrorCount
//   - RecordFailure：增加运行时失败计数和 RateLimitScore
//   - MarkDeploymentExhausted：标记配额耗尽
//   - MarkDeploymentCooldown：标记部署冷却
//   - SetStickyDeployment / ClearStickyDeployment：管理粘性路由
//
// 2.3 渠道级（Channel Level）
// ──────────────────────────
// 管理内容：
//   - 渠道启用/禁用状态（channel.Status）
//   - 渠道认证信息（Key、BaseURL 等）
//   - 自动禁用逻辑（DisableChannel）
//
// 存储位置：
//   - 数据库（channels 表）
//
// 涉及代码：
//   - controller/relay.go（路由循环中检查 channel.Status）
//   - monitor.DisableChannel（自动禁用渠道）
//
// 典型事件：
//   - 路由时检查 channel.Status != ChannelStatusEnabled → 跳过
//   - 错误达到阈值时 monitor.DisableChannel(channelID) → 自动禁用
//
// ============================================================================
// 三、层间边界规则
// ============================================================================
//
// 规则 1：模型级 429 不跨层到部署级
//   - Kilo 供应商的单个模型返回 429 时，只更新模型级状态
//     （MarkFreeProviderModelRateLimited）
//   - 不触发部署级冷却、不增加 RateLimitScore、不记录 DeploymentError
//   - 目的：允许同一部署内的其他模型继续服务，避免过度惩罚
//
// 规则 2：所有模型耗尽后才触发部署级处理
//   - 当 Kilo 部署的所有兼容模型都 429 耗尽时（attempt.HasNextModel() == false），
//     才进入部署级错误处理流程
//   - 此时 RecordDeploymentError + RecordFailure 只执行一次
//
// 规则 3：非 429 错误跳过剩余 Kilo 模型
//   - 500、认证失败等非 429 错误，立即跳过当前部署的所有剩余 Kilo 模型
//   - 通过 i += attempt.RemainingModelAttempts() 实现
//   - 这些错误按普通供应商失败处理（增加部署级失败计数）
//
// 规则 4：部署级状态变化触发粘性清除
//   - MarkDeploymentExhausted、MarkDeploymentCooldown、saveDeploymentCooldown
//     都会调用 ClearStickyDeploymentForDeployment
//   - 目的：部署不可用时，不再将新请求路由到该部署
//
// 规则 5：渠道级与部署级的单向影响
//   - 渠道禁用（channel.Status != Enabled）会导致部署被跳过并标记冷却
//   - 部署级错误（如认证失败）可能触发 monitor.DisableChannel，进而禁用渠道
//   - 但渠道级状态本身不由部署级代码直接管理
//
// ============================================================================
// 四、记账契约（relayWithFallbackUsing 核心规则）
// ============================================================================
//
// 以下 5 条契约已在 controller/relay.go 的 relayWithFallbackUsing 函数中实现，
// Phase 2 仅固化文档，不重构逻辑。
//
// 契约 1：中间模型 429 只更新模型级状态（MarkFreeProviderModelRateLimited），不处罚部署级。
// 契约 2：所有模型 429 耗尽后，供应商失败和限流分只增加一次（RecordDeploymentError + RecordFailure）。
// 契约 3：非 429 错误（500、认证失败等）直接跳过剩余 Kilo 模型（i += attempt.RemainingModelAttempts()）。
// 契约 4：只有 deployment 变化才记录供应商切换事件（fallbackSwitchLog），模型级轮换不触发。
// 契约 5：响应已写出后停止。一旦 c.Writer.Written() 返回 true，立即 return，不再尝试轮换或回退。
//
// ============================================================================
// 五、各层状态涉及类型和文件清单
// ============================================================================
//
// ┌──────────────────────────────────────────────────────────────────────────┐
// │  类型                          │  定义文件                        │  层级  │
// ├──────────────────────────────────────────────────────────────────────────┤
// │  DeploymentState               │  fallback/state.go               │  部署  │
// │  DeploymentCooldownState       │  fallback/state.go               │  部署  │
// │  DeploymentPersistentStateSnapshot │ fallback/state.go             │  部署  │
// │  DeploymentCooldownSnapshot    │  fallback/state.go               │  部署  │
// │  stickyDep（map）              │  fallback/state.go               │  部署  │
// │  DeploymentRuntimeState        │  fallback/quota.go               │  部署  │
// │  RuntimeStateSnapshot          │  fallback/quota.go               │  部署  │
// │  HealthStatus                  │  fallback/health.go              │  部署  │
// │  freeProviderModelRuntimeEntry │  fallback/free_provider_model_runtime.go │ 模型 │
// │  FreeProviderModelRuntimeEntrySnapshot │ fallback/free_provider_model_runtime.go │ 模型 │
// │  FreeProviderModelRuntimeSummary │ fallback/free_provider_model_runtime.go │ 模型 │
// └──────────────────────────────────────────────────────────────────────────┘
//
// 关键操作函数清单：
// ┌──────────────────────────────────────────────────────────────────────────┐
// │  函数                              │  定义文件                      │  层级  │
// ├──────────────────────────────────────────────────────────────────────────┤
// │  MarkFreeProviderModelRateLimited  │  free_provider_model_runtime.go │ 模型 │
// │  RecordFreeProviderModelSuccess    │  free_provider_model_runtime.go │ 模型 │
// │  IsFreeProviderModelCooling        │  free_provider_model_runtime.go │ 模型 │
// │  ResetFreeProviderModelRuntime     │  free_provider_model_runtime.go │ 模型 │
// ├──────────────────────────────────────────────────────────────────────────┤
// │  RecordDeploymentUsage             │  state.go                      │  部署  │
// │  RecordDeploymentSuccess           │  state.go                      │  部署  │
// │  RecordDeploymentError             │  state.go                      │  部署  │
// │  MarkDeploymentExhausted           │  state.go                      │  部署  │
// │  MarkDeploymentCooldown            │  state.go                      │  部署  │
// │  MarkDeploymentCooldownForDuration │  state.go                      │  部署  │
// │  GetDeploymentCooldown             │  state.go                      │  部署  │
// │  SetStickyDeployment               │  state.go                      │  部署  │
// │  ClearStickyDeployment             │  state.go                      │  部署  │
// │  ClearStickyDeploymentForDeployment│  state.go                      │  部署  │
// │  GetRuntimeState                   │  quota.go                      │  部署  │
// │  PassQuotaCheck                    │  quota.go                      │  部署  │
// │  RecordUsage                       │  quota.go                      │  部署  │
// │  RecordSuccess                     │  quota.go                      │  部署  │
// │  RecordFailure                     │  quota.go                      │  部署  │
// │  DecayRateLimitScores              │  quota.go                      │  部署  │
// │  StartHealthChecker                │  health.go                     │  部署  │
// │  GetHealthStatus                   │  health.go                     │  部署  │
// │  IsDeploymentHealthy               │  health.go                     │  部署  │
// ├──────────────────────────────────────────────────────────────────────────┤
// │  monitor.DisableChannel            │  monitor 包（外部）            │  渠道  │
// │  monitor.Emit                      │  monitor 包（外部）            │  渠道  │
// │  monitor.ShouldDisableChannel      │  monitor 包（外部）            │  渠道  │
// └──────────────────────────────────────────────────────────────────────────┘
//
// ============================================================================
// 六、版本信息
// ============================================================================
//
// 本文档对应 Phase 2 — 统一运行状态模型。
// 创建日期见 StateModelVersion 常量。
//
package fallback

// StateModelVersion 标记当前状态模型文档的版本。
// 当状态模型发生重大变更时（如新增层级、修改边界规则、重构记账契约），
// 应更新此版本号并同步更新本文件顶部注释。
const StateModelVersion = "phase2-2026-07-14"
