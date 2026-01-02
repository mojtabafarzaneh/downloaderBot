package bot

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mojtabafarzaneh/downloaderBot/internals/downloader"
)

func Start(bot *tgbotapi.BotAPI) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {

		if update.CallbackQuery != nil {
			dataParts := strings.SplitN(update.CallbackQuery.Data, "|", 2)
			if len(dataParts) != 2 {
				continue
			}

			formatID := dataParts[0]
			downloadLink := dataParts[1]
			chatID := update.CallbackQuery.Message.Chat.ID
			mess := update.CallbackQuery.Message.MessageID

			bot.Send(tgbotapi.NewDeleteMessage(chatID, mess))

			go func() {
				downloader.DownloadFromYouTube(formatID, chatID, downloadLink, bot)

			}()

			continue
		}

		if update.Message == nil || update.Message.Text == "" {
			continue
		}

		ProvidedUrl := update.Message.Text
		chatID := update.Message.Chat.ID

		switch {
		case strings.Contains(ProvidedUrl, "youtube.com") || strings.Contains(ProvidedUrl, "youtu.be"):
			formats, err := downloader.GetFormats(ProvidedUrl)
			if err != nil || len(formats) == 0 {
				bot.Send(tgbotapi.NewMessage(chatID, "Failed to fetch formats"))
				continue
			}

			var buttons [][]tgbotapi.InlineKeyboardButton
			for _, f := range formats {
				data := fmt.Sprintf("%s|%s", f.FormatID, ProvidedUrl)
				btn := tgbotapi.NewInlineKeyboardButtonData(f.Display, data)
				buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(btn))
			}
			keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)

			msg := tgbotapi.NewMessage(chatID, "Choose a format:")
			msg.ReplyMarkup = keyboard
			bot.Send(msg)
		case strings.Contains(ProvidedUrl, "instagram.com"):

			msg, _ := bot.Send(tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:           chatID,
					ReplyToMessageID: update.Message.MessageID,
				},
				Text: "Downloading files from Instagram",
			})

			go func(url string, chatID int64) {

				instagramPost, tempDir, err := downloader.InstagramDownloader(url)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(chatID, "Download failed"))
					log.Printf("gallery-dl error: %v", err)

					return
				}

				downloader.SendFilesToTelegram(*instagramPost, chatID, bot, tempDir, msg.MessageID, update.Message, url)

			}(ProvidedUrl, chatID)
		case
			strings.Contains(ProvidedUrl, "twitter.com") || strings.Contains(ProvidedUrl, "t.co") || strings.Contains(ProvidedUrl, "x.com"):
			u, err := url.Parse(ProvidedUrl)
			if err != nil {
				fmt.Println("Invalid URL:", err)
				continue
			}

			u.Host = "fxtwitter.com"
			modifiedURL := u.String()
			if update.Message.ReplyToMessage != nil {
				safeUser := strings.NewReplacer(
					"&", "&amp;",
					"<", "&lt;",
					">", "&gt;",
				).Replace(update.Message.From.UserName)
				safeReplyUser := strings.NewReplacer(
					"&", "&amp;",
					"<", "&lt;",
					">", "&gt;",
				).Replace(update.Message.ReplyToMessage.From.UserName)

				newMesg := tgbotapi.NewMessage(chatID,
					"\n\nSource: <a href=\""+modifiedURL+"\">Twitter</a>"+
						"\n\nSent by: <a href=\"tg://user?id="+strconv.FormatInt(update.Message.From.ID, 10)+"\">"+
						safeUser+
						"</a>"+
						"\n\nInReply To: <a href=\"tg://user?id="+strconv.FormatInt(update.Message.ReplyToMessage.From.ID, 10)+"\">"+
						safeReplyUser+
						"</a>",
				)
				newMesg.ParseMode = tgbotapi.ModeHTML

				bot.Send(newMesg)
				deluserLink := tgbotapi.NewDeleteMessage(chatID, update.Message.MessageID)
				bot.Request(deluserLink)

			} else {
				safeUser := strings.NewReplacer(
					"&", "&amp;",
					"<", "&lt;",
					">", "&gt;",
				).Replace(update.Message.From.UserName)

				newMesg := tgbotapi.NewMessage(chatID,
					"\n\nSource: <a href=\""+modifiedURL+"\">Twitter</a>"+
						"\n\nSent by: <a href=\"tg://user?id="+strconv.FormatInt(update.Message.From.ID, 10)+"\">"+
						safeUser+
						"</a>",
				)
				newMesg.ParseMode = tgbotapi.ModeHTML

				bot.Send(newMesg)
				deluserLink := tgbotapi.NewDeleteMessage(chatID, update.Message.MessageID)
				bot.Request(deluserLink)
			}

		default:
			continue
		}

	}
}
