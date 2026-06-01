package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DmitriusFalse/csd/internal/api"
	"github.com/DmitriusFalse/csd/internal/config"
	"github.com/DmitriusFalse/csd/internal/logger"
	"github.com/DmitriusFalse/csd/internal/metadata"
	"github.com/DmitriusFalse/csd/internal/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Manager struct {
	mu            sync.RWMutex
	tasks         map[string]*models.DownloadTask
	active        int
	maxConcurrent int
	queue         []*models.DownloadTask
	cfg           *config.Config
	downloader    *Downloader
	civitaiClient *api.CivitaiClient
	ctx           context.Context
	cancel        context.CancelFunc
	webhookURL    string
	webhookMethod string
	onUpdate      func()
}

func NewManager(cfg *config.Config, civitaiClient *api.CivitaiClient) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		tasks:         make(map[string]*models.DownloadTask),
		maxConcurrent: cfg.Queue.MaxConcurrent,
		cfg:           cfg,
		downloader:    New(),
		civitaiClient: civitaiClient,
		ctx:           ctx,
		cancel:        cancel,
		webhookURL:    cfg.LoraMgr.WebhookURL,
		webhookMethod: cfg.LoraMgr.WebhookMethod,
	}
	m.restoreQueue()
	return m
}

func (m *Manager) SetOnUpdate(fn func()) {
	m.onUpdate = fn
}

func (m *Manager) notifyUpdate() {
	if m.onUpdate != nil {
		m.onUpdate()
	}
}

func (m *Manager) AddTask(req models.DownloadRequest) (*models.DownloadTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task := &models.DownloadTask{
		ID:             uuid.New().String(),
		ModelVersionID: req.ModelVersionID,
		FileID:         req.FileID,
		ModelName:      req.ModelName,
		FileName:       req.FileName,
		FileSize:       req.FileSize,
		APIKey:         req.APIKey,
		Status:         models.StatusQueued,
		Priority:       req.Priority,
		AddedAt:        time.Now(),
	}

	if req.FileSize != "" {
		if size, err := ParseFileSize(req.FileSize); err == nil {
			task.FileSizeBytes = size
		}
	}

	modelType := req.ModelType
	baseModel := req.BaseModel

	if modelType == "" || baseModel == "" {
		apiKey := req.APIKey
		if apiKey == "" {
			apiKey = m.cfg.APIKey
		}

		info, err := m.civitaiClient.FetchModelInfo(req.ModelVersionID, apiKey)
		if err != nil {
			logger.Log.Warn("Failed to fetch model info, using provided values", zap.Error(err))
		} else {
			if modelType == "" {
				modelType = info.Type
			}
			if baseModel == "" {
				baseModel = info.BaseModel
			}
			if task.ModelName == "" {
				task.ModelName = info.ModelName
			}
			if len(info.Files) > 0 && task.FileName == "" {
				task.FileName = info.Files[0].Name
			}
			task.FileSizeBytes = int64(info.Files[0].SizeKB * 1024)
		}
	}

	task.ModelType = api.ParseModelType(modelType)
	task.BaseModel = baseModel

	savePath := config.GetSavePath(m.cfg.RootPath, task.ModelType, task.BaseModel, false, m.cfg.NSFW.FolderSuffix)
	if task.FileName == "" {
		task.FileName = fmt.Sprintf("model_%d.safetensors", task.ModelVersionID)
	}
	task.SavePath = filepath.Join(savePath, task.FileName)

	m.tasks[task.ID] = task

	if m.active < m.maxConcurrent {
		go m.startTask(task)
	} else {
		m.queue = append(m.queue, task)
		logger.Log.Info("Task queued",
			zap.String("id", task.ID),
			zap.String("model", task.ModelName),
			zap.Int("queue_position", len(m.queue)),
		)
	}

	m.notifyUpdate()
	return task, nil
}

func (m *Manager) startTask(task *models.DownloadTask) {
	m.mu.Lock()
	if task.Status != models.StatusQueued && task.Status != models.StatusPaused {
		m.mu.Unlock()
		return
	}
	task.Status = models.StatusDownloading
	now := time.Now()
	task.StartedAt = &now
	m.active++
	m.mu.Unlock()

	logger.Log.Info("Download started",
		zap.String("id", task.ID),
		zap.String("model", task.ModelName),
		zap.String("url", fmt.Sprintf("civitai.com/api/download/models/%d", task.ModelVersionID)),
	)

	ctx, cancel := context.WithCancel(m.ctx)
	defer cancel()

	err := m.downloader.Download(ctx, task, func(downloaded, total int64) {
		m.notifyUpdate()
	})

	m.mu.Lock()
	m.active--

	if err != nil {
		task.Status = models.StatusFailed
		task.Error = err.Error()
		logger.Log.Error("Download failed",
			zap.String("id", task.ID),
			zap.Error(err),
		)
	} else {
		task.Status = models.StatusCompleted
		now := time.Now()
		task.CompletedAt = &now
		task.Progress = 100
		logger.Log.Info("Download completed",
			zap.String("id", task.ID),
			zap.String("file", task.SavePath),
		)

		if m.cfg.Metadata.SaveJSON {
			meta := metadata.BuildMetadata(task, nil)
			if err := metadata.SaveJSON(meta, task.SavePath); err != nil {
				logger.Log.Warn("Failed to save metadata", zap.Error(err))
			}
		}

		m.fireWebhook(task)
	}

	m.saveQueue()
	m.processQueue()
	m.mu.Unlock()
	m.notifyUpdate()
}

func (m *Manager) processQueue() {
	for m.active < m.maxConcurrent && len(m.queue) > 0 {
		next := m.queue[0]
		m.queue = m.queue[1:]
		go m.startTask(next)
	}
}

func (m *Manager) PauseTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}

	if task.Status == models.StatusDownloading {
		task.Status = models.StatusPaused
		m.active--
		m.saveQueue()
		m.processQueue()
		m.notifyUpdate()
		return nil
	}

	return fmt.Errorf("task %s is not active", id)
}

func (m *Manager) ResumeTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}

	if task.Status == models.StatusPaused {
		task.Status = models.StatusQueued
		if m.active < m.maxConcurrent {
			go m.startTask(task)
		} else {
			m.queue = append(m.queue, task)
		}
		m.saveQueue()
		m.notifyUpdate()
		return nil
	}

	return fmt.Errorf("task %s is not paused", id)
}

func (m *Manager) CancelTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}

	if task.Status == models.StatusDownloading {
		m.active--
	}

	oldStatus := task.Status
	task.Status = models.StatusFailed
	task.Error = "cancelled"
	m.downloader.RemoveTempFile(task)

	m.saveQueue()
	if oldStatus == models.StatusDownloading {
		m.processQueue()
	}
	m.notifyUpdate()
	return nil
}

func (m *Manager) PauseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, task := range m.tasks {
		if task.Status == models.StatusDownloading {
			task.Status = models.StatusPaused
			m.active--
		}
	}
	m.queue = nil
	m.saveQueue()
	m.notifyUpdate()
}

func (m *Manager) ResumeAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, task := range m.tasks {
		if task.Status == models.StatusPaused {
			task.Status = models.StatusQueued
			if m.active < m.maxConcurrent {
				go m.startTask(task)
			} else {
				m.queue = append(m.queue, task)
			}
		}
	}
	m.saveQueue()
	m.notifyUpdate()
}

func (m *Manager) GetTask(id string) *models.DownloadTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tasks[id]
}

func (m *Manager) GetAllTasks() []*models.DownloadTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*models.DownloadTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		result = append(result, task)
	}
	return result
}

func (m *Manager) GetActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

func (m *Manager) GetQueueLength() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.queue)
}

func (m *Manager) fireWebhook(task *models.DownloadTask) {
	if !m.cfg.LoraMgr.Enabled || m.webhookURL == "" {
		return
	}

	go func() {
		for attempt := 0; attempt < 3; attempt++ {
			if err := metadata.SendWebhook(m.webhookURL, m.webhookMethod, task); err != nil {
				logger.Log.Warn("Webhook failed",
					zap.String("url", m.webhookURL),
					zap.Int("attempt", attempt+1),
					zap.Error(err),
				)
				time.Sleep(30 * time.Second)
				continue
			}
			logger.Log.Info("Webhook sent successfully",
				zap.String("task_id", task.ID),
			)
			return
		}
		logger.Log.Error("Webhook failed after 3 attempts",
			zap.String("task_id", task.ID),
		)
	}()
}

func (m *Manager) saveQueue() {
	state := models.QueueState{}

	for _, task := range m.tasks {
		if task.Status == models.StatusDownloading || task.Status == models.StatusPaused {
			state.ActiveDownloads = append(state.ActiveDownloads, *task)
		}
	}

	for _, task := range m.queue {
		state.QueuedDownloads = append(state.QueuedDownloads, *task)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		logger.Log.Error("Failed to marshal queue", zap.Error(err))
		return
	}

	if err := os.WriteFile("queue.json", data, 0644); err != nil {
		logger.Log.Error("Failed to save queue", zap.Error(err))
	}
}

func (m *Manager) restoreQueue() {
	data, err := os.ReadFile("queue.json")
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Log.Warn("Failed to read queue.json", zap.Error(err))
		}
		return
	}

	var state models.QueueState
	if err := json.Unmarshal(data, &state); err != nil {
		logger.Log.Warn("Failed to parse queue.json", zap.Error(err))
		return
	}

	for i := range state.ActiveDownloads {
		task := state.ActiveDownloads[i]
		task.Status = models.StatusPaused
		task.Error = ""
		m.tasks[task.ID] = &task
	}

	for i := range state.QueuedDownloads {
		task := state.QueuedDownloads[i]
		task.Status = models.StatusQueued
		task.Error = ""
		m.tasks[task.ID] = &task
		m.queue = append(m.queue, &task)
	}

	if len(m.tasks) > 0 {
		logger.Log.Info("Restored queue",
			zap.Int("active", len(state.ActiveDownloads)),
			zap.Int("queued", len(state.QueuedDownloads)),
		)

		for _, task := range m.tasks {
			existing := m.downloader.GetExistingBytes(task)
			if existing > 0 {
				task.DownloadedBytes = existing
				logger.Log.Info("Found partial download",
					zap.String("id", task.ID),
					zap.Int64("bytes", existing),
				)
			}
		}

		m.notifyUpdate()
	}
}

func (m *Manager) Shutdown() {
	m.cancel()
}

func (m *Manager) DeleteQueueFile() {
	os.Remove("queue.json")
}
