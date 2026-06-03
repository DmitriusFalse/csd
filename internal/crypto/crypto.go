package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
)

var appKey []byte

func Init(configPath string) error {
	keyPath := filepath.Join(filepath.Dir(configPath), "key.dat")
	data, err := os.ReadFile(keyPath)
	if err == nil && len(data) == 32 {
		appKey = data
		return nil
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}

	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return err
	}

	appKey = key
	return nil
}

func Decrypt(encoded string) (string, error) {
	if appKey == nil {
		return "", errors.New("crypto not initialized")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(appKey)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
