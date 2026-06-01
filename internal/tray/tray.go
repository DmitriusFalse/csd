package tray

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/DmitriusFalse/csd/internal/downloader"
	"github.com/DmitriusFalse/csd/internal/logger"
	"github.com/energye/systray"
	"go.uber.org/zap"
)

var (
	manager   *downloader.Manager
	rootPath  string
	onExitFn  func()
)

func Run(mgr *downloader.Manager, root string, onExit func()) {
	manager = mgr
	rootPath = root
	onExitFn = onExit
	systray.Run(onReady, onExitFn)
}

func onReady() {
	systray.SetTitle("CSD")
	systray.SetTooltip("Civitai Smart Downloader")

	iconBytes := generateIcon()
	if len(iconBytes) > 0 {
		systray.SetTemplateIcon(iconBytes, iconBytes)
	}

	activeDownloadsItem := systray.AddMenuItem("📥 Активные загрузки (0)", "Active downloads")
	activeDownloadsItem.Disable()

	pauseAllItem := systray.AddMenuItem("⏸ Пауза всех", "Pause all downloads")
	resumeAllItem := systray.AddMenuItem("▶ Возобновить все", "Resume all downloads")

	systray.AddSeparator()

	openFolderItem := systray.AddMenuItem("📂 Открыть папку загрузок", "Open downloads folder")

	settingsItem := systray.AddMenuItem("⚙ Настройки", "Settings")
	configItem := settingsItem.AddSubMenuItem("📄 Открыть config.yaml", "Open config file")
	changeKeyItem := settingsItem.AddSubMenuItem("🔑 Сменить API-ключ", "Change API key")

	systray.AddSeparator()

	quitItem := systray.AddMenuItem("❌ Выход", "Quit")

	openFolderItem.Click(func() {
		openDir(rootPath)
	})

	configItem.Click(func() {
		openConfig()
	})

	changeKeyItem.Click(func() {
		logger.Log.Info("API key change requested (not yet implemented)")
	})

	pauseAllItem.Click(func() {
		manager.PauseAll()
	})

	resumeAllItem.Click(func() {
		manager.ResumeAll()
	})

	quitItem.Click(func() {
		if manager.GetActiveCount() > 0 {
			logger.Log.Info("Quit with active downloads")
		}
		systray.Quit()
	})

	manager.SetOnUpdate(func() {
		activeCount := manager.GetActiveCount()
		queueLen := manager.GetQueueLength()
		activeDownloadsItem.SetTitle(fmt.Sprintf("📥 Активные загрузки (active: %d, queued: %d)", activeCount, queueLen))

		tooltip := "Civitai Smart Downloader"
		if activeCount > 0 {
			tooltip += fmt.Sprintf(" | %d active, %d queued", activeCount, queueLen)
		}
		systray.SetTooltip(tooltip)

		title := "CSD"
		if activeCount > 0 || queueLen > 0 {
			title = fmt.Sprintf("CSD [%da/%dq]", activeCount, queueLen)
		}
		systray.SetTitle(title)
	})
}

func openDir(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			logger.Log.Error("Failed to create directory", zap.Error(err))
			return
		}
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	_ = cmd.Start()
}

func openConfig() {
	configPath := "config.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logger.Log.Warn("Config file not found", zap.String("path", configPath))
		return
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("notepad", configPath)
	case "darwin":
		cmd = exec.Command("open", configPath)
	default:
		cmd = exec.Command("xdg-open", configPath)
	}
	_ = cmd.Start()
}

func generateIcon() []byte {
	w, h := 16, 16
	rgba := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cx, cy := x-w/2, y-h/2
			if cx*cx+cy*cy < 40 {
				idx := (y*w + x) * 4
				rgba[idx] = 60
				rgba[idx+1] = 130
				rgba[idx+2] = 255
				rgba[idx+3] = 255
			}
		}
	}
	return rgba
}
