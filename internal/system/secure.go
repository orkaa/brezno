package system

import (
	"runtime"
)

// SecureBytes wraps a byte slice with automatic zeroing to prevent
// sensitive data from remaining in memory longer than necessary.
type SecureBytes struct {
	data []byte
}

// NewSecureBytes creates a new SecureBytes instance from the given data.
// The provided byte slice is used directly (not copied), so the caller
// should not retain or modify it after passing it to this function.
func NewSecureBytes(data []byte) *SecureBytes {
	sb := &SecureBytes{data: data}

	// Set up a finalizer to zero memory when the object is garbage collected
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

// Zeroize explicitly zeros the underlying memory.
// This should be called via defer when the sensitive data is no longer needed.
func (s *SecureBytes) Zeroize() {
	if s == nil || s.data == nil {
		return
	}

	// Zero out the memory
	for i := range s.data {
		s.data[i] = 0
	}

	// Clear the reference
	s.data = nil
}

// Len returns the length of the underlying data.
func (s *SecureBytes) Len() int {
	if s == nil || s.data == nil {
		return 0
	}
	return len(s.data)
}
