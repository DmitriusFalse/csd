package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig
	if cfg.Server.Port != 8765 {
		t.Errorf("expected port 8765, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.RootPath != "D:/AI/Models" {
		t.Errorf("expected root D:/AI/Models, got %s", cfg.RootPath)
	}
	if cfg.Queue.MaxConcurrent != 2 {
		t.Errorf("expected max_concurrent 2, got %d", cfg.Queue.MaxConcurrent)
	}
	if cfg.Queue.RetryAttempts != 3 {
		t.Errorf("expected retry_attempts 3, got %d", cfg.Queue.RetryAttempts)
	}
	if cfg.LoraMgr.Enabled != false {
		t.Errorf("expected lora enabled false, got %v", cfg.LoraMgr.Enabled)
	}
	if cfg.LoraMgr.BaseURL != "http://127.0.0.1:8188" {
		t.Errorf("unexpected base url: %s", cfg.LoraMgr.BaseURL)
	}
	if cfg.LoraMgr.WebhookURL != "http://127.0.0.1:8188/api/lm/loras/scan?full_rebuild=false" {
		t.Errorf("unexpected webhook url: %s", cfg.LoraMgr.WebhookURL)
	}
	if cfg.LoraMgr.WebhookMethod != "GET" {
		t.Errorf("expected webhook method GET, got %s", cfg.LoraMgr.WebhookMethod)
	}
	if cfg.Metadata.SaveJSON != true {
		t.Errorf("expected save_json true")
	}
	if cfg.NSFW.AllowNSFW != true {
		t.Errorf("expected nsfw allowed")
	}
	if cfg.NSFW.SeparateFolder != true {
		t.Errorf("expected separate folder true")
	}
	if cfg.Updates.AutoCheck != true {
		t.Errorf("expected auto_check true")
	}
}

func TestLoadSaveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Server.Port != 8765 {
		t.Errorf("expected default port 8765, got %d", cfg.Server.Port)
	}

	cfg.Server.Port = 9999
	cfg.RootPath = "/tmp/test-models"
	cfg.LoraMgr.Enabled = true
	cfg.LoraMgr.WebhookURL = "http://localhost:8288/webhook"
	cfg.NSFW.AllowNSFW = false

	if err := Save(*cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if loaded.Server.Port != 9999 {
		t.Errorf("expected port 9999, got %d", loaded.Server.Port)
	}
	if loaded.RootPath != "/tmp/test-models" {
		t.Errorf("expected root /tmp/test-models, got %s", loaded.RootPath)
	}
	if loaded.LoraMgr.Enabled != true {
		t.Errorf("expected lora enabled true")
	}
	if loaded.LoraMgr.WebhookURL != "http://localhost:8288/webhook" {
		t.Errorf("unexpected webhook url: %s", loaded.LoraMgr.WebhookURL)
	}
	if loaded.NSFW.AllowNSFW != false {
		t.Errorf("expected nsfw disallowed")
	}
}

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent", "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load on missing dir should create it: %v", err)
	}
	if cfg.Server.Port != 8765 {
		t.Errorf("expected default port after creation")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("config file should have been created")
	}
}

func TestLoadInvalidYaml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("invalid: [yaml: \n  broken"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid yaml")
	}
}

func TestSaveCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "config.yaml")

	cfg := defaultConfig
	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save should create directory tree: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("file should exist after save")
	}
}

func TestPartialOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.yaml")

	partial := []byte(`
server:
  port: 7777
root_path: /custom/path
`)
	if err := os.WriteFile(path, partial, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Server.Port != 7777 {
		t.Errorf("expected port 7777, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host from defaults, got %s", cfg.Server.Host)
	}
	if cfg.RootPath != "/custom/path" {
		t.Errorf("expected custom root, got %s", cfg.RootPath)
	}
	if cfg.Queue.MaxConcurrent != 2 {
		t.Errorf("expected max_concurrent from defaults, got %d", cfg.Queue.MaxConcurrent)
	}
}

func TestRoundTripPreservesAllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.yaml")

	original := Config{
		Server: ServerConfig{Port: 1234, Host: "0.0.0.0"},
		APIKey: "secret-key-12345",
		RootPath: "/data/models",
		Queue: QueueConfig{
			MaxConcurrent:    5,
			RetryAttempts:    7,
			RetryDelaySec:    120,
			RateLimitDelayMs: 500,
		},
		LoraMgr: LoraManager{
			Enabled:       true,
			BaseURL:       "http://localhost:8188",
			WebhookURL:    "http://test:9999/webhook",
			WebhookMethod: "POST",
		},
		Metadata: MetadataConfig{
			SaveJSON:         false,
			DownloadPreviews: true,
			MaxPreviewCount:  10,
		},
		Logging: LoggingConfig{
			Level:      "debug",
			File:       "/var/log/csd.log",
			MaxSizeMB:  200,
			MaxBackups: 10,
			Compress:   false,
		},
		NSFW: NSFWConfig{
			AllowNSFW:      false,
			SeparateFolder: false,
			FolderSuffix:   "_ADULT",
		},
		Updates: UpdatesConfig{
			AutoCheck:        false,
			CheckIntervalHrs: 48,
			GitHubRepo:       "user/repo",
		},
	}

	if err := Save(original, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Server.Port != 1234 { t.Errorf("Server.Port") }
	if loaded.Server.Host != "0.0.0.0" { t.Errorf("Server.Host") }
	if loaded.APIKey != "secret-key-12345" { t.Errorf("APIKey") }
	if loaded.RootPath != "/data/models" { t.Errorf("RootPath") }
	if loaded.Queue.MaxConcurrent != 5 { t.Errorf("Queue.MaxConcurrent") }
	if loaded.Queue.RetryAttempts != 7 { t.Errorf("Queue.RetryAttempts") }
	if loaded.Queue.RetryDelaySec != 120 { t.Errorf("Queue.RetryDelaySec") }
	if loaded.Queue.RateLimitDelayMs != 500 { t.Errorf("Queue.RateLimitDelayMs") }
	if loaded.LoraMgr.Enabled != true { t.Errorf("LoraMgr.Enabled") }
	if loaded.LoraMgr.BaseURL != "http://localhost:8188" { t.Errorf("LoraMgr.BaseURL") }
	if loaded.LoraMgr.WebhookURL != "http://test:9999/webhook" { t.Errorf("LoraMgr.WebhookURL") }
	if loaded.LoraMgr.WebhookMethod != "POST" { t.Errorf("LoraMgr.WebhookMethod") }
	if loaded.Metadata.SaveJSON != false { t.Errorf("Metadata.SaveJSON") }
	if loaded.Metadata.DownloadPreviews != true { t.Errorf("Metadata.DownloadPreviews") }
	if loaded.Logging.Level != "debug" { t.Errorf("Logging.Level") }
	if loaded.NSFW.AllowNSFW != false { t.Errorf("NSFW.AllowNSFW") }
	if loaded.Updates.AutoCheck != false { t.Errorf("Updates.AutoCheck") }
}
