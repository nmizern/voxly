# Voxly

Self-hosted Telegram bot that transcribes voice and video messages to text. Pick your speech-to-text provider (Yandex, OpenAI, Groq, Deepgram or a self-hosted Whisper) and run it your way: a single binary for personal use, or a scalable split setup for busy chats.

## Overview

Forward a voice or video message to the bot, or add it to a chat, and it replies with the transcription. Self-hosting keeps your audio between your own server and the STT provider you choose, no extra middlemen.

**Stack**: Go 1.23, pluggable STT providers, optional PostgreSQL / Redis / RabbitMQ / S3, Docker.

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Telegram Bot Token ([@BotFather](https://t.me/BotFather))
- An API key for one STT provider (or a self-hosted Whisper server)

### Lite (single process, no infrastructure)

Best for personal use. No Postgres/Redis/RabbitMQ needed.

```bash
git clone https://github.com/nmizern/voxly.git
cd voxly
cp .env.example .env
# Set TELEGRAM_BOT_TOKEN, STT_PROVIDER and that provider's key
docker compose -f docker-compose.lite.yaml up -d --build
```

### Scale (bot + worker behind RabbitMQ/Redis/Postgres)

Best for busy multi-user chats.

```bash
cp .env.example .env
# Fill in credentials
docker compose up -d --build
```

Then in Telegram: send `/start`, then a voice or video message.

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

One binary (`voxly`) runs as `bot`, `worker`, or both (`all`), selected by role. The deployment mode wires the infrastructure:

- **lite**: `all` role in one process with an in-memory queue, cache and store. No external services.
- **scale**: separate `bot` and `worker` processes decoupled by RabbitMQ, with Redis cache and Postgres storage.

```
Telegram → bot → queue → worker → STT provider → reply
                          store / cache
```

The STT provider is an interface. Yandex stages audio in object storage and recognises by URI; OpenAI, Groq, Deepgram and Whisper receive the audio directly (no S3).

Voice messages, video notes (кружочки) and anything forwarded to the bot in a private chat are transcribed. Video notes have their audio pulled out with ffmpeg, which is bundled in the Docker image (install it yourself for a bare `go run`).

**Resilience**: circuit breaker, exponential backoff, rate limiting. The worker runs a bounded pool of concurrent consumers (`WORKER_CONCURRENCY`).

**Access**: in `allowlist` mode only listed users/chats and admins are served, with an optional per user daily quota to cap API spend.

**Observability**: Prometheus metrics and a `/healthz` liveness probe are served on `METRICS_ADDR` (`:9090`).

## Development

### Project Structure

```
cmd/voxly/          # single entrypoint (role-driven)
internal/
  app/              # dependency wiring for both modes
  bot/              # Telegram handlers
  worker/           # transcription processing
  stt/              # Transcriber interface + providers
  storage/          # Store interface (postgres, memory) + S3
  queue/            # Queue interface (rabbitmq, memory)
  config/           # typed configuration
pkg/
  cache/            # Cache interface (redis, memory)
  resilience/       # circuit breaker, retry, rate limiter
  logger/           # structured logging
migrations/         # database migrations
```

### Build & Test

```bash
go build -o bin/voxly ./cmd/voxly
go test ./...

# Run locally (lite)
VOXLY_MODE=lite STT_PROVIDER=openai OPENAI_API_KEY=... \
  TELEGRAM_BOT_TOKEN=... go run ./cmd/voxly
```

## Deployment

```bash
# Scale mode
docker compose up -d --build
docker compose logs -f
docker compose up -d --scale worker=3

# Lite mode
docker compose -f docker-compose.lite.yaml up -d --build
```

## License

MIT License - see [LICENSE](LICENSE) file.

---