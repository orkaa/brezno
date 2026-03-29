package system

import (
	"runtime"

	"golang.org/x/sys/unix"
)

// SecureBytes wraps a byte slice with automatic zeroing and memory locking to
// prevent sensitive data from remaining in memory or being swapped to disk.
type SecureBytes struct {
	data []byte
}

// NewSecureBytes creates a new SecureBytes instance from the given data.
// The provided byte slice is used directly (not copied), so the caller
// should not retain or modify it after passing it to this function.
// The memory is locked via mlock to prevent it from being swapped to disk.
func NewSecureBytes(data []byte) *SecureBytes {
	sb := &SecureBytes{data: data}

	if len(data) > 0 {
		// Lock memory to prevent the OS from swapping this page to disk.
		// Failure is tolerated — the tool still works, just without swap protection.
		unix.Mlock(data) //nolint:errcheck
	}

	// Finalizer as a safety net in case explicit Zeroize() is not called.
	runtime.SetFinalizer(sb, func(s *SecureBytes) {
		s.Zeroize()
	})

	return sb
}

// Bytes returns the underlying byte slice.
// The caller should not retain this slice or store it elsewhere.
func (s *SecureBytes) Bytes() []byte {
	if s == nil || s.data == nil {
		return nil
	}
	return s.data
}

// Zeroize explicitly zeros the underlying memory and unlocks it.
// Should be called via defer when the sensitive data is no longer needed.
func (s *SecureBytes) Zeroize() {
	if s == nil || s.data == nil {
		return
	}

	for i := range s.data {
		s.data[i] = 0
	}

	unix.Munlock(s.data) //nolint:errcheck
	s.data = nil
}

// Len returns the length of the underlying data.
func (s *SecureBytes) Len() int {
	if s == nil || s.data == nil {
		return 0
	}
	return len(s.data)
}
