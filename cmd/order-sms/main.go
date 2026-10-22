package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/infrai-examples/ecommerce-sms-suppression/internal/orderupdates"
)

func main() {
	client := &orderupdates.InfraiClient{APIKey: os.Getenv("INFRAI_API_KEY")}
	service := orderupdates.NewService(client)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /order-updates", func(w http.ResponseWriter, r *http.Request) {
		var event orderupdates.OrderEvent
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&event); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order event"})
			return
		}
		decision, err := service.Process(r.Context(), event)
		if err != nil {
			var apiErr *orderupdates.InfraiError
			if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
				writeJSON(w, apiErr.Status, map[string]string{"error": apiErr.Code})
				return
			}
			log.Printf("process order update: %v", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "delivery_failed"})
			return
		}
		writeJSON(w, http.StatusOK, decision)
	})

	server := &http.Server{Addr: envOr("ADDR", ":8080"), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("order SMS service listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
