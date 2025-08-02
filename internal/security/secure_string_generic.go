//go:build !darwin && !windows && !linux
// +build !darwin,!windows,!linux

// internal/security/secure_string_generic.go
package security

import (
	"crypto/rand"
)

// Platform-generic memory locking implementation (fallback)
func (s *SecureString) lockMemory() error {
	// Generic platforms don't have memory locking capabilities
	// This is a no-op but we track the intent
	s.locked = false
	return nil
}

func (s *SecureString) unlockMemory() error {
	// Generic platforms don't have memory locking capabilities
	s.locked = false
	return nil
}

// SecureClearBytes securely clears sensitive data from a byte slice using multiple pass overwriting
func SecureClearBytes(data []byte) {
	secureZero(data)
}

// secureZero overwrites memory with zeros multiple times for enhanced security
func secureZero(data []byte) {
	if len(data) == 0 {
		return
	}
	
	// Multiple pass overwriting for enhanced security
	// Pass 1: Random data
	rand.Read(data)
	
	// Pass 2: All ones
	for i := range data {
		data[i] = 0xFF
	}
	
	// Pass 3: All zeros
	for i := range data {
		data[i] = 0x00
	}
	
	// Pass 4: Random data again
	rand.Read(data)
	
	// Pass 5: Final zero
	for i := range data {
		data[i] = 0x00
	}
}

// getPageSize returns a default page size for generic platforms
func getPageSize() int {
	return 4096 // Standard page size fallback
}


