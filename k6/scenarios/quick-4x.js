import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '1m', target: 200 },   // Быстрый разгон
    { duration: '6m', target: 800 },   // Высокая нагрузка для 4 процессоров
    { duration: '1m', target: 0 },     // Быстрое завершение
  ],
  thresholds: {
    http_req_failed: ['rate<0.10'],    // 10% ошибок для быстрого теста
    http_reqs: ['rate>2000'],          // Больше 2000 RPS для 4 процессоров
  },
};

export default function () {
  const payload = JSON.stringify({
    source: `4x-test-${__VU}`,
    data: `Quick test ${__VU}-${__ITER}`,
    metadata: { test: '4x', vu: __VU }
  });

  const response = http.post('http://localhost:8080/api/ingest', payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  check(response, {
    'status ok': (r) => r.status === 200,
    'fast response': (r) => r.timings.duration < 1000,
  });

  sleep(0.05); // Меньше пауза для большей нагрузки
} 