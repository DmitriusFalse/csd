package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	APIKey    string          `yaml:"api_key"`
	RootPath  string          `yaml:"root_path"`
	Queue     QueueConfig     `yaml:"download_queue"`
	LoraMgr   LoraManager     `yaml:"lora_manager"`
	Metadata  MetadataConfig  `yaml:"metadata"`
	Logging   LoggingConfig   `yaml:"logging"`
	NSFW      NSFWConfig      `yaml:"nsfw"`
	Updates   UpdatesConfig   `yaml:"updates"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type QueueConfig struct {
	MaxConcurrent int `yaml:"max_concurrent"`
	RetryAttempts int `yaml:"retry_attempts"`
	RetryDelaySec int `yaml:"retry_delay_seconds"`
}

type LoraManager struct {
	Enabled       bool   `yaml:"enabled"`
	BaseURL       string `yaml:"base_url"`
	WebhookURL    string `yaml:"webhook_url"`
	WebhookMethod string `yaml:"webhook_method"`
	UseLmPath     bool   `yaml:"use_lm_path"`
}

type MetadataConfig struct {
	SaveJSON         bool `yaml:"save_json"`
	DownloadPreviews bool `yaml:"download_previews"`
	MaxPreviewCount  int  `yaml:"max_preview_count"`
}

type LoggingConfig struct {
	Level      string `yaml:"level"`
	File       string `yaml:"file"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	Compress   bool   `yaml:"compress"`
}

type NSFWConfig struct {
	AllowNSFW    bool   `yaml:"allow_nsfw"`
	SeparateFolder bool `yaml:"separate_folder"`
	FolderSuffix string `yaml:"folder_suffix"`
}

type UpdatesConfig struct {
	AutoCheck        bool   `yaml:"auto_check"`
	CheckIntervalHrs int    `yaml:"check_interval_hours"`
	GitHubRepo       string `yaml:"github_repo"`
}

var defaultConfig = Config{
	Server: ServerConfig{
		Port: 8765,
		Host: "127.0.0.1",
	},
	APIKey:   "",
	RootPath: "D:/AI/Models",
	Queue: QueueConfig{
		MaxConcurrent: 2,
		RetryAttempts: 3,
		RetryDelaySec: 60,
	},
	LoraMgr: LoraManager{
		Enabled:       false,
		BaseURL:       "http://127.0.0.1:8188",
		WebhookURL:    "http://127.0.0.1:8188/api/lm/loras/scan?full_rebuild=false",
		WebhookMethod: "GET",
	},
	Metadata: MetadataConfig{
		SaveJSON:         true,
		DownloadPreviews: false,
		MaxPreviewCount:  3,
	},
	Logging: LoggingConfig{
		Level:      "info",
		File:       "logs/app.log",
		MaxSizeMB:  100,
		MaxBackups: 5,
		Compress:   true,
	},
	NSFW: NSFWConfig{
		AllowNSFW:      true,
		SeparateFolder: true,
		FolderSuffix:   "_NSFW",
	},
	Updates: UpdatesConfig{
		AutoCheck:        true,
		CheckIntervalHrs: 24,
		GitHubRepo:       "DmitriusFalse/csd",
	},
}

func Load(path string) (*Config, error) {
	cfg := defaultConfig

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, err
			}
			if err := Save(cfg, path); err != nil {
				return nil, err
			}
			return &cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func Save(cfg Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
