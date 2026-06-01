package models

import "time"

type ModelType string

const (
	ModelTypeCheckpoint       ModelType = "Checkpoint"
	ModelTypeLORA             ModelType = "LORA"
	ModelTypeVAE              ModelType = "VAE"
	ModelTypeTextualInversion ModelType = "TextualInversion"
	ModelTypeLoCon            ModelType = "LoCon"
	ModelTypeControlNet       ModelType = "Controlnet"
)

type DownloadStatus string

const (
	StatusPending     DownloadStatus = "pending"
	StatusDownloading DownloadStatus = "downloading"
	StatusPaused      DownloadStatus = "paused"
	StatusCompleted   DownloadStatus = "completed"
	StatusFailed      DownloadStatus = "failed"
	StatusQueued      DownloadStatus = "queued"
)

type DownloadRequest struct {
	ModelVersionID int    `json:"modelVersionId"`
	FileID         int    `json:"fileId"`
	ModelType      string `json:"modelType,omitempty"`
	BaseModel      string `json:"baseModel,omitempty"`
	ModelName      string `json:"modelName,omitempty"`
	FileName       string `json:"fileName,omitempty"`
	FileSize       string `json:"fileSize,omitempty"`
	APIKey         string `json:"apiKey,omitempty"`
	Priority       int    `json:"priority,omitempty"`
	PreviewImage   string `json:"previewImage,omitempty"`
}

type DownloadTask struct {
	ID              string         `json:"id"`
	ModelVersionID  int            `json:"modelVersionId"`
	FileID          int            `json:"fileId"`
	ModelType       ModelType      `json:"modelType"`
	BaseModel       string         `json:"baseModel"`
	ModelName       string         `json:"modelName"`
	FileName        string         `json:"fileName"`
	FileSize        string         `json:"fileSize"`
	FileSizeBytes   int64          `json:"fileSizeBytes,omitempty"`
	DownloadedBytes int64          `json:"downloadedBytes"`
	Status          DownloadStatus `json:"status"`
	Priority        int            `json:"priority"`
	SavePath        string         `json:"savePath"`
	TempPath        string         `json:"tempPath,omitempty"`
	APIKey          string         `json:"-"`
	Error           string         `json:"error,omitempty"`
	Progress        float64        `json:"progress"`
	AddedAt         time.Time      `json:"addedAt"`
	StartedAt       *time.Time     `json:"startedAt,omitempty"`
	CompletedAt     *time.Time     `json:"completedAt,omitempty"`
	PreviewImage    string         `json:"previewImage,omitempty"`
}

type TasksResponse struct {
	Active  []*DownloadTask `json:"active"`
	Queued  []*DownloadTask `json:"queued"`
	History []*DownloadTask `json:"history"`
}

type QueueState struct {
	ActiveDownloads []DownloadTask `json:"active_downloads"`
	QueuedDownloads []DownloadTask `json:"queued_downloads"`
}

type CivitaiFile struct {
	ID       int                    `json:"id"`
	Name     string                 `json:"name"`
	SizeKB   float64                `json:"sizeKB"`
	Type     string                 `json:"type"`
	Metadata map[string]interface{} `json:"metadata"`
}

type CivitaiTag struct {
	Name string `json:"name"`
}

type CivitaiCreator struct {
	Username string `json:"username"`
	UserID   int    `json:"userId"`
}

type CivitaiPreviewImage struct {
	URL  string `json:"url"`
	Type string `json:"type"`
}

type CivitaiModelResponse struct {
	ID            int                  `json:"id"`
	Name          string               `json:"name"`
	Type          string               `json:"type"`
	Nsfw          bool                 `json:"nsfw"`
	NsfwLevel     string               `json:"nsfwLevel"`
	ModelID       int                  `json:"modelId"`
	ModelName     string               `json:"modelName"`
	BaseModel     string               `json:"baseModel"`
	Description   string               `json:"description"`
	Tags          []CivitaiTag         `json:"tags"`
	Creator       CivitaiCreator      `json:"creator"`
	Files         []CivitaiFile        `json:"files"`
	PreviewImages []CivitaiPreviewImage `json:"images"`
}

type ModelMetadata struct {
	CivitaiMetadata struct {
		ModelID        int       `json:"modelId"`
		ModelVersionID int       `json:"modelVersionId"`
		FileID         int       `json:"fileId"`
		ModelName      string    `json:"modelName"`
		ModelType      string    `json:"modelType"`
		BaseModel      string    `json:"baseModel"`
		DownloadedAt   time.Time `json:"downloadedAt"`
	} `json:"civitai_metadata"`
	Author struct {
		Username   string `json:"username"`
		UserID     int    `json:"userId"`
		ProfileURL string `json:"profileUrl"`
	} `json:"author"`
	FileInfo struct {
		FileName      string `json:"fileName"`
		FileSize      string `json:"fileSize"`
		FileSizeBytes int64  `json:"fileSizeBytes"`
		Format        string `json:"format"`
	} `json:"file_info"`
	Description         string                 `json:"description,omitempty"`
	Tags                []string               `json:"tags,omitempty"`
	RecommendedSettings map[string]interface{} `json:"recommended_settings,omitempty"`
	PreviewImages       []string               `json:"preview_images,omitempty"`
}
