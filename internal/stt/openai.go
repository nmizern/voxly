package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

type OpenAICompatible struct {
	name     string
	baseURL  string
	apiKey   string
	model    string
	language string
	client   *http.Client
}

func NewOpenAICompatible(name, baseURL, apiKey, model, language string) *OpenAICompatible {
	return &OpenAICompatible{
		name:     name,
		baseURL:  strings.TrimRight(baseURL, "/"),
		apiKey:   apiKey,
		model:    model,
		language: language,
		client:   &http.Client{Timeout: 5 * time.Minute},
	}
}

func (o *OpenAICompatible) Name() string { return o.name }

func (o *OpenAICompatible) Transcribe(ctx context.Context, a Audio) (*Result, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	part, err := w.CreateFormFile("file", a.Filename)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, a.Data); err != nil {
		return nil, fmt.Errorf("read audio: %w", err)
	}

	_ = w.WriteField("model", o.model)
	_ = w.WriteField("response_format", "json")
	if lang := primaryLang(o.language); lang != "" {
		_ = w.WriteField("language", lang)
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/audio/transcriptions", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s transcription failed: status=%d body=%s", o.name, resp.StatusCode, respBody)
	}

	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &Result{Text: parsed.Text, Raw: respBody}, nil
}
