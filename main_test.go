package main

import (
	"context"
	"strconv"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"yclients_bot/bot"
	"yclients_bot/yclients"
)

func TestBuildDateKeyboard(t *testing.T) {
	markup := buildDateKeyboard([]bot.DateOption{
		{Label: "18.03", Value: "2026-03-18"},
		{Label: "19.03", Value: "2026-03-19"},
		{Label: "20.03", Value: "2026-03-20"},
		{Label: "21.03", Value: "2026-03-21"},
		{Label: "22.03", Value: "2026-03-22"},
		{Label: "23.03", Value: "2026-03-23"},
		{Label: "24.03", Value: "2026-03-24"},
	})

	if len(markup.InlineKeyboard) != 3 {
		t.Fatalf("len(markup.InlineKeyboard) = %d, want %d", len(markup.InlineKeyboard), 3)
	}

	for i, want := range []int{3, 3, 1} {
		if len(markup.InlineKeyboard[i]) != want {
			t.Fatalf("len(markup.InlineKeyboard[%d]) = %d, want %d", i, len(markup.InlineKeyboard[i]), want)
		}
	}

	button := markup.InlineKeyboard[0][2]
	if button.Text != "20.03" {
		t.Fatalf("button.Text = %q, want %q", button.Text, "20.03")
	}
	if button.CallbackData == nil || *button.CallbackData != scheduleDateCallbackData("2026-03-20") {
		t.Fatalf("button.CallbackData = %v, want %q", button.CallbackData, scheduleDateCallbackData("2026-03-20"))
	}
}

func TestParseSelectedDateCallback(t *testing.T) {
	date, ok := parseSelectedDateCallback(scheduleDateCallbackData("2026-03-20"))
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if date != "2026-03-20" {
		t.Fatalf("date = %q, want %q", date, "2026-03-20")
	}

	if _, ok := parseSelectedDateCallback("wrong:2026-03-20"); ok {
		t.Fatal("ok = true, want false for invalid prefix")
	}
}

func TestBuildResponsesForUpdateSelectedDate(t *testing.T) {
	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("LoadLocation returned error: %v", err)
	}

	scheduler := &mainFakeScheduler{
		services: []yclients.Service{
			{ID: 1, Title: "Court 1"},
			{ID: 2, Title: "Court 2"},
		},
		slotsByKey: map[string][]time.Time{
			"1|2026-03-20": {time.Date(2026, time.March, 20, 8, 0, 0, 0, moscow)},
			"2|2026-03-20": {time.Date(2026, time.March, 20, 9, 0, 0, 0, moscow)},
		},
	}

	update := tgbotapi.Update{
		CallbackQuery: &tgbotapi.CallbackQuery{
			ID:   "callback-1",
			Data: scheduleDateCallbackData("2026-03-20"),
			Message: &tgbotapi.Message{
				MessageID: 77,
				Chat:      &tgbotapi.Chat{ID: 42},
			},
		},
	}

	responses, err := buildResponsesForUpdate(context.Background(), update, bot.Dependencies{
		Scheduler:  scheduler,
		LocationID: 1296020,
		Location:   moscow,
	}, "mybot", 100)
	if err != nil {
		t.Fatalf("buildResponsesForUpdate returned error: %v", err)
	}

	if len(responses) != 3 {
		t.Fatalf("len(responses) = %d, want %d", len(responses), 3)
	}

	callback, ok := responses[0].(tgbotapi.CallbackConfig)
	if !ok {
		t.Fatalf("responses[0] type = %T, want CallbackConfig", responses[0])
	}
	if callback.CallbackQueryID != "callback-1" {
		t.Fatalf("callback.CallbackQueryID = %q, want %q", callback.CallbackQueryID, "callback-1")
	}

	deleteMessage, ok := responses[1].(tgbotapi.DeleteMessageConfig)
	if !ok {
		t.Fatalf("responses[1] type = %T, want DeleteMessageConfig", responses[1])
	}
	if deleteMessage.ChatID != 42 {
		t.Fatalf("deleteMessage.ChatID = %d, want %d", deleteMessage.ChatID, 42)
	}
	if deleteMessage.MessageID != 77 {
		t.Fatalf("deleteMessage.MessageID = %d, want %d", deleteMessage.MessageID, 77)
	}

	message, ok := responses[2].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("responses[2] type = %T, want MessageConfig", responses[2])
	}
	if message.ChatID != 42 {
		t.Fatalf("message.ChatID = %d, want %d", message.ChatID, 42)
	}
	if message.Text != "📅 20.03.2026\n🕒 08:00, 09:00" {
		t.Fatalf("message.Text = %q, want %q", message.Text, "📅 20.03.2026\n🕒 08:00, 09:00")
	}

	if len(scheduler.slotCalls) != 2 {
		t.Fatalf("slotCalls = %d, want %d", len(scheduler.slotCalls), 2)
	}
}

func TestSendResponsesUsesRequestForCallback(t *testing.T) {
	client := &fakeTelegramClient{}
	responses := []tgbotapi.Chattable{
		tgbotapi.NewCallback("callback-1", ""),
		tgbotapi.NewDeleteMessage(42, 77),
		tgbotapi.NewMessage(42, "hello"),
	}

	if err := sendResponses(client, responses); err != nil {
		t.Fatalf("sendResponses returned error: %v", err)
	}

	if len(client.requested) != 2 {
		t.Fatalf("len(client.requested) = %d, want %d", len(client.requested), 2)
	}
	if len(client.sent) != 1 {
		t.Fatalf("len(client.sent) = %d, want %d", len(client.sent), 1)
	}

	if _, ok := client.requested[0].(tgbotapi.CallbackConfig); !ok {
		t.Fatalf("client.requested[0] type = %T, want CallbackConfig", client.requested[0])
	}
	if _, ok := client.requested[1].(tgbotapi.DeleteMessageConfig); !ok {
		t.Fatalf("client.requested[1] type = %T, want DeleteMessageConfig", client.requested[1])
	}
	if _, ok := client.sent[0].(tgbotapi.MessageConfig); !ok {
		t.Fatalf("client.sent[0] type = %T, want MessageConfig", client.sent[0])
	}
}

type mainFakeScheduler struct {
	services   []yclients.Service
	slotsByKey map[string][]time.Time
	slotCalls  []yclients.SearchTimeSlotsParams
}

func (f *mainFakeScheduler) AvailableServices(context.Context, int) ([]yclients.Service, error) {
	return f.services, nil
}

func (f *mainFakeScheduler) SearchAvailableTimeSlots(_ context.Context, params yclients.SearchTimeSlotsParams) ([]time.Time, error) {
	f.slotCalls = append(f.slotCalls, params)
	return f.slotsByKey[mainSlotKey(params.ServiceID, params.Date)], nil
}

func mainSlotKey(serviceID int, date string) string {
	return strconv.Itoa(serviceID) + "|" + date
}

type fakeTelegramClient struct {
	sent      []tgbotapi.Chattable
	requested []tgbotapi.Chattable
}

func (f *fakeTelegramClient) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	f.sent = append(f.sent, c)
	return tgbotapi.Message{}, nil
}

func (f *fakeTelegramClient) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	f.requested = append(f.requested, c)
	return &tgbotapi.APIResponse{Ok: true}, nil
}
