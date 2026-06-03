import http from 'k6/http';
import { sleep, check } from 'k6';

export const options = {
  vus: 10,
  duration: '30s',
};

export default function () {
  const res = http.get('http://43.204.103.85/api/health');

  check(res, {
    'status is 200': (r) => r.status === 200,
  });

  sleep(1);
}