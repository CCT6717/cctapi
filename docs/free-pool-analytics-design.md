# Free Pool Analytics 设计文档

> 版本: v0.1  
> 创建日期: 2026-06-21  
> 目标: 在不改动后端核心逻辑、不改 DB、不改 `data/fallback.json`/`one-api.exe`/`one-api.db` 的前提下，设计一套可观测性方案，用于分析 Free Pool（以及所有 fallback deployment）的运行时行为和健康度。

---

## 1. 背景与目标

cctapi 的 fallback 模块管理三个 virtual model（`cct/free`, `cct/low`, `cct/high`），每个 virtual model 下有多组 deployment。当前缺乏统一的聚合分析视图，排查问题时需要拼凑多个日志文件和 DB 表。

### 核心目标

- 回答"今天 Free Pool 整体表现如何"——成功率、429 频率、哪家 provider 最稳、哪个 key 在被限流
- 定位异常 deployment——哪个 key 持续 5xx、哪个被 cooldown 了却不恢复
- 支撑容量规划——每个 provider 的 headroom、每个 key 的 RPM/RPD 实际消耗 vs 限额
- 不增加核心路径延迟——分析是旁路读，不应影响请求转发

### 约束确认

| 约束 | 处理方式 |
|------|---------|
| 不改后端核心逻辑 | 新增包 `fallback/analytics/`，不侵入 relay 主流程 |
| 不改 DB | 最小方案纯 runtime state；完整方案需新增表（见方案分层） |
| 不改 `one-api.*` 文件 | 数据仅从 Go 层面内存结构和现有 DB 表读取 |
| 不写代码 | 本文档只列设计方案，不包含 Go 实现 |

---

## 2. 现有数据结构与可获取数据源

### 2.1 内存运行时状态

**`fallback/quota.go` — `DeploymentRuntimeState`**

```
DeploymentID    string
MinuteRequests  int      // 当前分钟请求数
DayRequests     int      // 当天请求数
MinuteTokens    int      // 当前分钟 Token 消耗
DayTokens       int      // 当天 Token 消耗
SuccessCount    int      // 累计成功数
FailureCount    int      // 累计失败数
RateLimitScore  int      // 速率限制惩罚分 (0-10)
LastError       string   // 最近一次错误信息
LastErrorAt     time.Time
```

**特点**: 进程级内存 map，进程重启后清零；分钟/天窗口滑动重置。

### 2.2 DB 持久化状态

**`fallback/state.go` — `DeploymentState`** (表 `deployment_states`)

```
DeploymentID         string     // UNIQUE + Date
Date                 string     // 配额日 (12:00 UTC+8 为界)
UsedPromptTokens     int
UsedCompletionTokens int
UsedTotalTokens      int64
RequestCount         int
SuccessCount         int
ErrorCount           int
ExhaustedUntil       *time.Time
CooldownUntil        *time.Time
LastErrorCode        string
LastErrorMessage     string
```

每条记录覆盖一个 deployment 的一个配额日。**不区分错误类型**（429/5xx/timeout 全部计入 ErrorCount）。

**`fallback/state.go` — `DeploymentCooldownState`** (表 `deployment_cooldown_states`)

```
DeploymentID  string     // UNIQUE
Reason        string
CooldownUntil *time.Time
```

最新一条 cooldown 记录，覆盖式写入，无历史。

### 2.3 健康检查状态

**`fallback/health.go` — `healthState`**

内存 map：`deploymentID -> HealthStatus`（healthy / rate_limited / invalid / error / unknown）。

注意：free deployment（`QuotaMode == "free"`）跳过健康检查，状态恒为 `unknown`。

### 2.4 分数快照

**`fallback/score_history.go` — `ScoreSnapshot`** (表 `score_snapshots`)

```
VirtualModel string
DeploymentID string
Score        float64
CreatedAt    time.Time
```

每次 `GetDeploymentScores()` 调用会记录所有 deployment 的当前评分快照。已有 DB migration，可直接查询。

### 2.5 配置数据

**`fallback/config.go` — `Config` / `DeploymentConfig` / `VirtualModelConfig` / `FreeProviderConfig`**

```
VirtualModelConfig: {Name, Strategy, Pools, ...}
DeploymentConfig:  {ID, ChannelID, RealModel, Pool, QualityTier, CostTier,
                     RPMLimit, RPDLimit, TPMLimit, TPDLimit, ...}
FreeProviderConfig: {providerName, Keys[], LimitsOverride, ...}
```

关键关系：
- `FreeProviderConfig.keys[]` → `SafeKeyHash(key)` → `deploymentID = "free:{provider}-{keyhash}"`
- `DeploymentConfig.Pool` = `"free"` 的都属于 Free Pool
- `DeploymentConfig.CostTier` = `"free"` 的 deployment 都属于免费层级

### 2.6 请求路径事件

relay 控制器 (`controller/relay.go`) 在每个请求完成后调用：

| 调用 | 记录什么 | 位置 |
|------|---------|------|
| `RecordUsage(depID, tokens)` | 增加 runtime 请求/Token 计数 | 成功后 |
| `RecordSuccess(depID)` | 增加 runtime SuccessCount | 成功后 |
| `RecordDeploymentSuccess(depID, usage)` | 增加 DB 层 request/success/token | 成功后（部分 mode） |
| `RecordDeploymentError(depID, err)` | 增加 DB 层 error count | 失败后 |
| `RecordFailure(depID, msg, isRateLimit)` | 增加 runtime failure + rate limit 分 | 失败后 |
| `MarkDeploymentExhausted/Cooldown` | 更新 DB 层状态 | 按错误分类 |
| `SetStickyDeployment(vm, depID)` | 内存 map | 成功后 |

---

## 3. 维度 — 指标映射

### 3.1 需要的分析维度

| 维度 | 现有数据源 | 是否可直接获取 |
|------|-----------|--------------|
| virtual_model | 配置 (Config.VirtualModels) + 路由上下文 | 需要从路由流程标记；现有 DB 无此字段 |
| provider | Config.Deployments[depID].ChannelType 或 free_pool 命名约定 (`free:{provider}-{hash}`) | 可从 deployment ID 解析 |
| key hash | deployment ID 编码 (`free:openrouter-a1b2c3d4`) | 可解析 |
| deployment | 现有所有结构都按 deployment ID 索引 | 可直接获取 |
| real_model | DeploymentConfig.RealModel | 可直接获取 |
| user | One API 原有 token/user 体系 | 需关联 relay 上下文 |
| time window | 分钟级（runtime）+ 日级（DB state） | 分钟级仅存活于内存；日级在 DB |

### 3.2 需要的分析指标

| 指标 | 现有来源 | 获取方式 | 粒度 |
|------|---------|---------|------|
| **请求量** | Runtime: MinuteRequests, DayRequests / DB: RequestCount | Runtime 实时，DB 日终 | deployment + 分钟/天 |
| **成功率** | Runtime: SuccessCount/(SuccessCount+FailureCount) / DB: SuccessCount/RequestCount | 计算 | deployment + 天 |
| **429 次数** | ⚠️ 无独立计数。`LastError` 含"429"字符串，RateLimitScore 反映趋势但不精确 | **需要新计数器** | — |
| **5xx 次数** | ⚠️ 无独立计数。`LastError` 含状态码字面 | **需要新计数器** | — |
| **timeout 次数** | ⚠️ 无独立计数 | **需要新计数器** | — |
| **Token 消耗** | Runtime: MinuteTokens, DayTokens / DB: UsedTotalTokens | 直接读取 | deployment + 分钟/天 |
| **quota headroom** | `DeploymentConfig.RPMLimit - MinuteRequests` 等 | Runtime 计算 | deployment + 分钟/天 |
| **cooldown 次数** | ⚠️ `DeploymentCooldownState` 仅存当前状态，无历史 | **需要历史表** | — |
| **fallback 次数** | ⚠️ 路由循环计数在 relay controller 中，未暴露为指标 | **需要新计数器** | — |
| **sticky 命中率** | `GetStickyDeployment()` 内存 map | 当前无命中计数 | — |
| **provider 分布** | 按 deployment ID 前缀分组统计 | 聚合计算 | 小时/天 |

### 3.3 评估结论：现有 Runtime State 的覆盖度

| 覆盖状态 | 指标 |
|---------|------|
| **可直接获取** (6/15) | 请求量、成功率、Token 消耗、quota headroom、provider 分布 |
| **需派生/推断** (3/15) | virtual_model 分布（需标记）、key hash（从 deployment ID 解析）、real_model 关联（从配置读取） |
| **需要新计数器** (6/15) | 429/5xx/timeout 分类计数、cooldown 次数、fallback 次数、sticky 命中率 |

---

## 4. API 设计

### 4.1 端点

```
GET /api/fallback/free-pool/analytics
```

### 4.2 查询参数

| 参数 | 类型 | 默认 | 说明 |
|------|------|------|------|
| `window` | string | `"1h"` | 时间窗口 `1m` / `1h` / `24h` |
| `provider` | string | 空（全部） | 过滤 provider，如 `"openrouter"`, `"groq"` |
| `deployment` | string | 空（全部） | 过滤单个 deployment ID |
| `virtual_model` | string | 空（全部） | 过滤 virtual model，如 `"cct/free"` |

### 4.3 响应结构

```json
{
  "window": "1h",
  "generated_at": "2026-06-21T10:00:00Z",
  "filters_applied": {
    "provider": "openrouter",
    "deployment": "",
    "virtual_model": "cct/free"
  },
  "summary": {
    "total_requests": 15820,
    "success_rate": 0.923,
    "total_tokens": 2840000,
    "avg_latency_ms": 1840,
    "active_deployments": 12,
    "deployments_in_cooldown": 2,
    "deployments_exhausted": 1
  },
  "by_deployment": [
    {
      "deployment_id": "free:openrouter-1f9cf7de",
      "provider": "openrouter",
      "key_hash": "1f9cf7de",
      "real_model": "openrouter/free",
      "requests": 3200,
      "success_count": 2890,
      "failure_count": 310,
      "success_rate": 0.903,
      "error_breakdown": {
        "rate_limit_429": 180,
        "server_error_5xx": 45,
        "timeout": 62,
        "other": 23
      },
      "token_consumption": {
        "minute_tokens": 45000,
        "day_tokens": 580000
      },
      "headroom": {
        "rpm": { "limit": 20, "used": 8, "remaining": 12, "usage_pct": 40.0 },
        "rpd": { "limit": 500, "used": 320, "remaining": 180, "usage_pct": 64.0 },
        "tpm": { "limit": 4000, "used": 750, "remaining": 3250, "usage_pct": 18.8 },
        "tpd": { "limit": 100000, "used": 58000, "remaining": 42000, "usage_pct": 58.0 }
      },
      "cooldown_count": 3,
      "current_cooldown": "cooling down: 429 until 2026-06-21T10:05:00Z",
      "health_status": "rate_limited",
      "score": 72.5
    }
  ],
  "by_provider": [
    {
      "provider": "openrouter",
      "deployment_count": 6,
      "total_requests": 8920,
      "success_rate": 0.915,
      "avg_429_rate": 0.042,
      "avg_5xx_rate": 0.015
    },
    {
      "provider": "groq",
      "deployment_count": 6,
      "total_requests": 6900,
      "success_rate": 0.935,
      "avg_429_rate": 0.038,
      "avg_5xx_rate": 0.008
    }
  ],
  "by_virtual_model": [
    {
      "virtual_model": "cct/free",
      "deployment_count": 12,
      "total_requests": 15820,
      "success_rate": 0.923
    }
  ],
  "sticky_stats": {
    "sticky_count": 14200,
    "non_sticky_count": 1620,
    "sticky_hit_rate": 0.898
  },
  "fallback_stats": {
    "total_attempts": 17050,
    "fallback_count": 1230,
    "avg_fallback_depth": 1.4
  }
}
```

---

## 5. 最小实现方案（纯 Runtime State）

### 5.1 方案概述

新增一组进程内计数器，与现有的 `DeploymentRuntimeState` 平级，但不依赖 DB。所有数据从内存读取，进程重启后丢失。

### 5.2 新增内存结构

```go
// 在 fallback/ 包内新增文件 analytics.go

type DeploymentErrorBreakdown struct {
    RateLimit429 int  // 429
    Server5xx    int  // 500-599
    Timeout      int  // timeout/context deadline
    Other        int  // 其他
}

type DeploymentAnalytics struct {
    DeploymentID       string
    ErrorBreakdown     DeploymentErrorBreakdown
    CooldownEventCount int
    FallbackCount      int
    StickyHitCount     int
    StickyMissCount    int
}
```

### 5.3 埋点位置

| 事件 | 现有调用位置 | 增加计数 |
|------|-------------|---------|
| 429 分类 | `RecordFailure(depID, msg, true)` + 分支 | `analytics.Record429(depID)` |
| 5xx 分类 | `RecordFailure(depID, msg, false)` + relay 返回码 | `analytics.Record5xx(depID)` |
| timeout | relay controller 超时路径 | `analytics.RecordTimeout(depID)` |
| cooldown | `ApplyCooldown()` 或 `MarkDeploymentCooldown*` | `analytics.RecordCooldown(depID, reason)` |
| fallback | relay fallback 循环中 `continue` 时 | `analytics.RecordFallback(depID)` |
| sticky 命中 | `GetStickyDeployment() == dep.ID` 时 | `analytics.RecordStickyHit(vm)` 或 miss |

**埋点改造范围**: 不侵入核心逻辑，仅在已有条件分支末尾加一行 `analytics.RecordXxx()` 调用。

### 5.4 依赖与风险

| 项目 | 说明 |
|------|------|
| 依赖 | 无 DB，纯内存 |
| 风险 | 进程重启后计数清零；分钟/天窗口覆盖后旧数据丢失 |
| 进程重启影响 | 所有分析数据归零，无法跨进程聚合 |
| 内存占用 | 极低，每个 deployment 约 200 字节 |
| 性能影响 | 每次计数仅一次 atomic 操作或 map 写 |
| 实现时间 | 约半天（含 API 端点） |

### 5.5 适用场景

- 实时仪表盘（只看"现在"的状态）
- 调试单个 deployment 的即时行为
- 不需要历史趋势的开发环境

---

## 6. 完整实现方案（需要 DB migration）

### 6.1 新增表设计

#### `free_pool_request_logs`

记录每个请求的结果（采样率可配置，默认 100%）。

```sql
CREATE TABLE free_pool_request_logs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    virtual_model   TEXT NOT NULL,              -- cct/free, cct/low, cct/high
    deployment_id   TEXT NOT NULL,              -- free:openrouter-1f9cf7de
    provider        TEXT NOT NULL,              -- openrouter, groq
    key_hash        TEXT NOT NULL,              -- 1f9cf7de
    real_model      TEXT NOT NULL,              -- openrouter/free
    user_id         INTEGER,                    -- 关联 one-api 用户
    success         BOOLEAN NOT NULL,           -- 是否成功
    error_category  TEXT,                       -- rate_limit / server_error / timeout / other
    status_code     INTEGER,                    -- HTTP 状态码
    latency_ms      INTEGER,                    -- 请求耗时
    tokens          INTEGER DEFAULT 0,          -- 消耗 Token 数，0 表示未知
    sticky_hit      BOOLEAN DEFAULT FALSE,      -- 是否命中 sticky deployment
    fallback_depth  INTEGER DEFAULT 0           -- 第几次 fallback 尝试成功/失败
);

CREATE INDEX idx_fpl_created_at ON free_pool_request_logs(created_at);
CREATE INDEX idx_fpl_vm ON free_pool_request_logs(virtual_model, created_at);
CREATE INDEX idx_fpl_deployment ON free_pool_request_logs(deployment_id, created_at);
CREATE INDEX idx_fpl_provider ON free_pool_request_logs(provider, created_at);
```

#### `free_pool_cooldown_events`

记录每次 cooldown 事件（区别于当前覆盖式状态表，保留历史）。

```sql
CREATE TABLE free_pool_cooldown_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    deployment_id   TEXT NOT NULL,
    reason          TEXT,
    duration_sec    INTEGER,                    -- cooldown 时长（秒）
    category        TEXT                        -- rate_limit / temporary / quota / invalid
);

CREATE INDEX idx_fpce_deployment ON free_pool_cooldown_events(deployment_id, created_at);
```

### 6.2 与现有表的关系

| 现有表 | 与新增表的关系 |
|--------|--------------|
| `deployment_states` | 补充细粒度错误分类和 latency；`deployment_states` 保留日累计值作主数据 |
| `deployment_cooldown_states` | 被 `free_pool_cooldown_events` 补充历史记录；当前 cooldown 状态仍从此表读 |
| `score_snapshots` | 无重叠；分数用于排序决策，analytics 用于可观测性 |
| `alert_events` | 无重叠；alert 是异常通知，analytics 是聚合视图 |

### 6.3 依赖与风险

| 项目 | 说明 |
|------|------|
| 依赖 | DB migration (`AutoMigrate`)，需要写 Go 建表代码 |
| 风险 | 新增表增加写入压力和存储量；需考虑采样率 |
| 存储量估算 | 假设每天 5 万请求，每条约 200 字节，每年约 3.6 GB |
| 采样策略 | 高流量环境可设采样率 10%（`free_pool_request_logs` 字段 `sampled BOOLEAN`） |
| 写入延迟 | 每次请求结束后异步写入 goroutine，不阻塞响应 |
| 数据保留 | 建议 TTL 自动清理（如保留 30 天活跃 + 12 个月归档） |

---

## 7. 方案对比

| 对比维度 | 最小方案（Runtime State） | 完整方案（DB） |
|---------|------------------------|--------------|
| 数据持久性 | 无，重启即失 | 持久化 |
| 历史趋势 | 无 | 支持按小时/天/周聚合 |
| 部署难度 | 极低，纯加内存结构 | 需要 migration + 写入路径 |
| 查询速度 | 纳秒级，全内存 | 毫秒级，需 SQL 聚合 |
| 存储成本 | 零 | 约 3.6 GB/年（5万请求/天） |
| 进程重启影响 | 全部清零 | 无影响 |
| 可实现时间 | 0.5-1 天 | 2-3 天 |
| 跨凌晨分析 | 不支持（日窗口重置） | 支持任意时间范围 |

---

## 8. 实现优先级建议

### P0 — 必要（立即做）

1. **`GET /api/fallback/free-pool/analytics` 端点**

   先空壳返回，只聚合现有 runtime state 中的数据（不新增计数器）。可以回答"现在哪些 deployment 在用、限额还剩多少"。

   - 数据来源：`SnapshotRuntimeState` + `DeploymentConfig` + `GetHealthStatus` + `GetDeploymentCooldown`
   - 不需要新 DB 表、不需要新计数器
   - 纯读操作，零风险

2. **provider/deployment/ready status 概览**

   在 analytics 响应中提供 `by_provider` 聚合（从 deployment ID 前缀解析 provider 名）。

### P1 — 高优先级（近期做）

3. **错误分类计数器（内存）**

   参照最小方案，新增 `DeploymentErrorBreakdown` 内存结构，在 `RecordFailure` 和 relay controller 中按状态码分类计数。

   - 不涉及 DB
   - 改动点：新增文件 + 修改 `RecordFailure` 签名或加新方法

4. **cooldown 事件计数**

   在同内存结构中增加 cooldown 计数。

### P2 — 中等优先级（规划期）

5. **请求日志表 `free_pool_request_logs`**

   新增 DB migration + 异步写入路径 + 采样率支持。这是"完整方案"的核心。

6. **cooldown 事件表 `free_pool_cooldown_events`**

   补充历史 cooldown 记录，解决当前覆盖式写入无历史的问题。

### P3 — 低优先级（未来）

7. **sticky 命中率统计**

   新增计数器 + 响应字段。

8. **用户维度的分析**

   关联 user_id，需要 relay 上下文传入 analytics 层，改动面较大。

9. **聚合视图缓存**

   对于 24h 窗口的聚合查询，增加内存缓存减少 DB 压力（例如每 60 秒刷新一次）。

---

## 9. 不做的边界

| 场景 | 原因 |
|------|------|
| 实时告警 | 已有 AlertManager，不应重复 |
| 自动扩缩容 | 超出免费池设计范围，AI 推理 key 无法水平扩展 |
| 详细请求体/响应体追踪 | 隐私和存储成本，不在 analytics 范围内 |
| 图表渲染 | 仅提供 JSON API，前端自行渲染 |
| 接入 Prometheus / Grafana | 如后续需要，应从现有 metrics 端点导出，本设计聚焦结构化 JSON API |

---

## 10. 附录：部署结构图

```
┌─────────────────────────────────────────────────────┐
│                   relay controller                   │
│  (controller/relay.go)                              │
│    ↓ success/failure → RecordUsage/RecordFailure     │
│    ↓ error category → MarkExhausted/MarkCooldown     │
└──────────┬──────────────────────────────────────────┘
           │
           ▼
┌──────────────────┐   ┌───────────────────────┐
│  Runtime State    │   │  DB State              │
│  (quota.go)       │   │  (state.go)            │
│  per-deployment   │   │  per-deployment-date   │
│  rpm/rpd/tpm/tpd  │   │  tokens/req/succ/err   │
│  succ/fail/rl     │   │  exhausted/cooldown    │
└────────┬─────────┘   └───────────┬───────────┘
         │                         │
         ▼                         ▼
┌─────────────────────────────────────────────────────┐
│              Analytics API 层                        │
│  GET /api/fallback/free-pool/analytics              │
│    1. 读 Runtime State → 实时指标                   │
│    2. 读 DB State → 日累计指标                      │
│    3. 读 Config → 维度和限额                        │
│    4. 读 CooldownState → cooldown 状态              │
│    5. 读 ScoreSnapshot → 评分历史（可选）            │
│    6. 聚合 → 按 provider/deployment/vm 分组         │
│    7. 返回 JSON                                     │
└─────────────────────────────────────────────────────┘
```

### 数据流说明

1. **写入路径**（已有，不改）：relay controller → `quota.go`/`state.go` → 内存 + DB
2. **读取路径**（新增只读）：Analytics API → 轮询各数据源 → 聚合 → JSON
3. **计数器增强**（P1）：relay controller 加 `analytics.Record429()` 等 → 新增内存 map
4. **DB 扩展**（P2）：新增 `free_pool_request_logs` 表，relay controller 异步写入
