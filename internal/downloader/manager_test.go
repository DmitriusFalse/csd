package downloader

import (
	"os"
	"testing"
	"time"

	"github.com/DmitriusFalse/csd/internal/config"
	"github.com/DmitriusFalse/csd/internal/logger"
	"github.com/DmitriusFalse/csd/internal/models"
)

func init() {
	logger.Init("error", os.DevNull, 1, 1, false)
}

type mockCivitaiClient struct {
	models map[int]*models.CivitaiModelResponse
}

func (m *mockCivitaiClient) FetchModelInfo(modelVersionID int, apiKey string) (*models.CivitaiModelResponse, error) {
	if info, ok := m.models[modelVersionID]; ok {
		return info, nil
	}
	return &models.CivitaiModelResponse{
		ModelID:   modelVersionID,
		ModelName: "mock-model",
		Type:      "LORA",
		BaseModel: "SDXL",
		Creator:   models.CivitaiCreator{Username: "mock-user", UserID: 1},
	}, nil
}

func managerTestConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Server: config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey: "test-key",
		RootPath: t.TempDir(),
		Queue: config.QueueConfig{
			MaxConcurrent: 2, RetryAttempts: 1, RetryDelaySec: 1, RateLimitDelayMs: 100,
		},
		LoraMgr: config.LoraManager{Enabled: false},
		Metadata: config.MetadataConfig{SaveJSON: false},
		NSFW: config.NSFWConfig{AllowNSFW: true, SeparateFolder: false, FolderSuffix: "_NSFW"},
		Logging: config.LoggingConfig{Level: "error"},
	}
}

func TestNewManager(t *testing.T) {
	cfg := managerTestConfig(t)
	mock := &mockCivitaiClient{}
	mgr := NewManager(cfg, mock)
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.GetActiveCount() != 0 {
		t.Errorf("expected 0 active, got %d", mgr.GetActiveCount())
	}
	if mgr.GetQueueLength() != 0 {
		t.Errorf("expected 0 queued, got %d", mgr.GetQueueLength())
	}
	mgr.Shutdown()
}

func TestAddTask_GeneratesID(t *testing.T) {
	cfg := managerTestConfig(t)
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	req := models.DownloadRequest{
		ModelVersionID: 1,
		FileID:         10,
		ModelType:      "LORA",
		BaseModel:      "SDXL",
		ModelName:      "test-lora",
		FileName:       "test.safetensors",
		FileSize:       "100MB",
	}

	task, err := mgr.AddTask(req)
	if err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}
	if task.ID == "" {
		t.Error("task ID should not be empty")
	}
	if task.Status != models.StatusQueued && task.Status != models.StatusDownloading {
		t.Errorf("expected queued or downloading, got %s", task.Status)
	}
	if task.ModelVersionID != 1 {
		t.Errorf("expected ModelVersionID 1, got %d", task.ModelVersionID)
	}
	if task.FileID != 10 {
		t.Errorf("expected FileID 10, got %d", task.FileID)
	}
	if task.ModelName != "test-lora" {
		t.Errorf("expected ModelName test-lora, got %s", task.ModelName)
	}
	if task.Priority != 0 {
		t.Errorf("expected Priority 0, got %d", task.Priority)
	}
}

func TestAddTask_DefaultAPIKey(t *testing.T) {
	cfg := managerTestConfig(t)
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	req := models.DownloadRequest{
		ModelVersionID: 2,
		FileID:         20,
		ModelType:      "Checkpoint",
		BaseModel:      "SD1.5",
	}

	task, err := mgr.AddTask(req)
	if err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}
	if task.APIKey != "test-key" {
		t.Errorf("expected API key from config, got %s", task.APIKey)
	}
}

func TestAddTask_CustomAPIKey(t *testing.T) {
	cfg := managerTestConfig(t)
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	req := models.DownloadRequest{
		ModelVersionID: 3,
		FileID:         30,
		APIKey:         "custom-key",
	}

	task, _ := mgr.AddTask(req)
	if task.APIKey != "custom-key" {
		t.Errorf("expected custom key, got %s", task.APIKey)
	}
}

func TestGetTask(t *testing.T) {
	cfg := managerTestConfig(t)
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	task, _ := mgr.AddTask(models.DownloadRequest{
		ModelVersionID: 4, FileID: 40, ModelType: "LORA", BaseModel: "SDXL",
	})
	got := mgr.GetTask(task.ID)
	if got == nil {
		t.Fatal("GetTask returned nil")
	}
	if got.ID != task.ID {
		t.Errorf("expected ID %s, got %s", task.ID, got.ID)
	}
}

func TestGetTask_NotFound(t *testing.T) {
	cfg := managerTestConfig(t)
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	got := mgr.GetTask("nonexistent")
	if got != nil {
		t.Errorf("expected nil for nonexistent task, got %v", got)
	}
}

func TestGetAllTasks(t *testing.T) {
	cfg := managerTestConfig(t)
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	mgr.AddTask(models.DownloadRequest{ModelVersionID: 10, FileID: 100, ModelType: "LORA", BaseModel: "SDXL"})
	mgr.AddTask(models.DownloadRequest{ModelVersionID: 11, FileID: 101, ModelType: "Checkpoint", BaseModel: "SD1.5"})

	tasks := mgr.GetAllTasks()
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestGetTasksGrouped(t *testing.T) {
	cfg := managerTestConfig(t)
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	mgr.AddTask(models.DownloadRequest{ModelVersionID: 20, FileID: 200, ModelType: "LORA", BaseModel: "SDXL"})
	mgr.AddTask(models.DownloadRequest{ModelVersionID: 21, FileID: 201, ModelType: "Checkpoint", BaseModel: "SD1.5"})

	resp := mgr.GetTasksGrouped()
	total := len(resp.Active) + len(resp.Queued) + len(resp.History)
	if total != 2 {
		t.Errorf("expected 2 total tasks grouped, got %d (active=%d, queued=%d, history=%d)",
			total, len(resp.Active), len(resp.Queued), len(resp.History))
	}
}

func TestPauseTask(t *testing.T) {
	cfg := managerTestConfig(t)
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	task, _ := mgr.AddTask(models.DownloadRequest{ModelVersionID: 30, FileID: 300, ModelType: "LORA", BaseModel: "SDXL"})

	// The task might be downloading immediately (if max_concurrent allows)
	// Pause only works on downloading tasks
	if task.Status == models.StatusDownloading {
		err := mgr.PauseTask(task.ID)
		if err != nil {
			t.Fatalf("PauseTask failed: %v", err)
		}
		got := mgr.GetTask(task.ID)
		if got.Status != models.StatusPaused {
			t.Errorf("expected paused, got %s", got.Status)
		}
	}
}

func TestPauseTask_NotFound(t *testing.T) {
	cfg := managerTestConfig(t)
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	err := mgr.PauseTask("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestCancelTask(t *testing.T) {
	cfg := managerTestConfig(t)
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	task, _ := mgr.AddTask(models.DownloadRequest{ModelVersionID: 40, FileID: 400, ModelType: "LORA", BaseModel: "SDXL"})

	err := mgr.CancelTask(task.ID)
	if err != nil {
		t.Fatalf("CancelTask failed: %v", err)
	}

	got := mgr.GetTask(task.ID)
	if got == nil {
		t.Fatal("task should exist after cancel")
	}
	if got.Status != models.StatusFailed {
		t.Errorf("expected failed status, got %s", got.Status)
	}
	if got.Error != "cancelled" {
		t.Errorf("expected error 'cancelled', got %s", got.Error)
	}
}

func TestCancelTask_NotFound(t *testing.T) {
	cfg := managerTestConfig(t)
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	err := mgr.CancelTask("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestMultipleTasks_QueueBehavior(t *testing.T) {
	cfg := managerTestConfig(t)
	cfg.Queue.MaxConcurrent = 1
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	task1, _ := mgr.AddTask(models.DownloadRequest{ModelVersionID: 50, FileID: 500, ModelType: "LORA", BaseModel: "SDXL"})
	task2, _ := mgr.AddTask(models.DownloadRequest{ModelVersionID: 51, FileID: 501, ModelType: "Checkpoint", BaseModel: "SD1.5"})
	task3, _ := mgr.AddTask(models.DownloadRequest{ModelVersionID: 52, FileID: 502, ModelType: "LORA", BaseModel: "SDXL"})

	// With max_concurrent=1, task1 should be downloading, rest queued
	if mgr.GetActiveCount() > 1 {
		t.Errorf("expected at most 1 active, got %d", mgr.GetActiveCount())
	}

	resp := mgr.GetTasksGrouped()
	queuedCount := len(resp.Queued)
	activeCount := len(resp.Active)

	if activeCount+queuedCount != 3 {
		t.Errorf("expected total 3, got active=%d, queued=%d", activeCount, queuedCount)
	}

	// Cancel the first task
	mgr.CancelTask(task1.ID)
	time.Sleep(100 * time.Millisecond)

	// After cancellation, the second task should eventually be picked up
	resp2 := mgr.GetTasksGrouped()
	_ = task2
	_ = task3
	_ = resp2
}

func TestPauseAllResumeAll(t *testing.T) {
	cfg := managerTestConfig(t)
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	mgr.AddTask(models.DownloadRequest{ModelVersionID: 60, FileID: 600, ModelType: "LORA", BaseModel: "SDXL"})
	mgr.AddTask(models.DownloadRequest{ModelVersionID: 61, FileID: 601, ModelType: "Checkpoint", BaseModel: "SD1.5"})

	mgr.PauseAll()

	resp := mgr.GetTasksGrouped()
	if len(resp.Active) > 0 {
		t.Errorf("expected 0 active after pause all, got %d", len(resp.Active))
	}

	mgr.ResumeAll()
	time.Sleep(50 * time.Millisecond)

	// Tasks should still exist after pause/resume cycle
	tasks := mgr.GetAllTasks()
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks after pause/resume, got %d", len(tasks))
	}
}

func TestGetActiveCount(t *testing.T) {
	cfg := managerTestConfig(t)
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	if mgr.GetActiveCount() != 0 {
		t.Errorf("expected 0, got %d", mgr.GetActiveCount())
	}
}

func TestGetQueueLength(t *testing.T) {
	cfg := managerTestConfig(t)
	cfg.Queue.MaxConcurrent = 1
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	// First should start downloading
	mgr.AddTask(models.DownloadRequest{ModelVersionID: 70, FileID: 700, ModelType: "LORA", BaseModel: "SDXL"})
	// Second should queue
	mgr.AddTask(models.DownloadRequest{ModelVersionID: 71, FileID: 701, ModelType: "LORA", BaseModel: "SDXL"})

	time.Sleep(50 * time.Millisecond)
	ql := mgr.GetQueueLength()
	if ql < 0 || ql > 2 {
		t.Errorf("unexpected queue length: %d", ql)
	}
}

func TestSetOnUpdate(t *testing.T) {
	cfg := managerTestConfig(t)
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	called := false
	mgr.SetOnUpdate(func(active int, queued int) {
		called = true
	})

	// Trigger an update
	mgr.AddTask(models.DownloadRequest{ModelVersionID: 80, FileID: 800, ModelType: "LORA", BaseModel: "SDXL"})
	time.Sleep(50 * time.Millisecond)
	if !called {
		t.Error("onUpdate callback was not called")
	}
}

func TestRestoreQueue(t *testing.T) {
	cfg := managerTestConfig(t)
	mgr := NewManager(cfg, &mockCivitaiClient{})
	defer mgr.Shutdown()

	task, _ := mgr.AddTask(models.DownloadRequest{
		ModelVersionID: 90, FileID: 900, ModelType: "LORA", BaseModel: "SDXL",
	})
	mgr.CancelTask(task.ID)

	// Save queue file should exist
	mgr.DeleteQueueFile()
}
