//go:build !darwin
// +build !darwin

package security

import (
	"crypto/rand"
	mathrand "math/rand"
	"time"
)

// secureZero securely overwrites memory with zeros (and random data for added security)
func secureZero(data []byte) {
	if len(data) == 0 {
		return
	}
	
	// Multiple passes for secure deletion
	// Pass 1: All zeros
	for i := range data {
		data[i] = 0
	}
	
	// Pass 2: All ones
	for i := range data {
		data[i] = 0xFF
	}
	
	// Pass 3: Random data
	if _, err := rand.Read(data); err != nil {
		// Fallback to math/rand
		r := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
		for i := range data {
			data[i] = byte(r.Intn(256))
		}
	}
	
	// Pass 4: Final zeros
	for i := range data {
		data[i] = 0
	}
}

// Platform-agnostic memory operations (no-op on non-Darwin platforms)
func (s *SecureString) lockMemory() error {
	// No platform-specific memory locking available
	s.locked = false
	return nil
}

func (s *SecureString) unlockMemory() error {
	// No platform-specific memory unlocking needed
	s.locked = false
	return nil
}

// SecureClearBytes securely clears sensitive data from a byte slice
func SecureClearBytes(data []byte) {
	secureZero(data)
}

// getPageSize returns a default page size for non-Darwin platforms
func getPageSize() int {
	return 4096 // Default page size
}
