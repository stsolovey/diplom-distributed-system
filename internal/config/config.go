package config

import (
	"os"
	"strconv"
	"strings"
)

const (
	defaultProcessorWorkers = 4
	defaultQueueSize        = 1000
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

	// Queue settings
	QueueType string // "memory", "nats" или "kafka"
	NATSURL   string // URL для подключения к NATS

	// Kafka settings
	KafkaBrokers       []string // список брокеров
	KafkaTopic         string   // топик для сообщений
	KafkaConsumerGroup string   // consumer group name

	// Composite adapter settings
	CompositeProviders []string // например: ["nats", "kafka"]
	CompositeStrategy  string   // "fail-fast" или "best-effort"
}

// LoadConfig загружает конфигурацию из переменных окружения.
func LoadConfig() *Config {
	return &Config{
		APIPort:          getEnv("API_PORT", "8080"),
		IngestPort:       getEnv("INGEST_PORT", "8081"),
		ProcessorPort:    getEnv("PROCESSOR_PORT", "8082"),
		ProcessorWorkers: getEnvAsInt("PROCESSOR_WORKERS", defaultProcessorWorkers),
		ProcessorURL:     getEnv("PROCESSOR_URL", "http://localhost:8082"),
		QueueSize:        getEnvAsInt("QUEUE_SIZE", defaultQueueSize),

		QueueType: getEnv("QUEUE_TYPE", "memory"),
		NATSURL:   getEnv("NATS_URL", "nats://localhost:4222"),

		KafkaBrokers:       getKafkaBrokers(),
		KafkaTopic:         getEnv("KAFKA_TOPIC", "diplom-messages"),
		KafkaConsumerGroup: getEnv("KAFKA_CONSUMER_GROUP", "processor-group"),

		CompositeProviders: getCompositeProviders(),
		CompositeStrategy:  getEnv("COMPOSITE_STRATEGY", "fail-fast"),
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

func getKafkaBrokers() []string {
	brokers := getEnv("KAFKA_BROKERS", "localhost:9092")

	return strings.Split(brokers, ",")
}

func getCompositeProviders() []string {
	providers := getEnv("COMPOSITE_PROVIDERS", "nats,kafka")

	return strings.Split(providers, ",")
}
