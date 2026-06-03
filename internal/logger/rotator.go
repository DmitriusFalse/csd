package logger

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type rotatingWriter struct {
	mu         sync.Mutex
	path       string
	maxSizeMB  int
	maxBackups int
	compress   bool
	file       *os.File
	size       int64
}

func (w *rotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}

	n, err = w.file.Write(p)
	w.size += int64(n)

	if w.size >= int64(w.maxSizeMB)*1024*1024 {
		w.rotate()
	}

	return n, err
}

func (w *rotatingWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Sync()
	}
	return nil
}

func (w *rotatingWriter) open() error {
	dir := filepath.Dir(w.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.file = f
	w.size = stat.Size()
	return nil
}

func (w *rotatingWriter) rotate() {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}

	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.%s", w.path, timestamp)
	_ = os.Rename(w.path, backupPath)

	w.size = 0

	if w.compress {
		w.compressFile(backupPath)
	}

	w.cleanup()
}

func (w *rotatingWriter) compressFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}

	gzPath := path + ".gz"
	gzFile, err := os.Create(gzPath)
	if err != nil {
		f.Close()
		return
	}

	gzWriter := gzip.NewWriter(gzFile)
	_, _ = io.Copy(gzWriter, f)
	_ = gzWriter.Close()
	gzFile.Close()
	f.Close()

	_ = os.Remove(path)
}

func (w *rotatingWriter) cleanup() {
	dir := filepath.Dir(w.path)
	base := filepath.Base(w.path)

	entries, _ := os.ReadDir(dir)
	var backups []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), base+".") {
			backups = append(backups, filepath.Join(dir, e.Name()))
		}
	}

	sort.Slice(backups, func(i, j int) bool {
		infoI, _ := os.Stat(backups[i])
		infoJ, _ := os.Stat(backups[j])
		if infoI == nil || infoJ == nil {
			return false
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})

	for len(backups) > w.maxBackups {
		_ = os.Remove(backups[len(backups)-1])
		backups = backups[:len(backups)-1]
	}
}
