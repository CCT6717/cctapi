# cctapi

> One API 定制分支 — 虚拟模型多渠道回退网关

基于 [songquanpeng/one-api](https://github.com/songquanpeng/one-api) 扩展，核心改动是 **虚拟模型回退系统**：一个模型名映射多个上游渠道，自动按权重/顺序切换，带额度追踪、智能排序和管理面板。

---

## 虚拟模型回退

定义一个虚拟模型（如 `high/auto`），配置多个真实上游部署，系统自动管理切换、限流和恢复。

```
客户端 ──→  cctapi  ──→ doubao-code (channel 1)  ──→ 成功
   │              │
   │              └──→ doubao-18   (channel 2)  ──→ 429 回退
   │                       │
   │                       └──→ doubao-16   (channel 3)  ──→ 成功
```

### 当前配置的虚拟模型

| 模型 | 用途 | 回退链 |
|------|------|--------|
| `high/auto` | 编程 | doubao-code → doubao-18 → doubao-16 |
| `low/auto` | 聊天 | openrouter-new-free → openrouter-old → openrouter-new |
| `all/auto` | 全渠道 | doubao 链 → openrouter 链 |
| `core/go` | 预留 | — |

---

## 功能

### 回退引擎
- **多渠道自动回退** — 当前渠道不可用 → 自动切下一个
- **权重路由 / 顺序路由** — 按权重分发或严格按序
- **智能排序** — 成功率 + 权重动态打分，自动调整部署优先级
- **并发限制** — 每个部署可独立设并发上限
- **日额度软/硬限制** — 软限制预警，硬限制强制跳过
- **429/503 冷却** — 读取上游 `Retry-After`，指数退避
- **错误码黑名单** — 可配置哪些错误触发回退
- **配置热重载** — `POST /api/fallback/config/reload`，不重启

### 管理面板
- `/fallback/status` — 一站式监控：状态、指标、评分、告警、日志、连通测试
- `/channel` — 虚拟模型编辑器，可视化管理部署链
- **5 张导航卡片** — 模型状态、运行数据、评分趋势、告警记录、切换日志

### 运维
- 历史数据自动清理（防 SQLite 膨胀）
- 启动 warm-up（避免重启后流量集中）
- Windows 开机自启脚本
- 烟雾测试脚本
- Prometheus 指标
- 配置备份（保存前自动备份旧配置）

---

## 快速开始

### Docker Compose

```yml
services:
  one-api:
    build: .
    ports:
      - "3007:3007"
    volumes:
      - ./data:/data
      - ./logs:/app/logs
    environment:
      - TZ=Asia/Shanghai
      - SESSION_SECRET=your-secret
```

```bash
docker compose up -d
```

### 手动构建

```bash
# 前端
cd web/default && npm install && npm run build && cd ../..

# 后端
go build -o one-api.exe .

# 运行
./one-api.exe
```

### 初始登录

地址：`http://localhost:3007`
账号：`root` / `123456`

---

## 配置

虚拟模型配置存储在 `data/fallback.json`。可通过 `/channel` 页面可视化编辑，或直接改文件后热重载。

```json
{
  "enabled": true,
  "virtual_models": {
    "high/auto": {
      "enabled": true,
      "description": "编程用虚拟模型",
      "routing_mode": "weighted",
      "fallback_order": ["doubao-code", "doubao-18"]
    }
  },
  "deployments": {
    "doubao-code": {
      "channel_id": 1,
      "real_model": "doubao-1.5-pro-256k",
      "priority": 1,
      "weight": 5,
      "daily_limit_tokens": 8000000,
      "soft_limit_ratio": 0.9,
      "hard_limit_ratio": 0.98,
      "max_concurrent_requests": 20
    }
  }
}
```

### 智能排序评分公式

```
score = base - (priority-1)×5
      + success_rate × 30
      - error_rate × 50
      - 200 (exhausted)
      - 100 (cooling down)
      - 50  (recent error)
```

---

## API 端点

| 路径 | 方法 | 说明 |
|------|------|------|
| `/api/fallback/states` | GET | 查看所有部署状态 |
| `/api/fallback/logs` | GET | 回退切换日志 |
| `/api/fallback/sort/scores` | GET | 各模型打分 |
| `/api/fallback/sort/history` | GET | 评分历史趋势 |
| `/api/fallback/alert/status` | GET | 告警状态 |
| `/api/fallback/alert/history` | GET | 告警历史 |
| `/api/fallback/deployments/:id/cooldown` | POST | 手动冷却 |
| `/api/fallback/deployments/:id/recover` | POST | 恢复部署 |
| `/api/fallback/config/reload` | POST | 热重载配置 |
| `/api/editor/config` | GET/POST | 加载/保存编辑器配置 |
| `/metrics` | GET | Prometheus 指标 |

---

## 项目结构

```
cctapi/
├── main.go                 入口，初始化回退系统
├── fallback/               回退核心包
│   ├── config.go           配置加载/验证/热重载
│   ├── state.go            数据库持久层
│   ├── error.go            错误分类
│   ├── sorting.go          智能排序
│   ├── weight.go           加权轮询
│   ├── concurrency.go      并发限制
│   ├── alert.go            用量告警
│   ├── cleanup.go          历史数据清理
│   └── ...
├── controller/relay.go     relayWithFallback() — 回退循环
├── middleware/              TokenAuth + Distribute 拦截虚拟模型
├── router/                  路由（回退相关 3 文件）
├── web/default/src/         React 前端（含 Fallback 面板）
└── data/fallback.json       虚拟模型配置
```

---

## 相关文档

- [CLAUDE.md](./CLAUDE.md) — 详细开发指南
- [docs/WINDOWS_AUTOSTART.md](./docs/WINDOWS_AUTOSTART.md) — Windows 开机自启

---

*上游项目：[One API](https://github.com/songquanpeng/one-api) by songquanpeng*