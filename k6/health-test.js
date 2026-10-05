import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  vus: 10,
  duration: "1m",
  thresholds: {
    // 95% of requests must complete within 500ms
    http_req_duration: ["p(95)<500"],
    // Error rate must stay below 1%
    http_req_failed: ["rate<0.01"],
  },
};

const BASE_URL = __ENV.BASE_URL || "http://localhost";

export default function () {
  const res = http.get(`${BASE_URL}/api/health`);

  check(res, {
    "status is 200": (r) => r.status === 200,
    "response has status ok": (r) => JSON.parse(r.body).status === "ok",
  });

  sleep(1);
}
