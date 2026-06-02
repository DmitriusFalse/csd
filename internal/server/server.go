package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/DmitriusFalse/csd/internal/api"
	"github.com/DmitriusFalse/csd/internal/config"
	"github.com/DmitriusFalse/csd/internal/downloader"
	"github.com/DmitriusFalse/csd/internal/logger"
	"github.com/DmitriusFalse/csd/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"
)

type Server struct {
	app        *fiber.App
	manager    *downloader.Manager
	port       int
	host       string
	configPath string
	logPath    string
	civitai    api.ModelInfoFetcher
}

func New(host string, port int, manager *downloader.Manager, configPath, logPath string, civitai api.ModelInfoFetcher) *Server {
	s := &Server{
		app: fiber.New(fiber.Config{
			DisableStartupMessage: true,
			ErrorHandler:          errorHandler,
		}),
		manager:    manager,
		port:       port,
		host:       host,
		configPath: configPath,
		logPath:    logPath,
		civitai:    civitai,
	}

	s.app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE",
	}))
	s.app.Use(recover.New())

	s.setupRoutes()
	return s
}

func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return jsonError(c, code, models.NewAPIError(
		models.ErrCodeServerError,
		err.Error(),
		code, false,
	))
}

func jsonError(c *fiber.Ctx, status int, apiErr *models.APIError) error {
	c.Status(status)
	return c.JSON(fiber.Map{
		"error":      apiErr.Message,
		"code":       apiErr.Code,
		"retryable":  apiErr.Retryable,
		"retryAfter": apiErr.RetryAfter,
	})
}

func (s *Server) setupRoutes() {
	s.app.Post("/download", s.handleDownload)

	s.app.Get("/queue", s.handleGetQueue)
	s.app.Get("/queue/:id", s.handleGetTask)
	s.app.Post("/queue/:id/pause", s.handlePauseTask)
	s.app.Post("/queue/:id/resume", s.handleResumeTask)
	s.app.Post("/queue/:id/cancel", s.handleCancelTask)
	s.app.Post("/queue/pause-all", s.handlePauseAll)
	s.app.Post("/queue/resume-all", s.handleResumeAll)

	s.app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"active": s.manager.GetActiveCount(),
			"queued": s.manager.GetQueueLength(),
		})
	})

	s.app.Get("/logs", func(c *fiber.Ctx) error {
		if s.logPath == "" {
			return c.Status(404).SendString("log file path not configured")
		}
		return c.SendFile(s.logPath)
	})

	s.app.Post("/logs/open", func(c *fiber.Ctx) error {
		if s.logPath == "" {
			return c.Status(404).JSON(fiber.Map{"error": "log path not configured"})
		}
		absPath, err := filepath.Abs(s.logPath)
		if err == nil {
			exec.Command("explorer", "/select,", absPath).Start()
		}
		return c.JSON(fiber.Map{"status": "opened"})
	})

	s.app.Get("/config", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(configPageHTML)
	})

	s.app.Get("/api/config", func(c *fiber.Ctx) error {
		cfg, err := config.Load(s.configPath)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		key := cfg.APIKey
		if len(key) > 4 {
			key = key[:4] + "****"
		} else if key != "" {
			key = "****"
		}
		return c.JSON(fiber.Map{
			"server":             cfg.Server,
			"api_key":            key,
			"root_path":          cfg.RootPath,
			"max_concurrent":     cfg.Queue.MaxConcurrent,
			"retry_attempts":     cfg.Queue.RetryAttempts,
			"retry_delay_seconds": cfg.Queue.RetryDelaySec,
			"allow_nsfw":         cfg.NSFW.AllowNSFW,
			"separate_folder":    cfg.NSFW.SeparateFolder,
			"save_json":          cfg.Metadata.SaveJSON,
			"log_level":          cfg.Logging.Level,
		"lora_enabled":       cfg.LoraMgr.Enabled,
		"lm_base_url":        cfg.LoraMgr.BaseURL,
		"webhook_url":        cfg.LoraMgr.WebhookURL,
		})
	})

	s.app.Post("/api/config", func(c *fiber.Ctx) error {
		var updates map[string]interface{}
		if err := c.BodyParser(&updates); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json"})
		}
		cfg, err := config.Load(s.configPath)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if v, ok := updates["root_path"]; ok {
			if s, ok := v.(string); ok {
				cfg.RootPath = s
			}
		}
		if v, ok := updates["max_concurrent"]; ok {
			if f, ok := v.(float64); ok {
				cfg.Queue.MaxConcurrent = int(f)
			}
		}
		if v, ok := updates["retry_attempts"]; ok {
			if f, ok := v.(float64); ok {
				cfg.Queue.RetryAttempts = int(f)
			}
		}
		if v, ok := updates["retry_delay_seconds"]; ok {
			if f, ok := v.(float64); ok {
				cfg.Queue.RetryDelaySec = int(f)
			}
		}
		if v, ok := updates["allow_nsfw"]; ok {
			if b, ok := v.(bool); ok {
				cfg.NSFW.AllowNSFW = b
			}
		}
		if v, ok := updates["separate_folder"]; ok {
			if b, ok := v.(bool); ok {
				cfg.NSFW.SeparateFolder = b
			}
		}
		if v, ok := updates["save_json"]; ok {
			if b, ok := v.(bool); ok {
				cfg.Metadata.SaveJSON = b
			}
		}
		if v, ok := updates["log_level"]; ok {
			if s, ok := v.(string); ok {
				cfg.Logging.Level = s
			}
		}
		if v, ok := updates["lora_enabled"]; ok {
			if b, ok := v.(bool); ok {
				cfg.LoraMgr.Enabled = b
			}
		}
		if v, ok := updates["lm_base_url"]; ok {
			if s, ok := v.(string); ok {
				cfg.LoraMgr.BaseURL = s
			}
		}
		if v, ok := updates["webhook_url"]; ok {
			if s, ok := v.(string); ok {
				cfg.LoraMgr.WebhookURL = s
			}
		}
		if err := config.Save(*cfg, s.configPath); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "saved"})
	})

	s.app.Get("/tasks", func(c *fiber.Ctx) error {
		return c.JSON(s.manager.GetTasksGrouped())
	})

	s.app.Get("/api/check-downloaded", s.handleCheckDownloaded)
	s.app.Get("/api/check-lm-health", s.handleCheckLMHealth)
	s.app.Post("/api/download-by-model-id", s.handleDownloadByModelID)
}

func (s *Server) handleCheckDownloaded(c *fiber.Ctx) error {
	name := c.Query("name")
	modelType := c.Query("type")
	if name == "" || modelType == "" {
		return jsonError(c, http.StatusBadRequest, models.NewAPIError(
			models.ErrCodeInvalidRequest, "name and type params required", 400, false,
		))
	}

	cfg, err := config.Load(s.configPath)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, models.NewAPIError(
			models.ErrCodeServerError, "failed to load config", 500, false,
		))
	}

	logger.Log.Debug("check-downloaded",
		zap.String("name", name),
		zap.String("type", modelType),
		zap.String("lm_base_url", cfg.LoraMgr.BaseURL),
	)

	found, item := checkLoraManager(cfg.LoraMgr.BaseURL, modelType, name)
	logger.Log.Debug("check-downloaded result", zap.Bool("found", found))
	if found {
		return c.JSON(fiber.Map{"downloaded": true, "source": "lm", "item": item})
	}
	return c.JSON(fiber.Map{"downloaded": false})
}

func (s *Server) handleDownloadByModelID(c *fiber.Ctx) error {
	var req struct {
		ModelID     int    `json:"modelId"`
		ModelName   string `json:"modelName"`
		PreviewImage string `json:"previewImage"`
		ModelType   string `json:"modelType"`
	}
	if err := c.BodyParser(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, models.NewAPIError(
			models.ErrCodeInvalidRequest, "invalid json", 400, false,
		))
	}
	if req.ModelID == 0 {
		return jsonError(c, http.StatusBadRequest, models.NewAPIError(
			models.ErrCodeInvalidRequest, "modelId required", 400, false,
		))
	}

	cfg, err := config.Load(s.configPath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Fetch model info from CivitAI to get latest version
	apiKey := cfg.APIKey
	apiURL := fmt.Sprintf("https://civitai.com/api/v1/models/%d", req.ModelID)

	client := &http.Client{Timeout: 15 * time.Second}
	httpReq, _ := http.NewRequest("GET", apiURL, nil)
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "CivitAI request failed: " + err.Error()})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.Status(502).JSON(fiber.Map{"error": fmt.Sprintf("CivitAI returned %d", resp.StatusCode)})
	}

	body, _ := io.ReadAll(resp.Body)
	var modelInfo struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Type     string `json:"type"`
		Versions []struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Files []struct {
				ID       int    `json:"id"`
				Name     string `json:"name"`
				SizeKB   float64 `json:"sizeKB"`
				Metadata struct {
					Fp string `json:"fp"`
				} `json:"metadata"`
			} `json:"files"`
		} `json:"modelVersions"`
	}
	if err := json.Unmarshal(body, &modelInfo); err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "parse failed: " + err.Error()})
	}

	if len(modelInfo.Versions) == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "no versions found"})
	}

	// Pick latest version
	latestVer := modelInfo.Versions[0]
	if len(latestVer.Files) == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "no files in latest version"})
	}

	firstFile := latestVer.Files[0]
	modelName := req.ModelName
	if modelName == "" {
		modelName = modelInfo.Name
	}

	task, err := s.manager.AddTask(models.DownloadRequest{
		ModelVersionID: latestVer.ID,
		FileID:         firstFile.ID,
		ModelName:      modelName,
		ModelType:      modelInfo.Type,
		FileName:       firstFile.Name,
		FileSize:       fmt.Sprintf("%.0f KB", firstFile.SizeKB),
		PreviewImage:   req.PreviewImage,
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"id":      task.ID,
		"status":  task.Status,
		"version": latestVer.ID,
		"file":    firstFile.ID,
	})
}

func (s *Server) handleCheckLMHealth(c *fiber.Ctx) error {
	cfg, err := config.Load(s.configPath)
	if err != nil {
		return c.JSON(fiber.Map{"reachable": false, "error": "config load failed"})
	}
	baseURL := cfg.LoraMgr.BaseURL
	if baseURL == "" {
		return c.JSON(fiber.Map{"reachable": false, "error": "lm_base_url not configured"})
	}
	resp, err := http.Get(baseURL + "/api/lm/loras/list?page_size=1")
	if err != nil {
		return c.JSON(fiber.Map{"reachable": false, "error": err.Error()})
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return c.JSON(fiber.Map{
		"reachable": resp.StatusCode == http.StatusOK,
		"status":    resp.StatusCode,
		"body_preview": string(body)[:min(len(string(body)), 200)],
	})
}

func checkLoraManager(baseURL, modelType, modelName string) (bool, map[string]interface{}) {
	if baseURL == "" {
		return false, nil
	}

	types := civitaiToLmTypes(modelType)

	for _, t := range types {
		base, err := url.Parse(baseURL + "/api/lm/" + t + "/list")
		if err != nil {
			logger.Log.Debug("check-downloaded: bad base url", zap.Error(err))
			continue
		}

		for _, params := range []url.Values{
			{"search": {modelName}, "fuzzy": {"true"}, "search_filename": {"true"}, "page_size": {"20"}},
			{"search": {modelName}, "page_size": {"20"}},
			{"search_modelname": {modelName}, "page_size": {"20"}},
			{"search_filename": {modelName}, "page_size": {"20"}},
		} {
			base.RawQuery = params.Encode()
			urlStr := base.String()
			logger.Log.Debug("check-downloaded: requesting lm", zap.String("url", urlStr))

			resp, err := http.Get(urlStr)
			if err != nil {
				logger.Log.Debug("check-downloaded: lm request failed", zap.String("type", t), zap.Error(err))
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}

			if resp.StatusCode != http.StatusOK {
				logger.Log.Debug("check-downloaded: lm bad status", zap.Int("status", resp.StatusCode), zap.String("body", string(body)))
				continue
			}

			items := parseLmListResponse(body)
			logger.Log.Debug("check-downloaded: lm items", zap.Int("count", len(items)), zap.String("body", string(body)))

			searchName := strings.ToLower(modelName)
			for _, item := range items {
				for _, key := range []string{"name", "filename", "model_name", "title", "file_name", "modelName", "fileName"} {
					if val, ok := item[key].(string); ok {
						if strings.Contains(strings.ToLower(val), searchName) {
							logger.Log.Debug("check-downloaded: matched", zap.String("key", key), zap.String("val", val))
							return true, item
						}
					}
				}
			}
		}
	}

	return false, nil
}

func parseLmListResponse(body []byte) []map[string]interface{} {
	// format 1: { "items": [...] }
	var withItems struct {
		Items []map[string]interface{} `json:"items"`
	}
	if json.Unmarshal(body, &withItems) == nil && len(withItems.Items) > 0 {
		return withItems.Items
	}

	// format 2: { "data": [...] }
	var withData struct {
		Data []map[string]interface{} `json:"data"`
	}
	if json.Unmarshal(body, &withData) == nil && len(withData.Data) > 0 {
		return withData.Data
	}

	// format 3: [...] (direct array)
	var arr []map[string]interface{}
	if json.Unmarshal(body, &arr) == nil && len(arr) > 0 {
		return arr
	}

	// format 4: { "models": [...] }
	var obj map[string]interface{}
	if json.Unmarshal(body, &obj) == nil {
		if list, ok := obj["models"].([]interface{}); ok {
			var result []map[string]interface{}
			for _, m := range list {
				if mm, ok := m.(map[string]interface{}); ok {
					result = append(result, mm)
				}
			}
			if len(result) > 0 {
				return result
			}
		}
		// format 5: { "results": [...] }
		if list, ok := obj["results"].([]interface{}); ok {
			var result []map[string]interface{}
			for _, m := range list {
				if mm, ok := m.(map[string]interface{}); ok {
					result = append(result, mm)
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}

	return nil
}

func civitaiToLmTypes(civitaiType string) []string {
	t := strings.ToLower(civitaiType)
	switch t {
	case "lora", "loras":
		return []string{"loras"}
	case "checkpoint", "checkpoints", "ckpt":
		return []string{"checkpoints"}
	case "textualinversion", "textual inversion", "embedding", "embeddings":
		return []string{"embeddings"}
	case "hypernetwork", "hypernetworks":
		return []string{"hypernetworks"}
	default:
		return []string{"loras", "checkpoints", "embeddings"}
	}
}

func (s *Server) handleDownload(c *fiber.Ctx) error {
	var req models.DownloadRequest
	if err := c.BodyParser(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, models.NewAPIError(
			models.ErrCodeInvalidRequest,
			"Некорректный JSON в запросе",
			400, false,
		))
	}

	if req.ModelVersionID == 0 {
		return jsonError(c, http.StatusBadRequest, models.NewAPIError(
			models.ErrCodeInvalidRequest,
			"modelVersionId обязателен",
			400, false,
		))
	}

	if req.FileID == 0 {
		return jsonError(c, http.StatusBadRequest, models.NewAPIError(
			models.ErrCodeInvalidRequest,
			"fileId обязателен",
			400, false,
		))
	}

	task, err := s.manager.AddTask(req)
	if err != nil {
		var apiErr *models.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.Code {
			case models.ErrCodeUnauthorized:
				return jsonError(c, http.StatusUnauthorized, apiErr)
			case models.ErrCodeForbidden:
				return jsonError(c, http.StatusForbidden, apiErr)
			case models.ErrCodeNotFound:
				return jsonError(c, http.StatusNotFound, apiErr)
			case models.ErrCodeRateLimited:
				return jsonError(c, http.StatusTooManyRequests, apiErr)
			case models.ErrCodeCloudflare:
				return jsonError(c, http.StatusServiceUnavailable, apiErr)
			default:
				return jsonError(c, http.StatusBadRequest, apiErr)
			}
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"id":      task.ID,
		"status":  task.Status,
		"message": fmt.Sprintf("Task %s created", task.ID[:8]),
	})
}

func (s *Server) handleGetQueue(c *fiber.Ctx) error {
	tasks := s.manager.GetAllTasks()
	return c.JSON(tasks)
}

func (s *Server) handleGetTask(c *fiber.Ctx) error {
	id := c.Params("id")
	task := s.manager.GetTask(id)
	if task == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "task not found"})
	}
	return c.JSON(task)
}

func (s *Server) handlePauseTask(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := s.manager.PauseTask(id); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "paused"})
}

func (s *Server) handleResumeTask(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := s.manager.ResumeTask(id); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "resumed"})
}

func (s *Server) handleCancelTask(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := s.manager.CancelTask(id); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "cancelled"})
}

func (s *Server) handlePauseAll(c *fiber.Ctx) error {
	s.manager.PauseAll()
	return c.JSON(fiber.Map{"status": "all_paused"})
}

func (s *Server) handleResumeAll(c *fiber.Ctx) error {
	s.manager.ResumeAll()
	return c.JSON(fiber.Map{"status": "all_resumed"})
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	return s.app.Listen(addr)
}

func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}

const configPageHTML = `<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Civitai Smart Downloader — Config</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#1a1a2e;color:#e0e0e0;padding:20px;max-width:640px;margin:auto}
h1{font-size:18px;margin-bottom:20px;background:linear-gradient(135deg,#667eea,#764ba2);-webkit-background-clip:text;-webkit-text-fill-color:transparent}
.section{background:#16213e;border:1px solid #1e2d4a;border-radius:10px;padding:16px;margin-bottom:16px}
.section h2{font-size:13px;color:#9ca3af;text-transform:uppercase;margin-bottom:12px}
.row{display:flex;flex-direction:column;gap:4px;margin-bottom:12px}
.row:last-child{margin-bottom:0}
.row label{font-size:12px;color:#9ca3af}
.row input,.row select{padding:8px 10px;border:1px solid #374151;border-radius:6px;background:#111827;color:#e0e0e0;font-size:13px;outline:none}
.row input:focus{border-color:#667eea}
.row.check{flex-direction:row;align-items:center;gap:8px}
.row.check input{width:18px;height:18px;accent-color:#667eea}
.row.check label{margin:0}
.btn{padding:10px 20px;border:none;border-radius:8px;background:linear-gradient(135deg,#667eea,#764ba2);color:#fff;font-size:14px;font-weight:600;cursor:pointer;width:100%}
.btn:hover{opacity:0.9}
#status{margin-top:12px;padding:10px;border-radius:6px;display:none;font-size:13px}
#status.ok{display:block;background:#065f46;color:#6ee7b7}
#status.err{display:block;background:#7f1d1d;color:#fca5a5}
</style>
</head>
<body>
<h1>Civitai Smart Downloader — Настройки</h1>
<div id="status"></div>
<div class="section">
<h2>Сервер</h2>
<div class="row"><label>Порт</label><input type="number" id="port" disabled></div>
<div class="row"><label>API-ключ</label><input id="api_key" disabled></div>
</div>
<div class="section">
<h2>Скачивание</h2>
<div class="row"><label>Путь сохранения (root_path)</label><input id="root_path"></div>
<div class="row"><label>Макс. одновременных</label><input type="number" id="max_concurrent"></div>
<div class="row"><label>Попыток при ошибке</label><input type="number" id="retry_attempts"></div>
<div class="row"><label>Задержка между попытками (сек)</label><input type="number" id="retry_delay_seconds"></div>
</div>
<div class="section">
<h2>NSFW</h2>
<div class="row check"><input type="checkbox" id="allow_nsfw"><label for="allow_nsfw">Разрешить NSFW</label></div>
<div class="row check"><input type="checkbox" id="separate_folder"><label for="separate_folder">Отдельная папка</label></div>
</div>
<div class="section">
<h2>Метаданные</h2>
<div class="row check"><input type="checkbox" id="save_json"><label for="save_json">Сохранять JSON</label></div>
</div>
<div class="section">
<h2>Логи</h2>
<div class="row"><label>Уровень</label><select id="log_level"><option>debug</option><option>info</option><option>warn</option><option>error</option></select></div>
</div>
		<div class="section">
		<h2>Lora Manager</h2>
		<div class="row check"><input type="checkbox" id="lora_enabled"><label for="lora_enabled">Включить webhook</label></div>
		<div class="row"><label>Base URL</label><input id="lm_base_url"></div>
		<div class="row"><label>Webhook URL</label><input id="webhook_url"></div>
		</div>
<button class="btn" onclick="save()">Сохранить</button>
<script>
async function load(){
  try{
    const r=await fetch('/api/config');const d=await r.json();
    set('port',d.server.port);set('api_key',d.api_key);
    set('root_path',d.root_path);set('max_concurrent',d.max_concurrent);
    set('retry_attempts',d.retry_attempts);set('retry_delay_seconds',d.retry_delay_seconds);
    setChk('allow_nsfw',d.allow_nsfw);setChk('separate_folder',d.separate_folder);
    setChk('save_json',d.save_json);set('log_level',d.log_level);
    setChk('lora_enabled',d.lora_enabled);set('lm_base_url',d.lm_base_url);set('webhook_url',d.webhook_url);
  }catch(e){show('err','Failed to load: '+e.message)}
}
function set(id,v){const e=document.getElementById(id);if(e)e.value=v??''}
function setChk(id,v){const e=document.getElementById(id);if(e)e.checked=!!v}
function get(id){return document.getElementById(id)?.value??''}
function getChk(id){return document.getElementById(id)?.checked??false}
async function save(){
  const body={root_path:get('root_path'),max_concurrent:parseInt(get('max_concurrent'))||2,
    retry_attempts:parseInt(get('retry_attempts'))||3,retry_delay_seconds:parseInt(get('retry_delay_seconds'))||60,
    allow_nsfw:getChk('allow_nsfw'),separate_folder:getChk('separate_folder'),
    save_json:getChk('save_json'),log_level:get('log_level'),
    lora_enabled:getChk('lora_enabled'),lm_base_url:get('lm_base_url'),webhook_url:get('webhook_url')};
  try{
    const r=await fetch('/api/config',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
    const d=await r.json();
    if(d.status==='saved')show('ok','Сохранено!');
    else show('err',d.error||'Error');
  }catch(e){show('err',e.message)}
}
function show(t,m){const e=document.getElementById('status');e.className=t;e.textContent=m;e.style.display='block';setTimeout(()=>{e.style.display='none'},4000)}
load();
</script>
</body>
</html>`
