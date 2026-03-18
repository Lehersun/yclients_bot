package yclients

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestSearchAvailableTimeSlotsIntegration(t *testing.T) {
	token := os.Getenv("YCLIENTS_BEARER_TOKEN")
	if token == "" {
		t.Skip("YCLIENTS_BEARER_TOKEN is not set")
	}

	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("LoadLocation returned error: %v", err)
	}

	today := time.Now().In(moscow).Format("2006-01-02")

	client := Client{
		BaseURL: "https://platform.yclients.com",
		Token:   token,
		Cookie:  os.Getenv("YCLIENTS_COOKIE"),
	}

	slots, err := client.SearchAvailableTimeSlots(context.Background(), SearchTimeSlotsParams{
		LocationID: 1296020,
		Date:       today,
	})
	if err != nil {
		t.Fatalf("SearchAvailableTimeSlots returned error: %v", err)
	}

	if len(slots) == 0 {
		t.Fatal("SearchAvailableTimeSlots returned no slots, want at least one")
	}

	for _, slot := range slots {
		if slot.IsZero() {
			t.Fatal("SearchAvailableTimeSlots returned a zero datetime")
		}

		if slot.Format("2006-01-02") != today {
			t.Fatalf("slot date = %q, want %q", slot.Format("2006-01-02"), today)
		}

		if slot.Location() == time.UTC && slot.Format(time.RFC3339) == slot.UTC().Format(time.RFC3339) {
			t.Fatal("SearchAvailableTimeSlots returned only UTC-normalized data, want parsed API offset")
		}
	}
}
