package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

type yenRate struct {
	Date  string  `json:"date"`
	Base  string  `json:"base"`
	Quote string  `json:"quote"`
	Rate  float64 `json:"rate"`
}

func registerExchangeRate(ctx context.Context, mux *http.ServeMux) {
	var latest atomic.Pointer[yenRate]
	mux.HandleFunc("GET /api/exchange-rate", func(w http.ResponseWriter, r *http.Request) {
		respond(w, latest.Load(), nil)
	})
	go func() {
		client := &http.Client{Timeout: 8 * time.Second}
		for ctx.Err() == nil {
			delay := 6 * time.Hour
			rate, err := fetchYenRate(ctx, client)
			if err != nil {
				log.Printf("USD/JPY exchange rate: %v", err)
				delay = 5 * time.Minute
			} else {
				latest.Store(&rate)
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
			case <-timer.C:
			}
		}
	}()
}

func fetchYenRate(ctx context.Context, client *http.Client) (yenRate, error) {
	var rate yenRate
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.frankfurter.dev/v2/rate/usd/jpy", nil)
	if err != nil {
		return rate, err
	}
	response, err := client.Do(req)
	if err != nil {
		return rate, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return rate, fmt.Errorf("HTTP status %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&rate); err != nil {
		return rate, err
	}
	if _, err := time.Parse(time.DateOnly, rate.Date); err != nil || rate.Base != "USD" || rate.Quote != "JPY" || rate.Rate <= 0 {
		return rate, fmt.Errorf("invalid USD/JPY quote")
	}
	return rate, nil
}
