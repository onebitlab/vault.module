//go:build darwin
// +build darwin

package security

import (
	"crypto/rand"
	"syscall"
)

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

// Platform-specific memory locking implementation for macOS
func (s *SecureString) lockMemory() error {
	if len(s.data) == 0 {
		return nil
	}

	// Lock data pages in memory to prevent swapping
	if err := syscall.Mlock(s.data); err != nil {
		return err
	}

	if len(s.pad) > 0 {
		if err := syscall.Mlock(s.pad); err != nil {
			// If locking pad fails, unlock data and return error
			syscall.Munlock(s.data)
			return err
		}
	}

	s.locked = true
	return nil
}

func (s *SecureString) unlockMemory() error {
	if !s.locked {
		return nil
	}

	if len(s.data) > 0 {
		if err := syscall.Munlock(s.data); err != nil {
			return err
		}
	}

	if len(s.pad) > 0 {
		if err := syscall.Munlock(s.pad); err != nil {
			return err
		}
	}

	s.locked = false
	return nil
}

// SecureClearBytes securely clears sensitive data from a byte slice using multiple pass overwriting
func SecureClearBytes(data []byte) {
	secureZero(data)
}
