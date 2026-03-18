package main

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"yclients_bot/bot"
	"yclients_bot/config"
	"yclients_bot/yclients"
)

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
		if update.Message == nil {
			continue
		}

		normalizedText, ok := bot.NormalizeIncomingText(bot.IncomingMessage{
			Text:        update.Message.Text,
			ChatType:    update.Message.Chat.Type,
			BotUsername: client.Self.UserName,
			IsReplyToBot: update.Message.ReplyToMessage != nil &&
				update.Message.ReplyToMessage.From != nil &&
				update.Message.ReplyToMessage.From.ID == client.Self.ID,
		})
		if !ok {
			continue
		}

		replyText, ok, err := bot.HandleText(context.Background(), normalizedText, deps)
		if err != nil {
			log.Printf("handle text: %v", err)
			replyText = "Failed to load schedule."
			ok = true
		}
		if !ok {
			continue
		}

		message := tgbotapi.NewMessage(update.Message.Chat.ID, replyText)
		if _, err := client.Send(message); err != nil {
			log.Printf("send reply: %v", err)
		}
	}

	return nil
}
