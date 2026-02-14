package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type KeyRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ProjectID string    `json:"project_id"`
	Role      string    `json:"role"`
	HashHex   string    `json:"hash_hex"`
	CreatedAt time.Time `json:"created_at"`
}

type FileKeyStore struct {
	path string

	mu   sync.RWMutex
	keys []KeyRecord

	bootstrapAdminKey string
}

func NewFileKeyStore(path string, bootstrapAdminKey string) *FileKeyStore {
	return &FileKeyStore{path: path, bootstrapAdminKey: strings.TrimSpace(bootstrapAdminKey)}
}

func (s *FileKeyStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.keys = nil
			return nil
		}
		return err
	}
	var keys []KeyRecord
	if err := json.Unmarshal(b, &keys); err != nil {
		return err
	}
	s.keys = keys
	return nil
}

func (s *FileKeyStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.keys, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o600)
}

func (s *FileKeyStore) Validate(apiKey string) (KeyRecord, bool) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return KeyRecord{}, false
	}

	// Bootstrap admin key via env, so first-run is possible.
	if s.bootstrapAdminKey != "" && apiKey == s.bootstrapAdminKey {
		return KeyRecord{
			ID:        "bootstrap_admin",
			Name:      "bootstrap",
			ProjectID: "platform",
			Role:      "admin",
			HashHex:   "",
			CreatedAt: time.Time{},
		}, true
	}

	h := hashKey(apiKey)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, k := range s.keys {
		if subtleEq(k.HashHex, h) {
			return k, true
		}
	}
	return KeyRecord{}, false
}

func (s *FileKeyStore) Create(projectID, role, name string) (rawKey string, rec KeyRecord, err error) {
	projectID = strings.TrimSpace(projectID)
	role = strings.TrimSpace(role)
	name = strings.TrimSpace(name)
	if projectID == "" {
		return "", KeyRecord{}, errors.New("project_id required")
	}
	if role == "" {
		role = "agent"
	}

	rawKey, err = newAPIKey()
	if err != nil {
		return "", KeyRecord{}, err
	}

	now := time.Now().UTC()
	rec = KeyRecord{
		ID:        fmt.Sprintf("key_%d", now.UnixNano()),
		Name:      name,
		ProjectID: projectID,
		Role:      role,
		HashHex:   hashKey(rawKey),
		CreatedAt: now,
	}

	s.mu.Lock()
	s.keys = append(s.keys, rec)
	s.mu.Unlock()

	if err := s.Save(); err != nil {
		return "", KeyRecord{}, err
	}
	return rawKey, rec, nil
}

func newAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// base32 without padding so it's shell-friendly.
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	return "ew3_" + strings.ToLower(enc.EncodeToString(b)), nil
}

func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func subtleEq(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
