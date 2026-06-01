import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  // Ramp up to 50 VUs, hold, then ramp down
  stages: [
    { duration: "30s", target: 10 },
    { duration: "1m",  target: 50 },
    { duration: "30s", target: 0  },
  ],
  thresholds: {
    // CPU endpoint is slow — allow up to 15s at p(95)
    http_req_duration: ["p(95)<15000"],
    http_req_failed: ["rate<0.05"],
  },
};

export default function () {
  const res = http.get("http://YOUR_SERVER_IP/cpu", {
    timeout: "30s",
  });

  check(res, {
    "status is 200": (r) => r.status === 200,
    "has primes_found": (r) => JSON.parse(r.body).primes_found > 0,
  });

  sleep(1);
}
