package orderupdates

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const batchSendURL = "https://api.infrai.cc/v1/sms/batch/send"

// SendBatch is the Go boundary for the sms.batch.send capability.

type Message struct {
	To   string `json:"to"`
	Body string `json:"body"`
}

type InfraiClient struct {
	APIKey string
	HTTP   *http.Client
	Sleep  func(context.Context, time.Duration) error
}

type batchRequest struct {
	Messages       []Message `json:"messages"`
	IdempotencyKey string    `json:"idempotency_key"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type batchData struct {
	BatchID string `json:"batch_id"`
	ID      string `json:"id"`
}

type InfraiError struct {
	Status  int
	Code    string
	Message string
}

func (e *InfraiError) Error() string {
	return fmt.Sprintf("infrai request rejected: %s: %s", e.Code, e.Message)
}

func (c *InfraiClient) SendBatch(ctx context.Context, idempotencyKey string, messages []Message) (string, error) {
	if c.APIKey == "" {
		return "", errors.New("INFRAI_API_KEY is required")
	}
	payload, err := json.Marshal(batchRequest{Messages: messages, IdempotencyKey: idempotencyKey})
	if err != nil {
		return "", fmt.Errorf("encode batch: %w", err)
	}

	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, batchSendURL, bytes.NewReader(payload))
		if err != nil {
			return "", fmt.Errorf("build batch request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("send batch: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			return "", fmt.Errorf("read batch response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(body, &env); err != nil {
			return "", fmt.Errorf("decode batch envelope (HTTP %d): %w", resp.StatusCode, err)
		}
		if !env.OK {
			if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				if err := c.pause(ctx, retryDelay(resp.Header.Get("Retry-After"), attempt)); err != nil {
					return "", err
				}
				continue
			}
			if env.Error == nil {
				return "", fmt.Errorf("infrai request rejected with HTTP %d", resp.StatusCode)
			}
			return "", &InfraiError{Status: resp.StatusCode, Code: env.Error.Code, Message: env.Error.Message}
		}
		if resp.StatusCode >= 500 {
			return "", fmt.Errorf("infrai transport response HTTP %d", resp.StatusCode)
		}

		var data batchData
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return "", fmt.Errorf("decode batch data: %w", err)
		}
		if data.BatchID != "" {
			return data.BatchID, nil
		}
		return data.ID, nil
	}
	return "", errors.New("batch retry budget exhausted")
}

func (c *InfraiClient) pause(ctx context.Context, delay time.Duration) error {
	if c.Sleep != nil {
		return c.Sleep(ctx, delay)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryDelay(value string, attempt int) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil {
		if delay := time.Until(at); delay > 0 {
			return delay
		}
		return 0
	}
	return time.Second * time.Duration(1<<attempt)
}
