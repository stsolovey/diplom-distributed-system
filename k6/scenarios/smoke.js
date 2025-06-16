// k6 Smoke Test - проверка базовой функциональности
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Кастомные метрики
export let errorRate = new Rate('errors');
export let processingTime = new Trend('processing_time');

// Конфигурация теста
export let options = {
  stages: [
    { duration: '30s', target: 5 },   // Разгон до 5 пользователей
    { duration: '1m', target: 10 },   // Удержание 10 пользователей
    { duration: '30s', target: 0 },   // Остановка
  ],
  thresholds: {
    http_req_duration: ['p(95)<100'], // 95% запросов < 100ms
    http_req_failed: ['rate<0.01'],   // Менее 1% ошибок
    errors: ['rate<0.01'],            // Менее 1% бизнес-ошибок
  },
};

// Базовые URL для сервисов
const INGEST_URL = __ENV.INGEST_URL || 'http://localhost:8081';
const PROCESSOR_URL = __ENV.PROCESSOR_URL || 'http://localhost:8082';

// Генерация тестовых данных
function generateTestMessage() {
  const sources = ['web-app', 'mobile-app', 'api-client', 'batch-job'];
  const types = ['user-action', 'system-event', 'error-report', 'analytics'];
  
  return {
    source: sources[Math.floor(Math.random() * sources.length)],
    data: `Test data from VU ${__VU} iteration ${__ITER} at ${new Date().toISOString()}`,
    metadata: {
      type: types[Math.floor(Math.random() * types.length)],
      priority: Math.random() > 0.8 ? 'high' : 'normal',
      userId: `user_${Math.floor(Math.random() * 1000)}`,
      sessionId: `session_${__VU}_${__ITER}`,
      timestamp: new Date().toISOString(),
    }
  };
}

// Главная функция теста
export default function() {
  // 1. Health Check для Ingest сервиса
  let ingestHealthResponse = http.get(`${INGEST_URL}/health`);
  check(ingestHealthResponse, {
    'ingest health check status is 200': (r) => r.status === 200,
    'ingest health check has healthy flag': (r) => JSON.parse(r.body).healthy === true,
  });
  
  // 2. Health Check для Processor сервиса
  let processorHealthResponse = http.get(`${PROCESSOR_URL}/health`);
  check(processorHealthResponse, {
    'processor health check status is 200': (r) => r.status === 200,
    'processor health check has healthy flag': (r) => JSON.parse(r.body).healthy === true,
  });
  
  // 3. Send Test Message
  let payload = JSON.stringify(generateTestMessage());
  let params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };
  
  let start = Date.now();
  let ingestResponse = http.post(`${INGEST_URL}/ingest`, payload, params);
  let duration = Date.now() - start;
  
  // Записываем время обработки
  processingTime.add(duration);
  
  // Проверки ответа
  let isSuccess = check(ingestResponse, {
    'ingest status is 200': (r) => r.status === 200,
    'ingest response has messageId': (r) => {
      try {
        let body = JSON.parse(r.body);
        return body.messageId !== undefined && body.messageId !== '';
      } catch (e) {
        return false;
      }
    },
    'ingest response has accepted status': (r) => {
      try {
        let body = JSON.parse(r.body);
        return body.status === 'accepted';
      } catch (e) {
        return false;
      }
    },
    'response time < 100ms': (r) => r.timings.duration < 100,
  });
  
  // Отмечаем ошибки
  errorRate.add(!isSuccess);
  
  // Пауза между запросами (имитация пользователя)
  sleep(Math.random() * 2 + 1); // 1-3 секунды
}

// Функция установки (выполняется один раз)
export function setup() {
  console.log('🔥 Starting Smoke Test');
  console.log(`Ingest Target: ${INGEST_URL}`);
  console.log(`Processor Target: ${PROCESSOR_URL}`);
  
  // Проверяем доступность сервисов
  let ingestResponse = http.get(`${INGEST_URL}/health`);
  if (ingestResponse.status !== 200) {
    throw new Error(`Ingest service is not available: ${ingestResponse.status}`);
  }
  
  let processorResponse = http.get(`${PROCESSOR_URL}/health`);
  if (processorResponse.status !== 200) {
    throw new Error(`Processor service is not available: ${processorResponse.status}`);
  }
  
  console.log('✅ Services are available, starting test...');
  return { startTime: new Date() };
}

// Функция очистки (выполняется один раз после теста)
export function teardown(data) {
  console.log('🏁 Smoke Test completed');
  console.log(`Started at: ${data.startTime}`);
  console.log(`Finished at: ${new Date()}`);
  
  // Проверяем финальное состояние сервисов
  let ingestHealth = http.get(`${INGEST_URL}/health`);
  let processorHealth = http.get(`${PROCESSOR_URL}/health`);
  
  console.log('📊 Final services status:');
  console.log(`Ingest: ${ingestHealth.status === 200 ? 'Healthy' : 'Unhealthy'}`);
  console.log(`Processor: ${processorHealth.status === 200 ? 'Healthy' : 'Unhealthy'}`);
} 