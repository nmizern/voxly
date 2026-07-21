# Voxly

Telegram bot for voice message transcription using Yandex SpeechKit.

## Overview

Bot that converts Telegram voice messages to text with Redis caching, RabbitMQ queue, and resilience patterns (circuit breaker, retry, rate limiting).

**Stack**: Go 1.23, PostgreSQL, Redis, RabbitMQ, Yandex SpeechKit, Yandex Object Storage (S3), Docker


## Quick Start

### Prerequisites

- Docker & Docker Compose
- Telegram Bot Token ([@BotFather](https://t.me/BotFather))
- Yandex Cloud Account with API key

### Setup

1. Clone and configure:
```bash
git clone https://github.com/nmizern/voxly.git
cd voxly
cp .env.example .env
# Edit .env with your credentials
```

2. Start services:
```bash
docker-compose up -d
```

3. Test bot in Telegram:
   - Send `/start` to activate
   - Send voice message
   - Receive transcription

## Configuration

Configuration is env-first: every value can be set through an environment
variable, optionally overlaid on `configs/config.yaml` for non-secret defaults.
Copy `.env.example` to `.env` and fill in the secrets for your setup.

Two things drive the rest of the config:

- `VOXLY_MODE` — `scale` (bot + worker behind RabbitMQ/Redis/Postgres) or
  `lite` (single process, in-memory infra). Infra drivers are derived from the
  mode unless set explicitly.
- `STT_PROVIDER` — `yandex`, `openai`, `groq`, `deepgram` or `whisper`. Only the
  selected provider's credentials are required; validation reports anything
  missing on startup.

See `.env.example` for the full list of variables.

## Architecture

```
Telegram → Bot (Go) → RabbitMQ → Worker (Go) → Yandex SpeechKit
                ↓                      ↓              ↓
            PostgreSQL ←──────── Redis Cache    Yandex S3
```

**Flow**: Voice message → Task creation → Queue → Download → S3 Upload → Recognition → Cache → Response

**Patterns**: Circuit Breaker, Exponential Backoff, Rate Limiting (10 req/s)

## Development

### Project Structure

```
cmd/
  bot/main.go              # Bot service entry
  worker/main.go           # Worker service entry
internal/
  bot/                     # Telegram bot logic
  worker/                  # Background processing
  speechkit/               # Yandex API client
  storage/                 # PostgreSQL + S3
  queue/                   # RabbitMQ
pkg/
  cache/                   # Redis cache interface
  resilience/              # Circuit breaker, retry, rate limiter
  logger/                  # Structured logging
migrations/                # Database migrations
```

### Build & Test

```bash
# Build
go build -o bin/bot ./cmd/bot
go build -o bin/worker ./cmd/worker

# Test
go test -v ./...

# Run locally
go run ./cmd/bot
go run ./cmd/worker
```

## Deployment

### Production (Docker Compose)

```bash
# On server
docker compose -f docker-compose.prod.yml up -d --build

# Check status
docker compose -f docker-compose.prod.yml ps

# View logs
docker compose -f docker-compose.prod.yml logs -f

# Scale workers
docker compose -f docker-compose.prod.yml up -d --scale worker=3
```

### Monitoring

```bash
# Redis stats
docker exec voxly-redis redis-cli INFO stats

# PostgreSQL
docker exec voxly-postgres pg_isready

# RabbitMQ
docker exec voxly-rabbitmq rabbitmqctl list_queues
```

## License

MIT License - see [LICENSE](LICENSE) file.

---