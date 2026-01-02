package downloader

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type MediaFile struct {
	Path string
	Type string
}
type InstagramPost struct {
	Caption string
	Media   []MediaFile
}

var sendMutex sync.Mutex

func InstagramDownloader(url string) (*InstagramPost, string, error) {

	uniqueDir := fmt.Sprintf("instagram_%d_%s", time.Now().UnixNano(), randString(5))
	tempDir := filepath.Join(os.TempDir(), uniqueDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, "", err
	}

	cookiesPath := os.Getenv("GALLERYDL_COOKIES")
	if cookiesPath == "" {
		return nil, "", errors.New("GALLERYDL_COOKIES not set")
	}
	cmd := exec.Command(
		"gallery-dl",
		"--cookies", cookiesPath,
		"-d", tempDir,
		"--directory", "",
		"-o", "%(playlist_index)s.%(ext)s",
		"--write-metadata",
		"--retries", "3",
		"--sleep-request", "2.0",
		"-o", "postprocessor.exec.command=ffmpeg -i {} -c:v libx265 -crf 30 -preset medium -vf scale=720:-2 -c:a aac -b:a 64k -movflags +faststart -tag:v hvc1 -y {}.tmp.mp4 && mv {}.tmp.mp4 {}", "-o", "postprocessor.exec.event=after",
		url,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, "", fmt.Errorf("gallery-dl error: %v, output: %s", err, string(output))
	}

	files, err := os.ReadDir(tempDir)
	if err != nil {
		return nil, "", err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	var media []MediaFile
	for _, f := range files {
		path := filepath.Join(tempDir, f.Name())
		ext := filepath.Ext(f.Name())
		fileType := "photo"
		if ext != ".jpg" && ext != ".png" && ext != ".mp4" {
			continue
		}
		if ext == ".mp4" {
			fileType = "video"
		}
		media = append(media, MediaFile{
			Path: path,
			Type: fileType,
		})
	}

	var caption string
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			data, _ := os.ReadFile(filepath.Join(tempDir, f.Name()))
			var j map[string]interface{}
			json.Unmarshal(data, &j)
			if desc, ok := j["description"].(string); ok && desc != "" {
				caption = desc
				break
			}
		}
	}

	return &InstagramPost{
		Caption: caption,
		Media:   media,
	}, tempDir, nil

}

func SendFilesToTelegram(
	instagramPost InstagramPost, chatID int64,
	bot *tgbotapi.BotAPI, tempDir string, messageId int, replyToMessage *tgbotapi.Message, url string) {

	sendMutex.Lock()
	defer sendMutex.Unlock()

	var tgMedia []interface{}
	for _, m := range instagramPost.Media {
		if m.Type == "video" {
			tgMedia = append(tgMedia, tgbotapi.NewInputMediaVideo(tgbotapi.FilePath(m.Path)))
		} else {
			tgMedia = append(tgMedia, tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(m.Path)))
		}
	}

	for i := 0; i < len(tgMedia); i += 10 {
		end := i + 10
		if end > len(tgMedia) {
			end = len(tgMedia)
		}
		bot.Send(tgbotapi.NewMediaGroup(chatID, tgMedia[i:end]))

	}

	if replyToMessage.ReplyToMessage != nil {
		msg := tgbotapi.NewMessage(chatID,
			html.EscapeString(instagramPost.Caption)+
				"\n\nSource: <a href=\""+url+"\">Instagram</a>"+
				"\n\nSent by: <a href=\"tg://user?id="+strconv.FormatInt(replyToMessage.From.ID, 10)+"\">"+
				html.EscapeString(replyToMessage.From.UserName)+
				"</a>"+"\n\nIn reply to: <a href=\"tg://user?id="+strconv.FormatInt(replyToMessage.ReplyToMessage.From.ID, 10)+"\">"+
				html.EscapeString(replyToMessage.ReplyToMessage.From.UserName)+
				"</a>",
		)

		msg.ParseMode = tgbotapi.ModeHTML
		msg.DisableWebPagePreview = true

		bot.Send(msg)

	} else {
		msg := tgbotapi.NewMessage(chatID,
			html.EscapeString(instagramPost.Caption)+
				"\n\nSource: <a href=\""+url+"\">Instagram</a>"+
				"\n\nSent by: <a href=\"tg://user?id="+strconv.FormatInt(replyToMessage.From.ID, 10)+"\">"+
				html.EscapeString(replyToMessage.From.UserName)+
				"</a>",
		)

		msg.ParseMode = tgbotapi.ModeHTML
		msg.DisableWebPagePreview = true

		bot.Send(msg)
	}

	del := tgbotapi.NewDeleteMessage(chatID, messageId)
	bot.Request(del)

	deluserLink := tgbotapi.NewDeleteMessage(chatID, replyToMessage.MessageID)
	bot.Request(deluserLink)

	os.RemoveAll(tempDir)
}

func randString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[randInt(len(letters))]
	}
	return string(b)
}

func randInt(max int) int {
	return int(os.Getpid() * int(os.Getuid()) % max)
}
