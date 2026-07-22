package config

import (
	"strings"
	"testing"
)

func TestApplyDefaults_DriversFollowMode(t *testing.T) {
	tests := []struct {
		mode                   Mode
		queue, cache, database string
	}{
		{ModeLite, "memory", "memory", "memory"},
		{ModeScale, "rabbitmq", "redis", "postgres"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			c := &Config{Mode: tt.mode}
			c.applyDefaults()

			if c.Queue.Driver != tt.queue {
				t.Errorf("queue driver = %q, want %q", c.Queue.Driver, tt.queue)
			}
			if c.Cache.Driver != tt.cache {
				t.Errorf("cache driver = %q, want %q", c.Cache.Driver, tt.cache)
			}
			if c.Database.Driver != tt.database {
				t.Errorf("database driver = %q, want %q", c.Database.Driver, tt.database)
			}
		})
	}
}

func TestApplyDefaults_KeepsExplicitDrivers(t *testing.T) {
	c := &Config{Mode: ModeLite, Queue: Queue{Driver: "rabbitmq"}}
	c.applyDefaults()

	if c.Queue.Driver != "rabbitmq" {
		t.Errorf("explicit driver overridden: got %q", c.Queue.Driver)
	}
}

func TestValidate_ValidLiteOpenAI(t *testing.T) {
	c := &Config{
		Mode:     ModeLite,
		Telegram: Telegram{Token: "token"},
		STT:      STT{Provider: ProviderOpenAI, OpenAI: OpenAISTT{APIKey: "sk-x"}},
		Worker:   Worker{Concurrency: 4},
		Access:   Access{Mode: "open"},
	}
	c.applyDefaults()

	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
}

func TestValidate_ReportsAllMissing(t *testing.T) {
	c := &Config{Mode: ModeScale, STT: STT{Provider: ProviderYandex}}
	c.applyDefaults()

	err := c.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	for _, want := range []string{"TELEGRAM_BOT_TOKEN", "YANDEX_API_KEY", "S3_ENDPOINT", "RABBITMQ_URL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q:\n%v", want, err)
		}
	}
}

func TestValidate_UnknownProvider(t *testing.T) {
	c := &Config{
		Mode:     ModeLite,
		Telegram: Telegram{Token: "token"},
		STT:      STT{Provider: "invalid"},
		Worker:   Worker{Concurrency: 1},
		Access:   Access{Mode: "open"},
	}
	c.applyDefaults()

	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("expected unknown provider error, got: %v", err)
	}
}
