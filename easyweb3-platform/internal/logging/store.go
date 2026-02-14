package logging

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Store interface {
	Create(l OperationLog) error
	Get(id string) (OperationLog, bool, error)
	List(f ListFilter) ([]OperationLog, error)
	Stats(f ListFilter) (map[string]int, error)
}

type FileStore struct {
	path string
	mu   sync.Mutex
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (s *FileStore) Create(l OperationLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

func (s *FileStore) Get(id string) (OperationLog, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return OperationLog{}, false, nil
	}

	f, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return OperationLog{}, false, nil
		}
		return OperationLog{}, false, err
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 2<<20)
	for sc.Scan() {
		var l OperationLog
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			continue
		}
		if l.ID == id {
			return l, true, nil
		}
	}
	if err := sc.Err(); err != nil {
		return OperationLog{}, false, err
	}
	return OperationLog{}, false, nil
}

func (s *FileStore) List(flt ListFilter) ([]OperationLog, error) {
	if flt.Limit <= 0 {
		flt.Limit = 100
	}
	if flt.Limit > 1000 {
		flt.Limit = 1000
	}

	f, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	// We'll scan all logs and keep last N that match.
	buf := make([]OperationLog, 0, flt.Limit)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 2<<20)
	for sc.Scan() {
		var l OperationLog
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			continue
		}
		if !match(l, flt) {
			continue
		}
		if len(buf) == flt.Limit {
			copy(buf, buf[1:])
			buf[len(buf)-1] = l
			continue
		}
		buf = append(buf, l)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return buf, nil
}

func (s *FileStore) Stats(flt ListFilter) (map[string]int, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]int{}, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	m := map[string]int{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 2<<20)
	for sc.Scan() {
		var l OperationLog
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			continue
		}
		if !match(l, flt) {
			continue
		}
		m[l.Action]++
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return m, nil
}

func match(l OperationLog, f ListFilter) bool {
	if f.ProjectID != "" && l.ProjectID != f.ProjectID {
		return false
	}
	if f.Action != "" && l.Action != f.Action {
		return false
	}
	if f.Level != "" && l.Level != f.Level {
		return false
	}
	if f.From != nil && l.CreatedAt.Before(f.From.UTC()) {
		return false
	}
	if f.To != nil && l.CreatedAt.After(f.To.UTC()) {
		return false
	}
	return true
}

func NewLogID(now time.Time, n int64) string {
	// Stable-enough for MVP; can switch to uuid later.
	return fmt.Sprintf("log_%d_%d", now.Unix(), n)
}
