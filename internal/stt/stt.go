// Package stt defines the speech-to-text abstraction and its providers.
package stt

import (
	"context"
	"encoding/json"
	"io"
)

// Audio is a single piece of audio to transcribe. Filename carries the
// extension providers use to sniff the format (e.g. "voice.ogg").
type Audio struct {
	Data     io.Reader
	Filename string
	MIME     string
	Duration int // seconds, 0 if unknown
}

type Result struct {
	Text string
	Raw  json.RawMessage
}

// Transcriber turns audio into text. Implementations must be safe for
// concurrent use.
type Transcriber interface {
	Transcribe(ctx context.Context, a Audio) (*Result, error)
	Name() string
}
