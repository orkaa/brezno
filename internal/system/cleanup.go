package system

import (
	"fmt"
	"os"
	"os/signal"
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

// Execute runs all cleanup functions in reverse order (LIFO) and clears the stack.
// Safe to call multiple times — subsequent calls are no-ops.
func (s *CleanupStack) Execute() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error
	for i := len(s.cleanups) - 1; i >= 0; i-- {
		if err := s.cleanups[i](); err != nil {
			errs = append(errs, err)
		}
	}
	s.cleanups = nil

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

// HandleSignals starts a goroutine that runs cleanup and exits if any of the
// given signals are received. Call the returned cancel func (via defer) to
// stop listening once the operation completes successfully.
func (s *CleanupStack) HandleSignals(sigs ...os.Signal) (cancel func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sigs...)
	go func() {
		if _, ok := <-ch; ok {
			s.Execute()
			os.Exit(1)
		}
	}()
	return func() {
		signal.Stop(ch)
		close(ch)
	}
}

// Clear removes all cleanup functions (call on success to prevent cleanup)
func (s *CleanupStack) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanups = nil
}
