package main

import (
	"fmt"
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/mojtabafarzaneh/downloaderBot/cmd/bot"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
	var botKey = os.Getenv("BOTKEY")
	if botKey == "" {
		log.Fatal("BOTKEY not set")
	}

	fmt.Println("Bot is starting...")

	b, err := tgbotapi.NewBotAPI(botKey)
	if err != nil {
		log.Fatal(err)
	}

	b.Debug = true
	//b.SetAPIEndpoint("http://localhost:8081/bot%s/%s")
	log.Printf("Authorized as %s", b.Self.UserName)

	bot.Start(b)
}
