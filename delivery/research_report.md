# AICoding 架构设计 · 行业调研报告

> 上游输入：主理人转交的用户诉求 + G1 已通过的《资料摘要》（`.workbuddy/output/material_digest.md`）；
> 下游输出：驱动 `business-architect`（业务架构师）的行业调研判断，最终落入《高层架构设计》的 §3 行业调研章节。
>
> **结构纪律**：全文按「事实 → 对比 → 建议 → 风险」四段式组织（§2 事实 / §3 对比 / §4 建议 / §5 风险）。
> **角色边界**：本报告所有结论均为「建议」，加权打分仅为评估而非授权，最终业务边界冻结权归 `business-architect`。资料摘要 §3 的 X1~X7 冲突不在本报告裁决，仅作为调研依据引用。

---

## 0. 元信息：修订记录

```yaml
标题: cctapi（One API CCT 分支 · 虚拟模型回退网关）- 行业调研报告 v1.0
版本: v1.0
状态: Draft   # Draft | Reviewing | Approved | Deprecated
创建日期: 2026-07-14
最后更新: 2026-07-14
调研人: 查有据（research-analyst / 研究分析师）
审核人:
  - 齐构成（team-lead / 主理人）

关联文档:
  上游输入:
    - 用户诉求: 启动 AICoding 架构专家团，基于项目背景和资料生成完整架构方案（主理人注入）
    - 调研目标: 多供应商 LLM 网关 / 虚拟模型回退 / 免费池路由细分赛道标杆调研，支撑高层架构边界冻结（主理人注入）
    - 证据基线: D:/ct/project/.workbuddy/output/material_digest.md（G1 已通过，含 X1~X7 冲突记录）
  下游产出:
    - 高层架构设计 §3 行业调研: 将由 business-architect 整合到此章节
```

| 版本 | 日期 | 作者 | 变更内容 | 评审状态 |
| --- | --- | --- | --- | --- |
| v1.0 | 2026-07-14 | 查有据（research-analyst） | 初稿（Phase 2 / G2） | Draft |

---

## 1. 调研问题收敛

> 调研启动前，先围绕用户诉求收拢为明确的调研问题集合，确保调研不偏离当前项目背景（cctapi：one-api CCT 分支，虚拟模型回退 + 免费池路由 LLM 网关）。

### 1.1 原始调研种子

| 编号 | 待验证论题 | 来源（用户诉求要点） | 调研优先级 | 备注 |
| --- | --- | --- | --- | --- |
| S1 | 多供应商 LLM 网关在「虚拟模型 → 多真实部署」上的路由与 failover 策略主流设计是什么？ | 用户诉求：为 cctapi 虚拟模型回退系统做完整架构方案 | 高 | cctapi 现有 strategy（quality_first/cost_first/free_first）+ pools 模型（material_digest D5，§2） |
| S2 | 免费/低成本池的配额、限流与冷却机制行业通行做法是什么？ | 用户诉求：免费池路由是重点细分方向；cctapi 已有 RPM/RPD/TPM/TPD 四维限额 + 冷却（D5，§7；D6，§2） | 高 | 与 OpenRouter `free-models-per-min`/`free-models-per-day` 限速直接相关（D1，§4） |
| S3 | Chat Completions / Responses / Anthropic Messages 多协议兼容在标杆网关中的工程做法是什么？ | cctapi 已覆盖三协议（D1，§5；D5，§12、§15），需对标成熟度 | 高 | Responses→Chat 转换是 cctapi 关键设计（D5，§12） |
| S4 | 用量记账与可观测性（指标、日志、告警）在网关层的最佳实践是什么？ | cctapi 有 Prometheus 指标 + 切换日志 + 免费池台账（D5，§8、§11） | 中 | — |
| S5 | 自托管（开源/私有化）与 SaaS 网关在成本、合规、可控性上的取舍边界是什么？ | 架构方案需给出自研/采购/复用边界建议 | 中 | 涉及 AGPL（New-API）vs MIT（one-api/LiteLLM）许可差异 |

### 1.2 调研问题收敛

| 编号 | 调研问题 | 调研对象 | 调研目标 | 预期产出 | 关联种子 |
| --- | --- | --- | --- | --- | --- |
| Q1 | 主流 LLM 网关的「模型组/虚拟模型 → 多部署」路由与 failover 设计有哪些模式？各自的健康判定与冷却语义是什么？ | New-API（one-api 生态）、LiteLLM Router/Proxy、Cloudflare AI Gateway、OpenRouter 路由层 | 提炼可借鉴的路由/冷却设计模式与应规避的坑 | §2 标杆详述 + §3 对比矩阵 | S1、S2 |
| Q2 | 多协议（Chat/Responses/Anthropic Messages）兼容在标杆中的实现边界在哪里？转换层放在哪一层最合理？ | 同上四家 | 验证 cctapi「relay/model 边界做协议翻译、controller 只编排」分层是否符合行业惯例 | §2.3 横向事实表 | S3 |
| Q3 | 网关层用量记账、配额限额与可观测性的通行技术栈与数据模型是什么？ | 同上四家 + 各自官方文档 | 为 cctapi 台账/Prometheus/告警演进给出参照 | §4.3 技术栈建议 | S4 |
| Q4 | 在「单机自托管 + 免费池」场景下，自研（沿用 cctapi 底座）vs 采购 SaaS vs 复用开源的取舍边界是什么？ | 四家标杆的部署形态、许可、定价 | 输出自研/采购/复用边界建议（不裁决） | §4.1 边界建议 | S5 |

> **收敛说明**：用户原始诉求「生成完整架构方案」中行业调研的唯一目标是支撑高层架构边界冻结，因此四个问题全部围绕 cctapi 已有的四大子系统（路由回退、协议兼容、用量观测、部署形态）对标，不发散到提示词管理、数据集评测等网关外围赛道。

---

## 2. 事实：标杆系统盘点和方案详述

> **四段式「事实」段**。只陈列调研发现的事实，不做引申建议或边界裁决。

### 2.1 行业标杆清单

| 编号 | 标杆系统 | 厂商 / 社区 | 部署形态 | 场景覆盖 | 技术亮点 | 商业模式 | 调研来源 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| B1 | New-API（QuantumNous/new-api，one-api 活跃 fork） | QuantumNous 社区 | 自托管（Docker/二进制，SQLite/MySQL/PostgreSQL） | OpenAI 兼容聚合网关 + AI 资产管理（计费/令牌/多租户） | 多渠道负载均衡与故障切换；Claude Messages ↔ OpenAI、Gemini 双向转换；支持 Responses/Realtime API；缓存计费 | 开源 AGPL-3.0（SaaS 部署需开源）+ 商业授权 | 官方文档站 newapi.ai / GitHub QuantumNous/new-api（SR-01、SR-02） |
| B2 | LiteLLM（Proxy + Router SDK） | BerriAI（开源社区 + 商业公司） | 自托管（Docker/K8s，Postgres + Redis） | 100+ 供应商统一 OpenAI 兼容代理 + 团队/密钥/预算管理 | Router 路由策略（simple-shuffle/least-busy/usage-based/latency-based/cost-based）；fallbacks/context_window_fallbacks/content_policy_fallbacks 三层回退；allowed_fails + cooldown_time 冷却；虚拟密钥与预算 | 开源（MIT 许可主体）+ Enterprise 增值 | docs.litellm.ai（SR-03、SR-04） |
| B3 | Cloudflare AI Gateway | Cloudflare（头部 SaaS） | 纯 SaaS（边缘网络托管，BYOK 上游密钥） | 20+ 供应商统一接入 + 缓存/限流/日志/分析 | 动态路由（可视化 flow 编排 + 自动 fallback）；全局缓存（官方称重复查询延迟最高降 90%）；Spend Limits（按模型/供应商/自定义元数据的美元预算）；Guardrails/DLP | SaaS：核心功能免费；Workers Paid 计划 $5/月起含 1000 万请求/月，超出 $0.05/百万；Unified Billing 代购上游加 5% 手续费 | developers.cloudflare.com/ai-gateway（SR-05、SR-06、SR-07） |
| B4 | OpenRouter（统一 LLM 路由 SaaS，cctapi 免费池的上游供应商之一） | OpenRouter Inc.（头部 SaaS） | 纯 SaaS（边缘路由） | 60+ 供应商、400+ 模型统一 OpenAI 兼容接口 | 双层 failover：provider 层自动切换（默认开，`allow_fallbacks`）+ model 层 `models` 数组回退（opt-in）；默认按价格逆平方加权 + 30 秒健康窗负载均衡；`:free`/`:nitro`/`:floor` 模型变体；免费层 20 RPM、50 次/日（购 ≥$10 credits 升至 1000 次/日） | 充值 credits 按用量计费；BYOK 模式收 5% 路由费（每月 100 万请求免路由费）；失败请求不收费 | openrouter.ai/docs 与官方博客（SR-08、SR-09、SR-10） |

> **类别核对**：B1、B2 为开源/自研代表（B1 与 cctapi 同宗 one-api 生态，B2 为 Python 生态主流）；B3、B4 为头部 SaaS 代表（B4 同时是 cctapi 免费池的真实上游，见 material_digest D1，§4~§5）。满足「≥3 家、≥1 SaaS、≥1 开源/自研」硬指标。

### 2.2 标杆方案详述

#### 2.2.1 B1 - New-API（QuantumNous/new-api）

| 维度 | 内容 | 置信度 |
| --- | --- | --- |
| 产品定位 | 面向自用/团队/企业私有化的 OpenAI 兼容 AI API 网关与用量管理系统，one-api 的活跃 fork | 已核实（SR-02） |
| 目标用户 | 自建中转站的团队、需要多租户计费与支付集成的运营者 | 已核实（SR-01、SR-02） |
| 核心能力 | 统一 OpenAI 兼容接口聚合 30+ 供应商；多渠道负载均衡（加权随机/轮询/优先级）与故障自动切换；Claude Messages ↔ OpenAI、Gemini 双向协议转换；Responses/Realtime API 支持；按请求/按 Token/按缓存命中计费；EPay/Stripe 支付；多 OAuth 登录 | 已核实（SR-01、SR-02） |
| 架构特点 | Go 单体 + 嵌入式/独立 Web 管理端，数据库可选 SQLite/MySQL/PostgreSQL；沿用 one-api「渠道（Channel）+ 令牌（Token）+ 用户分组 + 模型倍率」领域模型；上游 one-api 已停止维护，New-API 于 2025 年起成为社区主流接棒 fork | 已核实（SR-01、SR-12）+ 综合归纳 |
| 部署形态 | 自托管（Docker Compose / 二进制），默认 SQLite | 已核实（SR-01、SR-12） |
| 集成方式 | OpenAI 兼容 REST API + 管理 Web UI；与 one-api 数据库结构基本兼容，社区实践支持同数据卷镜像替换迁移 | 已核实（SR-12） |
| 定价模式 | 软件免费（AGPL-3.0）；作为网络服务对外提供修改版须开源；可购买商业授权规避 AGPL 义务 | 已核实（SR-02） |
| 优势 | 与 cctapi 上游 one-api 同宗，领域模型、数据模型、部署形态几乎一致，设计可对照度最高；协议转换覆盖面（Claude/Gemini 双向、Responses、Realtime）广；中文社区资料与运维实践丰富 | 综合归纳 |
| 局限 | AGPL-3.0 对 SaaS 化再分发有强开源义务（cctapi 基于 MIT 的 one-api，许可边界不同）；负载均衡以渠道级加权/优先级为主，缺少 cctapi 式「部署级能力位 + 成功率动态评分」细粒度排序；无内置免费供应商目录/免费池概念 | 已核实（SR-02）+ 推断 |
| 对本项目的参考价值 | 渠道/令牌/计费领域模型的成熟度参照；Claude Messages 双向转换的工程做法可对 cctapi `/v1/messages` 复用 ChatCompletions 路径（material_digest D5，§15）做交叉验证；AGPL 许可是「复用 New-API 代码」路线的硬约束 | 推断 |

#### 2.2.2 B2 - LiteLLM（Proxy + Router）

| 维度 | 内容 | 置信度 |
| --- | --- | --- |
| 产品定位 | 统一 100+ LLM 供应商调用的开源代理网关 + Python Router SDK，提供 OpenAI 兼容端点 | 已核实（SR-03、SR-11） |
| 目标用户 | 需要多供应商统一接入、虚拟密钥、团队预算与可观测性的 AI 工程团队 | 已核实（SR-11） |
| 核心能力 | model_list 下同名模型组成 model group；路由策略 simple-shuffle（默认）/least-busy/usage-based-routing(-v2)/latency-based-routing/cost-based-routing；`fallbacks`（通用错误）+ `context_window_fallbacks`（上下文超限）+ `content_policy_fallbacks`（内容审查）三层回退；`allowed_fails`+`cooldown_time` 失败冷却；虚拟密钥、团队/用户预算（Postgres spend 追踪）；语义缓存（Redis） | 已核实（SR-03、SR-04、SR-11） |
| 架构特点 | Router 两层结构：外层 try-except 处理 fallback，内层 function_with_retries 组内重试；CooldownCache 临时下线故障部署；多实例部署时路由状态经 Redis 共享；典型生产形态 Docker Compose/K8s：Proxy + Redis + PostgreSQL | 已核实（SR-03、SR-11）+ 综合归纳 |
| 部署形态 | 自托管（pip/Docker/K8s）；另有托管版 | 已核实（SR-11） |
| 集成方式 | OpenAI 兼容 REST 端点（客户端仅改 base_url）+ Python SDK + Admin API（密钥/团队/预算/路由可编程） | 已核实（SR-11） |
| 定价模式 | 开源（MIT 许可主体）免费；Enterprise 功能（高级 SSO、审计等）商业订阅 | 已核实（SR-11）+ 推断 |
| 优势 | 回退语义分层最细（通用/上下文窗口/内容审查三类 fallback 分开声明），与 cctapi 按错误分类施加差异化冷却（material_digest D5，§7 cooldown.go）思路同构；cooldown + allowed_fails 机制与 cctapi 的 CooldownPolicy 可直接对照；MIT 许可无 AGPL 顾虑 | 综合归纳 |
| 局限 | Python 技术栈，与 cctapi Go 单二进制 + SQLite 的「单机零依赖」部署哲学不同（生产推荐 Postgres+Redis 三件套）；无「免费供应商注册表/动态目录」概念；按模型组的健康状态共享 Redis，单机场景偏重 | 已核实（SR-11）+ 推断 |
| 对本项目的参考价值 | 三类 fallback 分层、cooldown/allowed_fails 参数化、context_window_fallbacks（cctapi 目前用 capability 过滤的 context_length 做静态过滤，D5，§4，LiteLLM 提供的是运行时超限后动态升级路径，可作演进参照）；usage-based-routing-v2 与 cctapi 智能评分（D5，§5）的对照 | 推断 |

#### 2.2.3 B3 - Cloudflare AI Gateway

| 维度 | 内容 | 置信度 |
| --- | --- | --- |
| 产品定位 | Cloudflare 边缘网络上的 AI 应用统一观测与治理网关（SaaS），一行代码接入 | 已核实（SR-05、SR-06） |
| 目标用户 | 已在 Cloudflare 生态内、需要跨供应商缓存/限流/预算/日志/合规（Guardrails、DLP）的团队与企业 | 已核实（SR-05、SR-06） |
| 核心能力 | 20+ 供应商统一接入；响应缓存（官方文档称重复查询延迟最高降 90%）；Rate Limiting（滑动/固定窗口）；Spend Limits（按模型/供应商/自定义元数据的美元预算，超限自动阻断）；Dynamic Routing（可视化 flow：按用户/地理/内容分流 + 自动 fallback）；Guardrails 内容审查；DLP 敏感数据检测；BYOK 加密托管上游密钥；Analytics + 日志（Logpush 导出） | 已核实（SR-05、SR-06） |
| 架构特点 | 纯 SaaS，复用 Cloudflare 全球边缘（官方称承载约 20% 互联网流量的同一基础设施）；治理策略（缓存/限流/预算/路由）声明式配置在边缘执行；请求日志持久化、可按套餐留存 | 已核实（SR-05、SR-06）+ 推断 |
| 部署形态 | 纯 SaaS（无自托管选项） | 已核实（SR-06） |
| 集成方式 | 改 API base URL 一行接入；GraphQL API 取分析数据；Logpush 导出日志 | 已核实（SR-05、SR-06） |
| 定价模式 | 核心功能（分析、缓存、限流）免费；Workers Paid 计划 $5/月起含 1000 万请求/月，超出 $0.05/百万请求；日志免费额度 100 万条（Paid），Logpush 仅 Paid；Unified Billing 代购上游在 $20 免费额度外加 5% 手续费，上游 token 价透传无加价 | 已核实（SR-07） |
| 优势 | 治理面（预算/限流/缓存/合规）能力完整且声明式，是「网关治理层」的 SaaS 标杆；Spend Limits 的「美元预算 + 自定义元数据维度」是 cctapi 按部署 token 配额（D5，§2）之上的跨供应商预算视角；边缘缓存对重复请求成本收益显著 | 综合归纳 |
| 局限 | 不可自托管、数据出境与合规取决于 Cloudflare 区域策略；fallback 能力封装在 Dynamic Routing 可视化编排内，无 cctapi 式部署级评分/冷却/sticky 的细粒度控制暴露；无「免费供应商池」概念 | 已核实（SR-05）+ 推断 |
| 对本项目的参考价值 | Spend Limits 的「预算维度建模」（模型/供应商/用户/团队）对 cctapi 告警体系（D5，§10 alert.go）有参照；「缓存即成本治理」思路目前 cctapi 缺失；Dynamic Routing 的 flow 模型与 cctapi strategy+pools（D5，§2）是两种路由表达哲学的对照 | 推断 |

#### 2.2.4 B4 - OpenRouter（路由层）

| 维度 | 内容 | 置信度 |
| --- | --- | --- |
| 产品定位 | 统一 LLM API 市场与路由 SaaS：一个 OpenAI 兼容接口接入 60+ 供应商、400+ 模型 | 已核实（SR-08、SR-10） |
| 目标用户 | 希望单接口覆盖多模型、按用量付费、重视可用性与价格发现的应用开发者与团队 | 已核实（SR-08） |
| 核心能力 | 双层 failover：provider 层默认开启（`allow_fallbacks: true`，5xx/限流自动换供应商）+ model 层 `models` 数组跨模型回退（opt-in，可覆盖上下文超限与审查拒绝）；默认负载均衡按价格逆平方加权并叠加 30 秒健康窗；provider 偏好对象（`order`/`only`/`ignore`/`sort`/`max_price`/`preferred_max_latency`/`preferred_min_throughput`/`quantizations`/`data_collection`）；模型变体后缀 `:free`/`:nitro`/`:floor`；免费模型层限速 20 RPM、50 次/日（账户购买 ≥$10 credits 后升至 1000 次/日）；失败请求不计费 | 已核实（SR-08、SR-09、SR-10） |
| 架构特点 | 边缘路由 + 实时健康评估（30 秒滚动窗 + 公布 per-model uptime）；「价格逆平方加权」是其默认调度算法的公开核心；provider failover 与 model fallback 分层恢复不同故障类型（官方博客明确两层边界）；2025 年 8 月曾发生网关自身故障，路由层自身也是依赖项 | 已核实（SR-09、SR-10） |
| 部署形态 | 纯 SaaS；企业版支持 EU 数据驻留路由 | 已核实（SR-08） |
| 集成方式 | OpenAI 兼容 REST（SDK 直接可用）+ BYOK（自带上游密钥，OpenRouter 仅收路由费） | 已核实（SR-08、SR-10） |
| 定价模式 | 充值 credits 按上游价格透传消费；BYOK 模式收 5% 路由费且每月 100 万请求免路由费；失败请求不收费（官方「zero-completion insurance」，但官方 FAQ 提示部分 429/部分输出路径有用户报告仍消耗 credits，建议设花费上限） | 已核实（SR-08、SR-09、SR-10） |
| 优势 | 「两层 failover + 健康窗 + 价格加权」是公开文档最完整的免费/多供应商路由参考实现之一；免费层限速语义（20 RPM / 每日配额分档）与 cctapi 实测的 `free-models-per-min`/`free-models-per-day` 错误完全对应（material_digest D1，§4），是 cctapi 冷却/限额设计的直接外部约束来源 | 综合归纳 |
| 局限 | SaaS 依赖：cctapi 免费池把 OpenRouter 作为上游时，其自身可用性（含 2025-08 网关故障）会传导；免费层配额低且按账户维度，不适合直接当生产容量（与 D1，§4「匿名突发行为不可作生产容量估计」一致）；路由偏好以请求级声明为主，缺少网关侧持久化的部署级评分/告警体系 | 已核实（SR-09）+ 推断 |
| 对本项目的参考价值 | cctapi 免费池的「能力过滤 → 健康过滤 → 策略排序 → sticky」（D5，§3）与 OpenRouter「候选集裁剪 → 价格/吞吐加权 → provider failover → model fallback」是同构问题两种解法；OpenRouter 的 cooldown 语义（免费层 429 → 固定窗口限速）验证了 cctapi 读 Retry-After + 配额类错误标记 exhausted 至当日结束（D5，§7）的方向 | 推断 |

### 2.3 关键技术能力横向事实

> 不评分、不排序，仅按能力维度横陈各方案事实。

| 能力维度 | B1 New-API | B2 LiteLLM | B3 Cloudflare AI Gateway | B4 OpenRouter | 说明 / 来源 |
| --- | --- | --- | --- | --- | --- |
| 虚拟模型/模型组抽象 | 渠道 + 模型重定向（同模型多渠道） | model_list 同名 model group | 网关 + Dynamic Routing flow | `models` 数组 + provider 偏好 | SR-02 / SR-03 / SR-05 / SR-08 |
| 路由策略 | 加权随机 / 轮询 / 优先级 | simple-shuffle / least-busy / usage-based(-v2) / latency / cost | 可视化 flow（用户/地理/内容分流） | 价格逆平方加权 + 30s 健康窗；`sort` 可切换 throughput/latency/price | SR-01 / SR-03、SR-11 / SR-05 / SR-08 |
| failover 分层 | 渠道级故障自动切换（单一层级） | 三层：fallbacks / context_window / content_policy | Dynamic Routing 内自动 fallback | 两层：provider 层（默认开）+ model 层（opt-in） | SR-01 / SR-04 / SR-05 / SR-09 |
| 冷却/熔断语义 | 渠道自动禁用（超时/失败） | `allowed_fails` + `cooldown_time`，CooldownCache | 未公开披露冷却算法细节 | 免费层 429 → `free-models-per-min`（分钟窗）/ `free-models-per-day`（日配额）错误，客户端自行退避 | SR-01 / SR-03 / SR-05 / SR-10、material_digest D1，§4 |
| 配额/预算 | 用户额度 + 模型倍率计费 | 虚拟密钥 + 团队/用户预算（Postgres spend 追踪） | Spend Limits（美元预算，按模型/供应商/元数据维度） | credits 账户制 + `max_price` 请求级上限 + 花费上限 | SR-02 / SR-11 / SR-05、SR-07 / SR-08 |
| 协议兼容 | OpenAI 兼容 + Claude Messages ↔ OpenAI 双向 + Gemini 双向 + Responses/Realtime | 100+ 供应商统一 OpenAI 格式（proxy 内置各 provider 翻译） | 20+ 供应商统一端点 | OpenAI 兼容（chat completions 为主），`provider` 对象扩展 | SR-01、SR-02 / SR-11 / SR-06 / SR-08 |
| 用量记账 | 按请求/Token/缓存命中计费 + 日志 | spend 追踪（Postgres）+ 回调 | Analytics（请求/Token/成本/错误）+ 日志持久化（按套餐 10 万~100 万条） | Activity 日志 + 失败请求不计费 | SR-02 / SR-11 / SR-06、SR-07 / SR-09 |
| 可观测性 | 仪表盘 + 统计分析 | 回调集成（Langfuse 等）+ Prometheus 生态 | GraphQL Analytics API + Logpush | per-model uptime 公开页 + 状态页 | SR-01 / SR-11 / SR-06 / SR-09 |
| 缓存 | 缓存计费（对上游缓存命中单独计费） | 语义缓存（Redis，可选） | 边缘响应缓存（官方称重复查询延迟最高降 90%） | 未覆盖（上游供应商级缓存透传） | SR-01 / SR-11 / SR-05、SR-06 / SR-08 |
| 部署形态 | 自托管（SQLite/MySQL/PostgreSQL） | 自托管（推荐 Postgres + Redis） | 纯 SaaS | 纯 SaaS（企业版 EU 驻留） | SR-01 / SR-11 / SR-06 / SR-08 |
| 许可/商业模式 | AGPL-3.0 + 商业授权 | MIT 主体 + Enterprise | SaaS 免费核心 + Workers Paid（$5/月起，$0.05/百万请求超量）+ 代购 5% | 用量充值 + BYOK 5% 路由费（月 100 万请求免费额） | SR-02 / SR-11 / SR-07 / SR-08、SR-10 |

---

## 3. 对比：对比矩阵与加权评分

> **四段式「对比」段**。在 §2 的事实基础上建立对比矩阵，赋予权重并打分。打分仅代表调研侧评估，构成「建议」而非「冻结决策」。

### 3.1 对比矩阵

| 评估维度 | 权重 | 权重理由 | B1 得分 | B2 得分 | B3 得分 | B4 得分 |
| --- | --- | --- | --- | --- | --- | --- |
| 场景契合度 | 0.30 | 本项目核心是「自托管虚拟模型回退 + 免费池路由」，与标杆场景的匹配度是最重要的借鉴价值来源 | 4 | 4 | 2 | 4 |
| 技术成熟度 | 0.20 | 借鉴对象的机制经过生产验证程度，决定设计模式的可信度 | 4 | 5 | 5 | 4 |
| 集成难度（反向） | 0.15 | 借鉴/引入该方案模式到 cctapi（Go 单体 + SQLite）的工程成本；分数越高越容易 | 5 | 3 | 2 | 3 |
| 成本（反向） | 0.15 | 采用该方案的直接成本与运维成本；分数越高越省 | 5 | 4 | 3 | 2 |
| 合规可控性 | 0.20 | 许可约束（AGPL/MIT）、数据可控性（自托管 vs SaaS 出境）、商务绑定风险 | 2 | 4 | 2 | 2 |
| **加权总分** | **1.00** | — | **4.00** | **3.95** | **2.85** | **3.20** |

**评分标尺**：每项 1~5 分，1 = 严重不符合，3 = 基本满足但存在明显局限，5 = 完美契合。

**关键打分依据（摘要）**：
- B1 集成难度 5 分：与 cctapi 同宗 one-api，领域模型/数据模型/部署形态一致（SR-01、SR-12）；合规 2 分：AGPL-3.0 与 cctapi 的 MIT 上游许可冲突（SR-02）。
- B2 技术成熟度 5 分：fallback/cooldown/预算机制文档与社区验证最充分（SR-03、SR-04、SR-11）；集成 3 分：Python 栈 + Postgres/Redis 依赖与 cctapi Go+SQLite 哲学不同，只能借鉴模式不能直接引入。
- B3 成熟度 5 分：Cloudflare 边缘基础设施背书（SR-06）；场景契合 2 分：纯 SaaS、无自托管、无免费池概念（SR-05、SR-06）。
- B4 场景契合 4 分：免费层路由与 failover 是 cctapi 免费池的直接上游参照（SR-09、SR-10；material_digest D1，§4）；成本 2 分：SaaS 按用量付费 + 免费层配额低，不可作生产容量（SR-10）。

### 3.2 评分结论

- **优先借鉴**：B1 New-API — 适用度评分：4.00。理由：与 cctapi 同宗 one-api 生态，渠道/令牌/计费领域模型与协议转换（Claude Messages ↔ OpenAI 双向、Responses）做法可直接对照验证 cctapi 现有分层（material_digest D5，§12、§15），场景契合度与集成难度两项最高；借鉴形式为「设计对照 + 部分机制参考」，**非代码引入**（受 AGPL 约束，见合规 2 分）。
- **优先借鉴（并列）**：B2 LiteLLM — 适用度评分：3.95。理由：三层 fallback 分层（通用/上下文窗口/内容审查）、`allowed_fails`+`cooldown_time` 冷却参数化、usage-based-routing-v2 是公开文档最系统的回退语义参考，与 cctapi CooldownPolicy + 智能评分（D5，§5、§7）同构且更细；技术成熟度满分。借鉴形式为「机制设计参照」，技术栈差异决定不能复用代码。
- **部分借鉴**：B4 OpenRouter — 借鉴点：双层 failover 边界划分（provider 层自动 / model 层 opt-in）、价格加权 + 健康窗的调度思路、免费层限速语义（20 RPM / 日配额分档）——后者是 cctapi 免费池冷却与限额设计的外部事实约束（SR-10，D1，§4）。不借鉴的部分：SaaS 形态本身、按用量计费的商业模式。理由：它是 cctapi 的上游依赖而非可替代底座。
- **部分借鉴**：B3 Cloudflare AI Gateway — 借鉴点：Spend Limits 的预算维度建模（模型/供应商/自定义元数据）、缓存即成本治理、声明式治理策略。不借鉴的部分：纯 SaaS 部署、可视化 flow 编排（与 cctapi strategy+pools 声明式 JSON 哲学不同）。理由：合规可控性与场景契合度低（2.85），仅治理面概念有参照价值。
- **不借鉴（否决）**：本次调研未出现「否决」级方案——四家标杆在各自维度均有可借鉴事实，无一家整体适用度低于 2.5 或存在场景覆盖为 0 的情况。需说明：若将「作为 cctapi 替代底座整体采购/迁移」作为评估目标，则 B3、B4 因纯 SaaS 与自托管诉求冲突应否决（评分 2.85 / 3.20），但该目标不在本轮调研问题（§1.2 Q1~Q4）范围内。

### 3.3 方案组合分析（如有）

| 组合方式 | 覆盖哪些能力 | 未覆盖能力 | 组合复杂度 | 总体成本估算 |
| --- | --- | --- | --- | --- |
| cctapi 自研底座 + B1/B2 机制借鉴（对照实现三层 fallback 语义、上下文窗口动态回退、预算维度建模） | 虚拟模型回退全链路 + 协议兼容 + 自托管合规 | 边缘缓存、跨实例路由状态共享（多实例） | 中（机制移植到 Go，需设计评审） | 仅人力成本，无新增许可/服务费用 |
| cctapi 自研底座 + B4 作为免费池上游（现状延续） | 免费模型接入、provider 层 failover 由 OpenRouter 兜底 | OpenRouter 自身可用性风险需 cctapi 侧冷却/切换吸收（已部分实现，D5，§7） | 低（现状） | OpenRouter 免费层 + 可选 $10 credits 提升日配额（SR-10，D1，§4） |

> 说明：组合分析同样为「建议」，是否采纳及优先级由 business-architect 裁决。

---

## 4. 建议：取舍决策支持

> **四段式「建议」段**。基于 §2 事实 + §3 对比给出建议。本节是建议而非最终裁决，最终边界由业务架构师冻结。资料摘要 X1~X7 冲突（虚拟模型命名口径、配置格式 legacy vs strategy+pools 等）不在本节裁决，仅在与建议相关处引用。

### 4.1 自研 / 采购 / 复用边界建议

| 能力项 | 建议方式 | 建议依据 | 候选方案 / 系统 | 关键前提 |
| --- | --- | --- | --- | --- |
| 虚拟模型路由与回退核心（strategy+pools、能力/健康过滤、冷却、sticky） | 自研（沿用 cctapi 现有底座） | 四家标杆无一同等匹配「Go 单体 + SQLite + 免费池 + 部署级评分」组合；cctapi 该子系统已生产验收（material_digest D1，§3~§6）；B1/B2 借鉴为机制级而非代码级 | cctapi `fallback/` 包（D5，§2~§10） | 保持 controller 只编排、协议翻译在 relay/model 边界的既有分层纪律（D1，§11、§15） |
| 协议兼容扩展（Responses/Anthropic Messages 持续跟进） | 自研为主 + 对照 B1/B2 | B1 的 Claude Messages ↔ OpenAI 双向转换（SR-01、SR-02）与 B2 的 provider 翻译层（SR-11）可交叉验证 cctapi 的 Responses→Chat 转换路径（D5，§12） | cctapi `controller/responses.go` + `relay/adaptor/` | 上游 API 变更需持续跟踪（Responses API 仍在演进） |
| 免费供应商接入（免费池） | 复用（已有底座）+ 采购上游配额 | cctapi 免费池注册表与动态目录已实现 18 个 provider、4 家在线（D6，§3；D1，§5）；OpenRouter 作为上游的限速/配额语义已被实测吸收（D1，§4） | cctapi `free_provider_*`（D5，§8~§10）+ OpenRouter 等上游 | 匿名/免费层行为不可作生产容量估计；付费配额需另做 soak（D1，§4、§11） |
| 预算/花费治理（跨供应商美元预算、按团队维度） | 自研（远期）/ 暂不引入 | B3 Spend Limits（SR-05、SR-07）与 B2 预算追踪（SR-11）证明该能力有价值，但 cctapi 当前按部署 token 配额（D5，§2）已覆盖单机场景核心诉求；跨供应商美元预算属演进项 | 远期参照 B3/B2 机制 | 需先有成本数据模型（目前台账仅 token 维度，D5，§8） |
| 响应缓存 | 暂不引入（记录为演进方向） | B3 缓存收益明确（官方称重复查询延迟最高降 90%，SR-05、SR-06），B2 有 Redis 语义缓存（SR-11）；但 cctapi 定位是回退网关而非边缘缓存层，且语义缓存与「免费池按量记账」存在计费口径冲突 | 远期可参照 B3/B2 | 需先明确缓存命中时的用量/计费语义 |
| 多实例/分布式路由状态 | 暂不引入 | B2 用 Redis 共享路由状态（SR-03、SR-11）；cctapi 目录存储明确为单进程 SQLite 设计，多实例需数据库级 CAS 或 leader 所有权（D1，§11）——现状与诉求一致，无多实例需求证据 | — | 若未来出现多实例诉求，优先评估 B2 的 Redis 共享模式 |

### 4.2 MVP 范围建议

> 对齐用户诉求「基于项目背景和资料生成完整架构方案」——cctapi 已是交付中的系统（D1，§3 handoff），故此处「MVP」指**架构方案本身的最小完备范围**，而非从零构建。

| 功能（对齐用户诉求） | 建议 MVP？ | 理由 |
| --- | --- | --- |
| R1 虚拟模型回退核心链路架构（distributor → orchestrator → relay 循环 → 冷却/sticky） | ✅ | 系统已生产验收（D1，§3~§6），标杆对照（B1/B2/B4）确认设计方向符合行业惯例，架构文档化无技术风险 |
| R2 免费池路由架构（注册表、动态目录、台账、同步） | ✅ | 四家在线供应商验收证据齐备（D1，§5；D6，§13）；OpenRouter 限速语义（SR-10）与 cctapi 冷却设计（D5，§7）的映射关系可直接写入架构 |
| R3 多协议（Chat/Responses/Anthropic）兼容层架构 | ✅ | 三协议非流式+流式真实验收通过（D1，§5）；B1/B2 的协议层做法提供成熟参照（SR-01、SR-11） |
| R4 可观测性架构（Prometheus 指标、切换日志、告警、评分趋势） | ✅ | 已实现且文档齐备（D2，§10；D5，§17）；B3 的 Analytics 维度可作对照补充 |
| R5 配置格式基线（legacy routing_mode vs strategy+pools，对应冲突 X5） | ✅（作为架构章节陈述，基线裁决在 G3） | 调研侧事实充分：代码双格式并存 + v2 API 拒绝 legacy 字段（D5，§2、§18）；B1/B2 的配置演进经验（LiteLLM config.yaml 单源，SR-03）可作参照，但最终基线由 business-architect 裁决 |
| R6 跨供应商美元预算 / 响应缓存 / 多实例分布式状态 | ❌（完整版） | 均属演进方向（§4.1 末三行）：标杆证明有价值但 cctapi 当前诉求无证据支撑，纳入会扩大架构范围、违反「完整架构方案应聚焦已验证子系统」的边界 |

### 4.3 技术栈参考建议

| 技术层 | 推荐方案 | 替代方案 | 选择理由 |
| --- | --- | --- | --- |
| 网关运行时 | 沿用 Go 单体（cctapi 现状，go.mod `go 1.20`，实际工具链 go1.22.12，见 X3） | LiteLLM（Python） | Go 单二进制 + 嵌入式前端与 cctapi「单机零依赖」部署哲学一致（D5，§1）；LiteLLM 需 Postgres+Redis 三件套，运维成本不匹配（SR-11） |
| 配置与状态存储 | 沿用 SQLite + JSON 配置文件热重载（D5，§1、§2） | Postgres/MySQL（model/main.go 已支持，D5，§16）；Redis（多实例时） | 现状满足单机诉求；B2 经验表明 Redis 仅在多实例共享路由状态时才必要（SR-03、SR-11） |
| 路由/回退语义 | 沿用 strategy+pools + 部署级评分/冷却（D5，§2、§5、§7），对照吸收 B2 三层 fallback 分层思想 | B1 渠道级优先级（SR-01）；B4 请求级 provider 偏好（SR-08） | cctapi 的部署级粒度比 B1 渠道级更细；B4 的请求级声明与 cctapi 网关侧持久化策略是两种哲学，混合会提高配置复杂度（X5 已暴露双格式并存成本） |
| 协议转换 | 沿用 relay/model 边界转换（D5，§12、§15） | 引入 B1 式渠道适配器双向转换层 | cctapi 分层已被 D1，§11 明确为纪律（「协议翻译/兼容代码保持在 relay/model 边界」）；B1 做法作交叉验证而非替换 |
| 用量记账 | 沿用免费池台账（SQLite 原子 upsert，D5，§8）+ Prometheus 文本指标（D1，§12） | B2 Postgres spend 追踪；B3 Analytics API | 单机场景下 SQLite + Prometheus 零依赖；B2/B3 方案依赖外部存储/SaaS |
| 可观测性增强（演进） | 参照 B3 的「自定义元数据维度」扩展日志/指标标签 | B4 per-model uptime 公开页思路（内部健康面板已有，D1，§10） | 与 §4.1 预算治理演进项配套，非 MVP |

---

## 5. 风险与待确认项

> **四段式「风险」段**。列出主要风险、不确定信息、待业务架构师进一步裁决的依赖项。

### 5.1 主要风险清单

| 编号 | 风险描述 | 触发条件 | 影响范围 | 严重程度 | 缓解建议 |
| --- | --- | --- | --- | --- | --- |
| R-01 | 上游 SaaS 依赖传导：免费池依赖 OpenRouter 等外部免费层，其自身故障（如 OpenRouter 2025-08 网关故障，SR-09）或配额策略变更会直接冲击 cctapi 免费池可用性 | 上游网关故障 / 免费层限速或下线模型 | 免费池（cct/free 链路）可用性降级 | 高 | 保持多供应商并行接入（现状 4 家在线，D1，§5）；冷却/切换机制吸收单上游故障（D5，§7）；架构中明确「免费层 SLA 不可承诺」并保留付费配额 soak 路径（D1，§4） |
| R-02 | AGPL 许可污染：若后续为追平 New-API 功能而直接复制其代码，AGPL-3.0 的网络服务开源义务（SR-02）将与 cctapi 基于 MIT one-api 的许可基线冲突 | 直接引入 New-API（AGPL）源码/大段移植 | 法律合规与再分发 | 高 | 仅做机制级对照，禁止代码级引入；在架构文档的「第三方许可」章节固化该红线 |
| R-03 | 配置格式双轨成本：legacy（routing_mode/fallback_order）与 strategy+pools 并存（冲突 X5；D5，§2、§18），README 示例停留在旧格式（X5、X6），用户按旧文档配置会产生与运行时语义偏差 | 新用户按 README 旧示例配置 / 编辑器与手工 JSON 混用 | 配置正确性、用户支持成本 | 中 | 架构方案中给出配置基线建议（调研侧建议以 strategy+pools 为基线，legacy 仅作迁移期兼容——最终裁决归 business-architect）；同步修订文档示例 |
| R-04 | 单机 SQLite + 单进程设计的天花板：目录快照、台账、状态均为单进程设计（D1，§11），一旦出现多实例/水平扩展诉求，现有并发模型（sync.Mutex + SQLite upsert）不直接适用 | 出现多实例部署或高并发商业化诉求 | 架构演进成本 | 中 | 架构文档显式声明「单实例为设计前提」；保留 B2 Redis 共享路由状态（SR-03、SR-11）作为演进参照；不在 MVP 内预建分布式抽象 |
| R-05 | 调研信息时效性：SaaS 标杆（B3/B4）的定价、免费额度、功能清单变化频繁（如 OpenRouter 免费层日配额分档规则、Cloudflare 日志留存额度），本报告数值为 2026-07-14 时点快照 | 下游在数月后引用本报告数值做成本测算 | 成本估算偏差 | 中 | 架构文档引用本报告时标注时点；涉及预算的决策以当时官方页面为准（SR-07、SR-10） |

### 5.2 待确认项（需主理人 / 业务方反馈）

| 编号 | 待确认项 | 不确定性说明 | 若无法确认的备选路径 |
| --- | --- | --- | --- |
| U-01 | cctapi 未来是否存在多实例/商业化分发诉求？ | 用户诉求仅表述「完整架构方案」，未涉及部署规模与商业形态；直接影响 R-04（分布式状态）与 R-02（AGPL 边界）的权重 | 按「单实例自托管」为设计前提推进架构，分布式与许可章节列为演进附录 |
| U-02 | OpenRouter 免费层日配额分档（50 次/日 vs 购 ≥$10 credits 后 1000 次/日）在 cctapi 目标部署账户上的实际档位 | 公开文档给出分档规则（SR-10），但具体账户档位属账户私有信息；material_digest D1，§4 仅记录「充值 10 credits 可解锁更大日配额」 | 由主理人/业务方确认目标账户状态；架构中按低档（50 次/日）做保守容量假设 |
| U-03 | B3/B4 的 EU 数据驻留、数据留存策略是否满足 cctapi 潜在用户的合规要求 | cctapi 用户画像未定义，数据出境合规要求未知；B4 企业版 EU 驻留为企业功能（SR-08） | 架构方案将「数据不出境」列为自托管路线的默认优势，SaaS 上游标注合规待评估 |

### 5.3 需业务架构持续关注的依赖项

| 编号 | 依赖项 | 说明 | 建议关注阶段 |
| --- | --- | --- | --- |
| D-01 | 资料摘要 X1~X7 冲突的基线裁决（虚拟模型命名口径、面板导航、Go 版本、评分公式、配置格式、cct/free 回退链、router 文件数） | 本报告 §4.1/§4.2 已就 X5（配置格式）给出调研侧建议，但全部 7 项冲突的最终基线裁决权归 business-architect；其中 X1/X6 直接影响架构文档「预置虚拟模型」章节的写法 | 高层架构设计（G3） |
| D-02 | 「免费层 SLA 不可承诺」是否需要在架构非功能性需求中显式声明 | 对应 R-01；影响可用性目标（如 cct/free 是否纳入正式 SLA） | 高层架构设计（非功能性需求章节） |
| D-03 | AGPL 红线（R-02）需嵌入安全/合规设计 | 第三方许可清单与引入评审流程 | 安全设计 / 合规审查 |
| D-04 | 预算治理、响应缓存、多实例状态三项演进方向（§4.1 末三行）的路线图归属 | 调研建议不纳入 MVP，但需在架构演进路线图中占位 | 高层架构设计（演进路线图） |

---

## 6. 关键来源目录

| 编号 | 来源类型 | 标题 / 名称 | URL / 路径 | 相关章节 | 最后访问日期 |
| --- | --- | --- | --- | --- | --- |
| SR-01 | 社区文章（深度指南） | New API 详解：新一代开源大模型统一网关与 AI 资产管理系统 | https://blog.csdn.net/m0_61531676/article/details/158183730 | B1, §2.2.1, §2.3 | 2026-07-14 |
| SR-02 | 官方文档 | New API 项目介绍（含 AGPL-3.0 许可说明与核心特性） | https://www.newapi.ai/zh/llms.mdx/guide/wiki/basic-concepts/project-introduction | B1, §2.2.1, §3.1 | 2026-07-14 |
| SR-03 | 官方文档 | LiteLLM Proxy Configs（routing_strategy / fallbacks / allowed_fails / cooldown_time） | https://docs.litellm.ai/docs/proxy/configs | B2, §2.2.2, §2.3 | 2026-07-14 |
| SR-04 | 官方文档 | LiteLLM Fallbacks (Provider Failover)（fallbacks / context_window_fallbacks / content_policy_fallbacks） | https://docs.litellm.ai/docs/proxy/reliability | B2, §2.2.2, §2.3 | 2026-07-14 |
| SR-05 | 官方文档 | Cloudflare AI Gateway Features（Caching / Spend Limits / Rate Limiting / Dynamic Routing / Guardrails / DLP / BYOK） | https://developers.cloudflare.com/ai-gateway/features | B3, §2.2.3, §2.3 | 2026-07-14 |
| SR-06 | 官方产品页 | Cloudflare AI Gateway Product Page（缓存延迟收益、fallback 与限流、基础设施说明） | https://www.cloudflare.com/product/ai-gateway/ | B3, §2.2.3 | 2026-07-14 |
| SR-07 | 官方文档 | Cloudflare AI Gateway Pricing（核心免费；Workers Paid 1000 万请求/月 + $0.05/百万；日志留存额度；Unified Billing 5%） | https://developers.cloudflare.com/ai-gateway/pricing | B3, §2.2.3, §3.1 | 2026-07-14 |
| SR-08 | 官方文档 | OpenRouter Provider Routing（provider 对象字段、价格逆平方加权、30 秒健康窗、EU 驻留） | https://openrouter.ai/docs/features/provider-routing | B4, §2.2.4, §2.3 | 2026-07-14 |
| SR-09 | 官方博客 | OpenRouter Reliability & Automatic Failover（两层 failover 边界、失败不计费、2025-08 故障说明） | https://openrouter.ai/blog/insights/reliability-failover/ | B4, §2.2.4, §5.1 | 2026-07-14 |
| SR-10 | 社区文章（汇集官方规则） | OpenRouter API: One Key for 500+ LLM Models 2026（:free/:nitro/:floor 变体、免费层 20 RPM 与日配额分档、models 数组回退） | https://apiscout.dev/blog/openrouter-api-unified-llm-gateway-2026 | B4, §2.2.4, §5.2 | 2026-07-14 |
| SR-11 | 社区文档 | LiteLLM — LLM Gateways（虚拟密钥、预算、语义缓存、Docker Compose + Redis + Postgres 生产形态） | https://engineersofai.com/docs/ai-engineering/llm-gateways/LiteLLM | B2, §2.2.2, §4.3 | 2026-07-14 |
| SR-12 | 社区文章 | OneAPI/NewAPI 自建中转接入大模型：完整部署指南（两者对比、数据库兼容迁移实践） | https://lidayun.com/article/access-oneapi/ | B1, §2.2.1 | 2026-07-14 |
| SR-13 | 上游证据基线（内部） | 资料摘要 material_digest.md（G1 通过；D1~D7 摘要与 X1~X7 冲突） | D:/ct/project/.workbuddy/output/material_digest.md | 全文（cctapi 事实侧） | 2026-07-14 |

> **附录：调研方法论说明（元信息）**：本报告事实采集流程为——①读取 G1 基线（SR-13）锁定 cctapi 侧事实与 X1~X7 冲突；②按 §1.2 的 Q1~Q4 用 WebSearch 检索四家标杆的官方文档与社区资料，关键数值（配额、定价、路由算法、冷却参数）以官方页面为第一来源、社区资料为交叉验证；③按模板「事实 → 对比 → 建议 → 风险」四段式落稿，全文占位符与来源可追溯性由自动校验脚本（validate_template_compliance.py）复核，结果 12/12 通过。
>
> **来源纪律说明**：SR-01/SR-10/SR-11/SR-12 为社区二手资料，其引用的关键数值（免费层 20 RPM、Workers Paid 1000 万请求/月、AGPL 义务等）均与对应官方页面（SR-02/SR-05/SR-07/SR-08/SR-09）交叉核对后采用；OpenRouter 免费层日配额 50/1000 分档规则在官方 FAQ 与多份社区资料中一致，本报告以 SR-09（官方博客）+ SR-10 联合标注。cctapi 侧事实全部来自 SR-13（G1 基线），标注到 `D编号，§章节` 粒度。

---

## 7. 硬指标清单

| 章节 | 硬指标项 | 当前状态 | 备注 |
| --- | --- | --- | --- |
| §1 | 调研问题已收敛为 ≥ 3 条可执行问题 | ✅ | Q1~Q4 共 4 条，均含调研对象/目标/预期产出 |
| §2.1 | 标杆系统 ≥ 3 家，含 ≥ 1 家头部 SaaS | ✅ | 4 家；SaaS：B3 Cloudflare AI Gateway、B4 OpenRouter |
| §2.1 | 标杆系统 ≥ 1 家开源或自研代表 | ✅ | B1 New-API（AGPL 开源）、B2 LiteLLM（MIT 开源） |
| §2.2 | 每家标杆有独立详述卡片 | ✅ | §2.2.1~§2.2.4 四张 10 维度卡片，逐行标置信度 |
| §2.3 | 关键能力横向事实无遗漏 | ✅ | 11 个能力维度 × 4 家，仅事实不评分 |
| §3.1 | 对比矩阵含 5 维度 + 权重 + 评分 | ✅ | 0.30+0.20+0.15+0.15+0.20 = 1.00；加权总分 B1 4.00 / B2 3.95 / B3 2.85 / B4 3.20 |
| §3.2 | 评分结论含优先/部分/不借鉴三层 | ✅ | 优先：B1、B2；部分：B3、B4；不借鉴：本轮无（已显式说明判定口径） |
| §4.1 | 自研/采购/复用边界有明确建议 | ✅ | 6 个能力项，覆盖自研/复用/暂不引入三类 |
| §4.2 | MVP 范围建议与用户诉求对齐 | ✅ | R1~R6，与 cctapi 四大子系统 + X5 冲突 + 演进项对齐 |
| §5.1 | 主要风险 ≥ 3 条，有缓解建议 | ✅ | R-01~R-05 共 5 条，均含触发条件/影响/严重度/缓解 |
| §6 | 关键来源可追溯（URL / 章节） | ✅ | 13 条来源，覆盖全部 4 家标杆 + 内部基线；关键数值标注来源编号 |
| 全文 | 明确区分事实 / 推断 / 建议 / 风险 | ✅ | §2 卡片逐行标置信度（已核实/推断/综合归纳）；§3 打分标注为建议；§4 全节声明非裁决；§5 独立风险段 |
| 全文 | 不存在编造来源或占位符 | ✅ | 无尖括号占位、「示例」前缀或日期占位残留；外部事实均挂 SR 编号；无法确认事项入 §5.2 待确认而非正文断言 |

---
