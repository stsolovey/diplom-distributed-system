import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
  stages: [
    { duration: '10s', target: 5 },   // Разогрев
    { duration: '15s', target: 10 },  // Основная нагрузка
    { duration: '5s', target: 0 },    // Завершение
  ],
  thresholds: {
    http_req_duration: ['p(95)<100'], // 95% запросов быстрее 100ms
    http_req_failed: ['rate<0.1'],    // Менее 10% ошибок
  },
};

const BASE_URL = 'http://localhost:8080';

export default function () {
  // Тестовое сообщение
  const payload = JSON.stringify({
    source: 'k6-demo',
    data: `Demo message ${__VU}-${__ITER}`,
    metadata: {
      type: 'demo',
      timestamp: new Date().toISOString(),
      vu: __VU,
      iteration: __ITER,
    }
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  // Отправка сообщения
  let response = http.post(`${BASE_URL}/api/v1/ingest`, payload, params);
  
  check(response, {
    'ingest status is 200': (r) => r.status === 200,
    'ingest response time < 100ms': (r) => r.timings.duration < 100,
  });

  // Проверка статуса системы (каждые 5 итераций)
  if (__ITER % 5 === 0) {
    let statusResponse = http.get(`${BASE_URL}/api/v1/status`);
    
    check(statusResponse, {
      'status endpoint is 200': (r) => r.status === 200,
      'status response contains services': (r) => r.body.includes('services'),
    });
  }

  sleep(0.1); // Небольшая пауза между запросами
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
  };
}

function textSummary(data, options) {
  const indent = options.indent || '';
  const enableColors = options.enableColors || false;
  
  let summary = `
${indent}📊 K6 Demo Test Results
${indent}========================
${indent}
${indent}🚀 Requests:
${indent}  Total: ${data.metrics.http_reqs.values.count}
${indent}  Failed: ${data.metrics.http_req_failed.values.rate * 100}%
${indent}
${indent}⏱️  Response Times:
${indent}  Average: ${data.metrics.http_req_duration.values.avg.toFixed(2)}ms
${indent}  95th percentile: ${data.metrics.http_req_duration.values['p(95)'].toFixed(2)}ms
${indent}  Max: ${data.metrics.http_req_duration.values.max.toFixed(2)}ms
${indent}
${indent}🎯 Thresholds:
${indent}  Response time P95 < 100ms: ${data.metrics.http_req_duration.values['p(95)'] < 100 ? '✅ PASS' : '❌ FAIL'}
${indent}  Error rate < 10%: ${data.metrics.http_req_failed.values.rate < 0.1 ? '✅ PASS' : '❌ FAIL'}
${indent}
${indent}✅ Demo completed successfully!
`;

  return summary;
} 