package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type NATSBroker struct {
	nc     *nats.Conn
	js     jetstream.JetStream
	config NATSConfig
}

type NATSConfig struct {
	URL           string
	StreamName    string
	SubjectPrefix string
	MaxReconnects int
	ReconnectWait time.Duration
}

const (
	natsBrokerMaxAge  = 24 * time.Hour
	natsBrokerMaxMsgs = 1000000
)

var ErrNATSBrokerClose = errors.New("errors closing NATS broker")

// NewNATSBroker создает новое подключение к NATS.
func NewNATSBroker(cfg NATSConfig) (*NATSBroker, error) {
	opts := []nats.Option{
		nats.MaxReconnects(cfg.MaxReconnects),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Printf("NATS disconnected: %v", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("NATS reconnected to %s", nc.ConnectedUrl())
		}),
	}

	nc, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()

		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	broker := &NATSBroker{
		nc:     nc,
		js:     js,
		config: cfg,
	}

	// Создаем stream если не существует
	if err := broker.ensureStream(); err != nil {
		nc.Close()

		return nil, err
	}

	return broker, nil
}

// ensureStream создает JetStream stream если он не существует.
func (b *NATSBroker) ensureStream() error {
	// Проверяем существование stream
	_, err := b.js.Stream(context.Background(), b.config.StreamName)
	if err == nil {
		// Stream уже существует
		log.Printf("Using existing stream: %s", b.config.StreamName)

		return nil
	}

	// Проверяем, действительно ли stream не найден
	if !isStreamNotFoundError(err) {
		// Это другая ошибка, не связанная с отсутствием stream
		return fmt.Errorf("failed to check stream existence: %w", err)
	}

	// Создаем новый stream
	streamConfig := jetstream.StreamConfig{
		Name:      b.config.StreamName,
		Subjects:  []string{b.config.SubjectPrefix + ".*"},
		Retention: jetstream.WorkQueuePolicy,
		MaxAge:    natsBrokerMaxAge,
		MaxMsgs:   natsBrokerMaxMsgs,
		Storage:   jetstream.FileStorage,
	}

	_, err = b.js.CreateStream(context.Background(), streamConfig)
	if err != nil {
		// Проверяем, не был ли stream создан между временем проверки и созданием
		if isStreamAlreadyExistsError(err) {
			log.Printf("Stream was created concurrently: %s", b.config.StreamName)

			return nil
		}

		return fmt.Errorf("failed to create stream: %w", err)
	}

	log.Printf("Created new stream: %s", b.config.StreamName)

	return nil
}

// isStreamNotFoundError проверяет, является ли ошибка "stream не найден".
func isStreamNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()

	return errMsg == "stream not found" ||
		errMsg == "nats: stream not found" ||
		errMsg == "jetstream stream not found" ||
		// API error с кодом 10059 означает "stream not found"
		errMsg == "nats: API error: code=404 err_code=10059 description=stream not found"
}

// isStreamAlreadyExistsError проверяет, уже ли существует stream.
func isStreamAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()

	return errMsg == "stream name already in use" ||
		errMsg == "nats: stream name already in use"
}

// Close закрывает соединение с NATS.
func (b *NATSBroker) Close() error {
	b.nc.Close()

	return nil
}
