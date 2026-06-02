package tray

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/DmitriusFalse/csd/internal/downloader"
	"github.com/DmitriusFalse/csd/internal/logger"
	"github.com/energye/systray"
	"go.uber.org/zap"
)

//go:embed icons/default.ico icons/01.ico icons/err.ico
var iconFS embed.FS

var (
	manager        *downloader.Manager
	rootPath       string
	onExitFn       func()
	defaultIcon    []byte
	icon01         []byte
	errIcon        []byte
	animTicker     *time.Ticker
	prevActive     int
)

func Run(mgr *downloader.Manager, root string, onExit func()) {
	manager = mgr
	rootPath = root
	onExitFn = onExit

	var err error
	defaultIcon, err = iconFS.ReadFile("icons/default.ico")
	if err != nil {
		logger.Log.Warn("Failed to load default icon", zap.Error(err))
	}
	icon01, err = iconFS.ReadFile("icons/01.ico")
	if err != nil {
		logger.Log.Warn("Failed to load 01 icon", zap.Error(err))
	}
	errIcon, err = iconFS.ReadFile("icons/err.ico")
	if err != nil {
		logger.Log.Warn("Failed to load err icon", zap.Error(err))
	}

	systray.Run(onReady, onExitFn)
}

func onReady() {
	systray.SetTitle("CSD")
	systray.SetTooltip("Civitai Smart Downloader")

	setTrayIcon(defaultIcon)

	activeDownloadsItem := systray.AddMenuItem("📥 Активные загрузки (0)", "Active downloads")
	activeDownloadsItem.Disable()

	pauseAllItem := systray.AddMenuItem("⏸ Пауза всех", "Pause all downloads")
	resumeAllItem := systray.AddMenuItem("▶ Возобновить все", "Resume all downloads")

	systray.AddSeparator()

	openFolderItem := systray.AddMenuItem("📂 Открыть папку загрузок", "Open downloads folder")
	configItem := systray.AddMenuItem("📄 Открыть config.yaml", "Open config file")

	systray.AddSeparator()

	sitesItem := systray.AddMenuItem("🌐 Сайты", "Websites")
	civitaiItem := sitesItem.AddSubMenuItem("civitai.com", "Open civitai.com")
	civitaiRedItem := sitesItem.AddSubMenuItem("civitai.red", "Open civitai.red")

	systray.AddSeparator()

	quitItem := systray.AddMenuItem("❌ Выход", "Quit")

	openFolderItem.Click(func() {
		openDir(rootPath)
	})

	configItem.Click(func() {
		openConfig()
	})

	civitaiItem.Click(func() {
		openURL("https://civitai.com")
	})

	civitaiRedItem.Click(func() {
		openURL("https://civitai.red")
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
		stopAnim()
		systray.Quit()
	})

	manager.SetOnUpdate(func(activeCount int, queueLen int) {
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

		if activeCount > 0 {
			if prevActive == 0 {
				startAnim()
			}
		} else {
			if prevActive > 0 {
				stopAnim()
				setTrayIcon(defaultIcon)
			}
		}
		prevActive = activeCount
	})
}

func startAnim() {
	stopAnim()
	frame := false
	animTicker = time.NewTicker(800 * time.Millisecond)
	go func() {
		for range animTicker.C {
			frame = !frame
			if frame && len(icon01) > 0 {
				setTrayIcon(icon01)
			} else if len(defaultIcon) > 0 {
				setTrayIcon(defaultIcon)
			}
		}
	}()
}

func stopAnim() {
	if animTicker != nil {
		animTicker.Stop()
		animTicker = nil
	}
}

func setTrayIcon(ico []byte) {
	if len(ico) > 0 {
		systray.SetTemplateIcon(ico, ico)
	}
}

func openDir(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			logger.Log.Error("Failed to create directory", zap.Error(err))
			return
		}
	}

	switch runtime.GOOS {
	case "windows":
		exec.Command("cmd", "/c", "start", "", path).Start()
	case "darwin":
		exec.Command("open", path).Start()
	default:
		exec.Command("xdg-open", path).Start()
	}
}

func openConfig() {
	configPath := "config.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logger.Log.Warn("Config file not found", zap.String("path", configPath))
		return
	}

	switch runtime.GOOS {
	case "windows":
		exec.Command("cmd", "/c", "start", "", configPath).Start()
	case "darwin":
		exec.Command("open", configPath).Start()
	default:
		exec.Command("xdg-open", configPath).Start()
	}
}

func openURL(url string) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("cmd", "/c", "start", "", url).Start()
	case "darwin":
		exec.Command("open", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}
