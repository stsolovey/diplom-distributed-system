// k6 Soak Test - длительное тестирование на устойчивость
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Кастомные метрики для soak теста
export let memoryLeakIndicator = new Trend('memory_leak_indicator');
export let performanceDegradation = new Trend('performance_degradation');
export let longRunErrors = new Rate('long_run_errors');
export let hourlyThroughput = new Counter('hourly_throughput');
export let systemHealth = new Rate('system_health_checks');

// Конфигурация soak теста (2 часа вместо 6 для домашнего ПК)
export let options = {
  stages: [
    { duration: '5m', target: 50 },      // Разгон до рабочей нагрузки
    { duration: '110m', target: 50 },    // Основная фаза: 110 минут стабильной нагрузки
    { duration: '5m', target: 0 },       // Плавная остановка
  ],
  
  // Пороги для длительного теста
  thresholds: {
    http_req_duration: [
      'p(95)<150',              // 95% < 150ms на протяжении всего теста
      'p(99)<300',              // 99% < 300ms
    ],
    http_req_failed: ['rate<0.02'],       // Менее 2% ошибок
    long_run_errors: ['rate<0.02'],       // Менее 2% ошибок за длительный период
    performance_degradation: ['p(95)<50'], // Деградация < 50ms в 95% случаев
    system_health_checks: ['rate>0.95'],   // 95% health checks успешны
  },
  
  // Настройки для длительного теста
  maxRedirects: 4,
  batch: 20,
  batchPerHost: 10,
  discardResponseBodies: false, // Сохраняем для анализа трендов
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Генератор сообщений для soak теста с вариациями
function generateSoakMessage() {
  const messagePatterns = [
    // Паттерн 1: Постоянные пользовательские действия
    {
      source: 'user_session',
      data: JSON.stringify({
        action: 'page_view',
        userId: Math.floor(Math.random() * 5000),
        page: `/page/${Math.floor(Math.random() * 100)}`,
        timestamp: new Date().toISOString(),
      }),
      size: 'small'
    },
    // Паттерн 2: Периодические системные события
    {
      source: 'system_monitor',
      data: JSON.stringify({
        event: 'metric_update',
        metrics: {
          cpu: Math.random() * 100,
          memory: Math.random() * 100,
          disk: Math.random() * 100,
        },
        timestamp: new Date().toISOString(),
      }),
      size: 'medium'
    },
    // Паттерн 3: Большие аналитические события (реже)
    {
      source: 'analytics_engine',
      data: JSON.stringify({
        batch_id: `batch_${Date.now()}`,
        events: Array.from({length: 20}, (_, i) => ({
          id: i,
          type: `event_${Math.floor(Math.random() * 10)}`,
          value: Math.random() * 1000,
        })),
        timestamp: new Date().toISOString(),
      }),
      size: 'large'
    }
  ];
  
  // Распределение типов сообщений: 60% small, 30% medium, 10% large
  let pattern;
  const rand = Math.random();
  if (rand < 0.6) {
    pattern = messagePatterns[0];
  } else if (rand < 0.9) {
    pattern = messagePatterns[1];
  } else {
    pattern = messagePatterns[2];
  }
  
  return {
    source: pattern.source,
    data: pattern.data,
    metadata: {
      testType: 'soak',
      hour: Math.floor(Date.now() / (1000 * 60 * 60)),
      messageSize: pattern.size,
      virtualUser: __VU,
      iteration: __ITER,
    }
  };
}

// Функция для вычисления метрик деградации производительности
let baselineLatency = null;
let lastHourlyCheck = 0;

export default function() {
  const message = generateSoakMessage();
  const payload = JSON.stringify(message);
  const currentHour = Math.floor(Date.now() / (1000 * 60 * 60));
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-Test-Type': 'soak',
      'X-Test-Hour': currentHour.toString(),
    },
    timeout: '15s',
  };
  
  // Отправляем запрос с замером времени
  const start = Date.now();
  const response = http.post(`${BASE_URL}/api/v1/ingest`, payload, params);
  const latency = Date.now() - start;
  
  // Устанавливаем baseline в первые 5 минут
  if (!baselineLatency && __ITER < 50) {
    if (__ITER === 49) { // На 50-й итерации фиксируем baseline
      baselineLatency = latency;
      console.log(`📊 Baseline latency established: ${baselineLatency}ms`);
    }
  }
  
  // Вычисляем деградацию производительности
  if (baselineLatency) {
    const degradation = latency - baselineLatency;
    performanceDegradation.add(degradation);
    
    // Индикатор потенциальной утечки памяти (растущая latency)
    if (degradation > baselineLatency * 2) {
      memoryLeakIndicator.add(degradation);
    }
  }
  
  // Подсчитываем throughput по часам
  hourlyThroughput.add(1);
  
  // Основные проверки
  const isSuccess = check(response, {
    'status is 200': (r) => r.status === 200,
    'has messageId': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.messageId && body.messageId.length > 0;
      } catch (e) {
        return false;
      }
    },
    'status is accepted': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.status === 'accepted';
      } catch (e) {
        return false;
      }
    },
    'latency not degraded severely': (r) => {
      if (!baselineLatency) return true;
      return r.timings.duration < baselineLatency * 5; // Не более 5x деградации
    },
  });
  
  longRunErrors.add(!isSuccess);
  
  // Проверка здоровья системы каждые 5 минут
  if (currentHour !== lastHourlyCheck && __ITER % 100 === 0) {
    lastHourlyCheck = currentHour;
    
    const healthResponse = http.get(`${BASE_URL}/health`);
    const statusResponse = http.get(`${BASE_URL}/api/v1/status`);
    
    const healthCheck = check(healthResponse, {
      'health endpoint available': (r) => r.status === 200,
      'service reports healthy': (r) => {
        try {
          return JSON.parse(r.body).healthy === true;
        } catch (e) {
          return false;
        }
      },
    });
    
    const statusCheck = check(statusResponse, {
      'status endpoint available': (r) => r.status === 200,
      'services are healthy': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body.ingest?.healthy && body.processor?.healthy;
        } catch (e) {
          return false;
        }
      },
    });
    
    systemHealth.add(healthCheck && statusCheck);
    
    // Логируем состояние системы каждый час
    if (statusResponse.status === 200) {
      try {
        const stats = JSON.parse(statusResponse.body);
        console.log(`🕐 Hour ${currentHour} system stats:`);
        console.log(`  Queue size: ${stats.processor?.stats?.queue?.CurrentSize || 'N/A'}`);
        console.log(`  Processed: ${stats.processor?.stats?.pool?.ProcessedCount || 'N/A'}`);
        console.log(`  Errors: ${stats.processor?.stats?.pool?.ErrorCount || 'N/A'}`);
        
        // Проверка на потенциальную утечку памяти (растущая очередь)
        const queueSize = stats.processor?.stats?.queue?.CurrentSize || 0;
        if (queueSize > 1000) {
          console.log(`⚠️ Warning: Large queue size detected (${queueSize})`);
        }
      } catch (e) {
        console.log(`❌ Could not parse system stats for hour ${currentHour}`);
      }
    }
  }
  
  // Вариативная пауза для имитации реального трафика
  const pauseVariation = Math.random();
  let sleepTime;
  
  if (pauseVariation < 0.1) {
    sleepTime = Math.random() * 5; // 10% запросов с длинной паузой (0-5s)
  } else if (pauseVariation < 0.3) {
    sleepTime = Math.random() * 2; // 20% запросов со средней паузой (0-2s)
  } else {
    sleepTime = Math.random() * 1; // 70% запросов с короткой паузой (0-1s)
  }
  
  sleep(sleepTime);
}

// Функция установки для soak теста
export function setup() {
  console.log('🕐 Starting Soak Test (2 hours)');
  console.log(`Target: ${BASE_URL}`);
  console.log('Test phases:');
  console.log('  0-5min: Ramp up to 50 users');
  console.log('  5-115min: Steady load with 50 users');
  console.log('  115-120min: Ramp down');
  
  // Проверяем готовность системы
  const healthResponse = http.get(`${BASE_URL}/health`);
  if (healthResponse.status !== 200) {
    throw new Error(`Health check failed: ${healthResponse.status}`);
  }
  
  const statusResponse = http.get(`${BASE_URL}/api/v1/status`);
  if (statusResponse.status !== 200) {
    throw new Error(`Status check failed: ${statusResponse.status}`);
  }
  
  // Получаем начальную статистику
  let initialStats = {};
  try {
    initialStats = JSON.parse(statusResponse.body);
    console.log('📊 Initial System State:');
    console.log(`  Queue size: ${initialStats.processor?.stats?.queue?.CurrentSize || 0}`);
    console.log(`  Processed: ${initialStats.processor?.stats?.pool?.ProcessedCount || 0}`);
    console.log(`  Memory indicators will be monitored for leaks`);
  } catch (e) {
    console.log('Could not parse initial statistics');
  }
  
  console.log('✅ System ready for 2-hour soak test');
  console.log('📋 Monitoring:');
  console.log('  - Performance degradation over time');
  console.log('  - Memory leak indicators');
  console.log('  - Error rate stability');
  console.log('  - System health every hour');
  
  return {
    startTime: Date.now(),
    initialStats: initialStats,
    testDuration: '2 hours'
  };
}

// Функция завершения soak теста
export function teardown(data) {
  const testDuration = (Date.now() - data.startTime) / 1000 / 60; // в минутах
  
  console.log('🏁 Soak Test completed');
  console.log(`Actual duration: ${testDuration.toFixed(1)} minutes`);
  
  // Даем системе время на завершение обработки
  console.log('⏳ Allowing system to finish processing...');
  sleep(30);
  
  // Финальный анализ
  const finalStatus = http.get(`${BASE_URL}/api/v1/status`);
  if (finalStatus.status === 200) {
    try {
      const finalStats = JSON.parse(finalStatus.body);
      const initialStats = data.initialStats;
      
      console.log('📊 Soak Test Analysis:');
      console.log('═'.repeat(60));
      
      // Анализ производительности
      const totalProcessed = (finalStats.processor?.stats?.pool?.ProcessedCount || 0) - 
                            (initialStats.processor?.stats?.pool?.ProcessedCount || 0);
      const totalErrors = (finalStats.processor?.stats?.pool?.ErrorCount || 0) - 
                         (initialStats.processor?.stats?.pool?.ErrorCount || 0);
      const finalQueueSize = finalStats.processor?.stats?.queue?.CurrentSize || 0;
      const initialQueueSize = initialStats.processor?.stats?.queue?.CurrentSize || 0;
      
      console.log(`  Messages processed: ${totalProcessed}`);
      console.log(`  Average throughput: ${(totalProcessed / (testDuration/60)).toFixed(1)} msg/hour`);
      console.log(`  Total errors: ${totalErrors}`);
      console.log(`  Error rate: ${totalProcessed > 0 ? (totalErrors / totalProcessed * 100).toFixed(3) : 0}%`);
      
      // Анализ стабильности
      console.log('\n🔍 Stability Analysis:');
      console.log(`  Queue size change: ${initialQueueSize} → ${finalQueueSize} (Δ${finalQueueSize - initialQueueSize})`);
      
      if (finalQueueSize <= initialQueueSize + 10) {
        console.log('✅ Queue remained stable (no significant growth)');
      } else if (finalQueueSize <= initialQueueSize + 100) {
        console.log('⚠️ Minor queue growth detected (monitor for memory leaks)');
      } else {
        console.log('❌ Significant queue growth (potential memory leak or processing bottleneck)');
      }
      
      // Оценка общего результата
      console.log('\n🎯 Soak Test Verdict:');
      const errorRate = totalProcessed > 0 ? (totalErrors / totalProcessed) : 0;
      const queueGrowth = finalQueueSize - initialQueueSize;
      
      if (errorRate < 0.02 && queueGrowth < 50) {
        console.log('🏆 EXCELLENT: System demonstrated high stability over 2 hours');
      } else if (errorRate < 0.05 && queueGrowth < 200) {
        console.log('✅ GOOD: System remained stable with minor issues');
      } else if (errorRate < 0.10 && queueGrowth < 500) {
        console.log('⚠️ CONCERNING: System showed stability issues, investigate further');
      } else {
        console.log('❌ POOR: System demonstrated instability, optimization required');
      }
      
    } catch (e) {
      console.log('❌ Could not analyze final statistics');
    }
  } else {
    console.log('❌ System not responding after soak test');
  }
  
  console.log('\n📋 Post-test recommendations:');
  console.log('  - Review memory usage trends');
  console.log('  - Check for any resource leaks');
  console.log('  - Analyze performance degradation patterns');
  console.log('  - Monitor system recovery time');
} 