# LoadLab

A deliberately simple system designed to fail under load — so you can learn DevOps by watching things break and recovering them.

## Stack

| Tool | Role |
|---|---|
| Docker + Docker Compose | Container orchestration |
| Nginx | Reverse proxy / load balancer |
| Node.js (Express) | Backend API with intentionally expensive endpoints |
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
   ▼
Node.js API :3000
   │
   ├── /health   → fast, always succeeds
   ├── /slow     → 3s artificial delay
   ├── /cpu      → prime number sieve (CPU spike)
   ├── /memory   → allocates 1M objects (memory spike)
   └── /metrics  → Prometheus scrape endpoint

Prometheus :9090  ◄── scrapes /metrics every 5s
   │
   ▼
Grafana :3003

node-exporter :9100  ◄── host OS metrics
```

## Quick Start

### 1. Start the stack

```bash
docker compose up -d --build
```

### 2. Verify services

| Service | URL |
|---|---|
| Backend (direct) | http://localhost:3000/health |
| Backend via Nginx | http://localhost/health |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3003 |

### 3. Configure Grafana

1. Open http://localhost:3003 (default login: `admin` / `admin`)
2. Add a Prometheus data source: `http://prometheus:9090`
3. Import dashboard ID **1860** (Node Exporter Full) for host metrics
4. Build a custom dashboard using these metrics:
   - `http_requests_total` — request rate by route
   - `http_request_duration_seconds` — latency percentiles
   - `process_cpu_seconds_total` — Node.js CPU usage
   - `nodejs_heap_size_used_bytes` — heap memory

### 4. Run load tests

Replace `YOUR_SERVER_IP` in the k6 scripts with `localhost` (local) or your EC2 public IP (remote).

```bash
# Light load — health endpoint only
k6 run k6/health-test.js

# CPU stress — ramps to 50 VUs hitting /cpu
k6 run k6/cpu-test.js

# Full stress — all endpoints, ramps to 300 VUs
k6 run k6/stress-test.js
```

## Endpoints

| Route | What it does | Why it's interesting |
|---|---|---|
| `GET /health` | Returns `{status: "ok"}` | Baseline — should never fail |
| `GET /slow` | Waits 3 seconds | Simulates slow DB / upstream |
| `GET /cpu` | Counts primes to 1,000,000 | Pegs a CPU core |
| `GET /memory` | Allocates 1M objects | Spikes heap usage |
| `GET /metrics` | Prometheus metrics | Scraped every 5s |

## What to observe

- **Latency climb**: Watch `http_request_duration_seconds` p95/p99 rise as VUs increase
- **CPU saturation**: `/cpu` under concurrent load will queue requests — one Node.js thread
- **Memory pressure**: Repeated `/memory` hits will trigger GC pauses visible in heap metrics
- **Error rate**: At high enough VUs, Nginx will start returning 502s — watch `http_req_failed` in k6
- **Recovery**: After the stress test ramps down, watch metrics return to baseline

## Deploying to AWS EC2

1. Launch an EC2 instance (t3.small or larger recommended)
2. Install Docker:
   ```bash
   sudo apt update && sudo apt install -y docker.io docker-compose-plugin
   sudo usermod -aG docker ubuntu
   ```
3. Clone this repo and run `docker compose up -d --build`
4. Open ports 80, 3003, 9090 in your security group
5. Update `YOUR_SERVER_IP` in k6 scripts to your EC2 public IP
6. Run k6 from a separate machine for realistic results

## Teardown

```bash
docker compose down
```
