import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Counter, Trend } from 'k6/metrics';

// Custom metrics
const errors = new Rate('errors');
const requestsTotal = new Counter('requests_total');
const processingLatency = new Trend('processing_latency');

// Configuration
const INGEST_URL = 'http://localhost:8081';
const PROCESSOR_URL = 'http://localhost:8082';

export const options = {
  stages: [
    { duration: '1m', target: 50 },   // Ramp up to 50 users
    { duration: '3m', target: 100 },  // Stay at 100 users
    { duration: '3m', target: 200 },  // Ramp up to 200 users
    { duration: '3m', target: 200 },  // Stay at 200 users
  ],
  thresholds: {
    http_req_duration: ['p(95)<100', 'p(99)<200'],
    http_req_failed: ['rate<0.05'],
    errors: ['rate<0.05'],
    requests_total: ['rate>500'],
    processing_latency: ['p(95)<50'],
  },
};

export function setup() {
  console.log('🚀 Starting Simple Load Test...');
  
  // Test connectivity
  const ingestHealth = http.get(`${INGEST_URL}/health`);
  const processorHealth = http.get(`${PROCESSOR_URL}/health`);
  
  if (ingestHealth.status !== 200) {
    throw new Error(`Ingest service not healthy: ${ingestHealth.status}`);
  }
  
  if (processorHealth.status !== 200) {
    throw new Error(`Processor service not healthy: ${processorHealth.status}`);
  }
  
  console.log('✅ All services are healthy');
  console.log('📊 Load test configuration:');
  console.log('  - Target: 200 VUs for 10 minutes');
  console.log('  - Simple JSON payloads');
  
  return {
    ingestUrl: INGEST_URL,
    processorUrl: PROCESSOR_URL,
    startTime: Date.now()
  };
}

export default function(data) {
  // Generate simple test data
  const messageTypes = ['user_action', 'system_event', 'analytics', 'error_report', 'metrics'];
  const messageType = messageTypes[Math.floor(Math.random() * messageTypes.length)];
  
  // Create simple payload - avoid complex nested JSON
  const simpleData = `{"event":"${messageType}","userId":${Math.floor(Math.random() * 10000)},"timestamp":"${new Date().toISOString()}"}`;
  
  // Create the request in the format expected by ingest service
  const requestBody = {
    source: `load_test_${messageType}`,
    data: simpleData,
    metadata: {
      testRun: 'simple_load_test',
      virtualUser: __VU.toString(),
      iteration: __ITER.toString(),
      messageType: messageType
    }
  };
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-Test-Run': 'simple_load_test',
      'X-Virtual-User': __VU.toString(),
    },
    timeout: '10s',
  };
  
  // Send request to ingest service
  const startTime = Date.now();
  const response = http.post(`${INGEST_URL}/ingest`, JSON.stringify(requestBody), params);
  const endTime = Date.now();
  
  // Record metrics
  requestsTotal.add(1);
  processingLatency.add(endTime - startTime);
  
  // Validate response
  const success = check(response, {
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
    'response time acceptable': (r) => r.timings.duration < 1000,
  });
  
  if (!success) {
    errors.add(1);
    console.log(`Error response: ${response.status} - ${response.body}`);
  }
  
  // Random sleep between requests
  sleep(Math.random() * 0.3 + 0.1);
}

export function teardown(data) {
  console.log('🏁 Simple Load Test completed');
  console.log(`Duration: ${Date.now() - data.startTime}ms`);
  
  // Final health check
  const ingestHealth = http.get(`${data.ingestUrl}/health`);
  const processorHealth = http.get(`${data.processorUrl}/health`);
  
  console.log('📊 Final Services Status:');
  console.log(`  Ingest: ${ingestHealth.status === 200 ? 'Healthy' : 'Unhealthy'}`);
  console.log(`  Processor: ${processorHealth.status === 200 ? 'Healthy' : 'Unhealthy'}`);
} 