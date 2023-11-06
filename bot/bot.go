package bot

import (
	"errors"
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

var NotFoundEnvError = errors.New("env not found")

type TelegramBot struct {
	token string
}

func NewTelegramBot() *TelegramBot {
	return new(TelegramBot)
}

func (tb *TelegramBot) SetTokenFromEnv(env string) error {
	if err := godotenv.Load(); err != nil {
		return err
	}
	tb.token = os.Getenv(env)
	if tb.token == "" {
		return NotFoundEnvError
	}
	return nil
}

func (tb *TelegramBot) Start(isDebug bool) error {
	bot, err := tgbotapi.NewBotAPI(tb.token)
	if err != nil {
		return err
	}

	bot.Debug = isDebug

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil { // If we got a message
			log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
			msg.ReplyToMessageID = update.Message.MessageID

			_, err = bot.Send(msg)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
