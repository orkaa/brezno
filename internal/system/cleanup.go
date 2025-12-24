package system

import (
	"fmt"
	"sync"
)

// CleanupStack manages cleanup operations in reverse order (LIFO)
// This mimics bash trap cleanup behavior
type CleanupStack struct {
	cleanups []func() error
	mu       sync.Mutex
}

// NewCleanupStack creates a new cleanup stack
func NewCleanupStack() *CleanupStack {
	return &CleanupStack{
		cleanups: make([]func() error, 0),
	}
}

// Add adds a cleanup function to the stack
func (s *CleanupStack) Add(cleanup func() error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanups = append(s.cleanups, cleanup)
}

// Execute runs all cleanup functions in reverse order (LIFO)
func (s *CleanupStack) Execute() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error
	// Execute in reverse order
	for i := len(s.cleanups) - 1; i >= 0; i-- {
		if err := s.cleanups[i](); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

// Clear removes all cleanup functions (call on success to prevent cleanup)
func (s *CleanupStack) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanups = nil
}
