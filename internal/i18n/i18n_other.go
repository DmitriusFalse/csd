//go:build !windows

package i18n

func detectWindowsLang() Lang {
	return LangEN
}
