# cctapi

> One API fork — Virtual model fallback gateway

A fork of [songquanpeng/one-api](https://github.com/songquanpeng/one-api) with a **virtual model fallback system**: one model name maps to multiple upstream deployments with automatic failover, weighted/sequential routing, quota tracking, smart sorting, and an admin UI.

---

## How It Works

Define a virtual model (e.g. `high/auto`), configure multiple upstream deployments, and the system automatically manages switching, rate-limiting, and recovery.

```
Client ──→  cctapi  ──→ doubao-code (channel 1)  ──→ success
   │              │
   │              └──→ doubao-18   (channel 2)  ──→ 429 fallback
   │                       │
   │                       └──→ doubao-16   (channel 3)  ──→ success
```

### Defined Virtual Models

| Model | Use Case | Fallback Chain |
|-------|----------|----------------|
| `high/auto` | Coding | doubao-code → doubao-18 → doubao-16 |
| `low/auto` | Chat (free) | openrouter-new-free → openrouter-old → openrouter-new |
| `all/auto` | All channels | doubao chain → openrouter chain |

---

## Features

### Fallback Engine
- **Multi-channel auto failover** — skip unavailable channels, try the next
- **Weighted / sequential routing** — distribute by weight or strict order
- **Smart sorting** — dynamic scoring by success rate + configurable weights
- **Concurrency limits** — per-deployment max concurrent requests
- **Daily quota soft/hard limits** — warn at soft limit, block at hard limit
- **429/503 cooldown** — reads upstream `Retry-After`, exponential backoff
- **Error code blacklist** — configure which errors trigger fallback
- **Hot-reload config** — `POST /api/fallback/config/reload`, no restart

### Admin Panel
- `/fallback/status` — dashboard: deployment states, metrics, scores, alerts, logs, connectivity test
- `/channel` — virtual model editor, manage deployment chains visually

### Observability
- Prometheus metrics (`/metrics`)
- Persistent fallback switch log (reason, status code, duration, request ID)
- Alert history (quota exhausted, cooldown, recovery)
- Score trend chart
- SQLite auto-cleanup (prevent unbounded growth)
- Config backup (before each save)

---

## Quick Start

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

### Manual Build

```bash
# Frontend
cd web/default && npm install && npm run build && cd ../..

# Backend
go build -o one-api.exe .

# Run
./one-api.exe
```

### First Login

- URL: `http://localhost:3007`
- Credentials: `root` / `123456`

---

## Configuration

Virtual model config is stored in `data/fallback.json` (added to `.gitignore`, never committed).
First-time setup:

```bash
cp data/fallback.json.example data/fallback.json
```

### Channel Setup (do this first)

Create two channels in the One API admin panel (`/channel`):

| Channel | Type | Base URL | Model | Get Key |
|---------|------|----------|-------|---------|
| Google Gemini | 55 (Gemini OpenAI Compatible) | `https://generativelanguage.googleapis.com/v1beta/openai/` | `gemini-2.0-flash-exp` | [aistudio.google.com](https://aistudio.google.com/) |
| OpenRouter | 24 | `https://openrouter.ai/api` | `openrouter/free` | [openrouter.ai/keys](https://openrouter.ai/keys) |

Then edit `fallback.json` and set the correct `channel_id` for each deployment.

Edit via `/channel` UI or directly, then hot-reload.

```json
{
  "enabled": true,
  "virtual_models": {
    "cct/free": {
      "enabled": true,
      "description": "Free model pool",
      "routing_mode": "sequential",
      "fallback_order": ["cct/gemini", "cct/openrouter"]
    }
  },
  "deployments": {
    "cct/gemini": {
      "channel_id": 0,
      "real_model": "gemini-2.0-flash-exp",
      "priority": 1,
      "weight": 100,
      "max_concurrent_requests": 10,
      "daily_limit_tokens": 0,
      "quota_mode": "free"
    },
    "cct/openrouter": {
      "channel_id": 0,
      "real_model": "openrouter/free",
      "priority": 2,
      "weight": 100,
      "max_concurrent_requests": 3,
      "daily_limit_tokens": 0,
      "quota_mode": "free"
    }
  }
}
```

### Smart Sort Formula

```
score = base - (priority-1)×5
      + success_rate × 30
      - error_rate × 50
      - 200 (exhausted)
      - 100 (cooling down)
      - 50  (recent error)
```

---

## API Endpoints

| Path | Method | Description |
|------|--------|-------------|
| `/api/fallback/states` | GET | All deployment states |
| `/api/fallback/logs` | GET | Fallback switch logs |
| `/api/fallback/sort/scores` | GET | Smart sort scores |
| `/api/fallback/sort/history` | GET | Score history (chart data) |
| `/api/fallback/alert/status` | GET | Alert status |
| `/api/fallback/alert/history` | GET | Alert history |
| `/api/fallback/deployments/:id/cooldown` | POST | Cool down a deployment |
| `/api/fallback/deployments/:id/recover` | POST | Recover a deployment |
| `/api/fallback/config/reload` | POST | Hot-reload config |
| `/api/editor/config` | GET/POST | Load/save editor config |
| `/metrics` | GET | Prometheus metrics |

---

## Project Structure

```
cctapi/
├── main.go                 Entry point, initializes fallback
├── fallback/               Core fallback package
│   ├── config.go           Config load / validate / hot-reload
│   ├── state.go            DB persistence layer
│   ├── error.go            Error classification
│   ├── sorting.go          Smart sorting
│   ├── weight.go           Weighted round-robin
│   ├── concurrency.go      Concurrency limiter
│   ├── alert.go            Usage alerts
│   ├── cleanup.go          Auto-cleanup
│   └── ...
├── controller/relay.go     relayWithFallback() — main loop
├── middleware/              TokenAuth + Distribute (virtual model intercept)
├── router/                 Routes (fallback split into 3 files)
├── web/default/src/        React frontend (Fallback panel)
└── data/fallback.json      Virtual model config
```

---

*Upstream: [One API](https://github.com/songquanpeng/one-api) by songquanpeng*