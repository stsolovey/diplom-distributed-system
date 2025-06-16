import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '1m', target: 500 },   
    { duration: '5m', target: 1600 },  // 8 процессоров = 1600 VUs
    { duration: '1m', target: 0 },     
  ],
  thresholds: {
    http_req_failed: ['rate<0.20'],    // 20% для экстремальной нагрузки
    http_reqs: ['rate>5000'],          // Больше 5000 RPS для 8 процессоров
  },
};

export default function () {
  const payload = JSON.stringify({
    source: `8x-test-${__VU}`,
    data: `Max load ${__VU}`,
    metadata: { test: '8x' }
  });

  const response = http.post('http://localhost:8080/api/ingest', payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  check(response, { 'ok': (r) => r.status === 200 });
  sleep(0.03);
} 