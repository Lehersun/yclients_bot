package bot

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"yclients_bot/yclients"
)

type Scheduler interface {
	AvailableServices(ctx context.Context, locationID int) ([]yclients.Service, error)
	SearchAvailableTimeSlots(ctx context.Context, params yclients.SearchTimeSlotsParams) ([]time.Time, error)
}

type Dependencies struct {
	Scheduler  Scheduler
	LocationID int
	Location   *time.Location
	Now        func() time.Time
}

type DateOption struct {
	Label string
	Value string
}

type Response struct {
	Text        string
	DateOptions []DateOption
}

type IncomingMessage struct {
	Text         string
	ChatType     string
	BotUsername  string
	IsReplyToBot bool
}

// ReplyForText decides whether an incoming text message should trigger a reply.
func ReplyForText(text string) (string, bool) {
	if strings.EqualFold(normalizeCommandText(text), "hello") {
		return "Hello!", true
	}

	return "", false
}

func NormalizeIncomingText(message IncomingMessage) (string, bool) {
	text := strings.TrimSpace(message.Text)
	if text == "" {
		return "", false
	}

	if message.ChatType == "" || message.ChatType == "private" {
		return normalizeCommandText(text), true
	}

	if message.IsReplyToBot {
		return normalizeCommandText(text), true
	}

	if message.BotUsername == "" {
		return "", false
	}

	if normalized, ok := normalizeMentionedSlashCommand(text, message.BotUsername); ok {
		return normalized, true
	}

	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", false
	}

	mention := "@" + strings.ToLower(message.BotUsername)
	if strings.ToLower(fields[0]) != mention {
		return "", false
	}

	normalized := normalizeCommandText(strings.TrimSpace(strings.Join(fields[1:], " ")))
	if normalized == "" {
		return "", false
	}

	return normalized, true
}

func HandleText(_ context.Context, text string, deps Dependencies) (Response, bool, error) {
	text = normalizeCommandText(text)

	if reply, ok := ReplyForText(text); ok {
		return Response{Text: reply}, true, nil
	}

	if !isScheduleCommand(text) {
		return Response{Text: helpText()}, true, nil
	}

	if deps.Scheduler == nil {
		return Response{Text: "Schedule is not configured."}, true, nil
	}

	if deps.LocationID == 0 {
		return Response{}, true, fmt.Errorf("location id is required")
	}

	location := deps.Location
	if location == nil {
		location = time.Local
	}

	nowFn := deps.Now
	if nowFn == nil {
		nowFn = time.Now
	}

	return Response{
		Text:        "Выберите дату",
		DateOptions: buildDateOptions(nowFn().In(location)),
	}, true, nil
}

func HandleSelectedDate(ctx context.Context, date string, deps Dependencies) (string, bool, error) {
	if deps.Scheduler == nil {
		return "Schedule is not configured.", true, nil
	}

	if deps.LocationID == 0 {
		return "", true, fmt.Errorf("location id is required")
	}

	location := deps.Location
	if location == nil {
		location = time.Local
	}

	selectedDate, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(date), location)
	if err != nil {
		return "", false, nil
	}

	services, err := deps.Scheduler.AvailableServices(ctx, deps.LocationID)
	if err != nil {
		return "", true, err
	}

	type slotResult struct {
		Slots []time.Time
		Err   error
	}

	results := make(chan slotResult, len(services))
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup

	for _, service := range services {
		wg.Add(1)
		go func(serviceID int) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			slots, err := deps.Scheduler.SearchAvailableTimeSlots(ctx, yclients.SearchTimeSlotsParams{
				LocationID: deps.LocationID,
				Date:       selectedDate.Format("2006-01-02"),
				ServiceID:  serviceID,
			})
			results <- slotResult{
				Slots: slots,
				Err:   err,
			}
		}(service.ID)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	timeSet := map[string]bool{}
	for result := range results {
		if result.Err != nil {
			return "", true, result.Err
		}

		for _, slot := range result.Slots {
			timeSet[slot.In(location).Format("15:04")] = true
		}
	}

	timeKeys := make([]string, 0, len(timeSet))
	for timeKey := range timeSet {
		timeKeys = append(timeKeys, timeKey)
	}
	sort.Strings(timeKeys)

	return formatSelectedDate(selectedDate, timeKeys), true, nil
}

func buildDateOptions(baseDate time.Time) []DateOption {
	options := make([]DateOption, 0, 7)
	for dayOffset := 0; dayOffset < 7; dayOffset++ {
		currentDate := baseDate.AddDate(0, 0, dayOffset)
		options = append(options, DateOption{
			Label: currentDate.Format("02.01"),
			Value: currentDate.Format("2006-01-02"),
		})
	}

	return options
}

func formatSelectedDate(selectedDate time.Time, timeKeys []string) string {
	if len(timeKeys) == 0 {
		return fmt.Sprintf("📅 %s\n🕒 Нет свободных слотов", selectedDate.Format("02.01.2006"))
	}

	return fmt.Sprintf("📅 %s\n🕒 %s", selectedDate.Format("02.01.2006"), strings.Join(timeKeys, ", "))
}

func normalizeCommandText(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}

	if !strings.HasPrefix(trimmed, "/") {
		return trimmed
	}

	command := strings.Fields(trimmed)[0]
	command = strings.TrimPrefix(command, "/")
	if command == "" {
		return ""
	}

	name, _, _ := strings.Cut(command, "@")
	return strings.TrimSpace(name)
}

func normalizeMentionedSlashCommand(text string, botUsername string) (string, bool) {
	command := strings.Fields(strings.TrimSpace(text))
	if len(command) == 0 || !strings.HasPrefix(command[0], "/") {
		return "", false
	}

	name := strings.TrimPrefix(command[0], "/")
	commandName, mentionedBot, hasMention := strings.Cut(name, "@")
	if !hasMention || !strings.EqualFold(mentionedBot, botUsername) {
		return "", false
	}

	return strings.TrimSpace(commandName), true
}

func isScheduleCommand(text string) bool {
	normalized := strings.TrimSpace(strings.ToLower(text))
	return normalized == "schedule" || normalized == "расписание"
}

func helpText() string {
	return strings.Join([]string{
		"Я показываю доступные падел-корты на Фрунзе.",
		"После выбора даты отправляю свободное время для бронирования.",
		"Как воспользоваться: напишите `расписание`.",
	}, "\n")
}
