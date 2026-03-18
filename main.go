package main

import (
	"errors"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"yclients_bot/bot"
	"yclients_bot/config"
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

		replyText, ok := bot.ReplyForText(update.Message.Text)
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
