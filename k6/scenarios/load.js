// k6 Load Test - реалистичная нагрузка для домашнего ПК
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Кастомные метрики
export let errorRate = new Rate('errors');
export let throughput = new Counter('requests_total');
export let processingLatency = new Trend('processing_latency');
export let messageSize = new Trend('message_size_bytes');

// Конфигурация нагрузки (адаптирована для домашнего ПК)
export let options = {
  stages: [
    { duration: '2m', target: 50 },    // Разгон до 50 пользователей
    { duration: '5m', target: 100 },   // Нагрузка 100 пользователей
    { duration: '5m', target: 200 },   // Пиковая нагрузка 200 пользователей
    { duration: '2m', target: 100 },   // Снижение до 100
    { duration: '2m', target: 0 },     // Плавная остановка
  ],
  
  // Пороговые значения для успешного теста
  thresholds: {
    http_req_duration: ['p(95)<100', 'p(99)<200'], // 95% < 100ms, 99% < 200ms
    http_req_failed: ['rate<0.05'],                 // Менее 5% ошибок
    errors: ['rate<0.05'],                          // Менее 5% бизнес-ошибок
    requests_total: ['rate>500'],                   // Минимум 500 RPS
    processing_latency: ['p(95)<50'],               // 95% обработки < 50ms
  },
  
  // HTTP настройки
  httpDebug: 'full', // Только при отладке
  insecureSkipTLSVerify: true,
  noConnectionReuse: false,
  
  // Ресурсы для домашнего ПК
  maxRedirects: 4,
  batch: 20,
  batchPerHost: 10,
};

const INGEST_URL = __ENV.INGEST_URL || 'http://localhost:8081';
const PROCESSOR_URL = __ENV.PROCESSOR_URL || 'http://localhost:8082';

// Генератор реалистичных сообщений различного размера
function generateRealisticMessage() {
  const messageTypes = [
    { type: 'user_action', size: 'small' },
    { type: 'system_event', size: 'medium' },
    { type: 'analytics_batch', size: 'large' },
    { type: 'error_report', size: 'medium' },
    { type: 'metrics_update', size: 'small' },
  ];
  
  const msgType = messageTypes[Math.floor(Math.random() * messageTypes.length)];
  
  let data;
  switch (msgType.size) {
    case 'small':
      data = `{"event":"${msgType.type}","userId":${Math.floor(Math.random() * 10000)},"timestamp":"${new Date().toISOString()}"}`;
      break;
    case 'medium':
      data = JSON.stringify({
        event: msgType.type,
        userId: Math.floor(Math.random() * 10000),
        sessionId: `session_${__VU}_${__ITER}`,
        timestamp: new Date().toISOString(),
        properties: {
          browser: 'Chrome/91.0',
          platform: 'Linux',
          screen: '1920x1080',
          referrer: 'https://example.com',
          url: `/page/${Math.floor(Math.random() * 100)}`,
        }
      });
      break;
    case 'large':
      let events = [];
      for (let i = 0; i < 10; i++) {
        events.push({
          id: Math.floor(Math.random() * 100000),
          type: `event_${i}`,
          value: Math.random() * 1000,
          tags: [`tag_${i}`, `category_${Math.floor(i/3)}`]
        });
      }
      data = JSON.stringify({
        batchType: msgType.type,
        events: events,
        metadata: {
          batchId: `batch_${__VU}_${__ITER}`,
          source: 'analytics_collector',
          timestamp: new Date().toISOString(),
        }
      });
      break;
  }
  
  return {
    source: `load_test_${msgType.type}`,
    data: data,
    metadata: {
      testRun: __ENV.TEST_RUN_ID || 'load_test',
      virtualUser: __VU,
      iteration: __ITER,
      messageType: msgType.type,
      messageSize: msgType.size,
      timestamp: new Date().toISOString(),
    }
  };
}

// Основная функция нагрузочного теста
export default function() {
  // Генерируем сообщение
  let message = generateRealisticMessage();
  let payload = JSON.stringify(message);
  
  // Записываем размер сообщения
  messageSize.add(payload.length);
  
  let params = {
    headers: {
      'Content-Type': 'application/json',
      'X-Test-Run': __ENV.TEST_RUN_ID || 'load_test',
      'X-Virtual-User': __VU.toString(),
    },
    timeout: '10s',
  };
  
  // Отправляем запрос с замером времени
  let start = Date.now();
  let response = http.post(`${INGEST_URL}/ingest`, payload, params);
  let processingTime = Date.now() - start;
  
  // Записываем метрики
  throughput.add(1);
  processingLatency.add(processingTime);
  
  // Проверки
  let isSuccess = check(response, {
    'status is 200': (r) => r.status === 200,
    'has messageId': (r) => {
      try {
        let body = JSON.parse(r.body);
        return body.messageId && body.messageId.length > 0;
      } catch (e) {
        return false;
      }
    },
    'status is accepted': (r) => {
      try {
        let body = JSON.parse(r.body);
        return body.status === 'accepted';
      } catch (e) {
        return false;
      }
    },
    'response time acceptable': (r) => r.timings.duration < 500,
    'no error in response': (r) => !r.body.includes('error'),
  });
  
  errorRate.add(!isSuccess);
  
  // Периодическая проверка статуса системы (каждые 50 итераций)
  if (__ITER % 50 === 0) {
    let ingestHealthResponse = http.get(`${INGEST_URL}/health`);
    let processorHealthResponse = http.get(`${PROCESSOR_URL}/health`);
    check(ingestHealthResponse, {
      'ingest service healthy': (r) => r.status === 200 && JSON.parse(r.body).healthy === true,
    });
    check(processorHealthResponse, {
      'processor service healthy': (r) => r.status === 200 && JSON.parse(r.body).healthy === true,
    });
  }
  
  // Имитация реального пользователя с различными паузами
  let pauseDuration;
  if (message.metadata.messageType === 'user_action') {
    pauseDuration = Math.random() * 0.5; // 0-500ms для пользовательских действий
  } else if (message.metadata.messageType === 'analytics_batch') {
    pauseDuration = Math.random() * 2; // 0-2s для батчей
  } else {
    pauseDuration = Math.random() * 1; // 0-1s для остальных
  }
  
  sleep(pauseDuration);
}

// Функция установки
export function setup() {
  console.log('🚀 Starting Load Test');
  console.log(`Ingest Target: ${INGEST_URL}`);
  console.log(`Processor Target: ${PROCESSOR_URL}`);
  console.log(`Test Run ID: ${__ENV.TEST_RUN_ID || 'not_set'}`);
  
  // Проверяем готовность системы
  let ingestHealthCheck = http.get(`${INGEST_URL}/health`);
  if (ingestHealthCheck.status !== 200) {
    throw new Error(`Ingest service health check failed: ${ingestHealthCheck.status}`);
  }
  
  let processorHealthCheck = http.get(`${PROCESSOR_URL}/health`);
  if (processorHealthCheck.status !== 200) {
    throw new Error(`Processor service health check failed: ${processorHealthCheck.status}`);
  }
  
  console.log('✅ Services are ready for load testing');
  
  // Прогрев системы
  console.log('🔥 Warming up the system...');
  for (let i = 0; i < 10; i++) {
    let warmupMessage = {
      source: 'warmup_test',
      data: `Warmup message ${i} at ${new Date().toISOString()}`,
      metadata: {
        type: 'warmup',
        priority: 'normal',
        timestamp: new Date().toISOString(),
      }
    };
    http.post(`${INGEST_URL}/ingest`, JSON.stringify(warmupMessage), {
      headers: { 'Content-Type': 'application/json' }
    });
  }
  
  console.log('✅ Warmup completed, starting load test...');
  return { 
    startTime: new Date(),
    testRunId: __ENV.TEST_RUN_ID || 'load_test'
  };
}

// Функция завершения
export function teardown(data) {
  console.log('🏁 Load Test completed');
  console.log(`Test Run: ${data.testRunId}`);
  console.log(`Duration: ${new Date() - data.startTime}ms`);
  
  // Финальная проверка состояния сервисов
  let ingestHealth = http.get(`${INGEST_URL}/health`);
  let processorHealth = http.get(`${PROCESSOR_URL}/health`);
  
  console.log('📊 Final Services Status:');
  console.log(`  Ingest: ${ingestHealth.status === 200 ? 'Healthy' : 'Unhealthy'}`);
  console.log(`  Processor: ${processorHealth.status === 200 ? 'Healthy' : 'Unhealthy'}`);
  
  console.log('💡 Analysis tips:');
  console.log('  - Check p95/p99 latencies in results');
  console.log('  - Monitor error rates and throughput');
  console.log('  - Review system resource usage');
} 