package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"example.com/private-order-downloads/infrai"
	"example.com/private-order-downloads/signeddownload"
)

const bucket = "private-order-receipts"

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := &infrai.Client{APIKey: key}
	if err := client.CreateBucket(context.Background(), bucket); err != nil {
		log.Fatal(err)
	}
	service := signeddownload.Service{Storage: signeddownload.InfraiStorage{Client: client}, Bucket: bucket, TTLSeconds: 300}

	http.HandleFunc("POST /orders/update", func(w http.ResponseWriter, r *http.Request) {
		var order signeddownload.OrderUpdate
		if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		update, err := service.BuildCustomerUpdate(r.Context(), order)
		if err != nil {
			status := http.StatusConflict
			if !errors.Is(err, signeddownload.ErrNotFulfilled) && !errors.Is(err, signeddownload.ErrReceiptMissing) {
				status = infrai.MapAPIError(err)
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, update)
	})
	log.Println("orderfiles listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
