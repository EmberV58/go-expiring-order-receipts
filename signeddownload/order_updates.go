package signeddownload

import (
	"context"
	"errors"
	"fmt"
)

type OrderStatus string

const (
	StatusCheckoutPending OrderStatus = "checkout_pending"
	StatusPaid            OrderStatus = "paid"
	StatusFulfilled       OrderStatus = "fulfilled"
)

var (
	ErrNotFulfilled   = errors.New("order is not fulfilled")
	ErrReceiptMissing = errors.New("receipt is not available")
)

type OrderUpdate struct {
	OrderID    string      `json:"order_id"`
	CustomerID string      `json:"customer_id"`
	Status     OrderStatus `json:"status"`
	ReceiptKey string      `json:"receipt_key"`
}

type HeadResult struct{ Found bool }
type LinkResult struct{ URL string }

type Storage interface {
	HeadObject(context.Context, string, string) (HeadResult, error)
	PresignGet(context.Context, string, string, int, string, string) (LinkResult, error)
}

type Service struct {
	Storage    Storage
	Bucket     string
	TTLSeconds int
}

type CustomerUpdate struct {
	OrderID        string      `json:"order_id"`
	Status         OrderStatus `json:"status"`
	ReceiptURL     string      `json:"receipt_url"`
	ExpiresSeconds int         `json:"expires_seconds"`
}

func (s Service) BuildCustomerUpdate(ctx context.Context, order OrderUpdate) (CustomerUpdate, error) {
	if order.Status != StatusFulfilled {
		return CustomerUpdate{}, ErrNotFulfilled
	}
	head, err := s.Storage.HeadObject(ctx, s.Bucket, order.ReceiptKey)
	if err != nil {
		return CustomerUpdate{}, err
	}
	if !head.Found {
		return CustomerUpdate{}, ErrReceiptMissing
	}
	ttl := s.TTLSeconds
	if ttl == 0 {
		ttl = 300
	}
	disposition := fmt.Sprintf(`attachment; filename="receipt-%s.pdf"`, order.OrderID)
	link, err := s.Storage.PresignGet(ctx, s.Bucket, order.ReceiptKey, ttl, disposition, "receipt-"+order.OrderID)
	if err != nil {
		return CustomerUpdate{}, err
	}
	return CustomerUpdate{OrderID: order.OrderID, Status: order.Status, ReceiptURL: link.URL, ExpiresSeconds: ttl}, nil
}
