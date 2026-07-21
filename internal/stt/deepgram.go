package stt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Deepgram struct {
	baseURL  string
	apiKey   string
	model    string
	language string
	client   *http.Client
}

func NewDeepgram(baseURL, apiKey, model, language string) *Deepgram {
	return &Deepgram{
		baseURL:  strings.TrimRight(baseURL, "/"),
		apiKey:   apiKey,
		model:    model,
		language: language,
		client:   &http.Client{Timeout: 5 * time.Minute},
	}
}

func (d *Deepgram) Name() string { return "deepgram" }

func (d *Deepgram) Transcribe(ctx context.Context, a Audio) (*Result, error) {
	q := url.Values{}
	q.Set("model", d.model)
	q.Set("smart_format", "true")
	if lang := primaryLang(d.language); lang != "" {
		q.Set("language", lang)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+"/v1/listen?"+q.Encode(), a.Data)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+d.apiKey)
	if a.MIME != "" {
		req.Header.Set("Content-Type", a.MIME)
	} else {
		req.Header.Set("Content-Type", "audio/ogg")
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deepgram transcription failed: status=%d body=%s", resp.StatusCode, respBody)
	}

	var parsed struct {
		Results struct {
			Channels []struct {
				Alternatives []struct {
					Transcript string `json:"transcript"`
				} `json:"alternatives"`
			} `json:"channels"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var text string
	if len(parsed.Results.Channels) > 0 && len(parsed.Results.Channels[0].Alternatives) > 0 {
		text = parsed.Results.Channels[0].Alternatives[0].Transcript
	}

	return &Result{Text: text, Raw: respBody}, nil
}
