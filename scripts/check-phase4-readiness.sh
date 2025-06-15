#!/bin/bash
# Phase 4 Readiness Check
# Проверяет готовность системы к нагрузочному тестированию

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Счетчики
CHECKS_PASSED=0
CHECKS_FAILED=0
WARNINGS=0

# Функции вывода
print_header() {
    echo -e "${BLUE}===================================================${NC}"
    echo -e "${BLUE}  Phase 4 Load Testing Readiness Check${NC}"
    echo -e "${BLUE}===================================================${NC}"
    echo ""
}

print_section() {
    echo -e "${BLUE}--- $1 ---${NC}"
}

print_check() {
    local check_name="$1"
    local status="$2"
    local message="$3"
    
    printf "%-40s" "$check_name"
    
    case $status in
        "PASS")
            echo -e "${GREEN}✅ PASS${NC} $message"
            ((CHECKS_PASSED++))
            ;;
        "FAIL")
            echo -e "${RED}❌ FAIL${NC} $message"
            ((CHECKS_FAILED++))
            ;;
        "WARN")
            echo -e "${YELLOW}⚠️  WARN${NC} $message"
            ((WARNINGS++))
            ;;
    esac
}

# Проверка команд
check_command() {
    local cmd="$1"
    local package="$2"
    local install_hint="$3"
    
    if command -v "$cmd" >/dev/null 2>&1; then
        local version
        case $cmd in
            "k6")
                version=$(k6 version | head -1 | awk '{print $2}')
                ;;
            "docker")
                version=$(docker --version | awk '{print $3}' | sed 's/,//')
                ;;
            "docker-compose")
                version=$(docker-compose --version | awk '{print $3}' | sed 's/,//')
                ;;
            "python3")
                version=$(python3 --version | awk '{print $2}')
                ;;
            *)
                version=$($cmd --version 2>/dev/null | head -1 | awk '{print $NF}' || echo "unknown")
                ;;
        esac
        print_check "$package" "PASS" "($version)"
    else
        print_check "$package" "FAIL" "$install_hint"
    fi
}

# Проверка портов
check_port() {
    local port="$1"
    local service="$2"
    
    if lsof -i :$port >/dev/null 2>&1; then
        print_check "Port $port ($service)" "WARN" "Port is already in use"
    else
        print_check "Port $port ($service)" "PASS" "Available"
    fi
}

# Проверка Docker
check_docker_status() {
    if docker info >/dev/null 2>&1; then
        print_check "Docker daemon" "PASS" "Running"
    else
        print_check "Docker daemon" "FAIL" "Not running or permission denied"
    fi
}

# Проверка дискового пространства
check_disk_space() {
    local available_gb
    available_gb=$(df . | awk 'NR==2 {print int($4/1024/1024)}')
    
    if [ "$available_gb" -gt 10 ]; then
        print_check "Disk space" "PASS" "${available_gb}GB available"
    elif [ "$available_gb" -gt 5 ]; then
        print_check "Disk space" "WARN" "${available_gb}GB available (recommended: 10GB+)"
    else
        print_check "Disk space" "FAIL" "${available_gb}GB available (minimum: 5GB)"
    fi
}

# Проверка памяти
check_memory() {
    if command -v free >/dev/null 2>&1; then
        local available_gb
        available_gb=$(free -g | awk '/^Mem:/ {print $7}')
        
        if [ "$available_gb" -gt 4 ]; then
            print_check "Available memory" "PASS" "${available_gb}GB available"
        elif [ "$available_gb" -gt 2 ]; then
            print_check "Available memory" "WARN" "${available_gb}GB available (recommended: 4GB+)"
        else
            print_check "Available memory" "FAIL" "${available_gb}GB available (minimum: 2GB)"
        fi
    else
        print_check "Available memory" "WARN" "Cannot check (free command not available)"
    fi
}

# Проверка CPU
check_cpu() {
    local cpu_cores
    cpu_cores=$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo "unknown")
    
    if [ "$cpu_cores" != "unknown" ]; then
        if [ "$cpu_cores" -ge 4 ]; then
            print_check "CPU cores" "PASS" "$cpu_cores cores"
        elif [ "$cpu_cores" -ge 2 ]; then
            print_check "CPU cores" "WARN" "$cpu_cores cores (recommended: 4+)"
        else
            print_check "CPU cores" "FAIL" "$cpu_cores cores (minimum: 2)"
        fi
    else
        print_check "CPU cores" "WARN" "Cannot determine CPU count"
    fi
}

# Проверка файлов проекта
check_project_files() {
    local files=(
        "k6/scenarios/smoke.js"
        "k6/scenarios/load.js"
        "k6/scenarios/spike.js"
        "k6/scenarios/soak.js"
        "docker/monitoring-compose.yml"
        "docker/monitoring/prometheus.yml"
        "scripts/run-all-tests.sh"
        "scripts/analyze-results.py"
    )
    
    for file in "${files[@]}"; do
        if [ -f "$file" ]; then
            print_check "$file" "PASS" ""
        else
            print_check "$file" "FAIL" "File missing"
        fi
    done
}

# Основная функция
main() {
    print_header
    
    print_section "System Requirements"
    check_cpu
    check_memory
    check_disk_space
    echo ""
    
    print_section "Required Tools"
    check_command "k6" "k6 load testing tool" "Run: make install-k6"
    check_command "docker" "Docker" "Install from docker.com"
    check_command "docker-compose" "Docker Compose" "Install docker-compose"
    check_command "python3" "Python 3" "Install python3"
    check_command "curl" "curl" "Install curl"
    check_command "jq" "jq JSON processor" "Install jq"
    echo ""
    
    print_section "Docker Environment"
    check_docker_status
    echo ""
    
    print_section "Port Availability"
    check_port 8080 "API Gateway"
    check_port 8081 "Ingest Service"
    check_port 8082 "Processor Service"
    check_port 9090 "Prometheus"
    check_port 3000 "Grafana"
    check_port 9093 "AlertManager"
    echo ""
    
    print_section "Project Files"
    check_project_files
    echo ""
    
    # Итоговая сводка
    echo -e "${BLUE}===================================================${NC}"
    echo -e "${BLUE}  Summary${NC}"
    echo -e "${BLUE}===================================================${NC}"
    echo ""
    echo -e "Checks passed:  ${GREEN}$CHECKS_PASSED${NC}"
    echo -e "Checks failed:  ${RED}$CHECKS_FAILED${NC}"
    echo -e "Warnings:       ${YELLOW}$WARNINGS${NC}"
    echo ""
    
    # Рекомендации
    if [ $CHECKS_FAILED -eq 0 ]; then
        if [ $WARNINGS -eq 0 ]; then
            echo -e "${GREEN}🎉 System is fully ready for Phase 4 load testing!${NC}"
            echo ""
            echo -e "Next steps:"
            echo -e "  ${BLUE}make phase4-run${NC}     # Run complete load testing suite"
            echo -e "  ${BLUE}make phase4-demo${NC}    # Quick demo with monitoring"
        else
            echo -e "${YELLOW}⚠️  System is mostly ready with minor issues${NC}"
            echo ""
            echo -e "You can proceed with:"
            echo -e "  ${BLUE}make phase4-run${NC}     # Run complete load testing suite"
        fi
    else
        echo -e "${RED}❌ System is not ready for Phase 4${NC}"
        echo ""
        echo -e "Required fixes:"
        echo -e "  - Install missing tools (see failures above)"
        echo -e "  - Fix Docker issues if any"
        echo -e "  - Ensure sufficient system resources"
    fi
    
    echo ""
}

# Запуск
main "$@" 