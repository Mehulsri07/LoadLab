const express = require("express");
const client = require("prom-client");

const app = express();

const register = new client.Registry();

client.collectDefaultMetrics({ register });

// --- Metrics ---

const requestCounter = new client.Counter({
  name: "http_requests_total",
  help: "Total number of HTTP requests",
  labelNames: ["route", "status"],
});

const requestDuration = new client.Histogram({
  name: "http_request_duration_seconds",
  help: "Duration of HTTP requests in seconds",
  labelNames: ["route"],
  buckets: [0.05, 0.1, 0.3, 0.5, 1, 2, 5, 10],
});

register.registerMetric(requestCounter);
register.registerMetric(requestDuration);

// --- Middleware: track duration for all routes ---

app.use((req, res, next) => {
  const end = requestDuration.startTimer({ route: req.path });
  res.on("finish", () => {
    requestCounter.inc({ route: req.path, status: res.statusCode });
    end();
  });
  next();
});

// --- Routes ---

// Basic health check — fast, always succeeds
app.get("/health", (req, res) => {
  res.json({ status: "ok", timestamp: new Date().toISOString() });
});

// Simulates a slow upstream dependency (3s delay)
app.get("/slow", async (req, res) => {
  await new Promise((resolve) => setTimeout(resolve, 3000));
  res.json({ endpoint: "slow", delay_ms: 3000 });
});

// CPU-intensive: counts primes up to 1,000,000
app.get("/cpu", (req, res) => {
  let count = 0;
  for (let i = 2; i < 1_000_000; i++) {
    let prime = true;
    for (let j = 2; j * j <= i; j++) {
      if (i % j === 0) {
        prime = false;
        break;
      }
    }
    if (prime) count++;
  }
  res.json({ endpoint: "cpu", primes_found: count });
});

// Memory-intensive: allocates a large array
app.get("/memory", (req, res) => {
  const arr = [];
  for (let i = 0; i < 1_000_000; i++) {
    arr.push({ index: i, value: Math.random() });
  }
  res.json({ endpoint: "memory", allocated: arr.length });
});

// Prometheus metrics scrape endpoint
app.get("/metrics", async (req, res) => {
  res.set("Content-Type", register.contentType);
  res.end(await register.metrics());
});

// --- Start ---

app.listen(3000, () => {
  console.log("LoadLab backend running on port 3000");
});
