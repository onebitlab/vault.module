// internal/security/secure_string.go - Полная интегрированная версия
package security

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// SecureZero is the public version of secureZero for external use
func SecureZero(data []byte) {
	secureZero(data)
}

// SecureString provides a secure way to store sensitive strings in memory
// with XOR encryption and platform-specific memory locking
type SecureString struct {
	data         []byte      // XOR encrypted data
	pad          []byte      // XOR pad for encryption
	locked       bool        // Track if memory is locked
	cleared      bool        // Track if already cleared
	mu           sync.RWMutex // Protect concurrent access
	description  string      // Description for cleanup tracking
	registeredForCleanup bool // Track if registered with shutdown manager
}

// NewSecureString creates a new SecureString with the given value
func NewSecureString(value string) *SecureString {
	if value == "" {
		return &SecureString{cleared: false}
	}
	
	// Check for oversized strings to prevent memory exhaustion
	if len(value) > 1024*1024 { // 1MB limit
		panic("SecureString: value too large")
	}
	
	data := []byte(value)
	pad := make([]byte, len(data))
	
	// Generate cryptographically secure random pad
	if _, err := rand.Read(pad); err != nil {
		panic(fmt.Sprintf("CRITICAL: failed to get random data for SecureString: %v", err))
	}
	
	// XOR encrypt the data
	encrypted := make([]byte, len(data))
	for i := range data {
		encrypted[i] = data[i] ^ pad[i]
	}
	
	// Securely clear the original data
	secureZero(data)
	
	s := &SecureString{
		data:    encrypted,
		pad:     pad,
		cleared: false,
	}
	
	// Lock memory AFTER data is ready but BEFORE storing sensitive data
	if err := s.lockMemory(); err != nil {
		// If locking fails, continue but log warning (implement logging later)
		// In production, you might want to fail here for maximum security
	}
	
	return s
}

// NewSecureStringWithRegistration creates a new SecureString and registers it for cleanup
func NewSecureStringWithRegistration(value string, description string) *SecureString {
	s := NewSecureString(value)
	if s != nil && !s.IsEmpty() {
		s.RegisterForCleanup(description)
	}
	return s
}

// RegisterForCleanup registers this SecureString with the resource manager
func (s *SecureString) RegisterForCleanup(description string) {
	if s != nil && !s.IsEmpty() && !s.registeredForCleanup {
		s.mu.Lock()
		s.description = description
		s.registeredForCleanup = true
		s.mu.Unlock()
		
		// Use dependency injection to register with resource manager
		if manager := GetResourceManager(); manager != nil {
			manager.RegisterSecureString(s, description)
		}
	}
}

// UnregisterFromCleanup removes this SecureString from the resource manager
func (s *SecureString) UnregisterFromCleanup() {
	if s != nil && s.registeredForCleanup {
		s.mu.Lock()
		s.registeredForCleanup = false
		s.mu.Unlock()
		
		// Use dependency injection to unregister from resource manager
		if manager := GetResourceManager(); manager != nil {
			manager.UnregisterSecureString(s)
		}
	}
}

// String returns the decrypted string value
// Creates a temporary copy that is automatically cleared
func (s *SecureString) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.cleared || s.data == nil || s.pad == nil {
		return ""
	}
	
	// Decrypt XOR data into temporary buffer
	decrypted := make([]byte, len(s.data))
	for i := range s.data {
		decrypted[i] = s.data[i] ^ s.pad[i]
	}
	
	// Convert to string
	result := string(decrypted)
	
	// Immediately clear temporary buffer
	secureZero(decrypted)
	
	return result
}

// WithValue safely executes a function with the decrypted value
// This prevents the string from remaining in memory longer than necessary
func (s *SecureString) WithValue(fn func(string) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.cleared || s.data == nil || s.pad == nil {
		return fn("")
	}
	
	// Decrypt XOR data into temporary buffer
	decrypted := make([]byte, len(s.data))
	defer secureZero(decrypted) // Ensure cleanup
	
	for i := range s.data {
		decrypted[i] = s.data[i] ^ s.pad[i]
	}
	
	// Execute function with string value
	return fn(string(decrypted))
}

// WithSecureOperation executes an operation with temporary access to the data
// and guarantees cleanup of temporary data
func (s *SecureString) WithSecureOperation(fn func([]byte) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.cleared || s.data == nil || s.pad == nil {
		return fn(nil)
	}
	
	// Decrypt to temporary buffer
	decrypted := make([]byte, len(s.data))
	defer secureZero(decrypted)
	
	for i := range s.data {
		decrypted[i] = s.data[i] ^ s.pad[i]
	}
	
	return fn(decrypted)
}

// GetHint returns a safe hint of the stored value (first and last 3 characters)
func (s *SecureString) GetHint() string {
	return s.WithValueSync(func(fullStr string) string {
		if len(fullStr) >= 6 {
			return fmt.Sprintf("%s...%s", fullStr[:3], fullStr[len(fullStr)-3:])
		} else if len(fullStr) > 0 {
			return fullStr
		}
		return ""
	})
}

// WithValueSync safely executes a function with the decrypted value and returns result
func (s *SecureString) WithValueSync(fn func(string) string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.cleared || s.data == nil || s.pad == nil {
		return fn("")
	}
	
	// Decrypt XOR data into temporary buffer
	decrypted := make([]byte, len(s.data))
	defer secureZero(decrypted) // Ensure cleanup
	
	for i := range s.data {
		decrypted[i] = s.data[i] ^ s.pad[i]
	}
	
	// Execute function with string value
	return fn(string(decrypted))
}

// MarshalJSON safely marshals the SecureString to JSON
func (s *SecureString) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.cleared || s.data == nil || s.pad == nil {
		return json.Marshal("")
	}
	
	// Use WithValue pattern to minimize exposure time
	var result []byte
	var err error
	
	// Decrypt into temporary buffer
	decrypted := make([]byte, len(s.data))
	defer secureZero(decrypted) // Ensure cleanup
	
	for i := range s.data {
		decrypted[i] = s.data[i] ^ s.pad[i]
	}
	
	// Marshal to JSON
	result, err = json.Marshal(string(decrypted))
	
	return result, err
}

// UnmarshalJSON safely unmarshals JSON into SecureString
func (s *SecureString) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Clear existing data first
	s.clearUnsafe()
	
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	
	if str == "" {
		s.cleared = false
		return nil
	}
	
	// Create new encrypted data
	dataBytes := []byte(str)
	pad := make([]byte, len(dataBytes))
	
	// Generate cryptographically secure random pad
	if _, err := rand.Read(pad); err != nil {
		panic(fmt.Sprintf("CRITICAL: failed to get random data for SecureString: %v", err))
	}
	
	// XOR encrypt the data
	encrypted := make([]byte, len(dataBytes))
	for i := range dataBytes {
		encrypted[i] = dataBytes[i] ^ pad[i]
	}
	
	// Securely clear the original data
	secureZero(dataBytes)
	
	s.data = encrypted
	s.pad = pad
	s.cleared = false
	
	// Lock the new memory
	if err := s.lockMemory(); err != nil {
		// Continue but note the error
	}
	
	return nil
}

// Clear securely clears all sensitive data from memory
func (s *SecureString) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearUnsafe()
}

// clearUnsafe performs the actual clearing without locking (internal use)
func (s *SecureString) clearUnsafe() {
	if s.cleared {
		return
	}
	
	// Unregister from cleanup first
	if s.registeredForCleanup {
		s.registeredForCleanup = false
		// Note: We don't call UnregisterFromCleanup here to avoid deadlock
		// The shutdown manager will handle dangling references gracefully
	}
	
	// Unlock memory before clearing
	s.unlockMemory()
	
	// Securely overwrite data multiple times
	if s.data != nil {
		secureZero(s.data)
		s.data = nil
	}
	
	if s.pad != nil {
		secureZero(s.pad)
		s.pad = nil
	}
	
	s.cleared = true
	s.locked = false
}

// IsCleared returns true if the SecureString has been cleared
func (s *SecureString) IsCleared() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cleared
}

// Len returns the length of the stored string without decrypting it
func (s *SecureString) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.cleared || s.data == nil {
		return 0
	}
	return len(s.data)
}

// IsEmpty returns true if the SecureString is empty or cleared
func (s *SecureString) IsEmpty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cleared || s.data == nil || len(s.data) == 0
}

// Clone creates a deep copy of the SecureString
func (s *SecureString) Clone() *SecureString {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.cleared || s.data == nil || s.pad == nil {
		return &SecureString{cleared: false}
	}
	
	// Create new SecureString with same decrypted value
	decrypted := make([]byte, len(s.data))
	defer secureZero(decrypted)
	
	for i := range s.data {
		decrypted[i] = s.data[i] ^ s.pad[i]
	}
	
	return NewSecureString(string(decrypted))
}

// GetDescription returns the description used for cleanup tracking
func (s *SecureString) GetDescription() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.description
}

// IsRegisteredForCleanup returns true if registered with shutdown manager
func (s *SecureString) IsRegisteredForCleanup() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.registeredForCleanup
}
