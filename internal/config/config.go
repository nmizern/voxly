package config

import (
	"voxly/pkg/logger"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	Telegram struct {
		Token string `yaml:"token" env:"TELEGRAM_BOT_TOKEN"`
	} `yaml:"telegram"`

	RabbitMQ struct {
		URL string `yaml:"url" env:"RABBITMQ_URL"`
	} `yaml:"rabbitmq"`

	SpeechKit struct {
		FolderID string `yaml:"folder_id" env:"YANDEX_FOLDER_ID"`
		APIKey   string `yaml:"api_key" env:"YANDEX_API_KEY"`
	} `yaml:"speechkit"`

	Postgres struct {
		DSN string `yaml:"dsn" env:"POSTGRES_DSN"`
	} `yaml:"postgres"`

	S3 struct {
		Endpoint  string `yaml:"endpoint" env:"S3_ENDPOINT"`
		AccessKey string `yaml:"access_key" env:"S3_ACCESS_KEY"`
		SecretKey string `yaml:"secret_key" env:"S3_SECRET_KEY"`
		Bucket    string `yaml:"bucket" env:"S3_BUCKET"`
	} `yaml:"s3"`

	Redis struct {
		Addr     string `yaml:"addr" env:"REDIS_ADDR" env-default:"localhost:6379"`
		Password string `yaml:"password" env:"REDIS_PASSWORD" env-default:""`
		DB       int    `yaml:"db" env:"REDIS_DB" env-default:"0"`
	} `yaml:"redis"`

	Worker struct {
		Concurrency string `yaml:"concurrency" env:"WORKER_CONCURRENCY" env-default:"4"`
	} `yaml:"worker"`
}

func LoadConfig() (*Config, error) {
	// Load .env file
	_ = godotenv.Load()

	var cfg Config
	if err := cleanenv.ReadConfig("configs/config.yaml", &cfg); err != nil {
		return nil, err
	}

	// Helper for logging
	if err := cleanenv.UpdateEnv(&cfg); err != nil {
		return nil, err
	}

	logger.Info("Config loaded successfully")
	return &cfg, nil
}
