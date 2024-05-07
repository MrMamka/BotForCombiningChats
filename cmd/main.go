package main

import (
	"log"

	"github.com/MrMamka/combchats/internal/bot"
)

func main() {
	tgBot := bot.NewTelegramBot()
	if err := tgBot.SetTokenFromEnv("BOT_TOKEN"); err != nil {
		log.Fatalf("Error while setting token: %v", err)
	}
	if err := tgBot.Start(true); err != nil {
		log.Fatalf("Error during bot working: %v", err)
	}
}
