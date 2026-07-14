# cctapi Kilo 429 模型轮换验证报告

## 验证范围

Kilo 模型级 HTTP 429 自动轮换功能，包含模型运行时注册表、能力感知规划器、请求级轮换执行、中文管理诊断展示，以及后续修复的部署级 `RateLimitScore` 严格 429 门控。

---

## 核心功能验证

### 提交 `c249e3b` — 模型运行时注册表

- 内存级模型冷却状态，支持 `Retry-After`、默认 60 秒冷却、自动过期。
- 并发读写保护（`sync.RWMutex`）。
- 快照深拷贝，防止外部修改内部状态。
- 成功请求重置模型冷却和连续 429 计数。

### 提交 `fb9fd28` — 能力感知模型规划器

- 非 Kilo 部署返回单一不可轮换尝试。
- Kilo 配置模型优先于目录顺序。
- 冷却模型从尝试列表中移除，但 `CompatibleCount` 仍包含它。
- 工具/JSON/视觉/流式/上下文需求过滤不兼容模型。
- 无目录时回退到配置模型。

### 提交 `2def385` — Relay 层模型轮换执行

- 首个 Kilo 模型 429 后切换第二个兼容模型并成功。
- 中间模型 429 不触发供应商冷却、降分或渠道禁用。
- 所有兼容模型 429 后，供应商只记一次失败和冷却。
- 非 429 错误直接跳过剩余 Kilo 模型。
- 已开始输出的流式响应禁止重试。
- 同一 Kilo 部署内部换模型不记录供应商切换事件。
- 手动恢复部署时清理模型级冷却。

### 提交 `1542875` — 管理诊断展示

- Runtime API 增加 `model_runtime` 字段。
- 免费池页面展示冷却模型数量、模型 ID、连续 429、冷却截止时间。
- 中文诊断信息符合 Free Pool 文案规范。

---

## 追加验证：部署级 RateLimitScore 严格 429 门控

**验证提交：** `63235c0` `fix(fallback): gate rate-limit score on HTTP 429`

**CI 运行：** [Run #11](https://github.com/CCT6717/cctapi/actions/runs/29323207674) — `completed/success`，`head_sha` 匹配 `63235c0d0a89021069f63f9539b7c626f96d64a3`

| Job | 状态 | 完成时间 |
|-----|------|----------|
| Go Build & Test | ✅ success | 2026-07-14T09:54:05Z |
| Repository Checks | ✅ success | 2026-07-14T09:51:05Z |
| Frontend Build & Test | ✅ success | 2026-07-14T09:51:43Z |
| E2E Tests | ✅ success | 2026-07-14T09:52:12Z |

### 源码审阅确认

- `isConfirmedHTTPRateLimit` 同时要求错误分类为 `RateLimit` 且 HTTP 状态码为 `429`。
- Kilo 模型轮换、`RecordFailure` 限流计分和跳过剩余模型共用同一判断。
- 真实 429 耗尽模型后仍只增加一次 `RateLimitScore`。
- 非 429 的限流文案只记录普通部署失败，`RateLimitScore` 保持为 `0`。
- `ApplyRelayCooldown` 继续使用既有错误分类，属于保留行为。

### 测试覆盖确认

- **真实 429**：`FailureCount == 1`、`RateLimitScore == 1`。
- **普通 500**：`FailureCount == 1`、`RateLimitScore == 0`。
- **500 且正文含 rate-limit 文案**：`FailureCount == 1`、`RateLimitScore == 0`，不发生模型级轮换或冷却。

### 本地验证（`D:/ct/project`）

```text
go build ./...              PASS
go vet ./...               PASS
go test ./... -count=1     PASS
go test -race ./fallback ./controller ./router -count=1  PASS
npm run lint (web/default) PASS
npm test (web/default)     PASS
npm run build (web/default) PASS
npm run build-storybook (web/default) PASS
```

---

## 最终一致性检查

```text
git log -5 --oneline --decorate
63235c0 (HEAD -> main, origin/main, origin/HEAD, fix/kilo-provider-rate-limit-score) fix(fallback): gate rate-limit score on HTTP 429
25bac32 docs: record Kilo model rotation acceptance
1542875 (feature/kilo-model-rate-limit-rotation) feat(fallback): show Kilo model cooldowns
2def385 feat(fallback): rotate Kilo models after rate limits
b3f84b4 fix(fallback): use provider defaults for Kilo catalog models

git rev-parse main          63235c0d0a89021069f63f9539b7c626f96d64a3
git rev-parse origin/main   63235c0d0a89021069f63f9539b7c626f96d64a3
git diff --check            (no whitespace errors)
git status --short          ?? .workbuddy/  ?? delivery/
```

---

## 闭环结论

- [x] CI #11 对正确 SHA `63235c0` 全绿。
- [x] 报告已创建（原文件不存在）。
- [x] `main` 与 `origin/main` 一致。
- [x] Kilo 429 轮换、非 429 跳过和部署限流计分均有双向回归覆盖。
- [x] 没有未处理的 P0–P2 问题。

**完成时间：** 2026-07-14
