package signeddownload

import (
	"context"
	"errors"
	"testing"
)

type fakeStorage struct {
	found        bool
	presignCalls int
}

func (f *fakeStorage) HeadObject(context.Context, string, string) (HeadResult, error) {
	return HeadResult{Found: f.found}, nil
}
func (f *fakeStorage) PresignGet(context.Context, string, string, int, string, string) (LinkResult, error) {
	f.presignCalls++
	return LinkResult{URL: "https://download.example/signed"}, nil
}

func TestBuildCustomerUpdate(t *testing.T) {
	tests := []struct {
		name      string
		status    OrderStatus
		found     bool
		wantErr   error
		wantCalls int
	}{
		{"checkout has no download", StatusCheckoutPending, true, ErrNotFulfilled, 0},
		{"paid has no download", StatusPaid, true, ErrNotFulfilled, 0},
		{"fulfillment waits for receipt", StatusFulfilled, false, ErrReceiptMissing, 0},
		{"fulfilled receipt gets link", StatusFulfilled, true, nil, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeStorage{found: tt.found}
			service := Service{Storage: storage, Bucket: "order-receipts", TTLSeconds: 300}
			got, err := service.BuildCustomerUpdate(context.Background(), OrderUpdate{
				OrderID: "ord_42", CustomerID: "cus_7", Status: tt.status, ReceiptKey: "receipts/ord_42.pdf",
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if storage.presignCalls != tt.wantCalls {
				t.Fatalf("presign calls = %d, want %d", storage.presignCalls, tt.wantCalls)
			}
			if tt.wantErr == nil && got.ReceiptURL == "" {
				t.Fatal("expected receipt URL")
			}
		})
	}
}
