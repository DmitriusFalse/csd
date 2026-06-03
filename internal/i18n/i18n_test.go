package i18n

import (
	"testing"
)

func TestTGetEn(t *testing.T) {
	tv := Tt("Hello", "Привет")
	if got := tv.Get(LangEN); got != "Hello" {
		t.Errorf("expected 'Hello', got '%s'", got)
	}
}

func TestTGetRu(t *testing.T) {
	tv := Tt("Hello", "Привет")
	if got := tv.Get(LangRU); got != "Привет" {
		t.Errorf("expected 'Привет', got '%s'", got)
	}
}

func TestTGetDefaultToEn(t *testing.T) {
	tv := Tt("Default", "По умолчанию")
	if got := tv.Get(Lang("unknown")); got != "Default" {
		t.Errorf("expected 'Default', got '%s'", got)
	}
}

func TestAllTranslationsHaveBothLanguages(t *testing.T) {
	translations := []T{
		PauseAll, ResumeAll, OpenFolder, OpenConfig,
		Sites, AutoStart, Donate, Quit, Language,
		TAuto, TEn, TRu, StatusActive,
	}
	for _, tr := range translations {
		if tr.en == "" {
			t.Error("translation missing en")
		}
		if tr.ru == "" {
			t.Error("translation missing ru")
		}
	}
}

func TestStatusActiveFormat(t *testing.T) {
	en := StatusActive.Get(LangEN)
	ru := StatusActive.Get(LangRU)
	if en == "" || ru == "" {
		t.Fatal("StatusActive translations missing")
	}
}

func TestDetectLangReturnsValue(t *testing.T) {
	lang := DetectLang()
	if lang != LangEN && lang != LangRU && lang != LangAuto {
		t.Errorf("unexpected lang value: %s", lang)
	}
}

func TestTtFactory(t *testing.T) {
	tv := Tt("a", "b")
	if tv.en != "a" || tv.ru != "b" {
		t.Errorf("Tt factory failed: %+v", tv)
	}
}
