// k6 Spike Test - тестирование экстремальной нагрузки
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Кастомные метрики для spike теста
export let errorRate = new Rate('spike_errors');
export let recoveryTime = new Trend('recovery_time');
export let spikeLatency = new Trend('spike_latency');
export let requestsPerSecond = new Counter('spike_rps');

// Конфигурация spike теста
export let options = {
  stages: [
    { duration: '1m', target: 50 },      // Baseline: 50 пользователей
    { duration: '30s', target: 500 },    // SPIKE: резкий рост до 500 пользователей
    { duration: '1m', target: 500 },     // Удержание пика
    { duration: '30s', target: 50 },     // Резкое снижение
    { duration: '2m', target: 50 },      // Recovery: проверка восстановления
  ],
  
  // Более мягкие пороги для spike теста
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% < 500ms (более мягко для spike)
    http_req_failed: ['rate<0.20'],   // До 20% ошибок допустимо в пике
    spike_errors: ['rate<0.25'],      // До 25% ошибок в spike фазе
    recovery_time: ['p(95)<100'],     // Восстановление < 100ms
  },
  
  // Настройки для экстремальной нагрузки
  maxRedirects: 2,
  batch: 50,
  batchPerHost: 25,
  discardResponseBodies: true, // Экономим память во время spike
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Простые сообщения для spike теста (минимизируем overhead)
function generateSpikeMessage() {
  return {
    source: 'spike_test',
    data: `spike_${__VU}_${__ITER}_${Date.now()}`,
    metadata: {
      spike: true,
      vu: __VU,
      iter: __ITER,
    }
  };
}

// Определение фазы теста по времени
function getCurrentPhase() {
  const elapsed = Date.now() - (__VU_START_TIME || Date.now());
  const elapsedMinutes = elapsed / 60000;
  
  if (elapsedMinutes < 1) return 'baseline';
  if (elapsedMinutes < 1.5) return 'spike_up';
  if (elapsedMinutes < 2.5) return 'spike_peak';
  if (elapsedMinutes < 3) return 'spike_down';
  return 'recovery';
}

// Основная функция spike теста
export default function() {
  const phase = getCurrentPhase();
  const message = generateSpikeMessage();
  const payload = JSON.stringify(message);
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-Test-Phase': phase,
      'X-Spike-Test': 'true',
    },
    timeout: phase === 'spike_peak' ? '30s' : '10s', // Больше timeout в пике
  };
  
  // Замеряем время запроса
  const start = Date.now();
  const response = http.post(`${BASE_URL}/api/v1/ingest`, payload, params);
  const duration = Date.now() - start;
  
  // Записываем метрики в зависимости от фазы
  requestsPerSecond.add(1);
  
  if (phase === 'spike_peak') {
    spikeLatency.add(duration);
  } else if (phase === 'recovery') {
    recoveryTime.add(duration);
  }
  
  // Проверки с учетом фазы теста
  let checks = {};
  
  if (phase === 'baseline' || phase === 'recovery') {
    // Строгие проверки для baseline и recovery
    checks = {
      'status is 200': (r) => r.status === 200,
      'has messageId': (r) => {
        try {
          return JSON.parse(r.body).messageId !== undefined;
        } catch (e) {
          return false;
        }
      },
      'response time reasonable': (r) => r.timings.duration < 200,
    };
  } else {
    // Мягкие проверки для spike фазы
    checks = {
      'not server error': (r) => r.status < 500,
      'response received': (r) => r.body.length > 0,
      'response time under limit': (r) => r.timings.duration < 1000,
    };
  }
  
  const isSuccess = check(response, checks);
  errorRate.add(!isSuccess);
  
  // Адаптивная пауза в зависимости от фазы
  let sleepTime;
  switch (phase) {
    case 'baseline':
    case 'recovery':
      sleepTime = Math.random() * 0.5; // 0-500ms
      break;
    case 'spike_up':
    case 'spike_down':
      sleepTime = Math.random() * 0.2; // 0-200ms
      break;
    case 'spike_peak':
      sleepTime = Math.random() * 0.1; // 0-100ms (максимальная нагрузка)
      break;
    default:
      sleepTime = 0.1;
  }
  
  sleep(sleepTime);
  
  // Периодическая проверка системы (реже в spike фазе)
  const checkInterval = phase === 'spike_peak' ? 200 : 50;
  if (__ITER % checkInterval === 0) {
    const statusResponse = http.get(`${BASE_URL}/api/v1/status`, {
      timeout: '5s'
    });
    
    if (statusResponse.status === 200) {
      try {
        const status = JSON.parse(statusResponse.body);
        // В spike фазе просто логируем, не проверяем строго
        if (phase === 'spike_peak') {
          console.log(`[${phase}] Queue size: ${status.processor?.stats?.queue?.CurrentSize || 'N/A'}`);
        }
      } catch (e) {
        // Игнорируем ошибки парсинга в spike фазе
      }
    }
  }
}

// Функция установки для spike теста
export function setup() {
  console.log('⚡ Starting Spike Test');
  console.log(`Target: ${BASE_URL}`);
  console.log('Test phases:');
  console.log('  0-1min: Baseline (50 users)');
  console.log('  1-1.5min: Spike Up (50→500 users)');
  console.log('  1.5-2.5min: Spike Peak (500 users)');
  console.log('  2.5-3min: Spike Down (500→50 users)');
  console.log('  3-5min: Recovery (50 users)');
  
  // Проверяем готовность системы
  const healthResponse = http.get(`${BASE_URL}/health`);
  if (healthResponse.status !== 200) {
    throw new Error(`Health check failed: ${healthResponse.status}`);
  }
  
  // Получаем baseline статистику
  const baselineStatus = http.get(`${BASE_URL}/api/v1/status`);
  let baselineStats = {};
  if (baselineStatus.status === 200) {
    try {
      baselineStats = JSON.parse(baselineStatus.body);
      console.log('📊 Baseline Statistics:');
      console.log(`  Queue size: ${baselineStats.processor?.stats?.queue?.CurrentSize || 0}`);
      console.log(`  Processed: ${baselineStats.processor?.stats?.pool?.ProcessedCount || 0}`);
    } catch (e) {
      console.log('Could not parse baseline statistics');
    }
  }
  
  console.log('✅ System ready for spike test');
  return {
    startTime: Date.now(),
    baselineStats: baselineStats
  };
}

// Функция завершения spike теста
export function teardown(data) {
  console.log('⚡ Spike Test completed');
  console.log(`Total duration: ${(Date.now() - data.startTime) / 1000}s`);
  
  // Ждем стабилизации системы
  console.log('⏳ Waiting for system stabilization...');
  sleep(10);
  
  // Получаем финальную статистику
  const finalStatus = http.get(`${BASE_URL}/api/v1/status`);
  if (finalStatus.status === 200) {
    try {
      const finalStats = JSON.parse(finalStatus.body);
      const baselineStats = data.baselineStats;
      
      console.log('📊 Spike Test Results:');
      console.log('─'.repeat(50));
      
      // Сравниваем статистику
      const processedDelta = (finalStats.processor?.stats?.pool?.ProcessedCount || 0) - 
                            (baselineStats.processor?.stats?.pool?.ProcessedCount || 0);
      const errorsDelta = (finalStats.processor?.stats?.pool?.ErrorCount || 0) - 
                         (baselineStats.processor?.stats?.pool?.ErrorCount || 0);
      
      console.log(`  Messages processed during test: ${processedDelta}`);
      console.log(`  Errors during test: ${errorsDelta}`);
      console.log(`  Final queue size: ${finalStats.processor?.stats?.queue?.CurrentSize || 0}`);
      console.log(`  Error rate: ${errorsDelta > 0 ? (errorsDelta / processedDelta * 100).toFixed(2) : 0}%`);
      
      // Оценка восстановления системы
      const finalQueueSize = finalStats.processor?.stats?.queue?.CurrentSize || 0;
      if (finalQueueSize < 10) {
        console.log('✅ System recovered successfully (queue drained)');
      } else if (finalQueueSize < 100) {
        console.log('⚠️ System partially recovered (small queue backlog)');
      } else {
        console.log('❌ System struggling to recover (large queue backlog)');
      }
      
    } catch (e) {
      console.log('❌ Could not analyze final statistics');
    }
  }
  
  console.log('');
  console.log('🔍 Analysis recommendations:');
  console.log('  - Check error spike during peak load');
  console.log('  - Verify system recovery time');
  console.log('  - Monitor queue backlog patterns');
  console.log('  - Review resource utilization graphs');
} 