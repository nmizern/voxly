package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
	"voxly/pkg/resilience"

	"github.com/google/uuid"
)

const (
	yandexRecognizeURL = "https://transcribe.api.cloud.yandex.net/speech/stt/v2/longRunningRecognize"
	yandexOperationURL = "https://operation.api.cloud.yandex.net/operations"
	yandexPollInterval = 5 * time.Second
	yandexMaxWait      = 30 * time.Minute
)

// ObjectStore is the subset of object storage the Yandex provider needs to
// stage audio for URI-based recognition. *storage.S3Storage satisfies it.
type ObjectStore interface {
	UploadFile(ctx context.Context, key string, body io.Reader, contentType string) (string, error)
	DeleteFile(ctx context.Context, key string) error
}

// Yandex recognises audio via SpeechKit long-running recognition. It stages
// the audio in object storage, transcribes it by URI, then removes it.
type Yandex struct {
	apiKey   string
	folderID string
	model    string
	language string
	store    ObjectStore
	client   *http.Client

	cb *resilience.CircuitBreaker
	rl *resilience.RateLimiter
}

func NewYandex(apiKey, folderID, model, language string, store ObjectStore) *Yandex {
	return &Yandex{
		apiKey:   apiKey,
		folderID: folderID,
		model:    model,
		language: language,
		store:    store,
		client:   &http.Client{Timeout: 30 * time.Second},
		cb:       resilience.NewCircuitBreaker(5, time.Minute),
		rl:       resilience.NewRateLimiter(10, time.Second),
	}
}

func (y *Yandex) Name() string { return "yandex" }

func (y *Yandex) Transcribe(ctx context.Context, a Audio) (*Result, error) {
	ext := path.Ext(a.Filename)
	if ext == "" {
		ext = ".ogg"
	}
	key := path.Join("voice", time.Now().Format("2006/01/02"), uuid.NewString()+ext)

	contentType := a.MIME
	if contentType == "" {
		contentType = "audio/ogg"
	}

	uri, err := y.store.UploadFile(ctx, key, a.Data, contentType)
	if err != nil {
		return nil, fmt.Errorf("stage audio in object storage: %w", err)
	}
	// Best-effort cleanup we don't need to keep the audio around.
	defer func() { _ = y.store.DeleteFile(ctx, key) }()

	opID, err := y.startRecognition(ctx, uri, ext)
	if err != nil {
		return nil, err
	}

	raw, text, err := y.waitForResult(ctx, opID)
	if err != nil {
		return nil, err
	}

	return &Result{Text: text, Raw: raw}, nil
}

func (y *Yandex) startRecognition(ctx context.Context, uri, ext string) (string, error) {
	if err := y.rl.Wait(ctx); err != nil {
		return "", fmt.Errorf("rate limit: %w", err)
	}

	reqBody := yandexRequest{
		Config: yandexConfig{Specification: yandexSpec{
			LanguageCode:      y.language,
			Model:             y.model,
			AudioEncoding:     encodingForExt(ext),
			SampleRateHertz:   48000,
			AudioChannelCount: 1,
			LiteratureText:    true,
		}},
		Audio: yandexAudioSource{URI: uri},
	}

	var opID string
	err := y.cb.Execute(func() error {
		body, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, yandexRecognizeURL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Api-Key "+y.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-folder-id", y.folderID)

		resp, err := y.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("recognition request failed: status=%d body=%s", resp.StatusCode, respBody)
		}

		var op yandexOperation
		if err := json.Unmarshal(respBody, &op); err != nil {
			return err
		}
		opID = op.ID
		return nil
	})
	if err != nil {
		return "", err
	}
	return opID, nil
}

func (y *Yandex) waitForResult(ctx context.Context, opID string) (json.RawMessage, string, error) {
	url := yandexOperationURL + "/" + opID
	deadline := time.Now().Add(yandexMaxWait)

	for {
		if time.Now().After(deadline) {
			return nil, "", fmt.Errorf("recognition timed out after %s", yandexMaxWait)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("Authorization", "Api-Key "+y.apiKey)

		resp, err := y.client.Do(req)
		if err != nil {
			return nil, "", err
		}
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("operation poll failed: status=%d body=%s", resp.StatusCode, respBody)
		}

		var op yandexOperation
		if err := json.Unmarshal(respBody, &op); err != nil {
			return nil, "", err
		}

		if op.Done {
			if op.Error != nil {
				return nil, "", fmt.Errorf("recognition failed: %s (code %d)", op.Error.Message, op.Error.Code)
			}
			raw, _ := json.Marshal(op.Response)
			return raw, extractText(op.Response), nil
		}

		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(yandexPollInterval):
		}
	}
}

func encodingForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".mp3":
		return "MP3"
	case ".wav":
		return "LINEAR16_PCM"
	default:
		return "OGG_OPUS"
	}
}

func extractText(r *yandexResult) string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	for _, chunk := range r.Chunks {
		for _, alt := range chunk.Alternatives {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(strings.TrimSpace(alt.Text))
		}
	}
	return b.String()
}

type yandexRequest struct {
	Config yandexConfig      `json:"config"`
	Audio  yandexAudioSource `json:"audio"`
}

type yandexConfig struct {
	Specification yandexSpec `json:"specification"`
}

type yandexSpec struct {
	LanguageCode      string `json:"languageCode"`
	Model             string `json:"model"`
	AudioEncoding     string `json:"audioEncoding"`
	SampleRateHertz   int    `json:"sampleRateHertz"`
	AudioChannelCount int    `json:"audioChannelCount"`
	LiteratureText    bool   `json:"literatureText"`
}

type yandexAudioSource struct {
	URI string `json:"uri"`
}

type yandexOperation struct {
	ID       string         `json:"id"`
	Done     bool           `json:"done"`
	Response *yandexResult  `json:"response,omitempty"`
	Error    *yandexOpError `json:"error,omitempty"`
}

type yandexOpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type yandexResult struct {
	Chunks []struct {
		Alternatives []struct {
			Text string `json:"text"`
		} `json:"alternatives"`
	} `json:"chunks"`
}
