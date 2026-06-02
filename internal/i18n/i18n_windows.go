//go:build windows

package i18n

import "syscall"

func detectWindowsLang() Lang {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetUserDefaultUILanguage")
	lang, _, _ := proc.Call()
	primary := lang & 0x3FF
	if primary == 0x19 {
		return LangRU
	}
	return LangEN
}
