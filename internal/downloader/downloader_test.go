package downloader

import (
	"testing"
)

func TestParseFileSize(t *testing.T) {
	tests := []struct {
		input string
		want  int64
		err   bool
	}{
		{"500 MB", 500 * 1024 * 1024, false},
		{"1 GB", 1 * 1024 * 1024 * 1024, false},
		{"150 KB", 150 * 1024, false},
		{"100 B", 100, false},
		{"", 0, true},
		{"abc", 0, true},
		{"0 GB", 0, false},
		{"1.5 MB", int64(1.5 * 1024 * 1024), false},
		{"10.75 GB", int64(10.75 * 1024 * 1024 * 1024), false},
	}

	for _, tt := range tests {
		got, err := ParseFileSize(tt.input)
		if tt.err {
			if err == nil {
				t.Errorf("ParseFileSize(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseFileSize(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseFileSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseFileSizeEdgeCases(t *testing.T) {
	if got, _ := ParseFileSize("0 B"); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
	if got, _ := ParseFileSize("1 KB"); got != 1024 {
		t.Errorf("expected 1024, got %d", got)
	}
	if got, _ := ParseFileSize("1 MB"); got != 1024*1024 {
		t.Errorf("expected %d, got %d", 1024*1024, got)
	}
	if got, _ := ParseFileSize("1 GB"); got != 1024*1024*1024 {
		t.Errorf("expected %d, got %d", 1024*1024*1024, got)
	}
}

func TestParseFileSizeCaseInsensitive(t *testing.T) {
	got, err := ParseFileSize("2 Gb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 2*1024*1024*1024 {
		t.Errorf("expected %d, got %d", 2*1024*1024*1024, got)
	}
}

func TestNewDownloader(t *testing.T) {
	d := New()
	if d == nil {
		t.Fatal("New() returned nil")
	}
}
