#!/bin/bash
set -e

# Фаза 4: Полный цикл нагрузочного тестирования
# Реализует все тесты из плана: Baseline, Scaled, Stress, Soak

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RESULTS_DIR="$PROJECT_ROOT/results/load_testing/$(date +%Y%m%d_%H%M%S)"
MONITORING_DIR="$PROJECT_ROOT/docker/monitoring"

# Цвета для логов
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Функции логирования
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Проверка зависимостей
check_dependencies() {
    log_info "Checking dependencies..."
    
    # Проверяем k6
    if ! command -v k6 &> /dev/null; then
        log_error "k6 is not installed. Run: ./scripts/install-k6.sh"
        exit 1
    fi
    
    # Проверяем Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        exit 1
    fi
    
    # Проверяем docker-compose
    if ! command -v docker-compose &> /dev/null; then
        log_error "docker-compose is not installed"
        exit 1
    fi
    
    # Проверяем curl и jq
    for cmd in curl jq; do
        if ! command -v $cmd &> /dev/null; then
            log_error "$cmd is not installed"
            exit 1
        fi
    done
    
    log_success "All dependencies are available"
}

# Подготовка мониторинга
setup_monitoring() {
    log_info "Setting up monitoring infrastructure..."
    
    # Проверяем наличие конфигураций мониторинга
    if [ ! -f "$MONITORING_DIR/prometheus.yml" ]; then
        log_error "Prometheus config not found at $MONITORING_DIR/prometheus.yml"
        exit 1
    fi
    
    # Запускаем мониторинг
    cd "$PROJECT_ROOT"
    docker-compose -f docker/monitoring-compose.yml up -d
    
    # Ждем готовности мониторинга
    log_info "Waiting for monitoring services to be ready..."
    for i in {1..30}; do
        if curl -sf http://localhost:9090/-/ready > /dev/null 2>&1; then
            break
        fi
        if [ $i -eq 30 ]; then
            log_error "Prometheus failed to start"
            exit 1
        fi
        sleep 2
    done
    
    for i in {1..30}; do
        if curl -sf http://localhost:3000/api/health > /dev/null 2>&1; then
            break
        fi
        if [ $i -eq 30 ]; then
            log_error "Grafana failed to start"
            exit 1
        fi
        sleep 2
    done
    
    log_success "Monitoring services are ready"
    log_info "Prometheus: http://localhost:9090"
    log_info "Grafana: http://localhost:3000 (admin/admin123)"
    log_info "AlertManager: http://localhost:9093"
}

# Подготовка системы под тест
setup_system() {
    local scale_factor=${1:-1}
    
    log_info "Setting up system for testing (scale factor: $scale_factor)..."
    
    cd "$PROJECT_ROOT"
    
    # Останавливаем существующие сервисы
    docker-compose -f docker/docker-compose.yml down || true
    
    # Запускаем систему
    if [ "$scale_factor" -gt 1 ]; then
        log_info "Scaling processor service to $scale_factor instances"
        docker-compose -f docker/docker-compose.yml up -d
        docker-compose -f docker/docker-compose.yml up -d --scale processor=$scale_factor
    else
        docker-compose -f docker/docker-compose.yml up -d
    fi
    
    # Ждем готовности системы
    log_info "Waiting for system to be ready..."
    local max_attempts=60
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if curl -sf http://localhost:8080/health > /dev/null 2>&1 && \
           curl -sf http://localhost:8081/health > /dev/null 2>&1 && \
           curl -sf http://localhost:8082/health > /dev/null 2>&1; then
            break
        fi
        
        attempt=$((attempt + 1))
        if [ $attempt -eq $max_attempts ]; then
            log_error "System failed to start within timeout"
            docker-compose -f docker/docker-compose.yml logs
            exit 1
        fi
        
        sleep 2
    done
    
    # Прогрев системы
    log_info "Warming up the system..."
    for i in {1..50}; do
        curl -sf -X POST http://localhost:8080/api/v1/ingest \
            -H "Content-Type: application/json" \
            -d '{"source":"warmup","data":"test","metadata":{"warmup":true}}' > /dev/null || true
    done
    
    # Даем системе время на стабилизацию
    sleep 10
    
    log_success "System is ready for testing"
}

# Выполнение теста
run_test() {
    local test_name=$1
    local test_file=$2
    local test_duration=$3
    
    log_info "Running $test_name test..."
    log_info "Expected duration: $test_duration"
    log_info "Results will be saved to: $RESULTS_DIR/${test_name}_results.json"
    
    # Создаем уникальный ID для теста
    local test_run_id="${test_name}_$(date +%Y%m%d_%H%M%S)"
    
    # Запускаем тест с выводом в файл
    cd "$PROJECT_ROOT"
    BASE_URL=http://localhost:8080 \
    TEST_RUN_ID=$test_run_id \
    k6 run --out json="$RESULTS_DIR/${test_name}_results.json" \
           --summary-export="$RESULTS_DIR/${test_name}_summary.json" \
           "$test_file" 2>&1 | tee "$RESULTS_DIR/${test_name}_output.log"
    
    local exit_code=${PIPESTATUS[0]}
    
    if [ $exit_code -eq 0 ]; then
        log_success "$test_name test completed successfully"
        
        # Собираем финальную статистику
        if [ -f "$RESULTS_DIR/${test_name}_summary.json" ]; then
            log_info "Test summary saved to ${test_name}_summary.json"
        fi
    else
        log_error "$test_name test failed with exit code $exit_code"
        return $exit_code
    fi
    
    # Даем системе время на восстановление между тестами
    log_info "Allowing system recovery time..."
    sleep 30
}

# Очистка ресурсов
cleanup() {
    log_info "Cleaning up test environment..."
    
    cd "$PROJECT_ROOT"
    
    # Останавливаем систему
    docker-compose -f docker/docker-compose.yml down || true
    
    # Останавливаем мониторинг (опционально)
    if [ "${KEEP_MONITORING:-false}" != "true" ]; then
        docker-compose -f docker/monitoring-compose.yml down || true
        log_info "Monitoring stopped. Use KEEP_MONITORING=true to keep it running."
    else
        log_info "Monitoring kept running for analysis"
    fi
    
    log_success "Cleanup completed"
}

# Обработка сигналов для корректной очистки
trap cleanup EXIT INT TERM

# Главная функция
main() {
    echo "🚀 Diplom Distributed System - Phase 4 Load Testing"
    echo "=================================================="
    echo ""
    
    # Создаем директорию для результатов
    mkdir -p "$RESULTS_DIR"
    
    # Проверяем зависимости
    check_dependencies
    
    # Настраиваем мониторинг
    setup_monitoring
    
    # Сохраняем информацию о тестовом прогоне
    cat > "$RESULTS_DIR/test_info.json" << EOF
{
  "test_run_id": "$(date +%Y%m%d_%H%M%S)",
  "start_time": "$(date -Iseconds)",
  "system_info": {
    "os": "$(uname -s)",
    "arch": "$(uname -m)",
    "kernel": "$(uname -r)"
  },
  "tools": {
    "k6_version": "$(k6 version | head -1)",
    "docker_version": "$(docker --version)",
    "compose_version": "$(docker-compose --version)"
  }
}
EOF
    
    log_info "Starting comprehensive load testing suite..."
    log_info "Results directory: $RESULTS_DIR"
    
    # 1. Smoke Test - базовая проверка функциональности
    setup_system 1
    run_test "smoke" "k6/scenarios/smoke.js" "2 minutes"
    
    # 2. Baseline Test - производительность одного инстанса  
    log_info "=== BASELINE TEST (1 instance) ==="
    setup_system 1
    run_test "baseline" "k6/scenarios/load.js" "16 minutes"
    
    # 3. Scaled Test - производительность с масштабированием
    log_info "=== SCALED TEST (4 processor instances) ==="
    setup_system 4
    run_test "scaled" "k6/scenarios/load.js" "16 minutes"
    
    # 4. Stress Test - экстремальная нагрузка
    log_info "=== STRESS TEST (spike load) ==="
    setup_system 2
    run_test "stress" "k6/scenarios/spike.js" "5 minutes"
    
    # 5. Soak Test - длительная стабильность (2 часа)
    if [ "${SKIP_SOAK:-false}" != "true" ]; then
        log_info "=== SOAK TEST (2 hours) ==="
        setup_system 2
        run_test "soak" "k6/scenarios/soak.js" "2 hours"
    else
        log_warning "Soak test skipped (SKIP_SOAK=true)"
    fi
    
    # Финальная информация
    echo ""
    echo "🎉 Load Testing Completed Successfully!"
    echo "======================================"
    echo ""
    echo "📊 Results: $RESULTS_DIR"
    echo "📈 Monitoring: http://localhost:3000"
    echo ""
    echo "💡 Next steps:"
    echo "  1. Review test results and Grafana dashboards"
    echo "  2. Analyze bottlenecks and optimization opportunities"  
    echo "  3. Plan Phase 5 based on performance insights"
    echo ""
}

# Обработка аргументов командной строки
case "${1:-}" in
    "smoke")
        check_dependencies
        setup_monitoring
        setup_system 1
        run_test "smoke" "k6/scenarios/smoke.js" "2 minutes"
        ;;
    "load")
        check_dependencies
        setup_monitoring
        setup_system 1
        run_test "load" "k6/scenarios/load.js" "16 minutes"
        ;;
    "spike")
        check_dependencies
        setup_monitoring
        setup_system 2
        run_test "spike" "k6/scenarios/spike.js" "5 minutes"
        ;;
    "soak")
        check_dependencies
        setup_monitoring
        setup_system 2
        run_test "soak" "k6/scenarios/soak.js" "2 hours"
        ;;
    *)
        main
        ;;
esac 