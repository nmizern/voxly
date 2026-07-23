package queue

type Publisher interface {
	PublishTask(task *VoiceTask) error
}

type Consumer interface {
	Consume(queueName string, concurrency int, handler func([]byte) error) error
}

type Queue interface {
	Publisher
	Consumer
	Close() error
}
