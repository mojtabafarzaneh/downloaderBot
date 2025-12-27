package downloader

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type YouTubeDownloader struct{}

func GetFormats(url string) ([]FormatInfo, error) {

	cookiesPath := os.Getenv("YTDLP_COOKIES")
	if cookiesPath == "" {
		return nil, errors.New("YTDLP_COOKIES not set")
	}
	cmd := exec.Command(
		"yt-dlp",
		"--cookies", cookiesPath,
		"-j",
		url,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run yt-dlp: %w", err)
	}

	var info VideoInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	var b strings.Builder
	b.WriteString("Available formats:\n\n")
	formats := []FormatInfo{}

	for _, f := range info.Formats {
		if f.FormatID == "" || f.Ext == "" {
			continue
		}

		size := f.Filesize
		if size == nil && f.TBR != nil {
			est := int64((*f.TBR * info.Duration * 1024) / 8)
			size = &est
		}

		sizeMB := "?"
		if size != nil {
			sizeMB = fmt.Sprintf("%.2f MB", float64(*size)/1024/1024)
		} else {
			continue
		}
		formatType := "unknown"
		if f.Vcodec != "none" && f.Acodec != "none" {
			formatType = "video"
		} else if f.Vcodec != "none" && f.Acodec == "none" {
			formatType = "video-only"
		} else if f.Vcodec == "none" && f.Acodec != "none" {
			formatType = "audio-only"
		}

		res := f.Resolution
		if res == "" && formatType == "audio-only" {
			res = "audio"
		} else if res == "" {
			res = "unknown"
		}
		switch res {
		case "256x144":
			res = "144p"
		case "426x240":
			res = "240p"
		case "640x360":
			res = "360p"
		case "854x480":
			res = "480p"
		case "1280x720":
			res = "720p"
		case "1920x1080":
			res = "1080p"
		case "2560x1440":
			res = "1440p"
		case "3840x2160":
			res = "2160p"
		}

		display := fmt.Sprintf("%s | %s | %s", f.Ext, res, sizeMB)
		formats = append(formats, FormatInfo{
			FormatID:   f.FormatID,
			Display:    display,
			FilesizeMB: sizeMB,
		})
	}
	return formats, nil
}
func DownloadFromYouTube(formatID string, chatID int64, downloadLink string, bot *tgbotapi.BotAPI) {

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

	bot.Send(tgbotapi.NewDeleteMessage(chatID, sentMsg.MessageID))
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
