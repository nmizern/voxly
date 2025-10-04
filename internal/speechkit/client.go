package speechkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"voxly/pkg/logger"

	"go.uber.org/zap"
)

const (
	RecognizeURL  = "https://transcribe.api.cloud.yandex.net/speech/stt/v2/longRunningRecognize"
	OperationURL  = "https://operation.api.cloud.yandex.net/operations"
	OperationPoll = 5 * time.Second
	MaxWaitTime   = 30 * time.Minute
)

type Client struct {
	apiKey   string
	folderID string
	client   *http.Client
}

// New Yandex SpeechKit client
func NewClient(apiKey, folderID string) *Client {
	return &Client{
		apiKey:   apiKey,
		folderID: folderID,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Async voice recognition
func (c *Client) StartRecognition(s3URI string) (string, error) {
	reqBody := RecognitionRequest{
		Config: RecognitionConfig{
			Specification: Specification{
				LanguageCode:      "ru-RU",
				Model:             "general:rc",
				AudioEncoding:     "OGG_OPUS",
				SampleRateHertz:   48000,
				AudioChannelCount: 1,
				ProfanityFilter:   false,
				LiteratureText:    true,
				RawResults:        false,
			},
		},
		Audio: AudioSource{
			URI: s3URI,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", RecognizeURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Api-Key %s", c.apiKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-folder-id", c.folderID)

	logger.Debug("Starting speech recognition", zap.String("s3_uri", s3URI))

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("recognition request failed: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	var opResp OperationResponse
	if err := json.Unmarshal(respBody, &opResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	logger.Info("Recognition started", zap.String("operation_id", opResp.ID))

	return opResp.ID, nil
}

// Polling operation status and returns result
func (c *Client) WaitForResult(operationID string) (*RecognitionResult, error) {
	url := fmt.Sprintf("%s/%s", OperationURL, operationID)
	startTime := time.Now()

	for {
		if time.Since(startTime) > MaxWaitTime {
			return nil, fmt.Errorf("recognition timeout exceeded")
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", fmt.Sprintf("Api-Key %s", c.apiKey))

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("operation check failed: status=%d, body=%s", resp.StatusCode, string(respBody))
		}

		var opResp OperationResponse
		if err := json.Unmarshal(respBody, &opResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		if opResp.Done {
			if opResp.Error != nil {
				return nil, fmt.Errorf("recognition failed: %s (code: %d)", opResp.Error.Message, opResp.Error.Code)
			}

			// Parse response
			var result RecognitionResult
			if opResp.Response != nil {
				responseBytes, err := json.Marshal(opResp.Response)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal response: %w", err)
				}

				if err := json.Unmarshal(responseBytes, &result); err != nil {
					return nil, fmt.Errorf("failed to unmarshal result: %w", err)
				}
			}

			logger.Info("Recognition completed",
				zap.String("operation_id", operationID),
				zap.Int("chunks", len(result.Chunks)))

			return &result, nil
		}

		logger.Debug("Recognition in progress",
			zap.String("operation_id", operationID),
			zap.Duration("elapsed", time.Since(startTime)))

		time.Sleep(OperationPoll)
	}
}

// Extracting complete text from recognition result
func (r *RecognitionResult) GetFullText() string {
	var text string
	for _, chunk := range r.Chunks {
		for _, alt := range chunk.Alternatives {
			text += alt.Text + " "
		}
	}
	return text
}
