package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDownloadStatusValues(t *testing.T) {
	tests := []struct {
		status DownloadStatus
		want   string
	}{
		{StatusPending, "pending"},
		{StatusDownloading, "downloading"},
		{StatusPaused, "paused"},
		{StatusCompleted, "completed"},
		{StatusFailed, "failed"},
		{StatusQueued, "queued"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("expected %s, got %s", tt.want, string(tt.status))
		}
	}
}

func TestModelTypeValues(t *testing.T) {
	tests := []struct {
		mt   ModelType
		want string
	}{
		{ModelTypeCheckpoint, "Checkpoint"},
		{ModelTypeLORA, "LORA"},
		{ModelTypeVAE, "VAE"},
		{ModelTypeTextualInversion, "TextualInversion"},
		{ModelTypeLoCon, "LoCon"},
		{ModelTypeControlNet, "Controlnet"},
	}
	for _, tt := range tests {
		if string(tt.mt) != tt.want {
			t.Errorf("expected %s, got %s", tt.want, string(tt.mt))
		}
	}
}

func TestDownloadRequestJSON(t *testing.T) {
	req := DownloadRequest{
		ModelVersionID: 123,
		FileID:         456,
		ModelType:      "LORA",
		BaseModel:      "SDXL",
		ModelName:      "test-model",
		FileName:       "test.safetensors",
		FileSize:       "500MB",
		APIKey:         "key123",
		Priority:       1,
		PreviewImage:   "https://example.com/preview.jpg",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded DownloadRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ModelVersionID != 123 { t.Errorf("ModelVersionID") }
	if decoded.FileID != 456 { t.Errorf("FileID") }
	if decoded.ModelType != "LORA" { t.Errorf("ModelType") }
	if decoded.BaseModel != "SDXL" { t.Errorf("BaseModel") }
	if decoded.PreviewImage != "https://example.com/preview.jpg" { t.Errorf("PreviewImage") }
}

func TestDownloadTaskJSON(t *testing.T) {
	now := time.Now()
	task := DownloadTask{
		ID:              "task-1",
		ModelVersionID:  789,
		FileID:          101,
		ModelType:       ModelTypeCheckpoint,
		BaseModel:       "SD1.5",
		ModelName:       "dreamshaper",
		FileName:        "dreamshaper.safetensors",
		FileSize:        "2GB",
		FileSizeBytes:   2000000000,
		DownloadedBytes: 500000000,
		Status:          StatusDownloading,
		Priority:        2,
		SavePath:        "/models/dreamshaper.safetensors",
		TempPath:        "/models/dreamshaper.safetensors.part",
		Error:           "",
		Progress:        25.0,
		AddedAt:         now,
		StartedAt:       &now,
		PreviewImage:    "https://example.com/preview.jpg",
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded DownloadTask
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != "task-1" { t.Errorf("ID") }
	if decoded.ModelVersionID != 789 { t.Errorf("ModelVersionID") }
	if decoded.FileID != 101 { t.Errorf("FileID") }
	if decoded.ModelType != ModelTypeCheckpoint { t.Errorf("ModelType") }
	if decoded.Status != StatusDownloading { t.Errorf("Status") }
	if decoded.Progress != 25.0 { t.Errorf("Progress") }
	if decoded.FileSizeBytes != 2000000000 { t.Errorf("FileSizeBytes") }
	if decoded.DownloadedBytes != 500000000 { t.Errorf("DownloadedBytes") }

	// APIKey should be hidden from JSON
	if decoded.APIKey != "" {
		t.Errorf("APIKey should be empty in JSON (tag: -)")
	}
}

func TestTasksResponseEmpty(t *testing.T) {
	resp := TasksResponse{}
	data, _ := json.Marshal(resp)
	var decoded TasksResponse
	json.Unmarshal(data, &decoded)
	if decoded.Active != nil || decoded.Queued != nil || decoded.History != nil {
		t.Errorf("expected nil slices")
	}
}

func TestTasksResponseWithData(t *testing.T) {
	resp := TasksResponse{
		Active:  []*DownloadTask{{ID: "a1", Status: StatusDownloading}},
		Queued:  []*DownloadTask{{ID: "q1", Status: StatusQueued}},
		History: []*DownloadTask{{ID: "h1", Status: StatusCompleted}},
	}
	data, _ := json.Marshal(resp)
	var decoded TasksResponse
	json.Unmarshal(data, &decoded)
	if len(decoded.Active) != 1 || decoded.Active[0].ID != "a1" { t.Errorf("Active") }
	if len(decoded.Queued) != 1 || decoded.Queued[0].ID != "q1" { t.Errorf("Queued") }
	if len(decoded.History) != 1 || decoded.History[0].ID != "h1" { t.Errorf("History") }
}

func TestCivitaiModelResponseJSON(t *testing.T) {
	jsonData := `{
		"id": 1,
		"name": "test-model",
		"type": "LORA",
		"nsfw": false,
		"modelId": 100,
		"modelName": "parent-model",
		"baseModel": "SDXL",
		"tags": [{"name": "style"}],
		"creator": {"username": "user1", "userId": 42},
		"files": [{"id": 1, "name": "file.safetensors", "sizeKB": 500000, "type": "Model"}],
		"images": [{"url": "https://example.com/img.jpg", "type": "image"}]
	}`

	var resp CivitaiModelResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.ID != 1 { t.Errorf("ID") }
	if resp.Name != "test-model" { t.Errorf("Name") }
	if resp.Type != "LORA" { t.Errorf("Type") }
	if resp.BaseModel != "SDXL" { t.Errorf("BaseModel") }
	if len(resp.Tags) != 1 || resp.Tags[0].Name != "style" { t.Errorf("Tags") }
	if resp.Creator.Username != "user1" { t.Errorf("Creator.Username") }
	if resp.Creator.UserID != 42 { t.Errorf("Creator.UserID") }
	if len(resp.Files) != 1 || resp.Files[0].Name != "file.safetensors" { t.Errorf("Files") }
	if len(resp.PreviewImages) != 1 || resp.PreviewImages[0].URL != "https://example.com/img.jpg" { t.Errorf("PreviewImages") }
}
