package orderupdates

import (
	"context"
	"testing"
)

type recordingSender struct {
	calls int
}

func (s *recordingSender) SendBatch(_ context.Context, _ string, _ []Message) (string, error) {
	s.calls++
	return "batch-42", nil
}

func TestProcessSuppressionDecision(t *testing.T) {
	tests := []struct {
		name      string
		event     OrderEvent
		decision  string
		reason    string
		wantCalls int
	}{
		{
			name:     "checkout opt-out stops before delivery",
			event:    OrderEvent{OrderID: "O-100", Phone: "+15550001000", Kind: CheckoutConfirmed, Detail: "paid", SMSOptOut: true},
			decision: "suppressed", reason: "customer_opted_out", wantCalls: 0,
		},
		{
			name:     "suppression snapshot blocks receipt",
			event:    OrderEvent{OrderID: "O-101", Phone: "+15550001001", Kind: ReceiptReady, Detail: "receipt R-9", Suppressed: true},
			decision: "suppressed", reason: "recipient_suppressed", wantCalls: 0,
		},
		{
			name:     "consented fulfillment is sent",
			event:    OrderEvent{OrderID: "O-102", Phone: "+15550001002", Kind: OrderFulfilled, Detail: "tracking 1Z42"},
			decision: "sent", wantCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &recordingSender{}
			got, err := NewService(sender).Process(context.Background(), tt.event)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if got.Decision != tt.decision || got.Reason != tt.reason {
				t.Fatalf("Process() = %#v, want decision %q reason %q", got, tt.decision, tt.reason)
			}
			if sender.calls != tt.wantCalls {
				t.Fatalf("SendBatch calls = %d, want %d", sender.calls, tt.wantCalls)
			}
		})
	}
}
