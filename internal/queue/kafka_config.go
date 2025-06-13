package queue

import (
	"time"

	"github.com/IBM/sarama"
)

const (
	// Timeouts configuration.
	kafkaProducerTimeout = 10 * time.Second
	kafkaNetDialTimeout  = 10 * time.Second
	kafkaNetReadTimeout  = 10 * time.Second
	kafkaNetWriteTimeout = 10 * time.Second

	// Consumer configuration.
	kafkaSessionTimeout    = 20 * time.Second
	kafkaHeartbeatInterval = 6 * time.Second
	kafkaFetchDefaultSize  = 1024 * 1024 // 1MB.

	// Retry configuration.
	kafkaProducerRetryMax = 5
)

// getKafkaProducerConfig returns configuration for Producer.
func getKafkaProducerConfig() *sarama.Config {
	config := sarama.NewConfig()
	config.Version = sarama.V3_5_0_0

	// Idempotent producer settings.
	config.Producer.Idempotent = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = kafkaProducerRetryMax
	config.Producer.Return.Successes = true

	// Required for idempotency.
	config.Net.MaxOpenRequests = 1

	// Timeouts.
	config.Producer.Timeout = kafkaProducerTimeout
	config.Net.DialTimeout = kafkaNetDialTimeout
	config.Net.ReadTimeout = kafkaNetReadTimeout
	config.Net.WriteTimeout = kafkaNetWriteTimeout

	// Compression.
	config.Producer.Compression = sarama.CompressionSnappy

	return config
}

// getKafkaConsumerConfig returns configuration for Consumer.
func getKafkaConsumerConfig() *sarama.Config {
	config := sarama.NewConfig()
	config.Version = sarama.V3_5_0_0

	// Consumer group settings.
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetOldest // Read from beginning.
	config.Consumer.Offsets.AutoCommit.Enable = true
	config.Consumer.Offsets.AutoCommit.Interval = 1 * time.Second

	// Session timeouts.
	config.Consumer.Group.Session.Timeout = kafkaSessionTimeout
	config.Consumer.Group.Heartbeat.Interval = kafkaHeartbeatInterval

	// Processing settings.
	config.Consumer.MaxProcessingTime = 1 * time.Minute
	config.Consumer.Fetch.Default = kafkaFetchDefaultSize

	return config
}
