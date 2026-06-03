package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func closeWriter(t *testing.T, w *rotatingWriter) {
	t.Helper()
	if w != nil && w.file != nil {
		w.file.Close()
		w.file = nil
	}
}

func TestRotatingWriterCreatesFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	w := &rotatingWriter{
		path:       logPath,
		maxSizeMB:  10,
		maxBackups: 3,
		compress:   false,
	}
	defer closeWriter(t, w)

	n, err := w.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 11 {
		t.Errorf("expected 11 bytes written, got %d", n)
	}
}

func TestRotatingWriterAppends(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "append.log")

	w := &rotatingWriter{
		path:       logPath,
		maxSizeMB:  10,
		maxBackups: 3,
		compress:   false,
	}
	defer closeWriter(t, w)

	w.Write([]byte("first "))
	w.Write([]byte("second"))

	data, _ := os.ReadFile(logPath)
	if string(data) != "first second" {
		t.Errorf("expected 'first second', got '%s'", string(data))
	}
}

func TestRotatingWriterRotates(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "rotate.log")

	w := &rotatingWriter{
		path:       logPath,
		maxSizeMB:  1,
		maxBackups: 2,
		compress:   false,
	}

	line := strings.Repeat("A", 1024*1024)
	for i := 0; i < 3; i++ {
		w.Write([]byte(line))
	}
	closeWriter(t, w)

	entries, _ := os.ReadDir(dir)
	var backupFiles int
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "rotate.log.") {
			backupFiles++
		}
	}
	if backupFiles == 0 {
		t.Error("expected backup files after rotation")
	}
}

func TestRotatingWriterCompression(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "compress.log")

	w := &rotatingWriter{
		path:       logPath,
		maxSizeMB:  1,
		maxBackups: 2,
		compress:   true,
	}

	line := strings.Repeat("B", 1024*1024)
	for i := 0; i < 3; i++ {
		w.Write([]byte(line))
	}
	closeWriter(t, w)

	entries, _ := os.ReadDir(dir)
	var gzFiles int
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".gz") {
			gzFiles++
		}
	}
	if gzFiles == 0 {
		t.Error("expected .gz backup files after rotation with compression")
	}
}

func TestRotatingWriterSync(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sync.log")

	w := &rotatingWriter{
		path:       logPath,
		maxSizeMB:  10,
		maxBackups: 1,
	}
	defer closeWriter(t, w)

	w.Write([]byte("data"))
	if err := w.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
}

func TestRotatingWriterMaxBackups(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "backup.log")

	w := &rotatingWriter{
		path:       logPath,
		maxSizeMB:  1,
		maxBackups: 3,
		compress:   false,
	}

	line := strings.Repeat("C", 1024*1024)
	for i := 0; i < 10; i++ {
		w.Write([]byte(line))
	}
	closeWriter(t, w)

	entries, _ := os.ReadDir(dir)
	var backups int
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "backup.log.") {
			backups++
		}
	}
	if backups > 4 {
		t.Errorf("too many backups: %d (max 3 + current)", backups)
	}
}

func TestZapThroughRotatingWriter(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "zap.log")

	w := &rotatingWriter{
		path:       logPath,
		maxSizeMB:  10,
		maxBackups: 1,
	}
	defer closeWriter(t, w)

	encoderCfg := zapcore.EncoderConfig{
		MessageKey: "message",
		LevelKey:   "level",
	}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.AddSync(w), zapcore.DebugLevel)
	logger := zap.New(core)

	logger.Info("via zap")

	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "via zap") {
		t.Errorf("expected 'via zap' in log output: %s", string(data))
	}
}
