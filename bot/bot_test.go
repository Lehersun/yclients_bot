package bot

import (
	"context"
	"strconv"
	"sync"
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
			name: "private slash command is normalized",
			input: IncomingMessage{
				Text:        "/schedule",
				ChatType:    "private",
				BotUsername: "mybot",
			},
			wantText: "schedule",
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
			name: "group mention russian schedule alias is stripped",
			input: IncomingMessage{
				Text:        "@mybot расписание",
				ChatType:    "group",
				BotUsername: "mybot",
			},
			wantText: "расписание",
			wantOK:   true,
		},
		{
			name: "group slash command for bot is normalized",
			input: IncomingMessage{
				Text:        "/schedule@mybot",
				ChatType:    "group",
				BotUsername: "mybot",
			},
			wantText: "schedule",
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

func TestHandleTextRussianScheduleAliasReturnsDatePicker(t *testing.T) {
	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("LoadLocation returned error: %v", err)
	}

	now := time.Date(2026, time.March, 18, 10, 0, 0, 0, moscow)
	scheduler := &fakeScheduler{}

	reply, ok, err := HandleText(context.Background(), "расписание", Dependencies{
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

	if reply.Text != "Выберите дату" {
		t.Fatalf("reply.Text = %q, want %q", reply.Text, "Выберите дату")
	}

	if len(reply.DateOptions) != 7 {
		t.Fatalf("len(reply.DateOptions) = %d, want %d", len(reply.DateOptions), 7)
	}
}

func TestReplyForTextSlashCommand(t *testing.T) {
	reply, ok := ReplyForText("/hello")
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if reply != "Hello!" {
		t.Fatalf("reply = %q, want %q", reply, "Hello!")
	}
}

func TestHandleTextScheduleReturnsDatePicker(t *testing.T) {
	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("LoadLocation returned error: %v", err)
	}

	now := time.Date(2026, time.March, 18, 10, 0, 0, 0, moscow)
	scheduler := &fakeScheduler{}

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

	if reply.Text != "Выберите дату" {
		t.Fatalf("reply.Text = %q, want %q", reply.Text, "Выберите дату")
	}

	wantDates := []DateOption{
		{Label: "18.03", Value: "2026-03-18"},
		{Label: "19.03", Value: "2026-03-19"},
		{Label: "20.03", Value: "2026-03-20"},
		{Label: "21.03", Value: "2026-03-21"},
		{Label: "22.03", Value: "2026-03-22"},
		{Label: "23.03", Value: "2026-03-23"},
		{Label: "24.03", Value: "2026-03-24"},
	}

	if len(reply.DateOptions) != len(wantDates) {
		t.Fatalf("len(reply.DateOptions) = %d, want %d", len(reply.DateOptions), len(wantDates))
	}

	for i, want := range wantDates {
		if reply.DateOptions[i] != want {
			t.Fatalf("reply.DateOptions[%d] = %#v, want %#v", i, reply.DateOptions[i], want)
		}
	}

	if scheduler.servicesCalls != 0 {
		t.Fatalf("servicesCalls = %d, want %d", scheduler.servicesCalls, 0)
	}

	if len(scheduler.slotCalls) != 0 {
		t.Fatalf("slotCalls = %d, want %d", len(scheduler.slotCalls), 0)
	}
}

func TestHandleSelectedDateSchedule(t *testing.T) {
	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("LoadLocation returned error: %v", err)
	}

	slotA := time.Date(2026, time.March, 20, 8, 0, 0, 0, moscow)
	slotB := time.Date(2026, time.March, 20, 9, 0, 0, 0, moscow)
	slotC := time.Date(2026, time.March, 20, 10, 0, 0, 0, moscow)
	slotD := time.Date(2026, time.March, 20, 13, 0, 0, 0, moscow)
	slotE := time.Date(2026, time.March, 20, 16, 0, 0, 0, moscow)
	slotF := time.Date(2026, time.March, 20, 17, 0, 0, 0, moscow)
	slotG := time.Date(2026, time.March, 20, 19, 0, 0, 0, moscow)
	slotH := time.Date(2026, time.March, 20, 22, 0, 0, 0, moscow)
	slotOtherDay := time.Date(2026, time.March, 21, 11, 0, 0, 0, moscow)

	scheduler := &fakeScheduler{
		services: []yclients.Service{
			{ID: 19432008, Title: "Падел 2 корт 2 часа вт-вс", PriceMin: 4800},
			{ID: 19346630, Title: "Падел корт 2 вт-вс 1 час", PriceMin: 2400},
			{ID: 23037453, Title: "Падел корт 3 вт-вс 2часа", PriceMin: 4800},
			{ID: 19346628, Title: "Падел корт 1 вт-вс 1 час", PriceMin: 2400},
		},
		slotsByKey: map[string][]time.Time{
			"19432008|2026-03-20": {slotA, slotB},
			"19346630|2026-03-20": {slotC, slotD},
			"23037453|2026-03-20": {slotE, slotF},
			"19346628|2026-03-20": {slotG, slotH},
			"19346628|2026-03-21": {slotOtherDay},
		},
	}

	reply, ok, err := HandleSelectedDate(context.Background(), "2026-03-20", Dependencies{
		Scheduler:  scheduler,
		LocationID: 1296020,
		Location:   moscow,
	})
	if err != nil {
		t.Fatalf("HandleSelectedDate returned error: %v", err)
	}

	if !ok {
		t.Fatal("HandleSelectedDate ok = false, want true")
	}

	wantReply := "📅 20.03.2026\n🕒 08:00, 09:00, 10:00, 13:00, 16:00, 17:00, 19:00, 22:00"
	if reply != wantReply {
		t.Fatalf("reply = %q, want %q", reply, wantReply)
	}

	if scheduler.servicesCalls != 1 {
		t.Fatalf("servicesCalls = %d, want %d", scheduler.servicesCalls, 1)
	}

	if len(scheduler.slotCalls) != 4 {
		t.Fatalf("slotCalls = %d, want %d", len(scheduler.slotCalls), 4)
	}

	for _, serviceID := range []int{19432008, 19346630, 23037453, 19346628} {
		if !hasSlotCall(scheduler.slotCalls, serviceID, "2026-03-20") {
			t.Fatalf("slotCalls = %#v, want service %d on 2026-03-20", scheduler.slotCalls, serviceID)
		}
	}

	if hasSlotCall(scheduler.slotCalls, 19346628, "2026-03-21") {
		t.Fatalf("slotCalls = %#v, do not want request for 2026-03-21", scheduler.slotCalls)
	}
}

type fakeScheduler struct {
	services      []yclients.Service
	slotsByKey    map[string][]time.Time
	servicesCalls int
	slotCalls     []yclients.SearchTimeSlotsParams
	mu            sync.Mutex
	current       int
	maxConcurrent int
}

func (f *fakeScheduler) AvailableServices(context.Context, int) ([]yclients.Service, error) {
	f.servicesCalls++
	return f.services, nil
}

func (f *fakeScheduler) SearchAvailableTimeSlots(_ context.Context, params yclients.SearchTimeSlotsParams) ([]time.Time, error) {
	f.mu.Lock()
	f.current++
	if f.current > f.maxConcurrent {
		f.maxConcurrent = f.current
	}
	f.slotCalls = append(f.slotCalls, params)
	f.mu.Unlock()

	time.Sleep(10 * time.Millisecond)

	f.mu.Lock()
	f.current--
	slots := f.slotsByKey[slotKey(params.ServiceID, params.Date)]
	f.mu.Unlock()

	return slots, nil
}

func slotKey(serviceID int, date string) string {
	return strconv.Itoa(serviceID) + "|" + date
}

func hasSlotCall(calls []yclients.SearchTimeSlotsParams, serviceID int, date string) bool {
	for _, call := range calls {
		if call.ServiceID == serviceID && call.Date == date {
			return true
		}
	}
	return false
}
