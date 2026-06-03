package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitGeneratesNewKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	if err := Init(cfgPath); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	keyPath := filepath.Join(dir, "key.dat")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("key.dat not created: %v", err)
	}
	if len(data) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(data))
	}
	if len(appKey) != 32 {
		t.Fatal("appKey not set")
	}
}

func TestInitLoadsExistingKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	existingKey := make([]byte, 32)
	for i := range existingKey {
		existingKey[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(dir, "key.dat"), existingKey, 0600); err != nil {
		t.Fatal(err)
	}

	if err := Init(cfgPath); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	for i, b := range appKey {
		if b != byte(i) {
			t.Fatalf("key mismatch at %d: %d != %d", i, b, i)
		}
	}
}

func TestDecryptWithoutInit(t *testing.T) {
	appKey = nil
	_, err := Decrypt("dGVzdA==")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestInitRejectsShortKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	shortKey := []byte("too short")
	if err := os.WriteFile(filepath.Join(dir, "key.dat"), shortKey, 0600); err != nil {
		t.Fatal(err)
	}

	if err := Init(cfgPath); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	keyPath := filepath.Join(dir, "key.dat")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 32 {
		t.Fatalf("expected new 32-byte key, got %d bytes", len(data))
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	testInit(t)
	_, err := Decrypt("not-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecryptShortCiphertext(t *testing.T) {
	testInit(t)
	_, err := Decrypt("AAEC") // valid base64 but too short (3 bytes decoded)
	if err == nil {
		t.Fatal("expected error for short ciphertext")
	}
}

func testInit(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := Init(cfgPath); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
}
