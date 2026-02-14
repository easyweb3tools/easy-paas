package notification

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type FileStore struct {
	path string
	mu   sync.RWMutex
	cfgs map[string]ProjectConfig
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path, cfgs: map[string]ProjectConfig{}}
}

func (s *FileStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.cfgs = map[string]ProjectConfig{}
			return nil
		}
		return err
	}
	var cfgs map[string]ProjectConfig
	if err := json.Unmarshal(b, &cfgs); err != nil {
		return err
	}
	if cfgs == nil {
		cfgs = map[string]ProjectConfig{}
	}
	s.cfgs = cfgs
	return nil
}

func (s *FileStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.cfgs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o600)
}

func (s *FileStore) Get(project string) (ProjectConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.cfgs[project]
	return cfg, ok
}

func (s *FileStore) Put(project string, cfg ProjectConfig) error {
	s.mu.Lock()
	s.cfgs[project] = cfg
	s.mu.Unlock()
	return s.Save()
}
