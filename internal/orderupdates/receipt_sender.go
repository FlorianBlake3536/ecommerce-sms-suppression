package orderupdates

import (
	"context"
	"errors"
	"fmt"
)

type EventKind string

const (
	CheckoutConfirmed EventKind = "checkout_confirmed"
	ReceiptReady      EventKind = "receipt_ready"
	OrderUpdated      EventKind = "order_updated"
	OrderFulfilled    EventKind = "order_fulfilled"
)

type OrderEvent struct {
	OrderID    string    `json:"order_id"`
	Phone      string    `json:"phone"`
	Kind       EventKind `json:"kind"`
	Detail     string    `json:"detail"`
	SMSOptOut  bool      `json:"sms_opt_out"`
	Suppressed bool      `json:"suppressed"`
}

type Decision struct {
	OrderID  string `json:"order_id"`
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
	BatchID  string `json:"batch_id,omitempty"`
}

type BatchSender interface {
	SendBatch(context.Context, string, []Message) (string, error)
}

type Service struct {
	sender BatchSender
}

func NewService(sender BatchSender) *Service {
	return &Service{sender: sender}
}

func (s *Service) Process(ctx context.Context, event OrderEvent) (Decision, error) {
	if event.OrderID == "" || event.Phone == "" || event.Detail == "" {
		return Decision{}, errors.New("order_id, phone, and detail are required")
	}
	if !validKind(event.Kind) {
		return Decision{}, fmt.Errorf("unsupported order event %q", event.Kind)
	}
	if event.SMSOptOut {
		return Decision{OrderID: event.OrderID, Decision: "suppressed", Reason: "customer_opted_out"}, nil
	}
	if event.Suppressed {
		return Decision{OrderID: event.OrderID, Decision: "suppressed", Reason: "recipient_suppressed"}, nil
	}

	batchID, err := s.sender.SendBatch(ctx, "order:"+event.OrderID+":"+string(event.Kind), []Message{{
		To:   event.Phone,
		Body: renderMessage(event),
	}})
	if err != nil {
		return Decision{}, err
	}
	return Decision{OrderID: event.OrderID, Decision: "sent", BatchID: batchID}, nil
}

func validKind(kind EventKind) bool {
	switch kind {
	case CheckoutConfirmed, ReceiptReady, OrderUpdated, OrderFulfilled:
		return true
	default:
		return false
	}
}

func renderMessage(event OrderEvent) string {
	labels := map[EventKind]string{
		CheckoutConfirmed: "Checkout confirmed",
		ReceiptReady:      "Receipt ready",
		OrderUpdated:      "Order update",
		OrderFulfilled:    "Order fulfilled",
	}
	return fmt.Sprintf("%s for order %s: %s", labels[event.Kind], event.OrderID, event.Detail)
}
