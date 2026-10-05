import http from "k6/http";
import { check } from "k6";

// Stress test: hammer all endpoints simultaneously to find the breaking point
export const options = {
  stages: [
    { duration: "1m",  target: 20  },  // warm up
    { duration: "2m",  target: 100 },  // ramp to moderate load
    { duration: "2m",  target: 200 },  // push hard
    { duration: "1m",  target: 300 },  // try to break it
    { duration: "2m",  target: 0   },  // recovery — watch metrics recover
  ],
  thresholds: {
    // These will likely fail under stress — that's the point
    http_req_duration: ["p(99)<10000"],
    http_req_failed: ["rate<0.10"],
  },
};

const BASE_URL = __ENV.BASE_URL || "http://localhost";

const ENDPOINTS = [
  `${BASE_URL}/api/health`,
  `${BASE_URL}/api/memory`,
  `${BASE_URL}/api/slow`,
  `${BASE_URL}/api/cpu`,
];

export default function () {
  const url = ENDPOINTS[Math.floor(Math.random() * ENDPOINTS.length)];
  const res = http.get(url, { timeout: "35s" });

  check(res, {
    "status is 200": (r) => r.status === 200,
  });

  // No sleep — maximum pressure
}
