package bot

import (
	"bufio"
	"fmt"
	"html"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mojtabafarzaneh/downloaderBot/internals/downloader"
)

func Start(bot *tgbotapi.BotAPI) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)
	// updates := bot.ListenForWebhook("/" + bot.Token)

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

			edit := tgbotapi.EditMessageTextConfig{
				BaseEdit: tgbotapi.BaseEdit{
					ChatID:    chatID,
					MessageID: mess,
				},
				Text: "Downloading...",
			}
			bot.Send(edit)

			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "Starting download...")
			if _, err := bot.Request(callback); err != nil {
				fmt.Println("Failed to answer callback:", err)
			}

			go func() {
				progressMsg := tgbotapi.NewMessage(chatID, "Downloading: 0%")
				sentMsg, _ := bot.Send(progressMsg)
				progressMsgID := sentMsg.MessageID
				lastUpdate := time.Now().Add(-6 * time.Second)

				tempFile := fmt.Sprintf("%s.mp4", formatID)

				cookiesPath := os.Getenv("YTDLP_COOKIES")
				if cookiesPath == "" {
					log.Fatal("YTDLP_COOKIES not set")
				}

				cmd := exec.Command(
					"yt-dlp",
					"--cookies", cookiesPath,
					"-f", formatID,
					"-o", tempFile,
					downloadLink,
					"--newline",
				)
				stdout, _ := cmd.StdoutPipe()
				cmd.Stderr = cmd.Stdout
				cmd.Start()

				scanner := bufio.NewScanner(stdout)
				for scanner.Scan() {
					line := scanner.Text()
					percent := parsePercentage(line)
					if percent > 0 && time.Since(lastUpdate) >= 5*time.Second {
						edit := tgbotapi.NewEditMessageText(chatID, progressMsgID,
							fmt.Sprintf("Downloading: %d%%", percent))
						bot.Send(edit)
						lastUpdate = time.Now()
					}
				}
				cmd.Wait()

				file, _ := os.Open(tempFile)
				stat, _ := file.Stat()
				pr := &progressReader{
					Reader: file,
					total:  stat.Size(),
					callback: func(percent int) {
						if time.Since(lastUpdate) >= 5*time.Second {
							edit := tgbotapi.NewEditMessageText(chatID, progressMsgID,
								fmt.Sprintf("Uploading: %d%%", percent))
							bot.Send(edit)
							lastUpdate = time.Now()
						}
					},
				}

				cmd.Wait()

				videoMsg := tgbotapi.NewVideo(chatID, tgbotapi.FileReader{
					Name:   stat.Name(),
					Reader: pr,
				})
				bot.Send(videoMsg)
				os.Remove(tempFile)

				edit := tgbotapi.NewEditMessageText(chatID, progressMsgID, "Download and upload complete!")
				bot.Send(edit)
			}()

			continue
		}

		if update.Message == nil || update.Message.Text == "" {
			continue
		}

		ProvidedUrl := update.Message.Text
		chatID := update.Message.Chat.ID

		var dl downloader.Downloader
		switch {
		case strings.Contains(ProvidedUrl, "youtube.com") || strings.Contains(ProvidedUrl, "youtu.be"):
			dl = downloader.YouTubeDownloader{}
			formats, err := dl.GetFormats(ProvidedUrl)
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

			newMesg := tgbotapi.NewMessage(chatID,
				"\n\nSource: <a href=\""+modifiedURL+"\">Twitter</a>"+
					"\n\nSent by: <a href=\"tg://user?id="+strconv.FormatInt(update.Message.From.ID, 10)+"\">"+
					html.EscapeString(update.Message.From.UserName)+
					"</a>")

			newMesg.ParseMode = tgbotapi.ModeHTML

			bot.Send(newMesg)
			deluserLink := tgbotapi.NewDeleteMessage(chatID, update.Message.MessageID)
			bot.Request(deluserLink)

		default:
			continue
		}

	}
}

func parsePercentage(line string) int {
	line = strings.TrimSpace(line)
	if !strings.Contains(line, "[download]") {
		return 0
	}

	re := regexp.MustCompile(`(\d+(\.\d+)?)%`)
	match := re.FindStringSubmatch(line)
	if len(match) < 2 {
		return 0
	}

	percent, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0
	}

	return int(percent)
}

type progressReader struct {
	Reader   io.Reader
	total    int64
	read     int64
	callback func(percent int)
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	pr.read += int64(n)
	if pr.total > 0 && pr.callback != nil {
		percent := int((float64(pr.read) / float64(pr.total)) * 100)
		pr.callback(percent)
	}
	return
}
