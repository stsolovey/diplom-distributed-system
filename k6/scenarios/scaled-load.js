import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '2m', target: 100 },   // Плавный разгон
    { duration: '3m', target: 400 },   // Увеличение до целевой нагрузки
    { duration: '10m', target: 400 },  // Стабильная нагрузка
    { duration: '2m', target: 0 },     // Плавное завершение
  ],
  thresholds: {
    http_req_failed: ['rate<0.05'],    // Менее 5% ошибок
    http_req_duration: ['p(95)<100'],  // P95 < 100ms
    http_req_duration: ['p(99)<200'],  // P99 < 200ms
    http_reqs: ['rate>1000'],          // Больше 1000 RPS (цель для 2 процессоров)
    checks: ['rate>0.95'],             // 95% проверок должны пройти
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  // Генерируем простые тестовые данные
  const payload = JSON.stringify({
    source: `scaled-test-${__VU}`,
    data: `Load test data from VU ${__VU} at ${Date.now()}`,
    metadata: {
      test_type: 'scaled_load',
      vu_id: __VU,
      iteration: __ITER,
      timestamp: Date.now()
    }
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  // Отправляем запрос через API Gateway
  const response = http.post(`${BASE_URL}/api/ingest`, payload, params);

  // Проверяем ответ
  const result = check(response, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
    'has success response': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.status === 'success' || body.message === 'Data ingested successfully';
      } catch (e) {
        return false;
      }
    },
    'no errors': (r) => r.status < 400,
  });

  // Небольшая пауза между запросами
  sleep(0.1);
} 