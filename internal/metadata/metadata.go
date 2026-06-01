package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DmitriusFalse/csd/internal/models"
)

type WebhookPayload struct {
	Action    string `json:"action"`
	ModelPath string `json:"model_path"`
	ModelType string `json:"model_type"`
	Metadata  *models.ModelMetadata `json:"metadata,omitempty"`
}

func BuildMetadata(task *models.DownloadTask, info *models.CivitaiModelResponse) *models.ModelMetadata {
	meta := &models.ModelMetadata{}

	meta.CivitaiMetadata.ModelID = task.ModelVersionID
	meta.CivitaiMetadata.ModelVersionID = task.ModelVersionID
	meta.CivitaiMetadata.FileID = task.FileID
	meta.CivitaiMetadata.ModelName = task.ModelName
	meta.CivitaiMetadata.ModelType = string(task.ModelType)
	meta.CivitaiMetadata.BaseModel = task.BaseModel
	meta.CivitaiMetadata.DownloadedAt = time.Now()

	meta.FileInfo.FileName = task.FileName
	meta.FileInfo.FileSize = task.FileSize
	meta.FileInfo.FileSizeBytes = task.FileSizeBytes

	if strings.HasSuffix(strings.ToLower(task.FileName), ".safetensors") {
		meta.FileInfo.Format = "SafeTensor"
	} else if strings.HasSuffix(strings.ToLower(task.FileName), ".ckpt") {
		meta.FileInfo.Format = "CKPT"
	} else {
		meta.FileInfo.Format = "Unknown"
	}

	if info != nil {
		meta.Author.Username = info.Creator.Username
		meta.Author.UserID = info.Creator.UserID
		meta.Author.ProfileURL = fmt.Sprintf("https://civitai.com/user/%s", info.Creator.Username)

		meta.Description = info.Description

		for _, tag := range info.Tags {
			meta.Tags = append(meta.Tags, tag.Name)
		}

		meta.CivitaiMetadata.ModelID = info.ModelID

		for _, img := range info.PreviewImages {
			meta.PreviewImages = append(meta.PreviewImages, img.URL)
		}
	}

	return meta
}

func SaveJSON(meta *models.ModelMetadata, modelPath string) error {
	jsonPath := strings.TrimSuffix(modelPath, filepath.Ext(modelPath)) + ".json"

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		return fmt.Errorf("write metadata file: %w", err)
	}

	return nil
}

func SendWebhook(url, method string, task *models.DownloadTask) error {
	var bodyReader io.Reader
	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		payload := WebhookPayload{
			Action:    "model_added",
			ModelPath: task.SavePath,
			ModelType: string(task.ModelType),
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal webhook: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
