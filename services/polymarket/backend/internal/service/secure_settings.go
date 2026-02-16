package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"slices"
	"strings"
)

const settingCryptoKeyEnv = "PM_SETTINGS_ENCRYPTION_KEY"
const settingCryptoPrevKeyEnv = "PM_SETTINGS_ENCRYPTION_PREV_KEY"

type encryptedSettingValue struct {
	Enc   string `json:"enc"`
	Nonce string `json:"nonce"`
	Data  string `json:"data"`
}

func ProtectSettingValue(key string, raw []byte) []byte {
	if !isSensitiveSettingKeyInternal(key) {
		return raw
	}
	gcm := loadPrimarySettingsGCM()
	if gcm == nil {
		return raw
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return raw
	}
	ct := gcm.Seal(nil, nonce, raw, []byte(strings.TrimSpace(strings.ToLower(key))))
	payload := encryptedSettingValue{
		Enc:   "aes-gcm-v1",
		Nonce: base64.StdEncoding.EncodeToString(nonce),
		Data:  base64.StdEncoding.EncodeToString(ct),
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return raw
	}
	return out
}

func RevealSettingValue(key string, raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}
	if !isSensitiveSettingKeyInternal(key) {
		return raw
	}
	var payload encryptedSettingValue
	if err := json.Unmarshal(raw, &payload); err != nil {
		return raw
	}
	if payload.Enc != "aes-gcm-v1" || payload.Nonce == "" || payload.Data == "" {
		return raw
	}
	nonce, err := base64.StdEncoding.DecodeString(payload.Nonce)
	if err != nil {
		return raw
	}
	ct, err := base64.StdEncoding.DecodeString(payload.Data)
	if err != nil {
		return raw
	}
	for _, gcm := range loadSettingsGCMs() {
		pt, err := gcm.Open(nil, nonce, ct, []byte(strings.TrimSpace(strings.ToLower(key))))
		if err == nil {
			return pt
		}
	}
	return raw
}

func ReencryptSensitiveValue(key string, raw []byte) ([]byte, bool) {
	if !isSensitiveSettingKeyInternal(key) {
		return raw, false
	}
	plain := RevealSettingValue(key, raw)
	encrypted := ProtectSettingValue(key, plain)
	if slices.Equal(encrypted, raw) {
		return raw, false
	}
	return encrypted, true
}

func loadPrimarySettingsGCM() cipher.AEAD {
	keyBytes := parseSettingsKey(strings.TrimSpace(os.Getenv(settingCryptoKeyEnv)))
	if len(keyBytes) == 0 {
		return nil
	}
	return newGCM(keyBytes)
}

func loadSettingsGCMs() []cipher.AEAD {
	keys := []string{
		strings.TrimSpace(os.Getenv(settingCryptoKeyEnv)),
		strings.TrimSpace(os.Getenv(settingCryptoPrevKeyEnv)),
	}
	out := make([]cipher.AEAD, 0, 2)
	seen := map[string]struct{}{}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keyBytes := parseSettingsKey(key)
		if len(keyBytes) == 0 {
			continue
		}
		if gcm := newGCM(keyBytes); gcm != nil {
			out = append(out, gcm)
		}
	}
	return out
}

func parseSettingsKey(k string) []byte {
	if strings.TrimSpace(k) == "" {
		return nil
	}
	// Prefer base64 key. fallback to raw bytes.
	keyBytes, err := base64.StdEncoding.DecodeString(k)
	if err != nil {
		keyBytes = []byte(k)
	}
	// Normalize key sizes accepted by AES.
	switch len(keyBytes) {
	case 16, 24, 32:
		// keep
	default:
		if len(keyBytes) < 16 {
			return nil
		}
		if len(keyBytes) < 24 {
			keyBytes = keyBytes[:16]
		} else if len(keyBytes) < 32 {
			keyBytes = keyBytes[:24]
		} else {
			keyBytes = keyBytes[:32]
		}
	}
	return keyBytes
}

func newGCM(keyBytes []byte) cipher.AEAD {
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil
	}
	return gcm
}

func isSensitiveSettingKeyInternal(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return false
	}
	markers := []string{"secret", "token", "password", "api_key", "private_key"}
	for _, m := range markers {
		if strings.Contains(k, m) {
			return true
		}
	}
	return false
}
