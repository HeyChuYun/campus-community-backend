package repository

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// EncryptString 使用 AES-GCM 加密字符串。
func EncryptString(keyText, plainText string) (string, error) {
	key := normalizeKey(keyText)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	cipherText := gcm.Seal(nil, nonce, []byte(plainText), nil)
	payload := append(nonce, cipherText...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

// DecryptString 使用 AES-GCM 解密字符串。
func DecryptString(keyText, cipherText string) (string, error) {
	key := normalizeKey(keyText)
	raw, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("cipher text too short")
	}

	nonce := raw[:gcm.NonceSize()]
	data := raw[gcm.NonceSize():]
	plainText, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", err
	}
	return string(plainText), nil
}

func normalizeKey(keyText string) []byte {
	key := make([]byte, 32)
	copy(key, []byte(keyText))
	return key
}
