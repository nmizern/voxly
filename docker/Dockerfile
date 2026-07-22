# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o voxly ./cmd/voxly

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata ffmpeg
WORKDIR /app

COPY --from=builder /app/voxly .
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/configs ./configs

RUN adduser -D -s /bin/sh appuser
USER appuser

CMD ["./voxly"]
