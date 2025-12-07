package downloader

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func Download(url string) (string, int64, error) {
	tmpDir, err := os.MkdirTemp("", "dl-*")
	if err != nil {
		return "", 0, err
	}

	output := filepath.Join(tmpDir, "video.%(ext)s")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp", "-f", "best", "-o", output, url)
	cmd.Dir = tmpDir
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return "", 0, fmt.Errorf("yt-dlp error: %w", err)
	}

	var filePath string
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			filePath = path
			return filepath.SkipDir
		}
		return nil
	})

	if filePath == "" {
		return "", 0, fmt.Errorf("downloaded file not found")
	}

	info, _ := os.Stat(filePath)
	return filePath, info.Size(), nil
}
