package orderupdates

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestSendBatchDecodesRejectionBeforeStatus(t *testing.T) {
	client := &InfraiClient{
		APIKey: "test-key",
		HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ok":false,"data":null,"error":{"code":"INVALID_INPUT","message":"messages required"},"metadata":{}}`)),
			}, nil
		})},
	}

	_, err := client.SendBatch(context.Background(), "order:O-1:receipt_ready", []Message{{To: "+15550001000", Body: "Receipt ready"}})
	apiErr, ok := err.(*InfraiError)
	if !ok || apiErr.Status != http.StatusBadRequest || apiErr.Code != "INVALID_INPUT" {
		t.Fatalf("SendBatch error = %#v, want decoded InfraiError", err)
	}
}

func TestSendBatchRetriesRateLimit(t *testing.T) {
	calls := 0
	client := &InfraiClient{
		APIKey: "test-key",
		HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				header := make(http.Header)
				header.Set("Retry-After", "2")
				return &http.Response{StatusCode: http.StatusTooManyRequests, Header: header, Body: io.NopCloser(strings.NewReader(`{"ok":false,"data":null,"error":{"code":"RATE_LIMITED","message":"retry later"},"metadata":{}}`))}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":true,"data":{"batch_id":"batch-9"},"error":null,"metadata":{}}`))}, nil
		})},
		Sleep: func(_ context.Context, delay time.Duration) error {
			if delay != 2*time.Second {
				t.Fatalf("retry delay = %v, want 2s", delay)
			}
			return nil
		},
	}

	id, err := client.SendBatch(context.Background(), "order:O-2:order_updated", []Message{{To: "+15550001000", Body: "Order update"}})
	if err != nil || id != "batch-9" || calls != 2 {
		t.Fatalf("SendBatch = (%q, %v), calls = %d", id, err, calls)
	}
}
