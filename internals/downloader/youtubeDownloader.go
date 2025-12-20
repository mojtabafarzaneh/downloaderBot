package downloader

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type YouTubeDownloader struct{}

func (yt YouTubeDownloader) GetFormats(url string) ([]FormatInfo, error) {
	cmd := exec.Command(
		"yt-dlp",
		"--cookies-from-browser", "firefox",
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
		if res == "256x144" {
			res = "144p"
		} else if res == "426x240" {
			res = "240p"
		} else if res == "640x360" {
			res = "360p"
		} else if res == "854x480" {
			res = "480p"
		} else if res == "1280x720" {
			res = "720p"
		} else if res == "1920x1080" {
			res = "1080p"
		} else if res == "2560x1440" {
			res = "1440p"
		} else if res == "3840x2160" {
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
func (yt YouTubeDownloader) Download(url, formatID, outputPath string) error {
	cmd := exec.Command(
		"yt-dlp",
		"--cookies-from-browser", "firefox",
		"-f", formatID,
		"-o", outputPath,
		url,
	)

	_, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
}
