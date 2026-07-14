# AICoding 架构设计 · 资料摘要

> 本文档做一件事：**精读主理人转交的全部原始资料，逐份、逐章节做出摘要**——后面任何人拿到这份摘要，都能通过章节号快速定位回原始文件的对应位置。

> 上游输入：主理人转交的全部原始资料（Markdown 文档 + Go 源码 + React/JS 前端）；
> 产出者：`knowledge-ingest-engineer`（知识摄入工程师 - 闻资料），经 G1 校验与人工审核通过后交付。

---

## 0. 元信息

```yaml
标题: cctapi（One API CCT 分支 · 虚拟模型回退网关）- 资料摘要 v1.0
版本: v1.0
状态: Draft
创建日期: 2026-07-14
整理人: 闻资料（knowledge-ingest-engineer）
审核人:
  - 齐构成（team-lead / 主理人）

原始资料清单:
  - D:/ct/project/AGENTS.md: 权威项目指引（定位、构建运行、分支、验收证据、UI 结构）
  - D:/ct/project/README.md: 中文项目说明
  - D:/ct/project/README.en.md: 英文项目说明
  - D:/ct/project/go.mod: Go 模块与关键依赖
  - D:/ct/project/（Go 源码树）: main.go、fallback/、controller/、middleware/、relay/relaymode/、model/、router/
  - D:/ct/project/docs/: 设计、运维、验收与证据文档（含 evidence/）
  - D:/ct/project/web/default/: Vite + React 前端（Fallback 面板、fallback-gateway 组件）
```

| 版本 | 日期 | 作者 | 变更内容 |
| --- | --- | --- | --- |
| v1.0 | 2026-07-14 | 闻资料（knowledge-ingest-engineer） | 初稿（重建团队 aicoding-arch-team-v2 从 Phase 1 重跑） |

---

## 1. 资料清单

> 列出全部原始资料，每份标注解析状态。解析失败或跳过的必须注明原因。
> 类型枚举适配：模板默认 `docx` / `pdf` / `pptx` / `xlsx`；本项目资料实际为 `md` / `go` / `ts`（主理人在转交指令中明确允许该适配），前端代码均为 `.js/.jsx`（JavaScript，无 `.ts` 文件）。

| 编号 | 文件名 | 类型 | 来源 | 解析状态 | 说明 |
| --- | --- | --- | --- | --- | --- |
| D1 | `AGENTS.md` | md | 项目维护者 | 已解析 | 优先级最高；权威指引，逐节精读 |
| D2 | `README.md` | md | 项目维护者 | 已解析 | 中文说明，逐节精读 |
| D3 | `README.en.md` | md | 项目维护者 | 已解析 | 英文说明，逐节精读；与 D2 存在差异（见 §3 X1/X2） |
| D4 | `go.mod` | go | 项目维护者 | 已解析 | 模块名与依赖清单全量读取 |
| D5 | Go 源码树（`main.go`、`fallback/`、`controller/`、`middleware/`、`relay/relaymode/`、`model/`、`router/`） | go | 项目维护者 | 已解析 | 重点文件精读（config.go / orchestrator.go / capability.go / free_provider_ledger.go / sorting.go / health.go / cooldown.go / free_provider_sync.go / relay.go / responses.go / distributor.go / rate-limit.go / relaymode/helper.go / model/main.go / router/fallback.go / fallback_gateway.go 等），其余文件按目录结构枚举 |
| D6 | `docs/`（14 个 md + `evidence/` 6 个 JSON + `superpowers/`） | md | 项目维护者 | 已解析 | 重点精读 fallback-real-test-checklist.md / free-pool.md / openrouter-auto-stability-runbook.md；其余按标题区与文件名归类；`evidence/*.json` 为验收输出数据，按 AGENTS.md 引用口径摘要，未逐字段展开；`docs/superpowers/` 含 plans/specs 子目录，未收到精读要求，仅登记 |
| D7 | `web/default/`（Vite + React 前端） | ts | 项目维护者 | 已解析 | 重点精读 `package.json` 与 `pages/Fallback/index.js`；组件按 AGENTS.md 免费池 UI 节列出的重要文件清单 + 目录枚举归类 |

---

## 2. 资料内容摘要

> 逐份文档按自身章节结构做摘要。每条摘要标注章节号（`D编号，§章节`），后面任何人想核实某个点，直接定位回原文对应位置即可。
> 引用方式标注：直接引用 / 数据提取 / 综合归纳 / 推断（推断类标注风险）。Go 源码资料按 `文件:行号` 粒度定位。
> 各表"章节"列的 §N 编号为本文档为每份资料章节顺序赋予的定位编号，与原文标题一一对应。

### D1：`AGENTS.md`

> 面向 AI 编码代理的权威项目指引：项目定位、构建运行、当前交付状态（handoff）、UI 结构约束、PR 检查清单。 — 来源：项目维护者

| 章节 | 内容摘要 |
| --- | --- |
| D1，§1 Project | cctapi 是 `songquanpeng/one-api` 的 CCT 分支，核心改动是虚拟模型回退系统；本地验证地址 `http://127.0.0.1:3008`；工作目录 `D:\ct\project`；当前集成分支 `main`（直接引用） |
| D1，§2 Build And Run | 必须先构建默认前端再构建 Go 二进制（Go 服务端嵌入 `web/build/default` 静态资源）；替换 3008 端口进程需停旧进程、重建、以 `--port 3008` 启动；检查命令：`go build ./...`、`go test ./fallback`、前端 `npm run build`；前端构建当前零 ESLint 警告（直接引用） |
| D1，§3 Current Handoff（总） | 最近验证交付日 2026-07-14；`main` 已推送 `origin/main`；最新 smoke 加固提交 `36a6dc5`；预览标签 `v0.1.0-freellmapi-preview`；前端工具链已从 CRA 迁移到 Vite 6.4.3 + Vitest 3.2.7 + Storybook Vite builder 10.5.0，react-scripts/Webpack 全移除；路由页组件全部 `React.lazy` + `Suspense` 按路由分块；Playwright smoke 含 16 项桌面/移动检查；Web 限流跳过不可变静态资源以避免路由 chunk 被 429；本地预览路由 `http://127.0.0.1:3008/fallback/free-pool`（数据提取） |
| D1，§4 Current Handoff（OpenRouter 验收） | 新增 openrouter/auto 生产 smoke 手册五件套（docs 下 5 个文件 + `scripts/fallback-openrouter-auto-smoke.ps1`）；2026-07-13 定速 soak：50 请求、5.2 秒间隔、无重试，50/50 成功，平均延迟 1737.01 ms、p95 3727.4 ms，观测到 14 个免费响应模型，usage/fallback 增量均 +50；突发探测 14 请求后 OpenRouter 返回 `free-models-per-min`，网关正确施加 60 秒冷却（外部免费层限速，非生产流量模型）；OpenRouter 日免费配额在验收后耗尽（`free-models-per-day`），充值 10 credits 可解锁更大日配额，不得当作本地回归诊断；验收证据 `docs/evidence/openrouter-auto-soak-2026-07-13.json`（数据提取） |
| D1，§5 Current Handoff（多供应商与目录） | 在线免费池已启用 OpenRouter、Kilo、OVH、Pollinations 四个部署：OpenRouter 保留一个存储 key，其余三家 keyless；强制供应商路由从 Kilo 到 Pollinations 通过（非流式+流式真实响应、sticky 复用、清理完整）；真实协议覆盖通过：Chat Completions / Responses / Anthropic Messages 非流式+流式（经 Kilo），嵌套数组参数的结构化工具调用经 Pollinations 通过；健康检查不再把非预期 4xx 标为 healthy，已移除过期的 OVH `Llama-3.1-8B-Instruct` 默认；动态目录刷新完成：Kilo（11 模型）、OVH（14 模型）2026-07-14 同步通过（2 成功 0 失败 0 跳过），目录 HTTP 超时 15 秒，证据 `docs/evidence/provider-catalog-refresh-2026-07-14.json`；手动 Kimi Code 渠道 `#22` 保持启用且在生成的免费池之外（数据提取） |
| D1，§6 Current Handoff（验证结果） | 前端测试：9 套件 36 用例全过（Vitest 3.2.7）；浏览器测试：16 项 Playwright 检查全过（桌面 Chromium + Mobile Chrome）；前端生产构建通过（Vite 6.4.3，ESLint 0 错误 0 警告）；Storybook 构建通过（仅资源体积警告）；本地 3008 端口 HTTP 200；`main` 上全量 Go 测试通过、Go 构建通过、`git diff --check` 无格式错误（仅换行警告）；当前安全构建基线为 Go 1.26.5 或同补丁线更高版本，Windows 需 `D:\ct\tools\w64devkit-1.23.0\bin` 提供 CGO（数据提取） |
| D1，§7 Runtime Files | 不得删除 `.env`、`one-api.db`、运行中的 `one-api.exe`、被进程锁定的日志；可清理旧忽略日志、旧备份二进制（如 `one-api.exe~`）（直接引用） |
| D1，§8 Fallback UI | 自定义 fallback UI 集中在 `/fallback/status` 部署状态面板下；独立 dashboard 快捷卡片已被有意移除，禁止在主 overview/dashboard 重新添加；当前面板导航五个区：Deployment status / Runtime data / Model scoring / Alert records / Switch logs；没有独立"连通测试"面板，连通测试位于 `/fallback/status` 的虚拟模型配置模块内（直接引用） |
| D1，§9 Free Pool UI | FreeLLMAPI/free-model-pool 管理 UI 位于 `/fallback/free-pool`；页面文案保持中文，保留品牌名与技术缩写（RPM、RPD、TPM、TPD、JSON、API key、token）；重要前端文件：`components/fallback-gateway/` 下 FreeModelPool.js / FreeProvidersEditor.js / FreeProviderRow.js / FreePoolWorkflowDashboard.js / freePoolUtils.js / gatewayConfigApi.js，及 `pages/Fallback/Fallback.css`、`pages/Fallback/index.js`；聚焦测试命令覆盖 freePoolUtils / freeProviderDisplay / FreeModelPool / Fallback 页面测试（直接引用+数据提取） |
| D1，§10 Added CCT API Features | 相对上游 One API 的重要新增：虚拟模型（一个模型名映射多个真实部署）；加权/顺序/固定路由；默认主题网关编辑器 `/fallback/gateway` 的"固定"按钮是"首选起始部署"语义（保存 `routing_mode = fallback` + 设置 `preferred_deployment`，运行时上游失败仍回退同 VM 内其他部署）；真后端固定路由仍存在，但不要把编辑器"固定"按钮理解为无回退固定路由；按部署 token 配额、软/硬限额、并发限制；手动冷却/恢复；智能评分趋势图；运行时健康面板（成功率/失败率/冷却数/耗尽数/Top 失败聚合）；告警历史与切换日志；保存前后端校验（含固定路由目标检查）；真实客户端 smoke 脚本；smoke 示例模型为 `high/auto`（直接引用） |
| D1，§11 FreeLLMAPI Integration Notes | cctapi 已有 `OpenAICompatible` 渠道路径，标准 OpenAI 风格上游（如 FreeLLMAPI）多数情况无需重写 relay 核心；协议翻译/兼容代码保持在 relay/model 边界，controller 只做编排；目标特性集：OpenAI 兼容 `/v1/chat/completions` 与 `/v1/models`、Responses 兼容、按需 Anthropic messages 兼容、模型自动路由与 sticky、重试/冷却/熔断、工具调用拯救、时序安全 bearer 或 x-api-key 认证、管理端可见真实供应商健康/用量/同步/运行时错误；已完成的后端对齐：真实健康检查与错误原因、模型同步/状态刷新、401/403 ModelAccess 长冷却 + sticky 失效、sticky 路由、工具调用参数修复、管理端展示真实运行时失败；已完成动态目录：8 MiB 响应上限 + 15 秒超时、快照事务化更新、per-model tools/JSON/vision/streaming/context 元数据应用于路由边界；剩余生产工作：持续生产流量前需用真实凭据/付费配额做定速 soak（匿名突发行为不可作生产容量估计）；目录存储为单进程 SQLite 设计，未来多实例需数据库级 CAS 或 leader 所有权（直接引用+综合归纳） |
| D1，§12 Important Files | 关键文件清单：`fallback/`（回退核心包）、`router/fallback.go`（fallback 管理 API 与内置 HTML 页面）、`controller/relay.go`（主回退 relay 循环）、`common/metrics.go`（Prometheus 文本指标）、`web/default/src/pages/Fallback/`（默认主题 fallback 面板）、`web/default/src/components/fallback-gateway/`（免费池与网关编辑器组件）、`web/default/src/components/FallbackConfigPanel.js`、`web/default/src/components/Footer.js`、`scripts/fallback-smoke.ps1`（真实客户端 smoke 脚本）（直接引用） |
| D1，§13 Smoke Test | 用真实 API token + 虚拟模型跑 `scripts/fallback-smoke.ps1`（环境变量 `CCT_API_BASE_URL`/`CCT_API_TOKEN`/`CCT_API_MODEL`）；脚本检查非流式与流式 `/v1/chat/completions` 及 `/metrics`；禁止在仓库文件中硬编码真实 token（直接引用） |
| D1，§14 CI | `.github/workflows/ci.yml` 包含：前端 install/ESLint/Vitest/Vite 构建/Storybook 构建；全量 Go 测试与二进制构建；仓库空白/冲突检查；Playwright 桌面/移动 E2E（CI 构建的前端 + Go 服务）（直接引用） |
| D1，§15 PR 检查清单 | 合并前必须通过：并发/数据一致性（共享可变状态用 `sync.Mutex`/`RWMutex`/`atomic`；数据库数值累加优先原子 `UPDATE ... SET col = col + ?`；多步 Redis 操作合并为 Lua 脚本消除 TOCTOU；改 `fallback/state.go` 后本地 `go test -race ./fallback -count=5` 必须全过）；错误处理契约（禁止生产代码 `fmt.Println` 输出错误，统一 `common.SysLog`/`SysError` 或结构化 JSON；429/500/503 等必须返回 `{success:false, message:"..."}`；不吞上游错误）；测试与 CI（新增并发/状态代码必须附回归测试；CI 的 `go test -race` 不得跳过；前端改动需过 lint/test/build/Playwright E2E）；分支与提交（conventional commits；功能分支合并后删除远端分支；禁止未经 review 直推 `main`）；结构关注点（controller 只编排；协议翻译放 `relay/model` 边界；不加独立连通测试面板）；Free Pool 文案规范（中文 UI、保留品牌名与缩写）（直接引用） |
| D1，§16 Footer Attribution | 默认页脚须保留上游归属并加 CCT 归属："CCT API is forked by CCT based on One API. One API is built by JustSong and licensed under MIT."；不得移除上游 One API / JustSong / MIT 归属（直接引用） |
| D1，§17 Implementation Notes | fallback 管理功能保持在 `/fallback/status`；默认主题优先用现有模式与 Semantic UI React；网关编辑器中部署归属与 per-VM 模式控制优先从 `fallback_order` 推导，`pools` 仅作兜底；同一 VM 内选择新的 `fixed`/`quota` 部署必须清除已有的同类选择（UI 不得显示多个激活行）；运行时面板 Top 失败模型/渠道当前由切换日志推导，是近似值——精确失败排名需要后端部署尝试事件表或专用健康聚合端点（直接引用） |

### D2：`README.md`

> 中文项目说明：虚拟模型回退概念、功能、快速开始、配置示例、API 端点、项目结构。 — 来源：项目维护者

| 章节 | 内容摘要 |
| --- | --- |
| D2，§1 标题/引言 | cctapi = One API 定制分支——虚拟模型多渠道回退网关；基于 `songquanpeng/one-api` 扩展，核心改动：一个模型名映射多个上游渠道，自动按权重/顺序切换，带额度追踪、智能排序和管理面板（直接引用） |
| D2，§2 虚拟模型回退 | 定义虚拟模型（如 `high/auto`）配置多个真实上游部署，系统自动管理切换/限流/恢复；示例链路：doubao-code 成功 / doubao-18 遇 429 回退 / doubao-16 成功（直接引用） |
| D2，§3 预置的虚拟模型 | 表格仅列出一个：`cct/free`（免费模型池，sequential 路由，回退链 Google Gemini → OpenRouter 兜底）（数据提取；与 D3，§3 的三模型表格不一致，见 §3 X1） |
| D2，§4 功能-回退引擎 | 多渠道自动回退；权重/顺序路由；智能排序（成功率+权重动态打分）；每部署独立并发上限；日额度软/硬限制（软预警、硬强制跳过）；429/503 冷却（读上游 `Retry-After`，指数退避）；错误码黑名单；配置热重载 `POST /api/fallback/config/reload`（直接引用） |
| D2，§5 功能-管理面板 | `/fallback/status` 一站式监控（状态、指标、评分、告警、日志、连通测试）；`/channel` 虚拟模型编辑器；"5 张导航卡片"——模型状态、运行数据、评分趋势、告警记录、切换日志（直接引用） |
| D2，§6 功能-运维 | 历史数据自动清理（防 SQLite 膨胀）；启动 warm-up；Windows 开机自启脚本；烟雾测试脚本；Prometheus 指标；配置备份（保存前自动备份）（直接引用） |
| D2，§7 快速开始 | Docker Compose（端口 3008、挂载 `./data` 与 `./logs`、`TZ=Asia/Shanghai`、`SESSION_SECRET`）；手动构建：前端 `cd web/default && npm install && npm run build`，后端 `go build -o one-api.exe .`；初始登录 `http://localhost:3008`，账号 `root` / `123456`（直接引用） |
| D2，§8 配置 | 配置存于 `data/fallback.json`（已入 `.gitignore`）；首次复制 `data/fallback.json.example`；第一步在 `/channel` 创建两个渠道：Google Gemini（类型 55，Gemini OpenAI 兼容，`gemini-2.0-flash-exp`）与 OpenRouter（类型 24，`openrouter/free`）；第二步经 `/channel` 可视化编辑或直接改 JSON 后热重载；示例配置：`cct/free` 虚拟模型 `routing_mode: sequential`，`fallback_order: ["cct/gemini", "cct/openrouter"]`，部署字段含 channel_id/real_model/priority/weight/max_concurrent_requests/daily_limit_tokens/quota_mode（直接引用；该示例为旧格式，与新 pools/strategy 格式关系见 §3 X5） |
| D2，§9 智能排序评分公式 | `score = base - (priority-1)×5 + success_rate×30 - error_rate×50 - 200(exhausted) - 100(cooling down) - 50(recent error)`（直接引用；代码实现见 D5，§5，软限额项为 -80 且文档未列，见 §3 X4） |
| D2，§10 API 端点 | 表格列出：`GET /api/fallback/states`、`GET /api/fallback/logs`、`GET /api/fallback/sort/scores`、`GET /api/fallback/sort/history`、`GET /api/fallback/alert/status`、`GET /api/fallback/alert/history`、`POST /api/fallback/deployments/:id/cooldown`、`POST /api/fallback/deployments/:id/recover`、`POST /api/fallback/config/reload`、`GET/POST /api/editor/config`、`GET /metrics`（数据提取） |
| D2，§11 项目结构 | 目录树：`main.go`（入口，初始化回退系统）；`fallback/`（config.go 配置加载/验证/热重载、state.go 数据库持久层、error.go 错误分类、sorting.go 智能排序、weight.go 加权轮询、concurrency.go 并发限制、alert.go 用量告警、cleanup.go 历史清理等）；`controller/relay.go`（relayWithFallback 回退循环）；`middleware/`（TokenAuth + Distribute 拦截虚拟模型）；`router/`（回退相关 3 文件——实际 router/ 下 fallback* 文件已超 10 个，见 D5，§17 与 §3 X7）；`web/default/src/`（React 前端含 Fallback 面板）；`data/fallback.json`（直接引用+综合归纳） |
| D2，§12 相关文档 | 指向 `CLAUDE.md`（详细开发指南）与 `docs/WINDOWS_AUTOSTART.md`（Windows 开机自启）（直接引用） |

### D3：`README.en.md`

> 英文项目说明，结构大体对应 D2 但内容存在版本差异。 — 来源：项目维护者

| 章节 | 内容摘要 |
| --- | --- |
| D3，§1 标题/引言 | One API fork——虚拟模型回退网关；自动故障转移、加权/顺序路由、配额追踪、智能排序、管理 UI（直接引用） |
| D3，§2 How It Works | 与 D2，§2 相同的 doubao 三级回退示意（直接引用） |
| D3，§3 Defined Virtual Models | 表格列出三个模型：`high/auto`（Coding，doubao-code → doubao-18 → doubao-16）、`low/auto`（Chat 免费，openrouter-new-free → openrouter-old → openrouter-new）、`all/auto`（全部渠道，doubao 链 → openrouter 链）（数据提取；与 D2，§3 仅 `cct/free` 一行不一致，见 §3 X1） |
| D3，§4 Features-Fallback Engine | 与 D2，§4 等价（多渠道故障转移、加权/顺序、智能排序、并发限制、日配额软/硬限制、429/503 冷却读 `Retry-After`、错误码黑名单、热重载）（直接引用） |
| D3，§5 Features-Admin Panel | `/fallback/status`（部署状态、指标、评分、告警、日志、连通测试）与 `/channel` 编辑器；**未提** D2，§5 中的"5 张导航卡片"（综合归纳；差异见 §3 X2） |
| D3，§6 Features-Observability | Prometheus 指标（`/metrics`）；持久化切换日志（reason、status code、duration、request ID）；告警历史（配额耗尽、冷却、恢复）；评分趋势图；SQLite 自动清理；保存前配置备份（直接引用） |
| D3，§7 Quick Start | 与 D2，§7 相同：Docker Compose、手动构建、初始登录 `root`/`123456`（直接引用） |
| D3，§8 Configuration | 与 D2，§8 相同的 fallback.json 渠道创建与 JSON 示例（Gemini 类型 55 + OpenRouter 类型 24，`routing_mode: sequential`）（直接引用） |
| D3，§9 Smart Sort Formula | 与 D2，§9 相同公式（直接引用） |
| D3，§10 API Endpoints | 与 D2，§10 相同端点表（数据提取） |
| D3，§11 Project Structure | 与 D2，§11 相同的目录树说明（`router/` 注明 "fallback split into 3 files"，见 §3 X7）（直接引用） |

### D4：`go.mod`

> Go 模块定义与依赖清单。 — 来源：项目维护者

| 章节 | 内容摘要 |
| --- | --- |
| D4，§1 module 声明 | 模块名保持上游 `github.com/songquanpeng/one-api`（go.mod:1）；`go 1.20`（go.mod:3）（直接引用；与 D1，§6 当前安全构建基线 Go 1.26.5 的差异见 §3 X3） |
| D4，§2 require（直接依赖） | Web 框架：gin v1.10.0 + gin-contrib（cors/gzip/sessions/static）；数据库：gorm v1.25.10 + sqlite/mysql/postgres 驱动；缓存：go-redis v8.11.5、patrickmn/go-cache；云/模型：aws-sdk-go-v2（bedrockruntime）、google.golang.org/api、cloud.google.com/go/iam；其他：jwt、copier、godotenv、tiktoken-go（token 计数）、gorilla/websocket、validator/v10、x/sync、x/crypto、x/image、testify/goconvey（测试）（数据提取，go.mod:5-35） |
| D4，§3 require（indirect） | 含 sonic/goccy-go-json（JSON 编解码）、mattn/go-sqlite3（SQLite CGO 驱动）、OpenTelemetry 系列、yaml.v3 等（数据提取，go.mod:37-110） |

### D5：Go 源码树

> 后端 Go 源码。重点文件精读、其余按目录枚举。章节定位采用 `文件:行号`。 — 来源：项目维护者

| 章节 | 内容摘要 |
| --- | --- |
| D5，§1 main.go 启动流程 | `main()` 顺序：`common.Init()` → 日志初始化 → 加载 fallback 配置（`FALLBACK_CONFIG_PATH` 环境变量，默认 `data/fallback.json`，失败则禁用 fallback，main.go:36-46）→ Gin release 模式 → `model.InitDB()`/`InitLogDB()` → 初始化 fallback 状态存储（main.go:60-67）→ `SyncFreePoolRuntime()` 同步免费池渠道与部署（main.go:70-74）→ 告警管理器、`WarmUpStickyState()`、`StartHistoryCleanup()`、`StartHealthChecker()`（main.go:77-82）→ 创建 root 账户 → Redis 初始化 → 选项/内存缓存/渠道缓存同步 → `openai.InitTokenEncoders()`、`client.Init()` → `StartFreeSync(nil)` 免费池同步（模型 6h + 额度 15m，须在 client.Init 之后，main.go:136-140）→ i18n → Gin server（RequestId/Language/Logger/session 中间件）→ `router.SetRouter(server, buildFS)` → 按 `PORT`/`--port` 启动（main.go:30-169）；`//go:embed web/build/*` 嵌入前端产物（main.go:27-28）；`loadHealthCheckConfig()` 从环境变量读健康检查配置：默认 disabled、间隔 300s、超时 10s（main.go:173-190）（直接引用+综合归纳） |
| D5，§2 fallback/config.go 配置模型 | 顶层 `Config`：enabled / virtual_models / deployments / free_providers / alert / smart_sort / blocked_error_codes（config.go:32-40）；`VirtualModelConfig`：strategy（quality_first / cost_first / free_first）、pools、allow_degrade_to_low/free，legacy 字段 routing_mode / preferred_deployment / fallback_order / fixed_deployment（config.go:42-55）；`DeploymentConfig`：channel_id、real_model、pool（paid_high/cheap/local/free）、quality_tier（high/medium/low）、cost_tier（free/cheap/paid）、能力位 supports_vision/stream/tools/json、context_length、priority、weight、max_concurrent_requests、daily_limit_tokens、quota_mode、soft/hard_limit_ratio、RPM/RPD/TPM/TPD 限额（config.go:57-81）；策略与路由模式归一化函数（默认 quality_first、默认 fallback 模式，config.go:90-128）；`loadConfigData` 解析+归一化，并检测旧格式配置：fixed 模式合成 `_fixed_{name}` 池、weighted/sequential 合成 `_legacy_{name}` 池（config.go:130-197）；部署默认值：pool=default、quality=medium、cost=paid、weight=100、soft=0.95、hard=1.0（config.go:199-221）（数据提取） |
| D5，§3 fallback/orchestrator.go 请求计划 | `DeploymentPlan` 含候选部署序列、preferred/sticky ID、能力/健康过滤前后计数（orchestrator.go:5-13）；`PrepareDeploymentPlanForRequest` 把虚拟模型查找、能力过滤、健康过滤、策略排序、preferred/sticky 提升合并为一个计划步骤：先 `GetDeploymentsForVirtualModel` → `FilterByCapability` → `FilterHealthyDeployments` → `SortByStrategy`（多候选时）→ 优先 preferred 其次 sticky（orchestrator.go:28-65）；健康过滤放行 healthy/unknown，剔除 invalid/error（orchestrator.go:69-77）（直接引用+综合归纳） |
| D5，§4 fallback/capability.go 能力检测 | `RequestCapabilities`：Vision/Stream/Tools/JSON/MaxTokens（capability.go:10-16）；`DetectRequestCapabilities` 从 OpenAI 兼容请求体检测：stream 标志、消息中 image_url 部分→vision、tools 非空→tools、response_format 为 json_object/json_schema→JSON、max_tokens/max_completion_tokens 累加为 MaxTokens 估计（capability.go:20-59）；`DeploymentSupports` 逐项比对部署能力位，上下文长度仅在双方为正且请求超出时过滤（capability.go:83-101）；`FilterByCapability` 对 `free:*-auto` 类部署走目录快照模型解析（`resolveFreeProviderCatalogModel`，capability.go:104-121）（直接引用+综合归纳） |
| D5，§5 fallback/sorting.go 智能排序 | `ScoreWeights` 默认：base_priority_penalty=5、success_rate_bonus=30、error_rate_penalty=50（sorting.go:21-28）；`CalculateScore`：base=100-(priority-1)×5；success_rate×30 加分；error_rate×50 扣分；exhausted 扣 200；冷却中扣 100；达软限额扣 80；近 5 分钟有错误（非 exhausted）扣 50（sorting.go:33-81 及后续行）（数据提取；与 D2，§9 公式表述差异见 §3 X4） |
| D5，§6 fallback/health.go 健康检查 | 健康状态枚举：healthy / rate_limited / invalid / error / unknown（health.go:21-27）；探测错误体上限 4096 字节、详情上限 500 runes（health.go:29-30）；`HealthCheckConfig`：enabled/interval_seconds/timeout_seconds（health.go:32-36）；`StartHealthChecker` 后台 goroutine 按间隔 ping 每个启用部署，默认间隔 5 分钟、超时 10 秒（health.go:51-76）（直接引用+数据提取） |
| D5，§7 fallback/cooldown.go 冷却策略 | `CooldownPolicy` 默认：分钟窗 429 短冷却 60s、日配额 429→24h、5xx/超时 30s、网关错误（502/503/504 无 retry-after）30s、relay 驱动冷却硬上限 300s（cooldown.go:18-24）；`CalculateRelayCooldownDuration`：优先用上游 Retry-After（上限 300s）；502/503/504 按尝试次数指数退避（60s×2^(n-1)，上限 300s）；其他默认 60s（cooldown.go:33-57）；`ApplyRelayCooldown`：配额类错误标记 exhausted 至当日结束；ModelAccess 类 401/403 标记 invalid（长冷却）（cooldown.go:59-71 及后续行）（数据提取） |
| D5，§8 fallback/free_provider_ledger.go 用量台账 | `FreeProviderUsageLedger` 表：provider + key_hash + model_name + period 为唯一索引（`idx_free_provider_usage_period`），记录 prompt/completion/total tokens、request_count、success_count（free_provider_ledger.go:15-28）；`RecordFreeProviderUsage` 仅对 `free:{provider}-{suffix}` 形态的 auto 部署记账（`parseAutoFreeDeploymentID`，free_provider_ledger.go:65-78）；按 UTC 当日 period 用 `ON CONFLICT` 原子 upsert 累加（free_provider_ledger.go:80-121 及后续行）（数据提取） |
| D5，§9 fallback/free_provider_sync.go 免费池同步 | `buildDesiredFreeProviderResources`：遍历 `free_providers` 配置，未知 provider 警告跳过、disabled 跳过（free_provider_sync.go:31-41）；每个 key 生成期望 channel/deployment：`SafeKeyHash(key)` → deploymentID `free:{provider}-{hash}`；模型列表优先取配置 `models`，其次已持久化目录快照（含 SelectedModel 优先作为 real_model），无快照用占位 `{provider}/free`（free_provider_sync.go:43-66 及后续行）（直接引用+综合归纳） |
| D5，§10 fallback/ 其余文件（枚举） | alert.go / alert_history.go（用量告警与历史）、cleanup.go（历史数据清理）、concurrency.go（并发槽）、error.go（错误分类，含 ErrorCategory：quota / rate_limit / model_access / temporary 等）、events.go（切换事件持久化）、free_pool.go（BuiltinFreeProviders 注册表、SafeKeyHash）、free_provider_catalog*.go（动态目录：抓取、运行时应用、持久化存储、8 MiB 上限/15s 超时）、free_provider_fetch.go（模型拉取）、free_provider_registry.go（616 行，供应商注册）、free_provider_scheduler.go（免费供应商调度）、quota.go（PassQuotaCheck 四维限额预检，220 行）、score_history.go（评分历史）、state.go（数据库持久层，D1，§15 要求改动后 `go test -race ./fallback -count=5`）、weight.go（加权轮询）；测试文件约 16 个（*_test.go，含 state_race_test、integration_test）（综合归纳，来自目录枚举与 D1/D2 交叉印证） |
| D5，§11 controller/relay.go 回退主循环 | `relayWithFallback`（relay.go:100 起）：读 request model → 读 body 一次 → 解析估算 token（`estimateTokenCount`）→ `DetectRequestCapabilities` + `PrepareDeploymentPlanForRequest` → 按候选部署循环：①`IsDeploymentAvailable` 状态过滤（Doubao 部署触达日限额标记 24h 冷却，relay.go:174-198）②渠道存在且启用校验（失败标记 60s 冷却，relay.go:200-232）③四维配额预检 `PassQuotaCheck`（relay.go:234-251）④并发槽 `TryAcquireDeploymentSlot`（relay.go:253-272）⑤非首次尝试写切换日志 + `IncFallbackSwitch`（relay.go:274-279）⑥设置 fallback ctxkey、重置 body、执行 `relayHelper`（按 relaymode 分发到 Text/Image/Audio/Proxy helper，relay.go:43-60）→ 成功：设置 sticky、上报监控、记录用量/成功、返回（relay.go:333-347）→ 失败：`ClassifyRelayError` 分类（单次结构化分类替代 4 次字符串扫描），不可回退错误直接写响应返回；已开始写流的响应不再回退；`monitor.ShouldDisableChannel` 渠道自动禁用跟踪；`ApplyRelayCooldown` 按错误类别施加冷却/exhausted/invalid；Doubao 配额错误额外 24h 冷却（relay.go:349-420）→ 全部失败：`IncFallbackFailed`、清除 sticky、触发 critical 告警（relay.go:422-430 及后续行）；`relayModeRecordsFallbackUsage` 仅 ChatCompletions/Completions/Embeddings/Moderations/Edits 记录 fallback usage（relay.go:62-69）（综合归纳） |
| D5，§12 controller/responses.go Responses 协议转换 | `RelayResponses`（responses.go:149-192）：解析 ResponsesRequest → `ToChatRequest()` 转为 chat completions 请求（不支持的输入返回 422）→ `rewriteResponsesContextForChatRelay` 临时改写 URL path 为 `/v1/chat/completions` 并复用主 relay（含 fallback 链路）→ `responsesCaptureWriter` 捕获内部响应（不直接写客户端）→ 非流式：`ChatCompletionToResponses` 转换后输出；流式：`ChatCompletionStreamToResponsesEvents` 转换 SSE 事件流；上游流无任何有效数据帧时输出 Responses 失败事件（responses.go:194-221）（综合归纳） |
| D5，§13 middleware/distributor.go 虚拟模型拦截 | `Distribute()` 中间件（distributor.go:23-99）：优先处理 `SpecificChannelId` 指定渠道；否则若 fallback 启用且 `fallback.IsVirtualModel(requestModel)` 为真 → 走 fallback 模式：`getFirstUsableFallbackDeployment` 选首个渠道启用的部署（仅用于引导/bootstrap 上下文，distributor.go:101-122）→ 设置 `FallbackEnabled/VirtualModel/DeploymentID/RealModel/FreeProviderName/ChannelID` 等 ctxkey；否则走普通模式 `CacheGetRandomSatisfiedChannel`；`SetupContextForSelectedChannel` 设置渠道类型/ID/名称/SystemPrompt/模型映射/Authorization 头，并把 request-id 与 Idempotency-Key 透传上游（distributor.go:124-173）（综合归纳） |
| D5，§14 middleware/rate-limit.go 限流 | Redis 限流用 Lua 脚本实现滑动窗口（`rateLimitScript`，rate-limit.go:18-51），key 为 `rateLimit:{mark}{clientIP}`；无 Redis 时用内存限流器；`rateLimitFactory` 在 maxRequestNum=0 或 Debug 模式时放行（rate-limit.go:82-99）；`GlobalWebRateLimit` 对静态资源路径跳过限流（`isStaticAsset`，rate-limit.go:101-110）；另有 GlobalAPI/Critical/Download/Upload 五档（rate-limit.go:112-126）（数据提取） |
| D5，§15 relay/relaymode/helper.go 协议路由 | `GetByPath` 按 URL 前缀映射 relay 模式：`/v1/chat/completions`→ChatCompletions；`/v1/responses`→Responses；`/v1/messages`（Anthropic）→ChatCompletions 复用；`/v1/completions`、`/v1/embeddings`（含后缀匹配）、`/v1/moderations`、`/v1/images/generations`、`/v1/edits`、`/v1/audio/speech|transcriptions|translations`、`/v1/oneapi/proxy`→Proxy；未匹配返回 Unknown（helper.go:5-35）（直接引用） |
| D5，§16 model/main.go 数据库初始化 | `chooseDB`：`SQL_DSN` 以 `postgres://` 开头用 PostgreSQL，非空用 MySQL，否则 SQLite（`common.SQLitePath` + busy_timeout，main.go:67-109）；`InitDB` 迁移 Channel/Token/User/Option/Redemption/Ability/Log 表（仅 master 节点，main.go:111-164）；`LOG_SQL_DSN` 可配独立日志库（main.go:166-201）；连接池参数：MaxIdleConns 默认 100、MaxOpenConns 默认 1000、ConnMaxLifetime 默认 60s（main.go:203-218）；`CreateRootAccountIfNeed`：无用户时创建 root/123456，配额 500000000000000，支持 `InitialRootAccessToken`/`InitialRootToken` 环境注入（main.go:24-65）（数据提取） |
| D5，§17 router/ 路由层 | `SetRelayRouter`（router/relay.go:10-39）：`/v1` 组挂 `RelayPanicRecover` + `TokenAuth` + `Distribute`；`/v1/responses` 走 `controller.RelayResponses`，`/v1/messages` 走 `controller.Relay`（Claude 兼容），images/edits/variations/files 部分端点为 RelayNotImplemented；`SetFallbackRouter`（router/fallback.go:17 起）：`/api/fallback` 组挂 `AdminAuth()`，端点含 states、deployments/:id/{reset,clear-exhausted,clear-cooldown,cooldown,recover,health,health-check}、batch-recover/batch-cooldown、config/reload、editor/config（GET/POST）、gateway 子组、manual-config（GET/PUT，排除免费池部署与 cct/free）、free-pool/{sync,usage,cleanup/dry-run}、alert/{status,history,read-all,unread-count,config,deployments/:id/silence|unsilence,silenced}、sort/{scores,history,order/*model,toggle}、summary、logs、virtual-models、deployments/runtime-status（router/fallback.go:20-452）（综合归纳+数据提取） |
| D5，§18 router/fallback_gateway.go 网关配置 v2 | `GET /api/fallback/gateway/config` 返回 v2 网关配置投影（fallback_gateway.go:14-21）；`GET/PUT /api/fallback/manual-config` 读写非免费池配置：PUT 时拒绝 legacy 字段（`containsLegacyFields`，fallback_gateway_projection.go 定义 `fixed_deployment` 为 v1 遗留键）、校验 free providers、深拷贝三个 map 字段避免并发读者看到半合并状态、合并时保留 `cct/free` 虚拟模型与免费池部署（fallback_gateway.go:23-121 及后续行）（综合归纳） |
| D5，§19 router/ 其余文件（枚举） | fallback_config.go（编辑器配置读写，`fallbackEditorConfigPath = "data/fallback.json"`）、fallback_config_service.go、fallback_gateway_service.go（28 行）、fallback_gateway_types.go、fallback_gateway_projection.go（v2 投影与 legacy 字段拒绝）、fallback_html.go（内置 HTML dashboard 页面渲染）、fallback_usage.go（`getFreePoolUsage` 免费池用量查询，支持 provider/key_hash/model/period 过滤）、fallback_test.go / fallback_gateway_test.go / fallback_usage_test.go / relay_test.go；api.go / dashboard.go / main.go / web.go 为上游既有路由（综合归纳，来自目录枚举与文件头部精读） |
| D5，§20 controller/ 与 middleware/ 其余文件（枚举） | controller/：auth/、billing.go、channel-billing.go、channel-test.go、channel.go、group.go、log.go、misc.go、model.go、option.go、redemption.go、token.go、user.go、responses_test.go、channel_scope_test.go（上游既有 + 少量测试）；middleware/：auth.go、cache.go、cors.go、gzip.go、language.go、logger.go、recover.go、request-id.go、turnstile-check.go、utils.go 及对应测试（目录枚举） |
| D5，§21 common/ 与 relay/ 子包（枚举） | common/：metrics.go（Prometheus 指标，D1，§12 点名）、rate-limit.go、redis.go、database.go、freeproviderquirks/（供应商 quirks）、claudeutil/（Claude/OpenAI 错误输出）、ctxkey/（上下文键）、i18n/、client/ 等；relay/：adaptor/（各渠道适配器）、apitype/、billing/、channeltype/（渠道类型常量）、constant/、controller/（relay helper 实现）、meta/、model/（GeneralOpenAIRequest/ResponsesRequest 等模型与转换）（目录枚举） |

### D6：`docs/`

> 设计、运维、验收文档集。重点三篇逐节精读，其余按标题区归类。 — 来源：项目维护者

| 章节 | 内容摘要 |
| --- | --- |
| D6，§1 free-pool.md-概述 | Free Pool 是 cct/free 三层网关的底层实现，自动管理上游免费供应商的 channel 和 deployment，无需手动创建；工作流：`cct/free` 请求 → `IsVirtualModel` → `relayWithFallback()` → `GetDeploymentsForVirtualModel`（pools:["free"] 筛选）→ 按 strategy + capability + health + quota 排序筛选 → 依次尝试，成功 sticky pin（直接引用） |
| D6，§2 free-pool.md-配置结构 | `free_providers` 顶层字段，每 provider 可配：enabled、keys（多 key 生成多 channel/deployment）、models（覆盖默认，首个为 real_model）、default_rpm/rpd/tpm/tpd、limits_override；`limits_override` 为 `*int` 指针语义：缺省/null=不覆盖、0=无限制、正数=覆盖、负数=校验拒绝；合并优先级"内置默认 ← default_* ← limits_override"；`PassQuotaCheck` 中限额为 0 表示跳过检查（不限制）（直接引用） |
| D6，§3 free-pool.md-Provider Registry | 内置 18 个 provider 表：openrouter（渠道类型 19，需 key，128K）、groq（需 key，32K）、kilo（keyless，32K）、pollinations（keyless，静态 `openai-fast`，其 /v1/models 不可用）、ovh（keyless，静态 15 个 chat 模型，262K）、siliconflow（keyless 可用，约 30 模型）、zhipu（keyless 可用，约 8 模型，128K）、mistral/togetherai/novita/cloudflare/cerebras/sambanova/github/chutes/fireworks/nebius/lambdalabs（预置但默认禁用，多需 key）；模型获取方式分动态（fetchModels，6h 刷新）/静态/通用 OpenAI 兼容拉取三类；前 7 个默认启用（数据提取） |
| D6，§4 free-pool.md-Key Hash 命名 | `SafeKeyHash` = SHA256 前 4 字节 hex（8 字符）；channel 名 `[CCT Auto] {provider}-{hash}`，deployment ID `free:{provider}-{hash}`；keyless 供应商用空串 hash；旧格式整数索引（`openrouter-0`）由 `isAutoDeploymentSuffix` 同时识别，迁移期不被误删（直接引用） |
| D6，§5 free-pool.md-路由链路 | 完整链路：distributor 拦截 → relayWithFallback → pools:["free"] 筛选 → 策略排序+能力过滤+健康过滤+限额预检+并发槽 → 改写 model 为 dep.RealModel 转发 → 成功记录 sticky；关键日志标记表（`[fallback] virtual model ... matched deployment`、`[fallback] sticky routing:`、`[free_pool]`、`[health] ping ... failed`）（直接引用） |
| D6，§6 free-pool.md-SyncFreePool | 触发时机：config reload（`validateConfigData` 之前）与手动 `POST /api/fallback/free-pool/sync`；7 步同步逻辑（扫描 `[CCT Auto]%` channel → 计算期望资源 → 创建/更新 → 已删 auto channel 只 disable 不删除（ManuallyDisabled）→ 写入 auto deployment → 移除 stale auto deployment → 保留手动创建的 `free:*` deployment）（直接引用） |
| D6，§7 free-pool.md-健康检查策略 | free deployment 用 `max_tokens=1` 最小 chat 请求做轻量真实探测；状态映射：200→healthy（清除 last_error）、401/403→invalid+长冷却、429→rate_limited+短冷却、5xx/网络错误→error+短冷却、未检查=unknown（允许参与路由）；失败写入 `/api/fallback/deployments/runtime-status` 的 last_error/last_error_at（直接引用） |
| D6，§8 free-pool.md-usage 字段 | 请求 cct/free 时 usage log 的 `model_name` 写虚拟模型名、`real_model_name` 写上游真实模型名（仅 fallback 路径有此区分）（直接引用，引用 relay/controller/helper.go） |
| D6，§9 free-pool.md-FAQ | 加 key 无需重启（reload 即可）；一 provider 多 key 各自生成 channel/deployment 轮流使用；unknown 健康态仍可路由；keyless 供应商 keys 留空数组即可；启用预置禁用供应商需 enabled+key+reload；Cloudflare 用 `account_id:token` 特殊认证且 fetchModels 返回空；所有启用供应商 deployment 混入 `pool:"free"` 统一排序（综合归纳） |
| D6，§10 fallback-real-test-checklist.md | 真实场景测试清单：准备用户 token/管理员 token/虚拟模型名/部署 ID 列表（PowerShell 环境变量示例）；自动化脚本 `scripts/fallback-smoke.ps1`（基础：非流式/流式/metrics；`-RunFaultScenarios` 故障场景：主部署冷却后切换、全部冷却后失败、恢复后成功，结束自动恢复）；手工核对 6 项：非流式、流式、429 切换、额度 95% 软阈值（需调低 daily_limit_tokens 实测）、全部失败（`fallback_failed_total` 增加 + 告警记录）、恢复后请求（手动恢复调用 `POST /api/fallback/deployments/:id/recover`）；验证入口 `/fallback/status`、`/fallback/scores` 及 metrics/states/logs/alert 接口；附测试结论模板（综合归纳） |
| D6，§11 openrouter-auto-stability-runbook.md | openrouter/auto 可复跑验收一体化：一键命令 `scripts/fallback-openrouter-auto-smoke.ps1 -OutputJson`；前置（服务可达、令牌有效、openrouter 已配置）；9 项强制验收（catalog 有 openrouter 且 enabled、config reload 成功、free-pool sync 成功、runtime-status 存在 `free:openrouter-` 记录、非流式 200 有 choices、流式有 SSE、usage 请求数/成功数增量>0、`fallback_requests_total` 正增长、`/fallback/free-pool` 2xx）；`pageContainsOpenRouterAuto` 仅信息字段（未认证 SPA 壳页不一定含路由数据）；OutputJson 关键字段清单；失败排查与最终提交块模板（综合归纳） |
| D6，§12 其余 docs（归类摘要） | API.md（上游 One API 扩展 API 说明：Cookie/Token 鉴权、JSON 响应格式、`/api/user/self`、`/api/topup`）；WINDOWS_AUTOSTART.md（Windows 本地启动与开机自启）；free-pool-analytics-design.md（Free Pool Analytics 可观测性设计 v0.1，2026-06-21，目标是不改后端核心/DB/配置前提下做运行时行为聚合分析；正文提到 fallback 模块管理 `cct/free`、`cct/low`、`cct/high` 三个 virtual model）；free-pool-ops.md（Free Pool 运维手册：从零配置到日常运维）；free-pool-security-audit.md（Native Free Pool 安全审查，2026-06-21，含 SafeKeyHash 可逆性分析，只读审查）；free-pool-troubleshooting.md（cct/free 故障排查 FAQ，如 "no available deployment"）；free-provider-real-smoke.md（真实免费供应商 smoke 检查，含仅元数据模式，强调 key 不得入文档/截图）；freellmapi-route-a.md（FreeLLMAPI Route A：cctapi 原生兼容 FreeLLMAPI 核心工作流，fallback 网关为唯一执行路径，含 `/v1/responses`、流式转换、key 替换/clear_keys、只读用量行、keyless 健康探测）；openrouter-auto-acceptance-checklist.md / openrouter-auto-deployment-acceptance.md / openrouter-auto-final-submission-template.md / openrouter-auto-stability-submission.md（openrouter/auto 验收清单、部署验收门、最终提交模板、稳定性验收提交记录）（综合归纳，来自标题区精读） |
| D6，§13 docs/evidence/ | 6 份验收证据 JSON：`openrouter-auto-soak-2026-07-13.json`（50/50 soak 证据）、`openrouter-auto-2026-07-12.json`、`openrouter-auto-2026-07-13.json`、`openrouter-auto-2026-07-13-vite-review.json`、`freellmapi-multi-provider-acceptance-2026-07-13.json`（多供应商 FreeLLMAPI 验收归档）、`provider-catalog-refresh-2026-07-14.json`（目录刷新证据）；数据指标以 D1，§3~§5 引用口径为准，未逐字段展开（数据提取） |
| D6，§14 docs/superpowers/ | 含 plans/ 与 specs/ 两个子目录；未收到精读要求，仅登记存在（目录枚举） |

### D7：`web/default/`（Vite + React 前端）

> 默认主题前端，已从 CRA 迁移到 Vite。重点精读 package.json 与 Fallback 页面入口。 — 来源：项目维护者

| 章节 | 内容摘要 |
| --- | --- |
| D7，§1 package.json-依赖 | 运行时依赖：react 18.2 + react-dom、react-router-dom v6、antd ^6.5.0、semantic-ui-react ^2.1.3 + semantic-ui-css、axios 1.18.1、recharts ^2.15.1（图表）、lucide-react（图标）、i18next/react-i18next、marked + dompurify（Markdown 渲染）、moment、react-toastify、react-turnstile、react-dropzone、history（数据提取，package.json dependencies） |
| D7，§2 package.json-工具链 | scripts：start=vite、build=`vite build && node scripts/sync-build.cjs`（构建后同步产物）、lint=eslint `--max-warnings=0`（零警告门禁）、test=`vitest run`、test:e2e=`playwright test`、storybook/build-storybook；devDependencies：vite ^6.4.3、vitest ^3.2.7、@vitejs/plugin-react ^4.7.0、@playwright/test ^1.61.1、storybook 10.5.0 + @storybook/react-vite、@testing-library/* 、eslint ^9.39.5、jsdom（数据提取，package.json scripts/devDependencies） |
| D7，§3 pages/Fallback/index.js 页面结构 | Fallback 页面入口（index.js，302 行）：非管理员显示权限警告（index.js:112-122）；状态与数据全部来自 `useFallbackPage()` hook（index.js:76-104）；面板切换 `renderActivePanel` 支持 7 个 key：free-pool（FreeModelPool 组件）、gateway（ModelEditor=FallbackConfigPanel）、metrics（MetricsPanel）、scores（ScoresPanel）、alerts（AlertsPanel）、logs（LogsPanel）、默认 status（StatusPanel）（index.js:124-180）；页面组成：可折叠快速说明卡片（GUIDE_SECTIONS）+ KpiCards + SummaryBar + PageHeader（最后刷新时间、自动刷新间隔、手动刷新按钮）+ 导航卡片网格（PANEL_ITEMS，`Link to=/fallback/${key}`，显示各自刷新间隔徽标）+ 内容面板（index.js:182-299）；图标映射 PANEL_ICON_COMPONENTS/GUIDE_ICON_COMPONENTS 使用 lucide-react（index.js:48-73）；样式 `Fallback.css`（综合归纳） |
| D7，§4 pages/Fallback/ 目录结构 | 子目录：hooks/（useFallbackPage 等）、panels/（SummaryBar/StatusPanel/MetricsPanel/ScoresPanel/AlertsPanel/LogsPanel/KpiCards）、utils/（fallbackHelpers：GUIDE_SECTIONS、PANEL_ITEMS、PANEL_REFRESH_INTERVALS、formatInterval/formatTime）；测试：index.test.js、Fallback.test.jsx（目录枚举+index.js 引用印证） |
| D7，§5 components/fallback-gateway/ 组件 | 按 D1，§9 重要文件清单 + 目录枚举：FreeModelPool.js（免费池主组件，有测试 FreeModelPool.test.jsx）、FreeProvidersEditor.js（供应商编辑）、FreeProviderRow.js（供应商行）、FreePoolWorkflowDashboard.js（工作流仪表盘，有 Storybook stories，fixtures 含 statusTone/statusText 需与 buildFreePoolWorkflowSummary 同步）、freePoolUtils.js + 测试、freeProviderDisplay.js + 测试、gatewayConfigApi.js（网关配置 API 客户端）、DeploymentsEditor.js、VirtualModelsEditor.js（综合归纳） |
| D7，§6 前端整体结构（枚举） | 路由页 pages/：About/Channel/Chat/Dashboard/Fallback/Home/Log/NotFound/Redemption/Setting/Token/TopUp/User；工程文件：vite.config.js、eslint.config.js、playwright.config.js、vitest.setup.js、vercel.json、index.html、scripts/（含 sync-build.cjs）、e2e/、public/、docs/、README.md；所有路由页组件以 React.lazy + Suspense 加载、生产入口按路由分块（D1，§3 交叉印证）（目录枚举） |

---

## 3. 冲突记录

> 不同资料对同一事实描述矛盾时，**并列保留两个版本**，不做裁决。

| 编号 | 冲突主题 | 版本 A | 出处 A | 版本 B | 出处 B | 差异说明 |
| --- | --- | --- | --- | --- | --- | --- |
| X1 | 预置/已定义虚拟模型清单 | 仅 `cct/free`（免费模型池，sequential，Gemini → OpenRouter） | D2，§3 预置的虚拟模型 | 三个：`high/auto`（doubao-code → doubao-18 → doubao-16）、`low/auto`（openrouter 三链）、`all/auto`（doubao 链 → openrouter 链） | D3，§3 Defined Virtual Models | 中英文 README 不同步；D1，§10 以 `high/auto` 作 smoke 示例模型，D6，§12 提到 `cct/free`、`cct/low`、`cct/high` 三个 virtual model，第三口径并存；可能为文档更新时点不同 |
| X2 | Fallback 面板导航构成 | "5 张导航卡片"：模型状态、运行数据、评分趋势、告警记录、切换日志 | D2，§5 功能-管理面板 | 无"导航卡片"表述，仅列 `/fallback/status` 与 `/channel` | D3，§5 Features-Admin Panel | D1，§8 称"独立 dashboard 快捷卡片已被有意移除、禁止重新添加"，且面板导航为五区；前端 index.js 实际渲染 7 项 PANEL_ITEMS 导航卡片（含 gateway、free-pool，见 D7，§3）；文档与代码存在多处口径差异，不做裁决 |
| X3 | Go 版本口径 | `go 1.20`（模块声明） | D4，§1 module 声明（go.mod:3） | 当前安全构建基线为 Go 1.26.5 或同补丁线更高版本；Windows CGO 使用 `D:\ct\tools\w64devkit-1.23.0\bin` | D1，§6 Current Handoff-验证结果 | go.mod 声明的最低语言版本与安全构建工具链版本不同；下游构建必须采用安全基线 |
| X4 | 智能排序评分公式 | `score = base - (priority-1)×5 + success_rate×30 - error_rate×50 - 200(exhausted) - 100(cooling down) - 50(recent error)` | D2，§9 智能排序评分公式；D3，§9 Smart Sort Formula | 代码实现另含"达软限额（soft limit）扣 80"项，且 base 起点为 100（`100 - (priority-1)×5`） | D5，§5 fallback/sorting.go（sorting.go:33-81） | 文档公式未列软限额 -80 项与 base=100 起点；可能为文档滞后于代码 |
| X5 | fallback.json 配置格式 | 示例使用 `routing_mode: sequential` + `fallback_order`（旧格式） | D2，§8 配置；D3，§8 Configuration | 新格式为 `strategy`（quality_first/cost_first/free_first）+ `pools`；旧格式字段标注 Legacy，加载时被转换为合成池（`_fixed_*` / `_legacy_*`） | D5，§2 fallback/config.go（config.go:42-55、130-197）；D6，§2 free-pool.md-配置结构 | README 示例停留在旧格式；代码对旧格式保留兼容转换；v2 网关 API 拒绝 legacy 字段（`fixed_deployment`，D5，§18）；并存口径，不做裁决 |
| X6 | `cct/free` 回退链构成 | Google Gemini → OpenRouter 兜底（`fallback_order: ["cct/gemini", "cct/openrouter"]`） | D2，§3 预置的虚拟模型与 §8 配置示例 | 当前在线免费池为 OpenRouter、Kilo、OVH、Pollinations 四个启用部署（Gemini 不在其中） | D1，§5 Current Handoff-多供应商与目录 | README 示例配置与当前生产实际免费池构成不同；可能为示例未随免费池演进更新 |
| X7 | router 中 fallback 相关文件数 | "回退相关 3 文件" / "fallback split into 3 files" | D2，§11 项目结构；D3，§11 Project Structure | 实际 router/ 下 fallback* 文件 10+（fallback.go / fallback_config.go / fallback_config_service.go / fallback_gateway.go / fallback_gateway_projection.go / fallback_gateway_service.go / fallback_gateway_types.go / fallback_html.go / fallback_usage.go + 3 个测试文件） | D5，§17 router/ 路由层与 §19 router/ 其余文件（目录枚举） | README 结构说明滞后于代码演进 |

---

## 4. 硬指标清单

| 章节 | 硬指标 | 状态 |
| --- | --- | --- |
| §0 | 元信息存在（标题/版本/状态/日期/整理人/审核人/原始资料清单 + 版本变更表） | ✅ |
| §1 | 每份资料有解析状态，失败/跳过注明原因 | ✅（D1~D7 全部已解析；D6 内 evidence JSON 未逐字段展开、superpowers 仅登记，均已在"说明"列注明处理方式） |
| §2 | 每份文档按章节逐条摘要，每条标注了 `D编号，§章节` | ✅（D1 按 17 节、D2 按 12 节、D3 按 11 节、D4 按 3 节、D5 按 21 节（含文件:行号）、D6 按 14 节、D7 按 6 节逐条摘要，章节列均以 `D编号，§N` 标注） |
| §3 | 冲突信息并列保留，不做裁决 | ✅（X1~X7 共 7 条冲突，均并列双版本 + 双出处，差异说明仅陈述可能原因，未做裁决） |
| §4 | 硬指标清单存在 | ✅（本节） |
| 全文 | 无残留模板占位符（尖括号占位、「示例/例」前缀占位、日期占位、待补充标记、数字占位） | ✅（定稿前已全文核查；正文中出现的花括号占位写法如 `free:{provider}-{hash}`、`[CCT Auto] {provider}-{hash}`、`{provider}/free`、`_fixed_{name}`、`_legacy_{name}` 均为对原文命名规则的转写，非模板占位残留） |

---

## 附录 A：生成流程

### 流程总览

| 步骤 | 动作 | 落入章节 |
| --- | --- | --- |
| Step0 | 读取模板 + 全部原始资料（D1~D7 路径清单、来源说明、整理目标来自主理人转交） | — |
| Step1 | 盘点资料清单，标注解析状态（类型枚举按主理人指令适配为 md/go/ts） | §1 |
| Step2 | 逐份打开资料，按自身章节结构逐条摘要（Go 源码定位到 `文件:行号`） | §2 |
| Step3 | 交叉比对不同资料，发现并记录矛盾（X1~X7） | §3 |
| Step4 | 逐项核验硬指标 + 全文占位符核查 + 自动校验脚本自测 | §4 |

```mermaid
flowchart LR
    S0[读取模板与资料] --> S1[盘点资料清单]
    S1 --> S2[逐份精读逐章节摘要]
    S2 --> S3[交叉比对记录冲突]
    S3 --> S4[硬指标自检]
```

### 整理原则

1. **逐份精读，不跨文档归并**：摘要按文档自身章节结构组织，不做跨文档的主题重组（那是下游的事）
2. **出处即章节号**：每条摘要标注 `D编号，§章节`，直接映射回原文位置；源码资料精确到 `文件:行号`
3. **冲突保留**：矛盾信息并列保留两个版本，不擅自裁决
4. **事实驱动**：以原始资料中的事实为准，不添加主观推断；推断类内容在 §2 中标注引用方式与风险

### 本轮执行记录（2026-07-14，重建团队 aicoding-arch-team-v2 重跑）

- 读取角色纪律文件与报告模板，识别全部模板占位标记（尖括号占位、「示例/例」前缀占位、日期占位、待补充标记、数字占位）。
- D1（AGENTS.md，372 行）全量精读；D2/D3 全量精读并交叉比对；D4 全量精读。
- D5：main.go、fallback/config.go（前 220 行）、orchestrator.go、capability.go（前 120 行）、sorting.go（前 80 行）、health.go（前 90 行）、cooldown.go（前 70 行）、free_provider_ledger.go（前 120 行）、free_provider_sync.go（前 70 行）、controller/relay.go（前 430 行）、controller/responses.go（全 244 行）、middleware/distributor.go（全 173 行）、middleware/rate-limit.go（全 126 行）、relay/relaymode/helper.go（全 35 行）、model/main.go（全 237 行）、router/fallback.go（前 120 行 + 端点 grep）、router/fallback_gateway.go（前 120 行）精读；fallback/、controller/、middleware/、relay/、model/、router/、common/ 目录全量枚举；router/relay.go、fallback_config.go、fallback_usage.go、fallback_html.go、fallback_gateway_projection.go 头部抽查。
- D6：free-pool.md（全 339 行）、fallback-real-test-checklist.md（全 181 行）、openrouter-auto-stability-runbook.md（全 92 行）全量精读；其余 11 个文档读取标题区（各前 12 行）归类；evidence/ 6 个 JSON 按文件名 + D1，§3~§5 引用口径登记。
- D7：package.json 全量精读；pages/Fallback/index.js（全 302 行）全量精读；components/fallback-gateway/、pages/Fallback/、pages/ 目录枚举。
- 交叉比对形成 7 条冲突记录（X1~X7），均并列保留未裁决。
- 运行自动校验 `validate_template_compliance.py --output-dir D:/ct/project/.workbuddy/output --filter material_digest.md`，目标 7/7 通过（回传时附实际结果）。

---

## 附录 B：解析 Skill

- `md`：Markdown 类项目指引、说明、设计/运维/验收文档（本项目 D1、D2、D3、D6）——按文档自身标题层级逐节精读
- `go`：Go 源码（本项目 D4 go.mod、D5 源码树）——重点文件按 `文件:行号` 精读，目录其余文件按结构枚举
- `ts`：前端工程（本项目 D7，实际为 Vite + React JavaScript `.js/.jsx`，无 `.ts` 文件；枚举名沿用主理人指令）——精读 package.json 与页面入口，组件按权威指引（D1，§9）列出的重要文件清单归类
- `docx`：Word 类产品/业务文档（本项目无此类资料）
- `pdf`：PDF 类规范、手册、报告（本项目无此类资料）
- `pptx`：PPT 类方案/汇报（本项目无此类资料）
- `xlsx`：Excel 类数据清单、指标表（本项目无此类资料）
