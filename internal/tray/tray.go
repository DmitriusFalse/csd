package tray

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/DmitriusFalse/csd/internal/downloader"
	"github.com/DmitriusFalse/csd/internal/i18n"
	"github.com/DmitriusFalse/csd/internal/logger"
	"github.com/energye/systray"
	"go.uber.org/zap"
	"golang.org/x/sys/windows/registry"
)

//go:embed icons/default.ico
var iconFS embed.FS

var (
	defaultIcon []byte

	manager    *downloader.Manager
	rootPath   string
	configPath string
	appVersion string
	onExitFn   func()
	lang       i18n.Lang
)

func Run(mgr *downloader.Manager, root, cfgPath, version string, onExit func()) {
	manager = mgr
	rootPath = root
	configPath = cfgPath
	appVersion = version
	onExitFn = onExit
	lang = i18n.DetectLang()

	var err error
	defaultIcon, err = iconFS.ReadFile("icons/default.ico")
	if err != nil {
		logger.Log.Warn("Failed to load default icon", zap.Error(err))
	}

	systray.Run(onReady, onExitFn)
}

func setMenuLang() {
	systray.SetTooltip("Civitai Smart Downloader v" + appVersion)
}

type menuItems struct {
	title, pauseAll, resumeAll, openFolder, openConfig,
	sites, civitai, civitaiRed, autoStart,
	donate, boosty, patreon, quit,
	langItem, langAuto, langEn, langRu *systray.MenuItem
}

var mi menuItems

func rebuildMenuText() {
	mi.pauseAll.SetTitle(i18n.PauseAll.Get(lang))
	mi.resumeAll.SetTitle(i18n.ResumeAll.Get(lang))
	mi.openFolder.SetTitle(i18n.OpenFolder.Get(lang))
	mi.openConfig.SetTitle(i18n.OpenConfig.Get(lang))
	mi.sites.SetTitle(i18n.Sites.Get(lang))
	mi.autoStart.SetTitle(i18n.AutoStart.Get(lang))
	mi.donate.SetTitle(i18n.Donate.Get(lang))
	mi.quit.SetTitle(i18n.Quit.Get(lang))
	mi.langItem.SetTitle(i18n.Language.Get(lang))
	mi.langAuto.SetTitle(i18n.TAuto.Get(lang))
	mi.langEn.SetTitle(i18n.TEn.Get(lang))
	mi.langRu.SetTitle(i18n.TRu.Get(lang))

	updateLangChecks()
}

func updateLangChecks() {
	mi.langAuto.Uncheck()
	mi.langEn.Uncheck()
	mi.langRu.Uncheck()
	switch lang {
	case i18n.LangAuto:
		mi.langAuto.Check()
	case i18n.LangEN:
		mi.langEn.Check()
	case i18n.LangRU:
		mi.langRu.Check()
	}
}

func onReady() {
	systray.SetTitle("CSD")
	setMenuLang()
	if len(defaultIcon) > 0 {
		systray.SetTemplateIcon(defaultIcon, defaultIcon)
	}

	mi.title = systray.AddMenuItem("Civitai Smart Downloader v"+appVersion, "Civitai Smart Downloader")
	mi.title.Disable()

	systray.AddSeparator()

	mi.pauseAll = systray.AddMenuItem(i18n.PauseAll.Get(lang), "Pause all downloads")
	mi.resumeAll = systray.AddMenuItem(i18n.ResumeAll.Get(lang), "Resume all downloads")

	systray.AddSeparator()

	mi.openFolder = systray.AddMenuItem(i18n.OpenFolder.Get(lang), "Open downloads folder")
	mi.openConfig = systray.AddMenuItem(i18n.OpenConfig.Get(lang), "Open config file")

	systray.AddSeparator()

	mi.sites = systray.AddMenuItem(i18n.Sites.Get(lang), "Websites")
	mi.civitai = mi.sites.AddSubMenuItem("civitai.com", "Open civitai.com")
	mi.civitaiRed = mi.sites.AddSubMenuItem("civitai.red", "Open civitai.red")

	systray.AddSeparator()

	mi.autoStart = systray.AddMenuItem(i18n.AutoStart.Get(lang), "Run on Windows startup")
	if isAutoStartEnabled() {
		mi.autoStart.Check()
	}

	systray.AddSeparator()

	mi.langItem = systray.AddMenuItem(i18n.Language.Get(lang), "Language")
	mi.langAuto = mi.langItem.AddSubMenuItem(i18n.TAuto.Get(lang), "Auto (system)")
	mi.langEn = mi.langItem.AddSubMenuItem(i18n.TEn.Get(lang), "English")
	mi.langRu = mi.langItem.AddSubMenuItem(i18n.TRu.Get(lang), "Russian")
	updateLangChecks()

	systray.AddSeparator()

	mi.donate = systray.AddMenuItem(i18n.Donate.Get(lang), "Donate")
	mi.boosty = mi.donate.AddSubMenuItem("Boosty", "Donate via Boosty")
	mi.patreon = mi.donate.AddSubMenuItem("Patreon", "Donate via Patreon")

	systray.AddSeparator()

	mi.quit = systray.AddMenuItem(i18n.Quit.Get(lang), "Quit")

	mi.openFolder.Click(func() {
		openDir(rootPath)
	})

	mi.openConfig.Click(func() {
		openConfig()
	})

	mi.civitai.Click(func() {
		openURL("https://civitai.com")
	})

	mi.civitaiRed.Click(func() {
		openURL("https://civitai.red")
	})

	mi.boosty.Click(func() {
		openURL("https://boosty.to/sir.geronis/donate")
	})

	mi.patreon.Click(func() {
		openURL("https://www.patreon.com/16134050/join")
	})

	mi.autoStart.Click(func() {
		enabled := !mi.autoStart.Checked()
		if err := setAutoStart(enabled); err != nil {
			logger.Log.Error("Failed to toggle autostart", zap.Error(err))
			return
		}
		if enabled {
			mi.autoStart.Check()
		} else {
			mi.autoStart.Uncheck()
		}
	})

	mi.pauseAll.Click(func() {
		manager.PauseAll()
	})

	mi.resumeAll.Click(func() {
		manager.ResumeAll()
	})

	mi.langAuto.Click(func() {
		lang = i18n.LangAuto
		lang = i18n.DetectLang()
		rebuildMenuText()
	})

	mi.langEn.Click(func() {
		lang = i18n.LangEN
		rebuildMenuText()
	})

	mi.langRu.Click(func() {
		lang = i18n.LangRU
		rebuildMenuText()
	})

	mi.quit.Click(func() {
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
			tooltip += " | " + fmt.Sprintf(i18n.StatusActive.Get(lang), activeCount, queueLen)
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
