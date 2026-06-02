package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DmitriusFalse/csd/internal/config"
	"github.com/DmitriusFalse/csd/internal/downloader"
	"github.com/DmitriusFalse/csd/internal/logger"
	"github.com/DmitriusFalse/csd/internal/models"
)

func init() {
	logger.Init("error", os.DevNull, 1, 1, false)
}

type mockCivitaiClient struct{}

func (m *mockCivitaiClient) FetchModelInfo(modelVersionID int, apiKey string) (*models.CivitaiModelResponse, error) {
	return &models.CivitaiModelResponse{
		ModelID:   modelVersionID,
		ModelName: "mock-model",
		Type:      "LORA",
		BaseModel: "SDXL",
		Creator:   models.CivitaiCreator{Username: "mock-user", UserID: 1},
	}, nil
}

func newTestServer(t *testing.T, cfg *config.Config) (*Server, *downloader.Manager) {
	t.Helper()
	mock := &mockCivitaiClient{}
	mgr := downloader.NewManager(cfg, mock)
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	config.Save(*cfg, configPath)
	s := New(cfg.Server.Host, cfg.Server.Port, mgr, configPath, "")
	return s, mgr
}

func TestHealthEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Queue:    config.QueueConfig{MaxConcurrent: 2},
		Logging:  config.LoggingConfig{Level: "error"},
	}
	s, _ := newTestServer(t, cfg)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := s.app.Test(req)
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}
}

func TestDownloadEndpoint_InvalidBody(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Queue:    config.QueueConfig{MaxConcurrent: 2},
		Logging:  config.LoggingConfig{Level: "error"},
	}
	s, _ := newTestServer(t, cfg)

	req := httptest.NewRequest("POST", "/download", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid body, got %d", resp.StatusCode)
	}
}

func TestDownloadEndpoint_MissingFields(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Queue:    config.QueueConfig{MaxConcurrent: 2},
		Logging:  config.LoggingConfig{Level: "error"},
	}
	s, _ := newTestServer(t, cfg)

	body := `{"modelVersionId": 0, "fileId": 0}`
	req := httptest.NewRequest("POST", "/download", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields, got %d", resp.StatusCode)
	}
}

func TestDownloadEndpoint_Success(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Queue:    config.QueueConfig{MaxConcurrent: 2, RetryAttempts: 1},
		Metadata: config.MetadataConfig{SaveJSON: false},
		Logging:  config.LoggingConfig{Level: "error"},
	}
	s, _ := newTestServer(t, cfg)

	body := `{"modelVersionId": 123, "fileId": 456, "modelType": "LORA", "baseModel": "SDXL", "modelName": "test"}`
	req := httptest.NewRequest("POST", "/download", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "queued" && result["status"] != "downloading" {
		t.Errorf("expected queued or downloading, got %v", result["status"])
	}
	if id, ok := result["id"]; !ok || id == "" {
		t.Errorf("expected task id in response")
	}
}

func TestTasksEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Queue:    config.QueueConfig{MaxConcurrent: 2},
		Logging:  config.LoggingConfig{Level: "error"},
	}
	s, _ := newTestServer(t, cfg)

	// Add a task first
	body := `{"modelVersionId": 1, "fileId": 10, "modelType": "LORA", "baseModel": "SDXL"}`
	req := httptest.NewRequest("POST", "/download", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	s.app.Test(req)

	// Get tasks
	req = httptest.NewRequest("GET", "/tasks", nil)
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var tasks models.TasksResponse
	json.NewDecoder(resp.Body).Decode(&tasks)
	total := len(tasks.Active) + len(tasks.Queued) + len(tasks.History)
	if total != 1 {
		t.Errorf("expected 1 task total, got %d (active=%d, queued=%d, history=%d)",
			total, len(tasks.Active), len(tasks.Queued), len(tasks.History))
	}
}

func TestGetQueueEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Queue:    config.QueueConfig{MaxConcurrent: 2},
		Logging:  config.LoggingConfig{Level: "error"},
	}
	s, _ := newTestServer(t, cfg)

	req := httptest.NewRequest("GET", "/queue", nil)
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetTaskEndpoint_NotFound(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Queue:    config.QueueConfig{MaxConcurrent: 2},
		Logging:  config.LoggingConfig{Level: "error"},
	}
	s, _ := newTestServer(t, cfg)

	req := httptest.NewRequest("GET", "/queue/nonexistent", nil)
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestCancelTaskEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Queue:    config.QueueConfig{MaxConcurrent: 2},
		Logging:  config.LoggingConfig{Level: "error"},
	}
	s, mgr := newTestServer(t, cfg)
	defer mgr.Shutdown()

	task, _ := mgr.AddTask(models.DownloadRequest{
		ModelVersionID: 42, FileID: 420, ModelType: "LORA", BaseModel: "SDXL",
	})

	req := httptest.NewRequest("POST", "/queue/"+task.ID+"/cancel", nil)
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPauseAllResumeAllEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Queue:    config.QueueConfig{MaxConcurrent: 2},
		Logging:  config.LoggingConfig{Level: "error"},
	}
	s, _ := newTestServer(t, cfg)

	req := httptest.NewRequest("POST", "/queue/pause-all", nil)
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("pause-all expected 200, got %d", resp.StatusCode)
	}

	req = httptest.NewRequest("POST", "/queue/resume-all", nil)
	resp, _ = s.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("resume-all expected 200, got %d", resp.StatusCode)
	}
}

func TestConfigEndpoint_GET(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "secret-test-key-12345",
		RootPath: "/models",
		Queue:    config.QueueConfig{MaxConcurrent: 3},
		LoraMgr:  config.LoraManager{Enabled: true, WebhookURL: "http://localhost:8288/webhook", WebhookMethod: "GET"},
		NSFW:     config.NSFWConfig{AllowNSFW: false, SeparateFolder: true, FolderSuffix: "_NSFW"},
		Logging:  config.LoggingConfig{Level: "info"},
	}
	config.Save(*cfg, cfgPath)

	mock := &mockCivitaiClient{}
	mgr := downloader.NewManager(cfg, mock)
	s := New(cfg.Server.Host, cfg.Server.Port, mgr, cfgPath, "")

	req := httptest.NewRequest("GET", "/api/config", nil)
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	if body["root_path"] != "/models" { t.Errorf("root_path") }
	if body["max_concurrent"] != float64(3) { t.Errorf("max_concurrent") }
	if body["lora_enabled"] != true { t.Errorf("lora_enabled") }
	if body["webhook_url"] != "http://localhost:8288/webhook" { t.Errorf("webhook_url") }

	// API key should be masked
	ak, _ := body["api_key"].(string)
	if !strings.HasSuffix(ak, "****") {
		t.Errorf("expected masked api key, got %s", ak)
	}
	if strings.Contains(ak, "12345") {
		t.Errorf("api key should not contain full value")
	}
}

func TestConfigEndpoint_POST(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: "/original",
		Queue:    config.QueueConfig{MaxConcurrent: 2, RetryAttempts: 3, RetryDelaySec: 60},
		LoraMgr:  config.LoraManager{Enabled: false, WebhookURL: ""},
		NSFW:     config.NSFWConfig{AllowNSFW: true, SeparateFolder: false},
		Logging:  config.LoggingConfig{Level: "info"},
	}
	config.Save(*cfg, cfgPath)

	mock := &mockCivitaiClient{}
	mgr := downloader.NewManager(cfg, mock)
	s := New(cfg.Server.Host, cfg.Server.Port, mgr, cfgPath, "")

	updateBody := `{
		"root_path": "/new-path",
		"max_concurrent": 5,
		"retry_attempts": 7,
		"retry_delay_seconds": 120,
		"allow_nsfw": false,
		"separate_folder": true,
		"save_json": false,
		"log_level": "debug",
		"lora_enabled": true,
		"webhook_url": "http://localhost:9999/webhook"
	}`

	req := httptest.NewRequest("POST", "/api/config", strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Verify config was saved
	loaded, _ := config.Load(cfgPath)
	if loaded.RootPath != "/new-path" { t.Errorf("root_path: got %s", loaded.RootPath) }
	if loaded.Queue.MaxConcurrent != 5 { t.Errorf("max_concurrent") }
	if loaded.Queue.RetryAttempts != 7 { t.Errorf("retry_attempts") }
	if loaded.Queue.RetryDelaySec != 120 { t.Errorf("retry_delay_seconds") }
	if loaded.NSFW.AllowNSFW != false { t.Errorf("allow_nsfw") }
	if loaded.NSFW.SeparateFolder != true { t.Errorf("separate_folder") }
	if loaded.Metadata.SaveJSON != false { t.Errorf("save_json") }
	if loaded.Logging.Level != "debug" { t.Errorf("log_level") }
	if loaded.LoraMgr.Enabled != true { t.Errorf("lora_enabled") }
	if loaded.LoraMgr.WebhookURL != "http://localhost:9999/webhook" { t.Errorf("webhook_url") }
}

func TestConfigEndpoint_POST_PartialUpdate(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: "/original",
		Queue:    config.QueueConfig{MaxConcurrent: 2, RetryAttempts: 3, RetryDelaySec: 60},
		LoraMgr:  config.LoraManager{Enabled: false, WebhookURL: "http://old:8080/wh"},
		NSFW:     config.NSFWConfig{AllowNSFW: true, SeparateFolder: false},
		Logging:  config.LoggingConfig{Level: "info"},
	}
	config.Save(*cfg, cfgPath)

	mock := &mockCivitaiClient{}
	mgr := downloader.NewManager(cfg, mock)
	s := New(cfg.Server.Host, cfg.Server.Port, mgr, cfgPath, "")

	// Only update root_path
	updateBody := `{"root_path": "/updated"}`
	req := httptest.NewRequest("POST", "/api/config", strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	loaded, _ := config.Load(cfgPath)
	if loaded.RootPath != "/updated" { t.Errorf("root_path") }
	if loaded.Queue.MaxConcurrent != 2 { t.Errorf("max_concurrent should remain unchanged") }
	if loaded.LoraMgr.WebhookURL != "http://old:8080/wh" { t.Errorf("webhook_url should remain unchanged") }
	if loaded.LoraMgr.Enabled != false { t.Errorf("lora_enabled should remain unchanged") }
}

func TestConfigEndpoint_POST_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := &config.Config{
		Server:  config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:  "test",
		RootPath: t.TempDir(),
		Logging: config.LoggingConfig{Level: "error"},
	}
	config.Save(*cfg, cfgPath)

	mock := &mockCivitaiClient{}
	mgr := downloader.NewManager(cfg, mock)
	s := New(cfg.Server.Host, cfg.Server.Port, mgr, cfgPath, "")

	req := httptest.NewRequest("POST", "/api/config", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestLogsEndpoint_NotConfigured(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Logging:  config.LoggingConfig{Level: "error"},
	}
	mock := &mockCivitaiClient{}
	mgr := downloader.NewManager(cfg, mock)
	s := New(cfg.Server.Host, cfg.Server.Port, mgr, "", "")

	req := httptest.NewRequest("GET", "/logs", nil)
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestLogsOpenEndpoint_NotConfigured(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Logging:  config.LoggingConfig{Level: "error"},
	}
	mock := &mockCivitaiClient{}
	mgr := downloader.NewManager(cfg, mock)
	s := New(cfg.Server.Host, cfg.Server.Port, mgr, "", "")

	req := httptest.NewRequest("POST", "/logs/open", nil)
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDownloadEndpoint_WithCustomAPIKey(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "server-key",
		RootPath: t.TempDir(),
		Queue:    config.QueueConfig{MaxConcurrent: 2},
		Logging:  config.LoggingConfig{Level: "error"},
	}
	s, _ := newTestServer(t, cfg)

	body := `{"modelVersionId": 555, "fileId": 666, "modelType": "LORA", "baseModel": "SDXL", "apiKey": "custom-key"}`
	req := httptest.NewRequest("POST", "/download", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestConcurrentRequests(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Queue:    config.QueueConfig{MaxConcurrent: 5},
		Logging:  config.LoggingConfig{Level: "error"},
	}
	s, _ := newTestServer(t, cfg)

	for i := 0; i < 10; i++ {
		body := fmt.Sprintf(`{"modelVersionId": %d, "fileId": %d, "modelType": "LORA", "baseModel": "SDXL"}`, i+1, i+1)
		req := httptest.NewRequest("POST", "/download", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := s.app.Test(req)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i, resp.StatusCode)
		}
	}

	req := httptest.NewRequest("GET", "/tasks", nil)
	resp, _ := s.app.Test(req)
	var tasks models.TasksResponse
	json.NewDecoder(resp.Body).Decode(&tasks)
	total := len(tasks.Active) + len(tasks.Queued) + len(tasks.History)
	if total != 10 {
		t.Errorf("expected 10 tasks total, got %d", total)
	}
}

func TestCORSHeaders(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Logging:  config.LoggingConfig{Level: "error"},
	}
	s, _ := newTestServer(t, cfg)

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Origin", "http://example.com")
	resp, _ := s.app.Test(req)
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected CORS header, got %s", resp.Header.Get("Access-Control-Allow-Origin"))
	}
}

func TestShutdown(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Logging:  config.LoggingConfig{Level: "error"},
	}
	s, mgr := newTestServer(t, cfg)

	// Shutdown should not panic
	s.Shutdown()
	mgr.Shutdown()
}

func TestConfigPageEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Logging:  config.LoggingConfig{Level: "error"},
	}
	s, _ := newTestServer(t, cfg)

	req := httptest.NewRequest("GET", "/config", nil)
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected HTML content type, got %s", ct)
	}
}

func TestCheckDownloaded_MissingParams(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Logging:  config.LoggingConfig{Level: "error"},
	}
	s, _ := newTestServer(t, cfg)

	req := httptest.NewRequest("GET", "/api/check-downloaded", nil)
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCheckDownloaded_NoLmConnection(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8765, Host: "127.0.0.1"},
		APIKey:   "test",
		RootPath: t.TempDir(),
		Logging:  config.LoggingConfig{Level: "error"},
		LoraMgr:  config.LoraManager{BaseURL: "http://127.0.0.1:1"},
	}
	config.Save(*cfg, cfgPath)
	mock := &mockCivitaiClient{}
	mgr := downloader.NewManager(cfg, mock)
	s := New(cfg.Server.Host, cfg.Server.Port, mgr, cfgPath, "")

	req := httptest.NewRequest("GET", "/api/check-downloaded?name=test&type=LORA", nil)
	resp, _ := s.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if body["downloaded"] != false {
		t.Errorf("expected downloaded=false when LM unreachable, got %v", body["downloaded"])
	}
}

func TestCivitaiToLmTypes(t *testing.T) {
	tests := []struct{ input string; expected []string }{
		{"LORA", []string{"loras"}},
		{"LoRA", []string{"loras"}},
		{"lora", []string{"loras"}},
		{"Checkpoint", []string{"checkpoints"}},
		{"CHECKPOINT", []string{"checkpoints"}},
		{"TextualInversion", []string{"embeddings"}},
		{"Embedding", []string{"embeddings"}},
		{"Hypernetwork", []string{"hypernetworks"}},
		{"unknown", []string{"loras", "checkpoints", "embeddings"}},
		{"", []string{"loras", "checkpoints", "embeddings"}},
	}
	for _, tt := range tests {
		result := civitaiToLmTypes(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("civitaiToLmTypes(%q) length: got %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("civitaiToLmTypes(%q)[%d]: got %s, want %s", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}
