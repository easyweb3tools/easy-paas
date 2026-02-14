package auth

import (
	"crypto/rand"
	"crypto/sha256"
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

type UserRecord struct {
	ID           string            `json:"id"`
	Username     string            `json:"username"`
	SaltHex      string            `json:"salt_hex"`
	PassHashHex  string            `json:"pass_hash_hex"`
	Grants       map[string]string `json:"grants,omitempty"` // project_id -> role
	Disabled     bool              `json:"disabled,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	LastLoginAt  *time.Time        `json:"last_login_at,omitempty"`
	LastGrantAt  *time.Time        `json:"last_grant_at,omitempty"`
	LastUpdatedAt *time.Time       `json:"last_updated_at,omitempty"`
}

type FileUserStore struct {
	path string

	mu    sync.RWMutex
	users []UserRecord
}

func NewFileUserStore(path string) *FileUserStore {
	return &FileUserStore{path: path}
}

func (s *FileUserStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.users = nil
			return nil
		}
		return err
	}
	var users []UserRecord
	if err := json.Unmarshal(b, &users); err != nil {
		return err
	}
	s.users = users
	return nil
}

func (s *FileUserStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.users, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o600)
}

func (s *FileUserStore) Create(username, password string) (UserRecord, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return UserRecord{}, errors.New("username required")
	}
	if len(username) < 3 || len(username) > 64 {
		return UserRecord{}, errors.New("username length must be 3..64")
	}
	if strings.ContainsAny(username, " \t\r\n") {
		return UserRecord{}, errors.New("username must not contain whitespace")
	}
	if len(password) < 8 || len(password) > 256 {
		return UserRecord{}, errors.New("password length must be 8..256")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if strings.EqualFold(u.Username, username) {
			return UserRecord{}, errors.New("username already exists")
		}
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return UserRecord{}, err
	}
	passHash := saltedHash(salt, []byte(password))

	now := time.Now().UTC()
	rec := UserRecord{
		ID:          fmt.Sprintf("user_%d", now.UnixNano()),
		Username:    username,
		SaltHex:     hex.EncodeToString(salt),
		PassHashHex: hex.EncodeToString(passHash),
		Grants:      map[string]string{},
		CreatedAt:   now,
	}
	s.users = append(s.users, rec)
	return rec, nil
}

func (s *FileUserStore) Authenticate(username, password, projectID string) (UserRecord, string, error) {
	username = strings.TrimSpace(username)
	projectID = strings.TrimSpace(projectID)
	if username == "" || password == "" {
		return UserRecord{}, "", errors.New("username and password required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.users {
		u := &s.users[i]
		if !strings.EqualFold(u.Username, username) {
			continue
		}
		if u.Disabled {
			return UserRecord{}, "", errors.New("user disabled")
		}
		salt, err := hex.DecodeString(u.SaltHex)
		if err != nil || len(salt) == 0 {
			return UserRecord{}, "", errors.New("invalid user record")
		}
		want, err := hex.DecodeString(u.PassHashHex)
		if err != nil || len(want) == 0 {
			return UserRecord{}, "", errors.New("invalid user record")
		}
		got := saltedHash(salt, []byte(password))
		if !subtleEq(hex.EncodeToString(want), hex.EncodeToString(got)) {
			return UserRecord{}, "", errors.New("invalid credentials")
		}

		role := ""
		if projectID != "" {
			role = u.Grants[projectID]
		} else if len(u.Grants) == 1 {
			for _, r := range u.Grants {
				role = r
			}
			for pid := range u.Grants {
				projectID = pid
			}
		}

		// Update last login timestamp (best effort).
		now := time.Now().UTC()
		u.LastLoginAt = &now
		u.LastUpdatedAt = &now
		_ = s.saveLocked()

		if role == "" || projectID == "" {
			return *u, "", errors.New("no grants")
		}

		return *u, role, nil
	}
	return UserRecord{}, "", errors.New("invalid credentials")
}

func (s *FileUserStore) Grant(userIDOrUsername, projectID, role string) (UserRecord, error) {
	userIDOrUsername = strings.TrimSpace(userIDOrUsername)
	projectID = strings.TrimSpace(projectID)
	role = strings.TrimSpace(role)
	if userIDOrUsername == "" {
		return UserRecord{}, errors.New("user_id or username required")
	}
	if projectID == "" {
		return UserRecord{}, errors.New("project_id required")
	}
	if role == "" {
		return UserRecord{}, errors.New("role required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.users {
		u := &s.users[i]
		if u.ID != userIDOrUsername && !strings.EqualFold(u.Username, userIDOrUsername) {
			continue
		}
		if u.Grants == nil {
			u.Grants = map[string]string{}
		}
		u.Grants[projectID] = role
		now := time.Now().UTC()
		u.LastGrantAt = &now
		u.LastUpdatedAt = &now
		if err := s.saveLocked(); err != nil {
			return UserRecord{}, err
		}
		return *u, nil
	}
	return UserRecord{}, errors.New("user not found")
}

func (s *FileUserStore) List() []UserRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]UserRecord, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	return out
}

func (s *FileUserStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.users, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o600)
}

func saltedHash(salt, password []byte) []byte {
	h := sha256.New()
	_, _ = h.Write(salt)
	_, _ = h.Write(password)
	sum := h.Sum(nil)
	return sum
}

