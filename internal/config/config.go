package config

import (
	"os"
	"strconv"
)

type Config struct {
	// API Gateway настройки
	APIPort string

	// Ingest Service настройки
	IngestPort string

	// Processor настройки
	ProcessorPort    string
	ProcessorWorkers int
	ProcessorURL     string // для HTTP bridge

	// Размер очереди
	QueueSize int
}

// LoadConfig загружает конфигурацию из переменных окружения
func LoadConfig() *Config {
	return &Config{
		APIPort:          getEnv("API_PORT", "8080"),
		IngestPort:       getEnv("INGEST_PORT", "8081"),
		ProcessorPort:    getEnv("PROCESSOR_PORT", "8082"),
		ProcessorWorkers: getEnvAsInt("PROCESSOR_WORKERS", 4),
		ProcessorURL:     getEnv("PROCESSOR_URL", "http://localhost:8082"),
		QueueSize:        getEnvAsInt("QUEUE_SIZE", 1000),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}
