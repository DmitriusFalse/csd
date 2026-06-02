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
	"golang.org/x/sys/windows/registry"
)

//go:embed icons/default.ico
var iconFS embed.FS

var (
	defaultIcon []byte

	manager   *downloader.Manager
	rootPath  string
	appVersion string
	onExitFn  func()
)

func Run(mgr *downloader.Manager, root, version string, onExit func()) {
	manager = mgr
	rootPath = root
	appVersion = version
	onExitFn = onExit

	var err error
	defaultIcon, err = iconFS.ReadFile("icons/default.ico")
	if err != nil {
		logger.Log.Warn("Failed to load default icon", zap.Error(err))
	}

	systray.Run(onReady, onExitFn)
}

func onReady() {
	systray.SetTitle("CSD")
	systray.SetTooltip("Civitai Smart Downloader v" + appVersion)
	if len(defaultIcon) > 0 {
		systray.SetTemplateIcon(defaultIcon, defaultIcon)
	}

	titleItem := systray.AddMenuItem("Civitai Smart Downloader v"+appVersion, "Civitai Smart Downloader")
	titleItem.Disable()

	systray.AddSeparator()

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

	autoStartItem := systray.AddMenuItem("🔄 Автозагрузка", "Run on Windows startup")
	if isAutoStartEnabled() {
		autoStartItem.Check()
	}

	systray.AddSeparator()

	donateItem := systray.AddMenuItem("❤ Поддержать", "Donate")
	boostyItem := donateItem.AddSubMenuItem("Boosty", "Donate via Boosty")
	patreonItem := donateItem.AddSubMenuItem("Patreon", "Donate via Patreon")

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

	boostyItem.Click(func() {
		openURL("https://boosty.to/sir.geronis/donate")
	})

	patreonItem.Click(func() {
		openURL("https://www.patreon.com/16134050/join")
	})

	autoStartItem.Click(func() {
		enabled := !autoStartItem.Checked()
		if err := setAutoStart(enabled); err != nil {
			logger.Log.Error("Failed to toggle autostart", zap.Error(err))
			return
		}
		if enabled {
			autoStartItem.Check()
		} else {
			autoStartItem.Uncheck()
		}
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

	var lastUpdate time.Time
	manager.SetOnUpdate(func(activeCount int, queueLen int) {
		if time.Since(lastUpdate) < 500*time.Millisecond {
			return
		}
		lastUpdate = time.Now()

		tooltip := "Civitai Smart Downloader v" + appVersion
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

const regKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const regName = "CivitaiSmartDownloader"

func isAutoStartEnabled() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(regName)
	return err == nil
}

func setAutoStart(enable bool) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	if enable {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer k.Close()
		return k.SetStringValue(regName, `"`+exe+`"`)
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.DeleteValue(regName)
}
