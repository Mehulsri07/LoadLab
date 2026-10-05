import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 10 },
    { duration: '30s', target: 25 },
    { duration: '30s', target: 50 },
    { duration: '30s', target: 75 },
    { duration: '30s', target: 100 },
    { duration: '30s', target: 0 },
  ],

  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<2000'],
  },
};

// Generic ramp to 100 VUs against one route: k6 run -e ROUTE=cpu k6/load-test.js
const BASE_URL = __ENV.BASE_URL || 'http://localhost';
const ROUTE = __ENV.ROUTE || 'health';

export default function () {
  const res = http.get(`${BASE_URL}/api/${ROUTE}`);

  check(res, {
    'status is 200': (r) => r.status === 200,
  });

  sleep(1);
}
