package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"voxly/pkg/logger"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

const (
	QueueNameVoiceProcessing = "voice_processing"
	ExchangeName             = "voxly"
)

type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	url     string
}

// New RabbitMQ client
func NewRabbitMQ(url string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Declare exchange
	err = ch.ExchangeDeclare(
		ExchangeName, // name
		"direct",     // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare queue
	_, err = ch.QueueDeclare(
		QueueNameVoiceProcessing, // name
		true,                     // durable
		false,                    // delete when unused
		false,                    // exclusive
		false,                    // no-wait
		nil,                      // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange
	err = ch.QueueBind(
		QueueNameVoiceProcessing, // queue name
		QueueNameVoiceProcessing, // routing key
		ExchangeName,             // exchange
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to bind queue: %w", err)
	}

	logger.Info("RabbitMQ connected successfully")

	return &RabbitMQ{
		conn:    conn,
		channel: ch,
		url:     url,
	}, nil
}

// Publish publishes a message to the queue
func (r *RabbitMQ) Publish(queueName string, body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := r.channel.PublishWithContext(
		ctx,
		ExchangeName, // exchange
		queueName,    // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	logger.Debug("Message published to queue",
		zap.String("queue", queueName),
		zap.Int("size", len(body)))

	return nil
}

// PublishTask publishes a VoiceTask to the queue
func (r *RabbitMQ) PublishTask(task *VoiceTask) error {
	body, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	return r.Publish(QueueNameVoiceProcessing, body)
}

// Consume starts consuming messages from the queue
func (r *RabbitMQ) Consume(queueName string, handler func([]byte) error) error {
	// Set QoS
	err := r.channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	msgs, err := r.channel.Consume(
		queueName, // queue
		"",        // consumer
		false,     // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	logger.Info("Starting to consume messages", zap.String("queue", queueName))

	for msg := range msgs {
		logger.Debug("Received message", zap.Int("size", len(msg.Body)))

		err := handler(msg.Body)
		if err != nil {
			logger.Error("Failed to handle message", zap.Error(err))
			// Reject and requeue
			msg.Nack(false, true)
		} else {
			// Acknowledge
			msg.Ack(false)
		}
	}

	return nil
}

// Close RabbitMQ connection
func (r *RabbitMQ) Close() error {
	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}
