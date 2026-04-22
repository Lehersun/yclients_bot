package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
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

func TestProcessUpdateSelectedDateShowsAndRemovesWaitingMessage(t *testing.T) {
	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("LoadLocation returned error: %v", err)
	}

	client := &fakeTelegramClient{
		sendResults: []tgbotapi.Message{
			{MessageID: 501},
			{MessageID: 502},
		},
	}
	scheduler := &mainFakeScheduler{
		services: []yclients.Service{
			{ID: 1},
			{ID: 2},
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

	err = processUpdate(context.Background(), client, update, bot.Dependencies{
		Scheduler:  scheduler,
		LocationID: 1296020,
		Location:   moscow,
	}, "mybot", 100)
	if err != nil {
		t.Fatalf("processUpdate returned error: %v", err)
	}

	wantCalls := []string{
		"request:tgbotapi.CallbackConfig",
		"request:tgbotapi.DeleteMessageConfig",
		"send:tgbotapi.MessageConfig",
		"request:tgbotapi.DeleteMessageConfig",
		"send:tgbotapi.MessageConfig",
	}
	if len(client.calls) != len(wantCalls) {
		t.Fatalf("len(client.calls) = %d, want %d", len(client.calls), len(wantCalls))
	}
	for i, want := range wantCalls {
		if client.calls[i] != want {
			t.Fatalf("client.calls[%d] = %q, want %q", i, client.calls[i], want)
		}
	}

	callback, ok := client.requested[0].(tgbotapi.CallbackConfig)
	if !ok {
		t.Fatalf("client.requested[0] type = %T, want CallbackConfig", client.requested[0])
	}
	if callback.CallbackQueryID != "callback-1" {
		t.Fatalf("callback.CallbackQueryID = %q, want %q", callback.CallbackQueryID, "callback-1")
	}

	deleteOriginal, ok := client.requested[1].(tgbotapi.DeleteMessageConfig)
	if !ok {
		t.Fatalf("client.requested[1] type = %T, want DeleteMessageConfig", client.requested[1])
	}
	if deleteOriginal.ChatID != 42 {
		t.Fatalf("deleteOriginal.ChatID = %d, want %d", deleteOriginal.ChatID, 42)
	}
	if deleteOriginal.MessageID != 77 {
		t.Fatalf("deleteOriginal.MessageID = %d, want %d", deleteOriginal.MessageID, 77)
	}

	waitingMessage, ok := client.sent[0].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("client.sent[0] type = %T, want MessageConfig", client.sent[0])
	}
	if waitingMessage.Text != waitingText {
		t.Fatalf("waitingMessage.Text = %q, want %q", waitingMessage.Text, waitingText)
	}

	deleteWaiting, ok := client.requested[2].(tgbotapi.DeleteMessageConfig)
	if !ok {
		t.Fatalf("client.requested[2] type = %T, want DeleteMessageConfig", client.requested[2])
	}
	if deleteWaiting.MessageID != 501 {
		t.Fatalf("deleteWaiting.MessageID = %d, want %d", deleteWaiting.MessageID, 501)
	}

	message, ok := client.sent[1].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("client.sent[1] type = %T, want MessageConfig", client.sent[1])
	}
	if message.Text != "📅 20.03.2026\n🕒 08:00, 09:00" {
		t.Fatalf("message.Text = %q, want %q", message.Text, "📅 20.03.2026\n🕒 08:00, 09:00")
	}

	if len(scheduler.slotCalls) != 2 {
		t.Fatalf("slotCalls = %d, want %d", len(scheduler.slotCalls), 2)
	}
}

func TestProcessUpdateSelectedDateRemovesWaitingMessageOnError(t *testing.T) {
	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("LoadLocation returned error: %v", err)
	}

	client := &fakeTelegramClient{
		sendResults: []tgbotapi.Message{
			{MessageID: 601},
			{MessageID: 602},
		},
	}
	scheduler := &mainFakeScheduler{
		availableServicesErr: errors.New("boom"),
	}

	update := tgbotapi.Update{
		CallbackQuery: &tgbotapi.CallbackQuery{
			ID:   "callback-2",
			Data: scheduleDateCallbackData("2026-03-20"),
			Message: &tgbotapi.Message{
				MessageID: 88,
				Chat:      &tgbotapi.Chat{ID: 43},
			},
		},
	}

	err = processUpdate(context.Background(), client, update, bot.Dependencies{
		Scheduler:  scheduler,
		LocationID: 1296020,
		Location:   moscow,
	}, "mybot", 100)
	if err == nil {
		t.Fatal("processUpdate error = nil, want non-nil")
	}

	wantCalls := []string{
		"request:tgbotapi.CallbackConfig",
		"request:tgbotapi.DeleteMessageConfig",
		"send:tgbotapi.MessageConfig",
		"request:tgbotapi.DeleteMessageConfig",
		"send:tgbotapi.MessageConfig",
	}
	if len(client.calls) != len(wantCalls) {
		t.Fatalf("len(client.calls) = %d, want %d", len(client.calls), len(wantCalls))
	}
	for i, want := range wantCalls {
		if client.calls[i] != want {
			t.Fatalf("client.calls[%d] = %q, want %q", i, client.calls[i], want)
		}
	}

	deleteWaiting, ok := client.requested[2].(tgbotapi.DeleteMessageConfig)
	if !ok {
		t.Fatalf("client.requested[2] type = %T, want DeleteMessageConfig", client.requested[2])
	}
	if deleteWaiting.MessageID != 601 {
		t.Fatalf("deleteWaiting.MessageID = %d, want %d", deleteWaiting.MessageID, 601)
	}

	failureMessage, ok := client.sent[1].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("client.sent[1] type = %T, want MessageConfig", client.sent[1])
	}
	if failureMessage.Text != "Failed to load schedule." {
		t.Fatalf("failureMessage.Text = %q, want %q", failureMessage.Text, "Failed to load schedule.")
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
	services             []yclients.Service
	slotsByKey           map[string][]time.Time
	slotCalls            []yclients.SearchTimeSlotsParams
	availableServicesErr error
	searchTimeSlotsErr   error
	mu                   sync.Mutex
}

func (f *mainFakeScheduler) AvailableServices(context.Context, int) ([]yclients.Service, error) {
	if f.availableServicesErr != nil {
		return nil, f.availableServicesErr
	}
	return f.services, nil
}

func (f *mainFakeScheduler) SearchAvailableTimeSlots(_ context.Context, params yclients.SearchTimeSlotsParams) ([]time.Time, error) {
	if f.searchTimeSlotsErr != nil {
		return nil, f.searchTimeSlotsErr
	}

	f.mu.Lock()
	f.slotCalls = append(f.slotCalls, params)
	f.mu.Unlock()

	return f.slotsByKey[mainSlotKey(params.ServiceID, params.Date)], nil
}

func mainSlotKey(serviceID int, date string) string {
	return strconv.Itoa(serviceID) + "|" + date
}

type fakeTelegramClient struct {
	sent        []tgbotapi.Chattable
	requested   []tgbotapi.Chattable
	calls       []string
	sendResults []tgbotapi.Message
}

func (f *fakeTelegramClient) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	f.calls = append(f.calls, "send:"+typeName(c))
	f.sent = append(f.sent, c)
	if len(f.sendResults) > 0 {
		result := f.sendResults[0]
		f.sendResults = f.sendResults[1:]
		return result, nil
	}
	return tgbotapi.Message{}, nil
}

func (f *fakeTelegramClient) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	f.calls = append(f.calls, "request:"+typeName(c))
	f.requested = append(f.requested, c)
	return &tgbotapi.APIResponse{Ok: true}, nil
}

func typeName(value any) string {
	return fmt.Sprintf("%T", value)
}
