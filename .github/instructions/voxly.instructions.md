# Voxly Project Instructions

## Overview

**Voxly** is a self hosted Telegram bot written in Go that transcribes voice
messages and video notes (кружочки) to text. The speech to text provider is
pluggable and the deployment topology is configurable, so the same codebase
runs as a single binary for personal use or as a scalable split setup for busy
chats.

## How it works

1. A user sends or forwards a voice message or video note.
2. The bot creates a task and publishes it to the queue.
3. A worker consumes the task, downloads the file, extracts audio from video
   notes with ffmpeg, and sends it to the configured STT provider.
4. The worker stores the transcript and replies to the original message.

## Deployment modes

- **lite** (`VOXLY_MODE=lite`): one process, in memory queue, cache and store,
  no external infrastructure.
- **scale** (`VOXLY_MODE=scale`): separate `bot` and `worker` roles decoupled
  by RabbitMQ, with Redis cache and Postgres storage.

The single `voxly` binary picks its behaviour from `VOXLY_ROLE` (`all`, `bot`
or `worker`); lite always runs `all`.

## STT providers

Selected by `STT_PROVIDER`, behind the `stt.Transcriber` interface:

- `yandex`: SpeechKit long running recognition, stages audio in Object Storage
  and recognises by URI (needs the S3 config).
- `openai`, `groq`, `whisper`: OpenAI compatible `/audio/transcriptions`.
- `deepgram`: pre recorded `/v1/listen`.

Direct upload providers need no object storage, which keeps audio between your
server and the provider. Always check the provider's current docs when touching
its client.

## Tech stack

- Go, telebot v4, pgx, go redis, amqp091, zap.
- Optional Postgres, Redis, RabbitMQ, S3 depending on mode and provider.
- Prometheus metrics and a `/healthz` probe on `METRICS_ADDR`.
- ffmpeg for video note audio extraction.
- Docker and docker compose (`docker-compose.yaml` for scale,
  `docker-compose.lite.yaml` for lite).

## Conventions

- Configuration is env first (`internal/config`), validated on startup.
- Access control: `allowlist` mode plus an optional per user daily quota.
- Keep the bot thin, do heavy work in the worker.
