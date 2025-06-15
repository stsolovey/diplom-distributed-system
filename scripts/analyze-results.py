#!/usr/bin/env python3
"""
Анализатор результатов нагрузочного тестирования k6
Создает детальные отчеты и графики на основе JSON результатов
"""

import json
import os
import sys
import argparse
from pathlib import Path
from datetime import datetime
import statistics
from typing import Dict, List, Any

def load_k6_results(file_path: str) -> List[Dict]:
    """Загружает результаты k6 из JSON файла"""
    results = []
    try:
        with open(file_path, 'r') as f:
            for line in f:
                if line.strip():
                    results.append(json.loads(line))
        return results
    except Exception as e:
        print(f"Error loading results from {file_path}: {e}")
        return []

def extract_metrics(results: List[Dict]) -> Dict[str, List]:
    """Извлекает метрики из результатов k6"""
    metrics = {
        'http_req_duration': [],
        'http_req_failed': [],
        'http_reqs': [],
        'vus': [],
        'iterations': [],
        'timestamps': []
    }
    
    for result in results:
        if result.get('type') == 'Point':
            metric_name = result.get('metric')
            value = result.get('data', {}).get('value', 0)
            timestamp = result.get('data', {}).get('time')
            
            if metric_name in metrics:
                metrics[metric_name].append(value)
                if timestamp:
                    metrics['timestamps'].append(datetime.fromisoformat(timestamp.replace('Z', '+00:00')))
    
    return metrics

def calculate_percentiles(data: List[float]) -> Dict[str, float]:
    """Вычисляет перцентили для данных"""
    if not data:
        return {}
    
    return {
        'p50': statistics.quantiles(data, n=2)[0] if len(data) > 1 else data[0],
        'p95': statistics.quantiles(data, n=20)[18] if len(data) > 19 else max(data),
        'p99': statistics.quantiles(data, n=100)[98] if len(data) > 99 else max(data),
        'min': min(data),
        'max': max(data),
        'avg': statistics.mean(data),
        'median': statistics.median(data)
    }

def generate_performance_report(test_name: str, metrics: Dict[str, List]) -> str:
    """Генерирует текстовый отчет по производительности"""
    report = f"# Performance Report: {test_name}\n\n"
    report += f"Generated: {datetime.now().isoformat()}\n\n"
    
    # Анализ времени ответа
    if metrics['http_req_duration']:
        duration_stats = calculate_percentiles(metrics['http_req_duration'])
        report += "## Response Time Analysis\n\n"
        report += f"- **Average**: {duration_stats['avg']:.2f}ms\n"
        report += f"- **Median (P50)**: {duration_stats['median']:.2f}ms\n"
        report += f"- **P95**: {duration_stats['p95']:.2f}ms\n"
        report += f"- **P99**: {duration_stats['p99']:.2f}ms\n"
        report += f"- **Min**: {duration_stats['min']:.2f}ms\n"
        report += f"- **Max**: {duration_stats['max']:.2f}ms\n\n"
        
        # Оценка производительности
        if duration_stats['p95'] < 100:
            report += "✅ **Excellent**: P95 latency under 100ms\n\n"
        elif duration_stats['p95'] < 200:
            report += "✅ **Good**: P95 latency under 200ms\n\n"
        elif duration_stats['p95'] < 500:
            report += "⚠️ **Acceptable**: P95 latency under 500ms\n\n"
        else:
            report += "❌ **Poor**: P95 latency exceeds 500ms\n\n"
    
    # Анализ ошибок
    if metrics['http_req_failed']:
        failed_requests = sum(metrics['http_req_failed'])
        total_requests = len(metrics['http_req_failed'])
        error_rate = (failed_requests / total_requests * 100) if total_requests > 0 else 0
        
        report += "## Error Analysis\n\n"
        report += f"- **Total Requests**: {total_requests}\n"
        report += f"- **Failed Requests**: {failed_requests}\n"
        report += f"- **Error Rate**: {error_rate:.2f}%\n\n"
        
        if error_rate < 1:
            report += "✅ **Excellent**: Error rate under 1%\n\n"
        elif error_rate < 5:
            report += "✅ **Good**: Error rate under 5%\n\n"
        elif error_rate < 10:
            report += "⚠️ **Concerning**: Error rate under 10%\n\n"
        else:
            report += "❌ **Poor**: Error rate exceeds 10%\n\n"
    
    # Анализ throughput
    if metrics['http_reqs']:
        total_requests = len(metrics['http_reqs'])
        if metrics['timestamps'] and len(metrics['timestamps']) > 1:
            duration_seconds = (metrics['timestamps'][-1] - metrics['timestamps'][0]).total_seconds()
            rps = total_requests / duration_seconds if duration_seconds > 0 else 0
            
            report += "## Throughput Analysis\n\n"
            report += f"- **Total Requests**: {total_requests}\n"
            report += f"- **Test Duration**: {duration_seconds:.1f}s\n"
            report += f"- **Average RPS**: {rps:.1f}\n\n"
            
            if rps >= 1000:
                report += "🏆 **Excellent**: Throughput exceeds 1000 RPS\n\n"
            elif rps >= 500:
                report += "✅ **Good**: Throughput exceeds 500 RPS\n\n"
            elif rps >= 100:
                report += "⚠️ **Acceptable**: Throughput exceeds 100 RPS\n\n"
            else:
                report += "❌ **Poor**: Throughput below 100 RPS\n\n"
    
    return report

def analyze_test_results(results_dir: str, test_name: str):
    """Анализирует результаты конкретного теста"""
    
    results_file = f"{results_dir}/{test_name}_results.json"
    if not os.path.exists(results_file):
        print(f"Results file not found: {results_file}")
        return
    
    print(f"Analyzing {test_name} test results...")
    
    # Загружаем и обрабатываем результаты
    results = load_k6_results(results_file)
    if not results:
        print(f"No valid results found in {results_file}")
        return
    
    metrics = extract_metrics(results)
    
    # Генерируем отчет
    report = generate_performance_report(test_name, metrics)
    
    # Сохраняем отчет
    report_file = f"{results_dir}/{test_name}_analysis.md"
    with open(report_file, 'w') as f:
        f.write(report)
    
    print(f"Analysis report saved: {report_file}")

def compare_test_results(results_dir: str, test_names: List[str]):
    """Сравнивает результаты нескольких тестов"""
    
    comparison_data = {}
    
    for test_name in test_names:
        results_file = f"{results_dir}/{test_name}_results.json"
        if not os.path.exists(results_file):
            print(f"Skipping {test_name} - results file not found")
            continue
        
        results = load_k6_results(results_file)
        if not results:
            continue
        
        metrics = extract_metrics(results)
        
        if metrics['http_req_duration']:
            duration_stats = calculate_percentiles(metrics['http_req_duration'])
            comparison_data[test_name] = duration_stats
    
    if comparison_data:
        # Создаем сравнительный отчет
        comparison_report = "# Test Comparison Report\n\n"
        comparison_report += "| Test | P50 (ms) | P95 (ms) | P99 (ms) | Avg (ms) | Max (ms) |\n"
        comparison_report += "|------|----------|----------|----------|----------|----------|\n"
        
        for test_name, stats in comparison_data.items():
            comparison_report += f"| {test_name} | {stats['p50']:.1f} | {stats['p95']:.1f} | {stats['p99']:.1f} | {stats['avg']:.1f} | {stats['max']:.1f} |\n"
        
        comparison_report += "\n## Analysis\n\n"
        
        # Находим лучший и худший результаты
        best_p95 = min(comparison_data.items(), key=lambda x: x[1]['p95'])
        worst_p95 = max(comparison_data.items(), key=lambda x: x[1]['p95'])
        
        comparison_report += f"- **Best P95 latency**: {best_p95[0]} ({best_p95[1]['p95']:.1f}ms)\n"
        comparison_report += f"- **Worst P95 latency**: {worst_p95[0]} ({worst_p95[1]['p95']:.1f}ms)\n\n"
        
        # Сохраняем сравнительный отчет
        comparison_file = f"{results_dir}/comparison_report.md"
        with open(comparison_file, 'w') as f:
            f.write(comparison_report)
        
        print(f"Comparison report saved: {comparison_file}")

def main():
    parser = argparse.ArgumentParser(description='Analyze k6 load test results')
    parser.add_argument('results_dir', help='Directory containing test results')
    parser.add_argument('--test', help='Specific test to analyze')
    parser.add_argument('--compare', nargs='+', help='Tests to compare')
    parser.add_argument('--all', action='store_true', help='Analyze all tests in directory')
    
    args = parser.parse_args()
    
    if not os.path.exists(args.results_dir):
        print(f"Results directory not found: {args.results_dir}")
        sys.exit(1)
    
    if args.test:
        analyze_test_results(args.results_dir, args.test)
    elif args.compare:
        compare_test_results(args.results_dir, args.compare)
    elif args.all:
        # Находим все JSON файлы результатов
        test_names = []
        for file in os.listdir(args.results_dir):
            if file.endswith('_results.json'):
                test_name = file.replace('_results.json', '')
                test_names.append(test_name)
        
        if test_names:
            print(f"Found tests: {', '.join(test_names)}")
            
            # Анализируем каждый тест
            for test_name in test_names:
                analyze_test_results(args.results_dir, test_name)
            
            # Создаем сравнительный отчет
            if len(test_names) > 1:
                compare_test_results(args.results_dir, test_names)
        else:
            print("No test result files found")
    else:
        parser.print_help()

if __name__ == '__main__':
    main() 