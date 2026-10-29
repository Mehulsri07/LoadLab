import http from "k6/http";
import { check } from "k6";

// One step of the per-pod capacity test: a fixed arrival rate against /work.
// Driven by scripts/capacity.py, which steps RATE up until the SLO breaks.
export const options = {
  scenarios: {
    step: {
      executor: "constant-arrival-rate",
      rate: Number(__ENV.RATE || 10),
      timeUnit: "1s",
      duration: __ENV.DURATION || "60s",
      preAllocatedVUs: 200,
    },
  },
};

const BASE_URL = __ENV.BASE_URL || "http://localhost:3000";

export default function () {
  const res = http.get(`${BASE_URL}/work`);

  check(res, {
    "status is 200": (r) => r.status === 200,
  });
}

export function handleSummary(data) {
  const out = {
    p95_ms: data.metrics.http_req_duration.values["p(95)"],
    failed_rate: data.metrics.http_req_failed.values.rate,
  };
  return { [__ENV.OUT || "capacity-step.json"]: JSON.stringify(out) };
}
