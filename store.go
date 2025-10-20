package orchestrator

import (
	"context"
	"sync"
)

// Store represents a contract for persistence
type Store interface {
	Save(ctx context.Context, data *Flow) error
	Load(ctx context.Context, idempotenceKey string) (*Flow, error)
}

// InMemoryStore is a simple store useful, not for production.
type InMemoryStore struct {
	mu    sync.RWMutex
	flows map[string]*Flow
}

// NewInMemoryStore creates new InMemoryStore instance
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		flows: map[string]*Flow{},
	}
}

// Save saves new Flow record or update pre-existing
func (s *InMemoryStore) Save(ctx context.Context, data *Flow) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flows[data.IdempotencyKey] = data
	return nil
}

// Load loads the existing Flow based specified idempotenceKey
func (s *InMemoryStore) Load(ctx context.Context, idempotenceKey string) (*Flow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.flows[idempotenceKey], nil
}
