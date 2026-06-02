package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	taskCancels   map[string]context.CancelFunc
	active        int
	maxConcurrent int
	queue         []*models.DownloadTask
	cfg           *config.Config
	downloader    *Downloader
	civitaiClient api.ModelInfoFetcher
	ctx           context.Context
	cancel        context.CancelFunc
	webhookURL    string
	webhookMethod string
	onUpdate      func(active int, queued int)
}

func NewManager(cfg *config.Config, civitaiClient api.ModelInfoFetcher) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		tasks:         make(map[string]*models.DownloadTask),
		taskCancels:   make(map[string]context.CancelFunc),
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

func (m *Manager) SetOnUpdate(fn func(active int, queued int)) {
	m.onUpdate = fn
}

func (m *Manager) notifyUpdate() {
	if m.onUpdate != nil {
		m.onUpdate(m.active, len(m.queue))
	}
}

func civitaiTypeToLmType(modelType string) string {
	t := strings.ToLower(modelType)
	switch t {
	case "checkpoint", "checkpoints", "ckpt":
		return "checkpoints"
	case "lora", "loras":
		return "loras"
	case "textualinversion", "textual inversion", "embedding", "embeddings":
		return "embeddings"
	case "hypernetwork", "hypernetworks":
		return "hypernetworks"
	default:
		return ""
	}
}

func resolveLmRootPath(baseURL, modelType string) (string, error) {
	lmType := civitaiTypeToLmType(modelType)
	if lmType == "" || baseURL == "" {
		return "", fmt.Errorf("unsupported type or empty base url")
	}
	apiURL := fmt.Sprintf("%s/api/lm/%s/roots", strings.TrimRight(baseURL, "/"), lmType)
	resp, err := http.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("lm returned %d", resp.StatusCode)
	}
	var roots []string
	if err := json.Unmarshal(body, &roots); err != nil {
		return "", err
	}
	if len(roots) == 0 {
		return "", fmt.Errorf("no roots for type %s", lmType)
	}
	return roots[0], nil
}

func (m *Manager) AddTask(req models.DownloadRequest) (*models.DownloadTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.APIKey == "" {
		req.APIKey = m.cfg.APIKey
	}

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
		PreviewImage:   req.PreviewImage,
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
			if apiErr, ok := err.(*models.APIError); ok {
				logger.Log.Warn("Failed to fetch model info",
					zap.String("code", string(apiErr.Code)),
					zap.Error(err),
				)
				return nil, err
			}
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
			if len(info.Files) > 0 {
				task.FileSizeBytes = int64(info.Files[0].SizeKB * 1024)
			}
		}
	}

	task.ModelType = api.ParseModelType(modelType)
	task.BaseModel = baseModel

	root := m.cfg.RootPath
	if req.SavePath != "" {
		root = req.SavePath
	} else if m.cfg.LoraMgr.UseLmPath && m.cfg.LoraMgr.BaseURL != "" && modelType != "" {
		if lmPath, err := resolveLmRootPath(m.cfg.LoraMgr.BaseURL, modelType); err == nil && lmPath != "" {
			root = lmPath
			logger.Log.Debug("using LM root path", zap.String("path", lmPath))
		} else {
			logger.Log.Debug("LM root path failed, using config", zap.Error(err))
		}
	}
	savePath := config.GetSavePath(root, task.ModelType, task.BaseModel, false, m.cfg.NSFW.FolderSuffix)
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
	task.Error = ""
	m.active++
	attempt := 1
	maxAttempts := m.cfg.Queue.RetryAttempts
	retryDelay := m.cfg.Queue.RetryDelaySec
	m.mu.Unlock()

	logger.Log.Info("Download started",
		zap.String("id", task.ID),
		zap.String("model", task.ModelName),
		zap.String("url", fmt.Sprintf("civitai.com/api/download/models/%d", task.ModelVersionID)),
	)

	dlCtx, dlCancel := context.WithCancel(m.ctx)
	m.mu.Lock()
	m.taskCancels[task.ID] = dlCancel
	m.mu.Unlock()

	for attempt <= maxAttempts {
		err := m.downloader.Download(dlCtx, task, func(downloaded, total int64) {
			m.notifyUpdate()
		})

		if err == nil {
			break
		}

		if models.IsDownloadCanceled(err) {
			m.mu.Lock()
			if task.Status != models.StatusFailed {
				task.Status = models.StatusPaused
				task.Error = "canceled"
			}
			m.downloader.RemoveTempFile(task)
			m.active--
			m.saveQueue()
			delete(m.taskCancels, task.ID)
			m.mu.Unlock()
			logger.Log.Info("Download cancelled", zap.String("id", task.ID))
			m.notifyUpdate()
			return
		}

		if models.IsRetryable(err) && attempt < maxAttempts {
			delay := time.Duration(retryDelay) * time.Second

			apiErr, _ := err.(*models.APIError)
			if apiErr != nil && apiErr.RetryAfter > 0 {
				delay = time.Duration(apiErr.RetryAfter) * time.Second
			}

			logger.Log.Warn("Download failed, retrying",
				zap.String("id", task.ID),
				zap.Int("attempt", attempt),
				zap.Int("max_attempts", maxAttempts),
				zap.Duration("delay", delay),
				zap.Error(err),
			)

			task.Error = fmt.Sprintf("Попытка %d: %s", attempt, err.Error())
			m.notifyUpdate()

			select {
			case <-time.After(delay):
			case <-m.ctx.Done():
				m.mu.Lock()
				task.Status = models.StatusPaused
				task.Error = "shutdown"
				m.active--
				m.saveQueue()
				m.mu.Unlock()
				return
			}

			attempt++
			continue
		}

		m.mu.Lock()
		task.Status = models.StatusFailed
		task.Error = err.Error()
		m.active--
		logger.Log.Error("Download failed permanently",
			zap.String("id", task.ID),
			zap.Int("attempts", attempt),
			zap.Error(err),
		)
		m.saveQueue()
		m.processQueue()
		delete(m.taskCancels, task.ID)
		m.mu.Unlock()
		m.notifyUpdate()
		return
	}

	m.mu.Lock()
	if task.Status == models.StatusDownloading {
		task.Status = models.StatusCompleted
		now := time.Now()
		task.CompletedAt = &now
		task.Progress = 100
		task.Error = ""
		m.active--

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

	task, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("task not found: %s", id)
	}

	if task.Status == models.StatusDownloading {
		m.active--
	}

	if cancel, exists := m.taskCancels[id]; exists {
		cancel()
		delete(m.taskCancels, id)
	}

	oldStatus := task.Status
	task.Status = models.StatusFailed
	task.Error = "cancelled"

	// Retry file removal a few times in case download goroutine still holds the handle
	for i := 0; i < 5; i++ {
		if err := os.Remove(task.TempPath); err == nil {
			break
		}
		if i < 4 {
			time.Sleep(200 * time.Millisecond)
		}
	}

	m.saveQueue()
	m.mu.Unlock()

	if oldStatus == models.StatusDownloading {
		m.mu.Lock()
		m.processQueue()
		m.mu.Unlock()
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

func (m *Manager) GetTasksGrouped() *models.TasksResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	resp := &models.TasksResponse{}

	for _, task := range m.tasks {
		switch task.Status {
		case models.StatusDownloading:
			resp.Active = append(resp.Active, task)
		case models.StatusQueued, models.StatusPaused:
			resp.Queued = append(resp.Queued, task)
		case models.StatusCompleted, models.StatusFailed:
			resp.History = append(resp.History, task)
		}
	}

	return resp
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

	var history []models.DownloadTask
	for _, task := range m.tasks {
		if task.Status == models.StatusCompleted || task.Status == models.StatusFailed {
			history = append(history, *task)
		}
	}
	if len(history) > 1 {
		sort.Slice(history, func(i, j int) bool {
			ti := history[i].CompletedAt
			tj := history[j].CompletedAt
			if ti == nil { ti = &history[i].AddedAt }
			if tj == nil { tj = &history[j].AddedAt }
			return ti.After(*tj)
		})
	}
	if len(history) > 20 {
		history = history[:20]
	}
	state.History = history

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

	for i := range state.History {
		task := state.History[i]
		m.tasks[task.ID] = &task
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
