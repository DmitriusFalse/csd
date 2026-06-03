package i18n

import (
	"os"
	"runtime"
	"strings"
)

type Lang string

const (
	LangEN   Lang = "en"
	LangRU   Lang = "ru"
	LangAuto Lang = "auto"
)

func DetectLang() Lang {
	if runtime.GOOS == "windows" {
		return detectWindowsLang()
	}
	l := os.Getenv("LANG")
	if strings.HasPrefix(l, "ru") || strings.HasPrefix(l, "uk") || strings.HasPrefix(l, "be") {
		return LangRU
	}
	return LangEN
}

type T struct {
	en, ru string
}

func (t T) Get(lang Lang) string {
	if lang == LangRU {
		return t.ru
	}
	return t.en
}

func Tt(en, ru string) T { return T{en: en, ru: ru} }

var (
	PauseAll      = Tt("⏸ Pause all", "⏸ Пауза всех")
	ResumeAll     = Tt("▶ Resume all", "▶ Возобновить все")
	OpenFolder    = Tt("📂 Open downloads folder", "📂 Открыть папку загрузок")
	OpenConfig    = Tt("📄 Open config.yaml", "📄 Открыть config.yaml")
	Sites         = Tt("🌐 Websites", "🌐 Сайты")
	AutoStart     = Tt("🔄 Run on Windows startup", "🔄 Запуск при старте Windows")
	Donate        = Tt("❤️ Donate", "❤ Поддержать")
	Quit          = Tt("❌ Quit", "❌ Выход")
	Language      = Tt("🌐 Language", "🌐 Язык")
	TAuto     = Tt("Auto (system)", "Авто (системы)")
	TEn       = Tt("English", "English")
	TRu       = Tt("Russian", "Русский")
	StatusActive  = Tt("%d active | %d queued", "%d активных | %d в очереди")
)
