// internal/tui/utils/security_utils.go
package utils

import (
	"fmt"     // ❌ Добавлен недостающий импорт
	"strings" // ❌ Добавлен недостающий импорт
	"sync"
	"time"
)

// SecurityManager manages security-related functionality
type SecurityManager struct {
	mu                sync.RWMutex
	sessionTimeout    time.Duration
	lastActivity      time.Time
	isLocked          bool
	failedAttempts    int
	maxFailedAttempts int
	lockoutDuration   time.Duration
	lockoutUntil      time.Time
}

var (
	securityManager *SecurityManager
	securityOnce    sync.Once
)

// GetSecurityManager returns the singleton security manager
func GetSecurityManager() *SecurityManager {
	securityOnce.Do(func() {
		securityManager = &SecurityManager{
			sessionTimeout:    30 * time.Minute,
			lastActivity:      time.Now(),
			isLocked:          false,
			maxFailedAttempts: 3,
			lockoutDuration:   5 * time.Minute,
		}
	})
	return securityManager
}

// UpdateActivity updates the last activity timestamp
func (sm *SecurityManager) UpdateActivity() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lastActivity = time.Now()
}

// IsSessionExpired checks if the session has expired
func (sm *SecurityManager) IsSessionExpired() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return time.Since(sm.lastActivity) > sm.sessionTimeout
}

// IsLocked checks if the system is locked due to failed attempts
func (sm *SecurityManager) IsLocked() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.isLocked && time.Now().After(sm.lockoutUntil) {
		sm.mu.RUnlock()
		sm.mu.Lock()
		sm.isLocked = false
		sm.failedAttempts = 0
		sm.mu.Unlock()
		sm.mu.RLock()
	}

	return sm.isLocked
}

// RecordFailedAttempt records a failed authentication attempt
func (sm *SecurityManager) RecordFailedAttempt() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.failedAttempts++
	if sm.failedAttempts >= sm.maxFailedAttempts {
		sm.isLocked = true
		sm.lockoutUntil = time.Now().Add(sm.lockoutDuration)
	}
}

// RecordSuccessfulAuth records a successful authentication
func (sm *SecurityManager) RecordSuccessfulAuth() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.failedAttempts = 0
	sm.isLocked = false
	sm.lastActivity = time.Now()
}

// GetLockoutTimeRemaining returns the remaining lockout time
func (sm *SecurityManager) GetLockoutTimeRemaining() time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.isLocked {
		return 0
	}

	remaining := time.Until(sm.lockoutUntil)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// SetSessionTimeout sets the session timeout duration
func (sm *SecurityManager) SetSessionTimeout(timeout time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessionTimeout = timeout
}

// GetFailedAttempts returns the number of failed attempts
func (sm *SecurityManager) GetFailedAttempts() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.failedAttempts
}

// ClearSession clears the current session
func (sm *SecurityManager) ClearSession() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.lastActivity = time.Time{}
	sm.failedAttempts = 0
	sm.isLocked = false
}

// SensitiveDataMask masks sensitive data for display
func SensitiveDataMask(data string, visibleChars int) string {
	if len(data) <= visibleChars*2 {
		return strings.Repeat("*", len(data))
	}

	prefix := data[:visibleChars]
	suffix := data[len(data)-visibleChars:]
	middle := strings.Repeat("*", len(data)-visibleChars*2)

	return prefix + middle + suffix
}

// ValidateInput validates user input for security
func ValidateInput(input string, inputType string) error {
	switch inputType {
	case "vault_name":
		return validateVaultName(input)
	case "wallet_prefix":
		return validateWalletPrefix(input)
	case "derivation_path":
		return validateDerivationPath(input)
	case "file_path":
		return validateFilePath(input)
	default:
		return nil
	}
}

// validateVaultName validates vault name
func validateVaultName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("vault name cannot be empty")
	}
	if len(name) > 50 {
		return fmt.Errorf("vault name too long (max 50 characters)")
	}
	// Check for invalid characters
	for _, char := range name {
		if !isValidNameChar(char) {
			return fmt.Errorf("vault name contains invalid characters")
		}
	}
	return nil
}

// validateWalletPrefix validates wallet prefix
func validateWalletPrefix(prefix string) error {
	if len(prefix) == 0 {
		return fmt.Errorf("wallet prefix cannot be empty")
	}
	if len(prefix) > 30 {
		return fmt.Errorf("wallet prefix too long (max 30 characters)")
	}
	// Check for invalid characters
	for _, char := range prefix {
		if !isValidNameChar(char) {
			return fmt.Errorf("wallet prefix contains invalid characters")
		}
	}
	return nil
}

// validateDerivationPath validates BIP32 derivation path
func validateDerivationPath(path string) error {
	if len(path) == 0 {
		return fmt.Errorf("derivation path cannot be empty")
	}
	if !strings.HasPrefix(path, "m/") {
		return fmt.Errorf("derivation path must start with 'm/'")
	}
	// TODO: Add more comprehensive BIP32 path validation
	return nil
}

// validateFilePath validates file path
func validateFilePath(path string) error {
	if len(path) == 0 {
		return fmt.Errorf("file path cannot be empty")
	}
	if len(path) > 255 {
		return fmt.Errorf("file path too long")
	}
	// TODO: Add more comprehensive file path validation
	return nil
}

// isValidNameChar checks if character is valid for names
func isValidNameChar(char rune) bool {
	return (char >= 'a' && char <= 'z') ||
		(char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9') ||
		char == '-' || char == '_'
}
