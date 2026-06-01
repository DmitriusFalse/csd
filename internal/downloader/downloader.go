package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DmitriusFalse/csd/internal/logger"
	"github.com/DmitriusFalse/csd/internal/models"
	"go.uber.org/zap"
)

const chunkSize = 1 * 1024 * 1024 // 1 MB
const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

type ProgressCallback func(downloaded, total int64)

type Downloader struct {
	client *http.Client
}

func New() *Downloader {
	return &Downloader{
		client: &http.Client{
			Timeout: 0,
		},
	}
}

func (d *Downloader) buildURL(versionID, fileID int, apiKey string) string {
	url := fmt.Sprintf("https://civitai.com/api/download/models/%d?fileId=%d", versionID, fileID)
	if apiKey != "" {
		url += "&token=" + apiKey
	}
	return url
}

func (d *Downloader) getFileSize(url string) (int64, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HEAD request failed: %s", resp.Status)
	}

	length := resp.Header.Get("Content-Length")
	if length == "" {
		return 0, nil
	}
	return strconv.ParseInt(length, 10, 64)
}

func (d *Downloader) Download(ctx context.Context, task *models.DownloadTask, onProgress ProgressCallback) error {
	downloadURL := d.buildURL(task.ModelVersionID, task.FileID, task.APIKey)

	totalBytes, err := d.getFileSize(downloadURL)
	if err != nil {
		totalBytes = 0
	}
	task.FileSizeBytes = totalBytes

	dir := filepath.Dir(task.SavePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	tempPath := task.SavePath + ".part"
	task.TempPath = tempPath

	var downloadedBytes int64
	var file *os.File

	stat, err := os.Stat(tempPath)
	if err == nil && stat.Size() > 0 {
		downloadedBytes = stat.Size()
	}

	if err == nil && downloadedBytes > 0 {
		file, err = os.OpenFile(tempPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open partial file: %w", err)
		}
	} else {
		downloadedBytes = 0
		file, err = os.Create(tempPath)
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}
	}
	defer file.Close()

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	if downloadedBytes > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", downloadedBytes))
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("download failed (status %d)", resp.StatusCode)
	}

	if totalBytes == 0 {
		length := resp.Header.Get("Content-Length")
		if l, err := strconv.ParseInt(length, 10, 64); err == nil {
			totalBytes = l
			task.FileSizeBytes = totalBytes
		}
	}

	if totalBytes > 0 && downloadedBytes > 0 {
		rangeHeader := resp.Header.Get("Content-Range")
		if rangeHeader != "" {
			parts := strings.Split(rangeHeader, "/")
			if len(parts) == 2 {
				if total, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					totalBytes = total
					task.FileSizeBytes = totalBytes
				}
			}
		}
	}

	buf := make([]byte, chunkSize)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write error: %w", writeErr)
			}
			downloadedBytes += int64(n)
			task.DownloadedBytes = downloadedBytes

			if totalBytes > 0 {
				task.Progress = float64(downloadedBytes) / float64(totalBytes) * 100
			}

			if onProgress != nil {
				onProgress(downloadedBytes, totalBytes)
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read error: %w", readErr)
		}
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if totalBytes > 0 && downloadedBytes != totalBytes {
		return fmt.Errorf("incomplete download: %d of %d bytes", downloadedBytes, totalBytes)
	}

	if err := os.Rename(tempPath, task.SavePath); err != nil {
		return fmt.Errorf("rename file: %w", err)
	}

	task.Progress = 100
	task.DownloadedBytes = downloadedBytes
	task.FileSizeBytes = totalBytes

	logger.Log.Info("Download completed",
		zap.String("file", task.SavePath),
		zap.Int64("bytes", totalBytes),
	)

	return nil
}

func (d *Downloader) GetExistingBytes(task *models.DownloadTask) int64 {
	stat, err := os.Stat(task.TempPath)
	if err != nil {
		return 0
	}
	return stat.Size()
}

func (d *Downloader) RemoveTempFile(task *models.DownloadTask) {
	os.Remove(task.TempPath)
}

func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func ParseFileSize(text string) (int64, error) {
	text = strings.TrimSpace(text)
	parts := strings.Split(text, " ")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid size: %s", text)
	}

	value, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}

	unit := strings.ToUpper(parts[1])
	switch unit {
	case "B":
		return int64(value), nil
	case "KB":
		return int64(value * 1024), nil
	case "MB":
		return int64(value * 1024 * 1024), nil
	case "GB":
		return int64(value * 1024 * 1024 * 1024), nil
	case "TB":
		return int64(value * 1024 * 1024 * 1024 * 1024), nil
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
}

func init() {
	_ = time.Now
}
