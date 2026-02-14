package cache

import (
	"context"
	"sync"
	"time"
)

type memItem struct {
	v       []byte
	expires time.Time
	noexp   bool
}

type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]memItem
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: map[string]memItem{}}
}

func (s *MemoryStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	_ = ctx
	s.mu.RLock()
	it, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if !it.noexp && !it.expires.IsZero() && time.Now().After(it.expires) {
		s.mu.Lock()
		delete(s.items, key)
		s.mu.Unlock()
		return nil, false, nil
	}
	out := make([]byte, len(it.v))
	copy(out, it.v)
	return out, true, nil
}

func (s *MemoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	_ = ctx
	it := memItem{v: clone(value)}
	if ttl <= 0 {
		it.noexp = true
	} else {
		it.expires = time.Now().Add(ttl)
	}
	s.mu.Lock()
	s.items[key] = it
	s.mu.Unlock()
	return nil
}

func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	_ = ctx
	s.mu.Lock()
	delete(s.items, key)
	s.mu.Unlock()
	return nil
}

func clone(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}
