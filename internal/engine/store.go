package engine

import (
	"sync"

	"github.com/GooferByte/kalpi/internal/models"
)

// ExecutionStore is the interface for persisting and querying execution results.
// The default implementation is in-memory; swap for a Redis or DB-backed version
// by satisfying this interface.
type ExecutionStore interface {
	// Save stores or overwrites an execution result.
	Save(result *models.ExecutionResult)
	// Get returns a result by execution ID.
	Get(id string) (*models.ExecutionResult, bool)
	// List returns all stored results.
	List() []*models.ExecutionResult
}

type inMemoryStore struct {
	mu      sync.RWMutex
	results map[string]*models.ExecutionResult
}

// NewInMemoryStore returns an in-memory ExecutionStore.
func NewInMemoryStore() ExecutionStore {
	return &inMemoryStore{
		results: make(map[string]*models.ExecutionResult),
	}
}

func (s *inMemoryStore) Save(r *models.ExecutionResult) {
	s.mu.Lock()
	s.results[r.ExecutionID] = r
	s.mu.Unlock()
}

func (s *inMemoryStore) Get(id string) (*models.ExecutionResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.results[id]
	return r, ok
}

func (s *inMemoryStore) List() []*models.ExecutionResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*models.ExecutionResult, 0, len(s.results))
	for _, r := range s.results {
		out = append(out, r)
	}
	return out
}
