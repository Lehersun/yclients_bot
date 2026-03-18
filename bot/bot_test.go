package bot

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"yclients_bot/yclients"
)

func TestReplyForText(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantReply string
		wantOK    bool
	}{
		{
			name:      "exact hello matches",
			input:     "hello",
			wantReply: "Hello!",
			wantOK:    true,
		},
		{
			name:      "mixed case hello matches",
			input:     "Hello",
			wantReply: "Hello!",
			wantOK:    true,
		},
		{
			name:      "other text is ignored",
			input:     "bye",
			wantReply: "",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReply, gotOK := ReplyForText(tt.input)

			if gotReply != tt.wantReply {
				t.Fatalf("reply = %q, want %q", gotReply, tt.wantReply)
			}

			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
		})
	}
}

func TestNormalizeIncomingText(t *testing.T) {
	tests := []struct {
		name     string
		input    IncomingMessage
		wantText string
		wantOK   bool
	}{
		{
			name: "private chat plain text is allowed",
			input: IncomingMessage{
				Text:     "hello",
				ChatType: "private",
			},
			wantText: "hello",
			wantOK:   true,
		},
		{
			name: "group plain text is ignored",
			input: IncomingMessage{
				Text:        "hello",
				ChatType:    "group",
				BotUsername: "mybot",
			},
			wantText: "",
			wantOK:   false,
		},
		{
			name: "group mention is stripped",
			input: IncomingMessage{
				Text:        "@mybot hello",
				ChatType:    "group",
				BotUsername: "mybot",
			},
			wantText: "hello",
			wantOK:   true,
		},
		{
			name: "reply to bot is allowed",
			input: IncomingMessage{
				Text:         "schedule",
				ChatType:     "supergroup",
				BotUsername:  "mybot",
				IsReplyToBot: true,
			},
			wantText: "schedule",
			wantOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotText, gotOK := NormalizeIncomingText(tt.input)
			if gotText != tt.wantText {
				t.Fatalf("text = %q, want %q", gotText, tt.wantText)
			}
			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
		})
	}
}

func TestHandleTextSchedule(t *testing.T) {
	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("LoadLocation returned error: %v", err)
	}

	now := time.Date(2026, time.March, 18, 10, 0, 0, 0, moscow)
	slotA := time.Date(2026, time.March, 18, 9, 0, 0, 0, moscow)
	slotB := time.Date(2026, time.March, 18, 9, 0, 0, 0, moscow)
	slotC := time.Date(2026, time.March, 18, 9, 0, 0, 0, moscow)
	slotD := time.Date(2026, time.March, 18, 10, 0, 0, 0, moscow)
	slotE := time.Date(2026, time.March, 19, 11, 0, 0, 0, moscow)

	scheduler := &fakeScheduler{
		services: []yclients.Service{
			{ID: 19432008, Title: "Падел 2 корт 2 часа вт-вс", PriceMin: 4800},
			{ID: 19346630, Title: "Падел корт 2 вт-вс 1 час", PriceMin: 2400},
			{ID: 23037453, Title: "Падел корт 3 вт-вс 2часа", PriceMin: 4800},
			{ID: 19346628, Title: "Падел корт 1 вт-вс 1 час", PriceMin: 2400},
		},
		slotsByKey: map[string][]time.Time{
			"19432008|2026-03-18": {slotA},
			"19346630|2026-03-18": {slotB, slotD},
			"23037453|2026-03-18": {slotC},
			"19346628|2026-03-19": {slotE},
		},
	}

	reply, ok, err := HandleText(context.Background(), "schedule", Dependencies{
		Scheduler:  scheduler,
		LocationID: 1296020,
		Location:   moscow,
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("HandleText returned error: %v", err)
	}

	if !ok {
		t.Fatal("HandleText ok = false, want true")
	}

	if scheduler.servicesCalls != 1 {
		t.Fatalf("servicesCalls = %d, want %d", scheduler.servicesCalls, 1)
	}

	if len(scheduler.slotCalls) != 28 {
		t.Fatalf("slotCalls = %d, want %d", len(scheduler.slotCalls), 28)
	}

	if scheduler.slotCalls[0].ServiceID != 19432008 || scheduler.slotCalls[0].Date != "2026-03-18" {
		t.Fatalf("first slot call = %#v, want service 19432008 on 2026-03-18", scheduler.slotCalls[0])
	}

	if scheduler.slotCalls[6].ServiceID != 19432008 || scheduler.slotCalls[6].Date != "2026-03-24" {
		t.Fatalf("seventh slot call = %#v, want service 19432008 on 2026-03-24", scheduler.slotCalls[6])
	}

	if scheduler.slotCalls[7].ServiceID != 19346630 || scheduler.slotCalls[7].Date != "2026-03-18" {
		t.Fatalf("eighth slot call = %#v, want service 19346630 on 2026-03-18", scheduler.slotCalls[7])
	}

	wantReply := strings.Join([]string{
		"18.03.2026:",
		"09:00 - корт 2 (на час, на два), корт 3 (на два)",
		"10:00 - корт 2 (на час)",
		"19.03.2026:",
		"11:00 - корт 1 (на час)",
	}, "\n")

	if reply != wantReply {
		t.Fatalf("reply = %q, want %q", reply, wantReply)
	}
}

type fakeScheduler struct {
	services      []yclients.Service
	slotsByKey    map[string][]time.Time
	servicesCalls int
	slotCalls     []yclients.SearchTimeSlotsParams
}

func (f *fakeScheduler) AvailableServices(context.Context, int) ([]yclients.Service, error) {
	f.servicesCalls++
	return f.services, nil
}

func (f *fakeScheduler) SearchAvailableTimeSlots(_ context.Context, params yclients.SearchTimeSlotsParams) ([]time.Time, error) {
	f.slotCalls = append(f.slotCalls, params)
	return f.slotsByKey[slotKey(params.ServiceID, params.Date)], nil
}

func slotKey(serviceID int, date string) string {
	return strconv.Itoa(serviceID) + "|" + date
}
