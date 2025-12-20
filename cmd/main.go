package main

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mojtabafarzaneh/downloaderBot/cmd/bot"
)

func main() {
	// Entry point for the bot application

	var botKey = "8421057487:AAG30VyBpJ8yNOtlNavMWyivrFK-LwluOZ0"

	fmt.Println("Bot is starting...")

	b, err := tgbotapi.NewBotAPI(botKey)
	if err != nil {
		log.Fatal(err)
	}

	b.Debug = true

	log.Printf("Authorized as %s", b.Self.UserName)

	bot.Start(b)
}
