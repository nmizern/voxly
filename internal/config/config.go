package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Mode string

const (
	// ModeLite runs everything in one process with in-memory infra — for
	// personal self-hosting. ModeScale splits bot and worker behind
	// RabbitMQ/Redis/Postgres for busy chats.
	ModeLite  Mode = "lite"
	ModeScale Mode = "scale"
)

type Provider string

const (
	ProviderYandex   Provider = "yandex"
	ProviderOpenAI   Provider = "openai"
	ProviderGroq     Provider = "groq"
	ProviderDeepgram Provider = "deepgram"
	ProviderWhisper  Provider = "whisper"
)

type Config struct {
	Mode Mode   `yaml:"mode" env:"VOXLY_MODE" env-default:"scale"`
	Role string `yaml:"role" env:"VOXLY_ROLE" env-default:"all"` // all | bot | worker

	Telegram      Telegram      `yaml:"telegram"`
	STT           STT           `yaml:"stt"`
	Storage       Storage       `yaml:"storage"`
	Database      Database      `yaml:"database"`
	Queue         Queue         `yaml:"queue"`
	Cache         Cache         `yaml:"cache"`
	Worker        Worker        `yaml:"worker"`
	Access        Access        `yaml:"access"`
	Observability Observability `yaml:"observability"`
}

type Telegram struct {
	Token string `yaml:"token" env:"TELEGRAM_BOT_TOKEN"`
}

type STT struct {
	Provider Provider `yaml:"provider" env:"STT_PROVIDER" env-default:"yandex"`
	Language string   `yaml:"language" env:"STT_LANGUAGE" env-default:"ru-RU"`

	Yandex   YandexSTT   `yaml:"yandex"`
	OpenAI   OpenAISTT   `yaml:"openai"`
	Groq     GroqSTT     `yaml:"groq"`
	Deepgram DeepgramSTT `yaml:"deepgram"`
	Whisper  WhisperSTT  `yaml:"whisper"`
}

// Yandex recognises audio from a URI, so it also needs Storage.S3.
type YandexSTT struct {
	APIKey   string `yaml:"api_key" env:"YANDEX_API_KEY"`
	FolderID string `yaml:"folder_id" env:"YANDEX_FOLDER_ID"`
	Model    string `yaml:"model" env:"YANDEX_MODEL" env-default:"general"`
}

type OpenAISTT struct {
	APIKey  string `yaml:"api_key" env:"OPENAI_API_KEY"`
	BaseURL string `yaml:"base_url" env:"OPENAI_BASE_URL" env-default:"https://api.openai.com/v1"`
	Model   string `yaml:"model" env:"OPENAI_MODEL" env-default:"gpt-4o-transcribe"`
}

type GroqSTT struct {
	APIKey  string `yaml:"api_key" env:"GROQ_API_KEY"`
	BaseURL string `yaml:"base_url" env:"GROQ_BASE_URL" env-default:"https://api.groq.com/openai/v1"`
	Model   string `yaml:"model" env:"GROQ_MODEL" env-default:"whisper-large-v3-turbo"`
}

type DeepgramSTT struct {
	APIKey  string `yaml:"api_key" env:"DEEPGRAM_API_KEY"`
	BaseURL string `yaml:"base_url" env:"DEEPGRAM_BASE_URL" env-default:"https://api.deepgram.com"`
	Model   string `yaml:"model" env:"DEEPGRAM_MODEL" env-default:"nova-2"`
}

// Self-hosted OpenAI-compatible Whisper server; keeps audio on your own box.
type WhisperSTT struct {
	BaseURL string `yaml:"base_url" env:"WHISPER_BASE_URL" env-default:"http://localhost:9000/v1"`
	Model   string `yaml:"model" env:"WHISPER_MODEL" env-default:"whisper-1"`
}

type Storage struct {
	S3 S3 `yaml:"s3"`
}

type S3 struct {
	Endpoint  string `yaml:"endpoint" env:"S3_ENDPOINT"`
	Region    string `yaml:"region" env:"S3_REGION" env-default:"ru-central1"`
	AccessKey string `yaml:"access_key" env:"S3_ACCESS_KEY"`
	SecretKey string `yaml:"secret_key" env:"S3_SECRET_KEY"`
	Bucket    string `yaml:"bucket" env:"S3_BUCKET"`
}

type Database struct {
	Driver string `yaml:"driver" env:"DB_DRIVER"` // postgres | sqlite | none
	DSN    string `yaml:"dsn" env:"DATABASE_URL"`
}

type Queue struct {
	Driver   string   `yaml:"driver" env:"QUEUE_DRIVER"` // rabbitmq | memory
	RabbitMQ RabbitMQ `yaml:"rabbitmq"`
}

type RabbitMQ struct {
	URL string `yaml:"url" env:"RABBITMQ_URL"`
}

type Cache struct {
	Driver string `yaml:"driver" env:"CACHE_DRIVER"` // redis | memory
	Redis  Redis  `yaml:"redis"`
}

type Redis struct {
	Addr     string `yaml:"addr" env:"REDIS_ADDR" env-default:"localhost:6379"`
	Password string `yaml:"password" env:"REDIS_PASSWORD"`
	DB       int    `yaml:"db" env:"REDIS_DB" env-default:"0"`
}

type Worker struct {
	Concurrency int `yaml:"concurrency" env:"WORKER_CONCURRENCY" env-default:"4"`
}

// Access guards a self-hosted bot from unbounded API costs. In "allowlist"
// mode only the listed users/chats are served; admins are always allowed.
type Access struct {
	Mode           string  `yaml:"mode" env:"ACCESS_MODE" env-default:"open"` // open | allowlist
	AllowedUsers   []int64 `yaml:"allowed_user_ids" env:"ACCESS_ALLOWED_USER_IDS"`
	AllowedChats   []int64 `yaml:"allowed_chat_ids" env:"ACCESS_ALLOWED_CHAT_IDS"`
	AdminIDs       []int64 `yaml:"admin_ids" env:"ACCESS_ADMIN_IDS"`
	UserDailyLimit int     `yaml:"user_daily_limit" env:"ACCESS_USER_DAILY_LIMIT" env-default:"0"` // 0 = unlimited
}

type Observability struct {
	LogLevel    string `yaml:"log_level" env:"LOG_LEVEL" env-default:"info"`   // debug | info | warn | error
	LogFormat   string `yaml:"log_format" env:"LOG_FORMAT" env-default:"json"` // json | console
	MetricsAddr string `yaml:"metrics_addr" env:"METRICS_ADDR" env-default:":9090"`
}

// Load reads config from an optional YAML file (CONFIG_PATH, else
// configs/config.yaml) overlaid with environment variables, applies
// mode-aware defaults and validates the result.
func Load() (*Config, error) {
	_ = godotenv.Load()

	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		if _, err := os.Stat("configs/config.yaml"); err == nil {
			path = "configs/config.yaml"
		}
	}

	var cfg Config
	if path != "" {
		if err := cleanenv.ReadConfig(path, &cfg); err != nil {
			return nil, fmt.Errorf("read config file %q: %w", path, err)
		}
	} else if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, fmt.Errorf("read environment config: %w", err)
	}

	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// applyDefaults picks infra drivers based on the mode so a lite deployment
// works with no external services out of the box.
func (c *Config) applyDefaults() {
	if c.Mode == "" {
		c.Mode = ModeScale
	}
	if c.Role == "" {
		c.Role = "all"
	}
	if c.Queue.Driver == "" {
		c.Queue.Driver = pick(c.Mode, "memory", "rabbitmq")
	}
	if c.Cache.Driver == "" {
		c.Cache.Driver = pick(c.Mode, "memory", "redis")
	}
	if c.Database.Driver == "" {
		c.Database.Driver = pick(c.Mode, "memory", "postgres")
	}
}

func pick(mode Mode, lite, scale string) string {
	if mode == ModeLite {
		return lite
	}
	return scale
}

// Validate reports every missing or invalid setting at once.
func (c *Config) Validate() error {
	var errs []string
	require := func(cond bool, msg string) {
		if !cond {
			errs = append(errs, msg)
		}
	}

	require(c.Mode == ModeLite || c.Mode == ModeScale,
		fmt.Sprintf("mode: must be %q or %q, got %q", ModeLite, ModeScale, c.Mode))
	require(c.Telegram.Token != "", "telegram.token (TELEGRAM_BOT_TOKEN) is required")

	switch c.STT.Provider {
	case ProviderYandex:
		require(c.STT.Yandex.APIKey != "", "stt.yandex.api_key (YANDEX_API_KEY) is required")
		require(c.STT.Yandex.FolderID != "", "stt.yandex.folder_id (YANDEX_FOLDER_ID) is required")
		require(c.Storage.S3.Endpoint != "", "storage.s3.endpoint (S3_ENDPOINT) is required for the yandex provider")
		require(c.Storage.S3.AccessKey != "", "storage.s3.access_key (S3_ACCESS_KEY) is required for the yandex provider")
		require(c.Storage.S3.SecretKey != "", "storage.s3.secret_key (S3_SECRET_KEY) is required for the yandex provider")
		require(c.Storage.S3.Bucket != "", "storage.s3.bucket (S3_BUCKET) is required for the yandex provider")
	case ProviderOpenAI:
		require(c.STT.OpenAI.APIKey != "", "stt.openai.api_key (OPENAI_API_KEY) is required")
	case ProviderGroq:
		require(c.STT.Groq.APIKey != "", "stt.groq.api_key (GROQ_API_KEY) is required")
	case ProviderDeepgram:
		require(c.STT.Deepgram.APIKey != "", "stt.deepgram.api_key (DEEPGRAM_API_KEY) is required")
	case ProviderWhisper:
		require(c.STT.Whisper.BaseURL != "", "stt.whisper.base_url (WHISPER_BASE_URL) is required")
	default:
		errs = append(errs, fmt.Sprintf("stt.provider: unknown provider %q", c.STT.Provider))
	}

	switch c.Queue.Driver {
	case "rabbitmq":
		require(c.Queue.RabbitMQ.URL != "", "queue.rabbitmq.url (RABBITMQ_URL) is required for the rabbitmq driver")
	case "memory":
	default:
		errs = append(errs, fmt.Sprintf("queue.driver: unknown driver %q", c.Queue.Driver))
	}

	switch c.Cache.Driver {
	case "redis":
		require(c.Cache.Redis.Addr != "", "cache.redis.addr (REDIS_ADDR) is required for the redis driver")
	case "memory":
	default:
		errs = append(errs, fmt.Sprintf("cache.driver: unknown driver %q", c.Cache.Driver))
	}

	switch c.Database.Driver {
	case "postgres":
		require(c.Database.DSN != "", "database.dsn (DATABASE_URL) is required for the postgres driver")
	case "memory":
	default:
		errs = append(errs, fmt.Sprintf("database.driver: unknown driver %q", c.Database.Driver))
	}

	require(c.Worker.Concurrency > 0, "worker.concurrency must be greater than 0")

	switch c.Role {
	case "all", "bot", "worker":
	default:
		errs = append(errs, fmt.Sprintf("role: must be \"all\", \"bot\" or \"worker\", got %q", c.Role))
	}

	switch c.Access.Mode {
	case "open", "allowlist":
	default:
		errs = append(errs, fmt.Sprintf("access.mode: must be \"open\" or \"allowlist\", got %q", c.Access.Mode))
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

func (c *Config) IsAdmin(userID int64) bool {
	for _, id := range c.Access.AdminIDs {
		if id == userID {
			return true
		}
	}
	return false
}
