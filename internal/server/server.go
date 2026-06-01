package server

import (
	"fmt"
	"errors"
	"net/http"

	"github.com/DmitriusFalse/csd/internal/downloader"
	"github.com/DmitriusFalse/csd/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

type Server struct {
	app     *fiber.App
	manager *downloader.Manager
	port    int
	host    string
}

func New(host string, port int, manager *downloader.Manager) *Server {
	s := &Server{
		app: fiber.New(fiber.Config{
			DisableStartupMessage: true,
			ErrorHandler:          errorHandler,
		}),
		manager: manager,
		port:    port,
		host:    host,
	}

	s.app.Use(recover.New())
	s.app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, X-Bridge-Secret",
	}))

	s.setupRoutes()

	return s
}

func errorHandler(c *fiber.Ctx, err error) error {
	var apiErr *models.APIError
	if errors.As(err, &apiErr) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":      apiErr.Message,
			"code":       apiErr.Code,
			"retryable":  apiErr.Retryable,
			"retryAfter": apiErr.RetryAfter,
		})
	}

	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error":     err.Error(),
		"code":      models.ErrCodeServerError,
		"retryable": false,
	})
}

func jsonError(c *fiber.Ctx, statusCode int, apiErr *models.APIError) error {
	return c.Status(statusCode).JSON(fiber.Map{
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
