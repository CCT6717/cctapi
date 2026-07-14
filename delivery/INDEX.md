# cctapi 完整架构方案 · 交付索引（G6）

> 本文档由 AICoding 架构专家团主理人齐构成在 Phase 6 / G6 阶段编写，作为全部架构产物的索引与合稿说明。  
> 交付时间：2026-07-14  
> 下游产物路径：`D:/ct/project/delivery/`

---

## 文档状态与能力状态说明

交付文档采用**双字段状态**：

| 字段 | 取值 | 含义 |
|------|------|------|
| **文档状态** | 草稿 / 已审核 / 已冻结 / 已归档 | 描述文档本身在审核流程中的阶段 |
| **能力状态** | `CURRENT` / `TARGET` / `DEFERRED` / `UNVERIFIED` | 描述文档所描述的系统能力是否已实施 |

| 能力状态 | 含义 | 判定标准 |
|----------|------|----------|
| `CURRENT` | 当前已实现 | 有代码支撑，并已在当前支持环境通过测试、CI 或验收 |
| `TARGET` | 已决定，待实施 | 架构已确定，但代码尚未实施或尚未通过验收测试 |
| `DEFERRED` | 远期演进 | 多实例、商业化等远期能力，不在当前 MVP |
| `UNVERIFIED` | 待验证 | 文档中存在，但尚无代码或运行证据支持 |

> **区分原则**：
> - 文档通过 G3~G6 审核仅代表**文档格式与逻辑完整性通过**，不等于功能已发布。
> - 功能发布需通过独立发布门禁（R1 代码审查 / R2 CI 通过 / R3 真实验收 / R4 证据脱敏确认）。
> - 单实例自托管、SQLite 主存、Kilo 模型级 429 轮换等已验证能力标记为 `CURRENT`。
> - WAF、KMS/Vault、云 VPC 等标记为 `TARGET` 或 `UNVERIFIED`。
> - Redis、多实例、多区域标记为 `DEFERRED`。
> - RPO/RTO、日志保留期等未验收项标记为 `TARGET`。

---

## 1. 交付物清单

| 序号 | 文档 | 路径 | 负责角色 | Gate | 模板校验 | 人工审核 | 文档状态 | 能力状态 |
|------|------|------|----------|------|----------|----------|------|------|
| 1 | 资料摘要 | `delivery/material_digest.md` | 闻资料（knowledge-ingest-engineer） | G1 | 7/7 ✅ | 通过 ✅ | 已归档 | CURRENT |
| 2 | 调研报告 | `delivery/research_report.md` | 查有据（research-analyst） | G2 | 12/12 ✅ | 通过 ✅ | 已归档 | CURRENT |
| 3 | 高层架构设计 | `delivery/高层架构设计.md` | 许边界（business-architect） | G3 | 12/12 ✅ | 通过 ✅ | 已冻结 | CURRENT |
| 4 | 系统设计 | `delivery/系统设计.md` | 高见远（system-architect） | G4 | 11/11 ✅ | 通过 ✅ | 已冻结 | CURRENT |
| 5 | UserStory | `delivery/UserStory.md` | 顾全景（product-story-designer） | G4 | 5/5 ✅ | 通过 ✅ | 已冻结 | CURRENT |
| 6 | 部署设计 | `delivery/部署设计.md` | 毕落地（platform-architect） | G5 | 8/8 ✅ | 通过 ✅ | 已冻结 | TARGET |
| 7 | 安全设计 | `delivery/安全设计.md` | 严守正（security-architect） | G5 | 9/9 ✅ | 通过 ✅ | 已冻结 | TARGET |
| 8 | 部署拓扑图 | `delivery/cctapi-deployment-topology.drawio` | 毕落地 | G5 | — | 通过 ✅ | 已归档 | CURRENT |
| 9 | 交付索引（本文件） | `delivery/INDEX.md` | 齐构成（team-lead） | G6 | — | 待审核 | 已冻结 | CURRENT |

---

## 2. 运行时决策与关键裁决

### 2.1 启动时运行时决策（Phase 0 / G0）

| 决策项 | 取值 | 说明 |
|--------|------|------|
| `need_ingest` | true | 基于 AGENTS.md / README / 源码 / docs 做结构化资料摘要 |
| `need_research` | true | 做行业标杆调研（New-API / LiteLLM / Cloudflare AI Gateway / OpenRouter） |
| `need_cloud_baseline_check` | false | 不强制绑定云厂商现状，按自托管形态描述部署 |
| `U-01` | 单实例自托管 | 分布式状态、多实例/商业化分发列为演进附录，不占 MVP |

### 2.2 G1~G5 阶段产物审核结论

| Gate | 审核结论 | 备注 |
|------|----------|------|
| G1 | 通过 | `material_digest.md` 7/7 校验通过，冲突 X1~X7 并列保留 |
| G2 | 通过 | `research_report.md` 12/12 校验通过，加权结论为建议而非冻结决策 |
| G3 | 通过 | `高层架构设计.md` 12/12 校验通过，X1/X5/X6 等冲突由业务架构师给出基线裁决 |
| G4 | 通过 | `系统设计.md` 11/11 + `UserStory.md` 5/5 校验通过 |
| G5 | 通过 | `部署设计.md` 8/8 + `安全设计.md` 9/9 校验通过，交叉一致性 diff 6 项全部一致 |

---

## 3. 术语统一结果

| 术语 | 统一英文 | 业务定义 | 出现文档 |
|------|----------|----------|----------|
| 虚拟模型 | Virtual Model (VM) | 一个模型名映射到多个真实上游部署，运行时按策略+能力+健康+配额排序并依次尝试 | 全部 |
| 部署 | Deployment | 真实上游渠道 + 真实模型 + 配额/能力/优先级配置 | 全部 |
| 回退 / Fallback | Fallback | 当前部署失败时按策略切换到下一个可用部署 | 全部 |
| 粘性复用 | Sticky Reuse | 请求成功后，后续同类请求优先复用同一部署，直到其失败或冷却 | 全部 |
| 冷却 | Cooldown | 部署失败/配额耗尽后，在一段时间内跳过该部署 | 全部 |
| 免费池 | Free Pool | `cctapi/free` 虚拟模型，自动管理多个免费上游供应商 | 全部 |
| 策略 | Strategy | `quality_first` / `cost_first` / `free_first`，canonical 配置格式（legacy `routing_mode` 仅作兼容） | 高层架构、系统设计、部署设计 |
| 池 | Pools | `paid_high` / `cheap` / `local` / `free`，用于部署分组 | 高层架构、系统设计 |
| 用量记账 | Usage Ledger | 按「供应商 + key 摘要 + 模型 + 日」维度聚合的用量台账 | 系统设计、部署设计 |
| 限流 | Rate Limiting | 全局/单接口/单用户维度的请求速率控制，可选 Redis 后端 | 系统设计、安全设计 |
| 多协议兼容 | Multi-Protocol | 支持 Chat Completions / Responses / Anthropic Messages（后两者通过转 chat 复用回退） | 高层架构、系统设计、UserStory |

---

## 4. 冲突裁决清单（G3 最终基线）

`material_digest.md` 中并列保留的 X1~X7 冲突，由 `business-architect` 在 G3 阶段给出明确基线，下游文档已统一执行：

| 冲突编号 | 冲突主题 | 最终基线（G3 裁决） | 下游执行状态 |
|----------|----------|----------------------|--------------|
| X1 | 预置虚拟模型命名口径 | 以 `cct/free` 为当前实际在线虚拟模型；`high/auto` / `low/auto` / `all/auto` 作为英文 README 历史口径 | 已统一 |
| X2 | Fallback 面板导航构成 | 以前端实际 7 区 PANEL_ITEMS 为准（gateway / free-pool / status / metrics / scores / alerts / logs） | 已统一 |
| X3 | Go 版本口径 | 以实际构建工具链 Go 1.26.5 或同补丁线更高版本 为准；go.mod 声明 `go 1.20` 作为最低语言版本，CI/构建使用 1.26.x | 已统一 |
| X4 | 评分公式 | 以 `sorting.go` 实现为准：base=100，含 exhausted -200 / 冷却 -100 / 软限额 -80 / 近错 -50 | 已统一 |
| X5 | 配置格式 | 以 `strategy + pools` 为 canonical 配置格式；legacy `routing_mode` / `fallback_order` / `fixed_deployment` 仅作迁移期兼容，v2 网关 API 拒绝写入 legacy | 已统一 |
| X6 | cct/free 回退链 | 以当前在线免费池（OpenRouter / Kilo / OVH / Pollinations）为实际基线；README 中 Gemini → OpenRouter 示例为旧文档示例 | 已统一 |
| X7 | router fallback 文件数 | 以实际 10+ 文件为准；README「3 文件」为旧文档口径 | 已统一 |

---

## 5. 文档间引用关系

```
material_digest.md (G1 事实基线)
    ↓
research_report.md (G2 行业调研与建议)
    ↓
高层架构设计.md (G3 业务边界冻结)
    ↓
    ├─ 系统设计.md (G4 系统蓝图)
    │       ├─ 部署设计.md (G5 运行环境)
    │       └─ 安全设计.md (G5 安全控制)
    │
    └─ UserStory.md (G4 用户故事与验收标准)
```

---

## 6. 已知限制与待确认项（不影响 G6 归档）

以下事项已在各自文档中标注为「待人工确认」或「演进附录」，不影响本次 MVP 方案归档：

1. **云厂商选型**：`UNVERIFIED` — 未绑定具体云厂商，资源清单按通用云厂商规格描述，可用等价 IaaS 替换。   
   来源：部署设计 §1.1 / §8.5
2. **Prometheus/Grafana/主机安全/堡垒机**：`TARGET` — 作为复用资源，上线前需运维团队确认余量。   
   来源：部署设计 §2.3 / §5.1
3. **固定公网出口 IP**：`UNVERIFIED` — 是否必须取决于上游供应商白名单要求。   
   来源：部署设计 §3.2.3 / §7.2
4. **Redis 启用**：`DEFERRED` — 由业务侧根据限流需求决定。   
   来源：部署设计 §2.2.4 / 系统设计 §3.1.3
5. **KMS/Vault 演进**：`TARGET` — 本期以环境变量 + 文件挂载为主；演进阶段引入 `kms-cctapi-prod` / `vault-cctapi-prod`。   
   来源：部署设计 §2.2.6 / 安全设计 §6.1
6. **多实例/商业化分发**：`DEFERRED` — U-01 明确列为演进附录，不在 MVP。   
   来源：高层架构设计 §4.3 / 系统设计 §5.5
7. **RPO ≤ 24h / RTO ≤ 30min**：`TARGET` — 已写入文档，尚未通过真实演练验收。   
   来源：部署设计 §3.3.1 / 安全设计 §3.6
8. **日志保留期 ≥ 180 天**：`TARGET` — 已写入文档，尚未通过真实运行验证。   
   来源：部署设计 §2.2.5 / 安全设计 §7.2.4

---

## 7. 版本统一

| 文档 | 版本 | 日期 | 修订人 |
|------|------|------|--------|
| material_digest.md | v1.0 | 2026-07-14 | 闻资料 |
| research_report.md | v1.0 | 2026-07-14 | 查有据 |
| 高层架构设计.md | v1.0 | 2026-07-14 | 许边界 |
| 系统设计.md | v1.0 | 2026-07-14 | 高见远 |
| UserStory.md | v1.0 | 2026-07-14 | 顾全景 |
| 部署设计.md | v1.0 | 2026-07-14 | 毕落地 |
| 安全设计.md | v1.0 | 2026-07-14 | 严守正 |
| INDEX.md（本文件） | v1.0 | 2026-07-14 | 齐构成 |

---

## 8. 主理人合稿结论

- 全部五份主文档（高层架构设计、系统设计、UserStory、部署设计、安全设计）已按模板产出并通过自动校验。
- 行业调研报告与资料摘要作为证据材料已一并归档。
- 术语、版本、引用、冲突裁决已统一。
- 未发现文档间冲突或悬空引用。
- **建议 G6 通过，归档交付**。

---

**decision**: "全量交付方案已统一，可归档"
