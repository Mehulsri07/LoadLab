import http from "k6/http";
import { check, sleep } from "k6";
import { randomItem } from "https://jslib.k6.io/k6-utils/1.4.0/index.js";

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

const BASE_URL = "http://YOUR_SERVER_IP";

const ENDPOINTS = [
  `${BASE_URL}/health`,
  `${BASE_URL}/memory`,
  `${BASE_URL}/slow`,
  `${BASE_URL}/cpu`,
];

export default function () {
  const url = randomItem(ENDPOINTS);
  const res = http.get(url, { timeout: "35s" });

  check(res, {
    "status is 200": (r) => r.status === 200,
  });

  // No sleep — maximum pressure
}
