# LoadLab

A deliberately simple system designed to fail under load — so you can learn DevOps by watching things break and recovering them.

## Stack

| Tool | Role |
|---|---|
| Docker + Docker Compose | Container orchestration |
| Nginx | Reverse proxy, static file server, sub-path routing |
| Go (net/http) | Backend API with intentionally expensive endpoints |
| Prometheus | Metrics collection |
| Grafana | Metrics visualization |
| node-exporter | Host-level metrics (CPU, memory, disk) |
| k6 | Load generation |

## Architecture

```
Internet
   │
   ▼
Nginx :80
   │
   ├── /           → frontend dashboard (static HTML)
   ├── /api/*      → Go backend :3000
   ├── /grafana/   → Grafana :3000  (proxied, no extra port needed)
   └── /prometheus/→ Prometheus :9090 (proxied, no extra port needed)

Go API :3000
   ├── /health     → fast health check
   ├── /slow       → 3s artificial delay
   ├── /cpu        → counts primes by trial division (CPU spike)
   ├── /memory     → allocates 1M objects (memory spike)
   ├── /info       → runtime info (Go version, uptime, environment)
   ├── /status     → internal health probe for Prometheus + Grafana
   └── /metrics    → Prometheus scrape endpoint

Prometheus :9090  ◄── scrapes /metrics + node-exporter every 5s
   │
   ▼
Grafana :3000

node-exporter :9100  ◄── host OS metrics
```

## Frontend Dashboard

A single-page observability dashboard served by Nginx at `/`.

Features:
- **System Status** — live health checks for Backend API, Prometheus, and Grafana with response times. Auto-refreshes every 30s.
- **Endpoint Testing** — execute any backend endpoint directly from the browser. Response displayed inline below each button and in the console panel.
- **System Output** — terminal-style console with timestamped, colour-coded log output.
- **Monitoring** — one-click links to Grafana and Prometheus routed through Nginx on port 80.
- **System Information** — populated live from `GET /api/info` (Go version, uptime, environment, server time).

## Quick Start

### 1. Start the stack

```bash
docker compose up -d --build
```

### 2. Verify services

| Service | URL |
|---|---|
| Dashboard | http://localhost/ |
| Backend API | http://localhost/api/health |
| Prometheus | http://localhost/prometheus/ |
| Grafana | http://localhost/grafana/ |

### 3. Configure Grafana

1. Open http://localhost/grafana/ (default login: `admin` / `admin`)
2. Add a Prometheus data source: `http://prometheus:9090`
3. Import dashboard ID **1860** (Node Exporter Full) for host metrics
4. Build a custom dashboard using these metrics:
   - `http_requests_total` — request rate by route
   - `http_request_duration_seconds` — latency percentiles
   - `process_cpu_seconds_total` — Go process CPU
   - `go_memstats_alloc_bytes` — Go heap memory
   - `go_goroutines` — active goroutines

### 4. Run load tests

Scripts target `http://localhost` by default. Point them at a remote host with `-e BASE_URL=http://<EC2 public IP>`.

```bash
# Light load — health endpoint only
k6 run k6/health-test.js

# CPU stress — ramps to 50 VUs hitting /cpu
k6 run k6/cpu-test.js

# Full stress — all endpoints, ramps to 300 VUs
k6 run k6/stress-test.js

# Generic ramp to 100 VUs against one route (health, cpu, memory, slow)
k6 run -e ROUTE=memory k6/load-test.js
```

## Backend API

All routes are proxied through Nginx at `/api/*`.

| Route | What it does | Why it's interesting |
|---|---|---|
| `GET /health` | `{status:"ok", timestamp}` | Baseline — should never fail |
| `GET /slow` | 3 second delay | Simulates slow DB / upstream dependency |
| `GET /cpu` | Counts primes to 1,000,000 | Pegs a CPU core, visible in Grafana |
| `GET /memory` | Allocates 1M structs | Spikes Go heap, triggers GC |
| `GET /info` | Runtime metadata | Go version, uptime, environment |
| `GET /status` | Pings Prometheus + Grafana | Used by dashboard status cards |
| `GET /metrics` | Prometheus text format | Scraped every 5s |

## Environment Variables

| Variable | Service | Effect |
|---|---|---|
| `APP_ENV` | backend | Returned in `/info` response. Set to `production` in docker-compose. |
| `GF_AUTH_ANONYMOUS_ENABLED` | grafana | Allows read-only access without login |
| `GF_SERVER_ROOT_URL` | grafana | Required for sub-path proxying via Nginx. Hard-coded to the EC2 IP in docker-compose — change it (and `GF_SERVER_DOMAIN`) to your own host. |
| `GF_SERVER_SERVE_FROM_SUB_PATH` | grafana | Tells Grafana it lives at `/grafana/` |

## What to observe

- **Latency climb** — watch `http_request_duration_seconds` p95/p99 rise as VUs increase
- **CPU saturation** — `/cpu` under concurrent load; watch `go_goroutines` spike as requests queue
- **Memory pressure** — repeated `/memory` hits spike `go_memstats_alloc_bytes` and trigger GC pauses
- **Error rate** — at high enough VUs, Nginx returns 502s; watch `http_req_failed` in k6
- **Recovery** — after the stress test ramps down, watch all metrics return to baseline

## Deploying to AWS EC2

1. Launch an EC2 instance (t3.small or larger recommended)
2. Install Docker:
   ```bash
   sudo apt update && sudo apt install -y docker.io docker-compose-plugin
   sudo usermod -aG docker ubuntu
   ```
3. Clone this repo:
   ```bash
   git clone https://github.com/Mehulsri07/LoadLab.git
   cd LoadLab
   ```
4. Open **port 80 only** in your EC2 security group — Grafana and Prometheus are proxied through Nginx, no extra ports needed
5. Start the stack:
   ```bash
   docker compose up -d --build
   ```
6. Run k6 from a separate machine with `-e BASE_URL=http://<EC2 public IP>` for realistic results

## Updating a running deployment

```bash
# On your local machine
git add .
git commit -m "your message"
git push origin main

# On the EC2 server
git pull origin main
docker compose up -d --build   # only if backend code changed
docker compose up -d           # config/frontend changes only
```

Grafana dashboards and data persist across redeployments via the `grafana-data` Docker volume. `docker compose down` does not remove volumes. To wipe everything including data:

```bash
docker compose down -v
```

## Teardown

```bash
# Stop containers, keep data volumes
docker compose down

# Stop containers and delete all data
docker compose down -v
```
