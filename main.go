package main

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"yclients_bot/bot"
	"yclients_bot/config"
	"yclients_bot/yclients"
)

const scheduleDateCallbackPrefix = "schedule_date:"

func main() {
	token, err := config.LoadTelegramToken(".env")
	if err != nil {
		log.Fatal("TELEGRAM_BOT_TOKEN is required")
	}

	if err := run(token); err != nil {
		log.Fatal(err)
	}
}

func run(token string) error {
	client, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return err
	}

	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return err
	}

	deps := bot.Dependencies{
		LocationID: 1296020,
		Location:   moscow,
	}

	if yToken := os.Getenv("YCLIENTS_BEARER_TOKEN"); yToken != "" {
		deps.Scheduler = &yclients.Client{
			BaseURL: "https://platform.yclients.com",
			Token:   yToken,
		}
	}

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := client.GetUpdatesChan(updateConfig)
	if updates == nil {
		return errors.New("telegram updates channel was not created")
	}

	for update := range updates {
		responses, err := buildResponsesForUpdate(context.Background(), update, deps, client.Self.UserName, client.Self.ID)
		if err != nil {
			log.Printf("handle update: %v", err)
			if failureResponse, ok := failureResponseForUpdate(update); ok {
				if sendErr := sendResponses(client, []tgbotapi.Chattable{failureResponse}); sendErr != nil {
					log.Printf("send failure reply: %v", sendErr)
				}
			}
			continue
		}

		if err := sendResponses(client, responses); err != nil {
			log.Printf("send reply: %v", err)
		}
	}

	return nil
}

type telegramSender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
}

func buildResponsesForUpdate(ctx context.Context, update tgbotapi.Update, deps bot.Dependencies, botUsername string, botID int64) ([]tgbotapi.Chattable, error) {
	if update.Message != nil {
		normalizedText, ok := bot.NormalizeIncomingText(bot.IncomingMessage{
			Text:        update.Message.Text,
			ChatType:    update.Message.Chat.Type,
			BotUsername: botUsername,
			IsReplyToBot: update.Message.ReplyToMessage != nil &&
				update.Message.ReplyToMessage.From != nil &&
				update.Message.ReplyToMessage.From.ID == botID,
		})
		if !ok {
			return nil, nil
		}

		reply, ok, err := bot.HandleText(ctx, normalizedText, deps)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}

		message := tgbotapi.NewMessage(update.Message.Chat.ID, reply.Text)
		if len(reply.DateOptions) > 0 {
			message.ReplyMarkup = buildDateKeyboard(reply.DateOptions)
		}

		return []tgbotapi.Chattable{message}, nil
	}

	if update.CallbackQuery != nil {
		selectedDate, ok := parseSelectedDateCallback(update.CallbackQuery.Data)
		if !ok || update.CallbackQuery.Message == nil || update.CallbackQuery.Message.Chat == nil {
			return nil, nil
		}

		replyText, ok, err := bot.HandleSelectedDate(ctx, selectedDate, deps)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}

		return []tgbotapi.Chattable{
			tgbotapi.NewCallback(update.CallbackQuery.ID, ""),
			tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID),
			tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, replyText),
		}, nil
	}

	return nil, nil
}

func sendResponses(client telegramSender, responses []tgbotapi.Chattable) error {
	for _, response := range responses {
		switch response.(type) {
		case tgbotapi.CallbackConfig:
			if _, err := client.Request(response); err != nil {
				return err
			}
		case tgbotapi.DeleteMessageConfig:
			if _, err := client.Request(response); err != nil {
				return err
			}
		default:
			if _, err := client.Send(response); err != nil {
				return err
			}
		}
	}

	return nil
}

func failureResponseForUpdate(update tgbotapi.Update) (tgbotapi.MessageConfig, bool) {
	if update.Message != nil && update.Message.Chat != nil {
		return tgbotapi.NewMessage(update.Message.Chat.ID, "Failed to load schedule."), true
	}

	if update.CallbackQuery != nil && update.CallbackQuery.Message != nil && update.CallbackQuery.Message.Chat != nil {
		return tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Failed to load schedule."), true
	}

	return tgbotapi.MessageConfig{}, false
}

func buildDateKeyboard(options []bot.DateOption) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, (len(options)+2)/3)
	currentRow := make([]tgbotapi.InlineKeyboardButton, 0, 3)

	for _, option := range options {
		currentRow = append(currentRow, tgbotapi.NewInlineKeyboardButtonData(option.Label, scheduleDateCallbackData(option.Value)))
		if len(currentRow) == 3 {
			rows = append(rows, currentRow)
			currentRow = make([]tgbotapi.InlineKeyboardButton, 0, 3)
		}
	}

	if len(currentRow) > 0 {
		rows = append(rows, currentRow)
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func scheduleDateCallbackData(date string) string {
	return scheduleDateCallbackPrefix + date
}

func parseSelectedDateCallback(data string) (string, bool) {
	if !strings.HasPrefix(data, scheduleDateCallbackPrefix) {
		return "", false
	}

	selectedDate := strings.TrimSpace(strings.TrimPrefix(data, scheduleDateCallbackPrefix))
	if selectedDate == "" {
		return "", false
	}

	return selectedDate, true
}
