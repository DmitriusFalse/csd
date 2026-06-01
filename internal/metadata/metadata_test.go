package metadata

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DmitriusFalse/csd/internal/models"
)

func TestBuildMetadataNoInfo(t *testing.T) {
	task := &models.DownloadTask{
		ModelVersionID: 100,
		FileID:         200,
		ModelName:      "test-lora",
		FileName:       "test-lora.safetensors",
		FileSize:       "100MB",
		FileSizeBytes:  100000000,
		ModelType:      models.ModelTypeLORA,
		BaseModel:      "SDXL",
	}

	meta := BuildMetadata(task, nil)

	if meta.CivitaiMetadata.ModelID != 100 {
		t.Errorf("expected ModelID from task, got %d", meta.CivitaiMetadata.ModelID)
	}
	if meta.CivitaiMetadata.ModelVersionID != 100 {
		t.Errorf("expected ModelVersionID 100, got %d", meta.CivitaiMetadata.ModelVersionID)
	}
	if meta.CivitaiMetadata.FileID != 200 {
		t.Errorf("expected FileID 200, got %d", meta.CivitaiMetadata.FileID)
	}
	if meta.CivitaiMetadata.ModelName != "test-lora" {
		t.Errorf("expected ModelName test-lora, got %s", meta.CivitaiMetadata.ModelName)
	}
	if meta.CivitaiMetadata.ModelType != "LORA" {
		t.Errorf("expected ModelType LORA, got %s", meta.CivitaiMetadata.ModelType)
	}
	if meta.CivitaiMetadata.BaseModel != "SDXL" {
		t.Errorf("expected BaseModel SDXL, got %s", meta.CivitaiMetadata.BaseModel)
	}
	if meta.CivitaiMetadata.DownloadedAt.IsZero() {
		t.Errorf("expected DownloadedAt to be set")
	}
	if meta.FileInfo.FileName != "test-lora.safetensors" {
		t.Errorf("expected FileName test-lora.safetensors")
	}
	if meta.FileInfo.FileSize != "100MB" {
		t.Errorf("expected FileSize 100MB")
	}
	if meta.FileInfo.FileSizeBytes != 100000000 {
		t.Errorf("expected FileSizeBytes 100000000")
	}
	if meta.FileInfo.Format != "SafeTensor" {
		t.Errorf("expected Format SafeTensor, got %s", meta.FileInfo.Format)
	}
	if meta.Author.Username != "" {
		t.Errorf("expected empty author without info")
	}
	if len(meta.Tags) != 0 {
		t.Errorf("expected no tags without info")
	}
}

func TestBuildMetadataWithInfo(t *testing.T) {
	task := &models.DownloadTask{
		ModelVersionID: 100,
		FileID:         200,
		ModelName:      "test-checkpoint",
		FileName:       "test-checkpoint.ckpt",
		FileSize:       "2GB",
		FileSizeBytes:  2000000000,
		ModelType:      models.ModelTypeCheckpoint,
		BaseModel:      "SD1.5",
	}

	info := &models.CivitaiModelResponse{
		ModelID:   50,
		ModelName: "parent-model",
		Name:      "test-checkpoint",
		Type:      "Checkpoint",
		Creator: models.CivitaiCreator{
			Username: "artist1",
			UserID:   99,
		},
		Description: "A great model",
		Tags: []models.CivitaiTag{
			{Name: "realistic"},
			{Name: "portrait"},
		},
		PreviewImages: []models.CivitaiPreviewImage{
			{URL: "https://example.com/1.jpg", Type: "image"},
			{URL: "https://example.com/2.jpg", Type: "video"},
		},
	}

	meta := BuildMetadata(task, info)

	if meta.CivitaiMetadata.ModelID != 50 {
		t.Errorf("expected ModelID from info (50), got %d", meta.CivitaiMetadata.ModelID)
	}
	if meta.FileInfo.Format != "CKPT" {
		t.Errorf("expected Format CKPT for .ckpt file, got %s", meta.FileInfo.Format)
	}
	if meta.Author.Username != "artist1" {
		t.Errorf("expected author artist1, got %s", meta.Author.Username)
	}
	if meta.Author.UserID != 99 {
		t.Errorf("expected UserID 99")
	}
	if meta.Author.ProfileURL != "https://civitai.com/user/artist1" {
		t.Errorf("unexpected profile URL: %s", meta.Author.ProfileURL)
	}
	if meta.Description != "A great model" {
		t.Errorf("expected description")
	}
	if len(meta.Tags) != 2 || meta.Tags[0] != "realistic" || meta.Tags[1] != "portrait" {
		t.Errorf("unexpected tags: %v", meta.Tags)
	}
	if len(meta.PreviewImages) != 2 || meta.PreviewImages[0] != "https://example.com/1.jpg" {
		t.Errorf("unexpected preview images")
	}
}

func TestBuildMetadataUnknownFormat(t *testing.T) {
	task := &models.DownloadTask{
		FileName: "model.unknown",
	}
	meta := BuildMetadata(task, nil)
	if meta.FileInfo.Format != "Unknown" {
		t.Errorf("expected Unknown format, got %s", meta.FileInfo.Format)
	}
}

func TestSaveJSON(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.safetensors")

	meta := &models.ModelMetadata{}
	meta.CivitaiMetadata.ModelName = "test"
	meta.CivitaiMetadata.ModelType = "LORA"
	meta.FileInfo.FileName = "model.safetensors"
	meta.FileInfo.Format = "SafeTensor"

	if err := SaveJSON(meta, modelPath); err != nil {
		t.Fatalf("SaveJSON failed: %v", err)
	}

	jsonPath := filepath.Join(dir, "model.json")
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Errorf("JSON file not created at %s", jsonPath)
	}
}

func TestSendWebhookPOST(t *testing.T) {
	var received struct {
		body   string
		method string
		path   string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.method = r.Method
		received.path = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		received.body = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	task := &models.DownloadTask{
		SavePath: "/models/test.safetensors",
		ModelType: models.ModelTypeLORA,
	}

	err := SendWebhook(server.URL, "POST", task)
	if err != nil {
		t.Fatalf("SendWebhook failed: %v", err)
	}

	if received.method != "POST" { t.Errorf("expected POST, got %s", received.method) }
	if !strings.Contains(received.body, "model_added") { t.Errorf("body missing action") }
	if !strings.Contains(received.body, "test.safetensors") { t.Errorf("body missing filename") }
	if !strings.Contains(received.body, "LORA") { t.Errorf("body missing model type") }
}

func TestSendWebhookGET(t *testing.T) {
	var method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			if len(b) > 0 {
				t.Error("GET request should have empty body")
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	task := &models.DownloadTask{
		SavePath: "/models/test.safetensors",
	}

	err := SendWebhook(server.URL, "GET", task)
	if err != nil {
		t.Fatalf("SendWebhook GET failed: %v", err)
	}

	if method != "GET" { t.Errorf("expected GET, got %s", method) }
}

func TestSendWebhookNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("something went wrong"))
	}))
	defer server.Close()

	task := &models.DownloadTask{}
	err := SendWebhook(server.URL, "POST", task)
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status code: %v", err)
	}
}

func TestSendWebhookConnectionRefused(t *testing.T) {
	task := &models.DownloadTask{}
	err := SendWebhook("http://127.0.0.1:1/nonexistent", "POST", task)
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestSendWebhookWithAllPayloadFields(t *testing.T) {
	var payload WebhookPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		payload = WebhookPayload{}
		_ = json.Unmarshal(b, &payload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	task := &models.DownloadTask{
		SavePath: "/data/models/lora-test.safetensors",
		ModelType: models.ModelTypeLORA,
	}

	SendWebhook(server.URL, "POST", task)

	if payload.Action != "model_added" { t.Errorf("action: %s", payload.Action) }
	if payload.ModelPath != "/data/models/lora-test.safetensors" { t.Errorf("path: %s", payload.ModelPath) }
	if payload.ModelType != "LORA" { t.Errorf("type: %s", payload.ModelType) }
}
