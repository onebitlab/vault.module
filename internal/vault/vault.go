package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
	"vault.module/internal/audit"
	"vault.module/internal/config"
	"vault.module/internal/constants"
	"vault.module/internal/errors"
	"vault.module/internal/security"
)

const (
	CurrentVaultVersion = 1
)

// secureBufferWriter is a custom writer that accumulates data into a SecureString
// for secure handling of decrypted vault data
type secureBufferWriter struct {
	buffer *security.SecureString
}

// Write implements io.Writer interface for secure data collection
func (w *secureBufferWriter) Write(p []byte) (n int, err error) {
	if w.buffer == nil {
		return 0, fmt.Errorf("secureBufferWriter: buffer is nil")
	}
	if err := w.buffer.AppendData(p); err != nil {
		return 0, fmt.Errorf("secureBufferWriter: failed to append data: %v", err)
	}
	return len(p), nil
}

// VaultHeader with version support for future migrations
type VaultHeader struct {
	Version int   `json:"version"`
	Data    Vault `json:"data"`
}

// Address defines the structure for a single address.
type Address struct {
	Index      int                    `json:"index"`
	Path       string                 `json:"path"`
	Address    string                 `json:"address"`
	PrivateKey *security.SecureString `json:"privateKey"`
}

// Wallet defines the structure for a wallet, which can be HD or a single key.
type Wallet struct {
	Mnemonic       *security.SecureString `json:"mnemonic,omitempty"`
	DerivationPath string                 `json:"derivationPath,omitempty"`
	Addresses      []Address              `json:"addresses"`
	Notes          string                 `json:"notes"`
}

// Vault is the root structure of our vault (the JSON file).
type Vault map[string]Wallet

// Sanitize creates a "clean" copy of the wallet for safe display.
func (w Wallet) Sanitize() Wallet {
	sanitizedWallet := w
	if sanitizedWallet.Mnemonic != nil && sanitizedWallet.Mnemonic.String() != "" {
		sanitizedWallet.Mnemonic = security.NewSecureString("[REDACTED]")
	}

	sanitizedAddresses := make([]Address, len(w.Addresses))
	for i, addr := range w.Addresses {
		sanitizedAddresses[i] = addr
		sanitizedAddresses[i].PrivateKey = security.NewSecureString("[REDACTED]")
	}
	sanitizedWallet.Addresses = sanitizedAddresses
	return sanitizedWallet
}

// Clear clears all secrets from the wallet.
func (w *Wallet) Clear() {
	if w.Mnemonic != nil {
		w.Mnemonic.Clear()
		w.Mnemonic = nil
	}
	for i := range w.Addresses {
		if w.Addresses[i].PrivateKey != nil {
			w.Addresses[i].PrivateKey.Clear()
			w.Addresses[i].PrivateKey = nil
		}
	}
}

// GetMnemonicHint returns a safe hint of the mnemonic (first and last word)
func (w *Wallet) GetMnemonicHint() string {
	if w.Mnemonic == nil {
		return ""
	}

	// Use WithValueSync to safely access mnemonic
	return w.Mnemonic.WithValueSync(func(mnemonicStr string) string {
		if mnemonicStr == "" {
			return ""
		}

		// Split into words for mnemonic
		words := strings.Fields(mnemonicStr)
		if len(words) >= 2 {
			return fmt.Sprintf("%s...%s", words[0], words[len(words)-1])
		}
		return "mnemonic"
	})
}

// New creates an empty vault.
func New() Vault {
	return make(Vault)
}

// detectVaultFormat attempts to detect if data is versioned or legacy format
func detectVaultFormat(data []byte) (bool, error) {
	// Try to unmarshal as VaultHeader first
	var header VaultHeader
	if err := json.Unmarshal(data, &header); err == nil {
		// Check if it has version field and valid structure
		if header.Version > 0 && header.Data != nil {
			return true, nil // Versioned format
		}
	}

	// Try to unmarshal as legacy Vault format
	var legacyVault Vault
	if err := json.Unmarshal(data, &legacyVault); err == nil {
		return false, nil // Legacy format
	}

	return false, errors.New(errors.ErrCodeInternal, "unable to detect vault format")
}

// validateVaultVersion checks if vault version is supported
func validateVaultVersion(version int) error {
	if version < 1 {
		return errors.New(errors.ErrCodeFormatInvalid, "invalid vault version: must be >= 1")
	}
	if version > CurrentVaultVersion {
		return errors.New(errors.ErrCodeFormatInvalid,
			fmt.Sprintf("unsupported vault version %d (current max: %d) - please update your vault client",
				version, CurrentVaultVersion))
	}
	return nil
}

// isProcessRunning checks if a process with given PID is still running
// Uses more robust process existence checking with proper error handling
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false // Invalid PID
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix systems, signal 0 can be used to check if process exists
	// This is the standard way to check process existence without affecting it
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Check specific error types to distinguish between permission and non-existence
		if errno, ok := err.(syscall.Errno); ok {
			// ESRCH means no such process
			// EPERM means process exists but we don't have permission to signal it
			return errno != syscall.ESRCH
		}
		return false
	}
	return true
}

// cleanupStaleLock removes lock file if the process that created it is no longer running
// Enhanced with better validation and atomic operations
func cleanupStaleLock(lockFileName string) error {
	// Use O_RDONLY to avoid race conditions with concurrent cleanup attempts
	lockFile, err := os.OpenFile(lockFileName, os.O_RDONLY, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Lock file doesn't exist, nothing to clean
		}
		return err
	}
	defer lockFile.Close()

	// Try to acquire a shared lock to ensure we're not interfering with active lock holder
	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_SH|unix.LOCK_NB); err != nil {
		// Lock is actively held by another process, don't clean it up
		audit.Logger.Debug("Lock file is actively held, not cleaning up",
			slog.String("lock_file", lockFileName))
		return nil
	}
	defer unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)

	// Read PID from the lock file
	data, err := os.ReadFile(lockFileName)
	if err != nil {
		return err
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		// Invalid PID format, assume stale and remove
		audit.Logger.Warn("Found lock file with invalid PID format, removing",
			slog.String("lock_file", lockFileName),
			slog.String("content", pidStr))
		return os.Remove(lockFileName)
	}

	// Validate PID range
	if pid <= 0 || pid > 4194304 { // Max PID on most systems
		audit.Logger.Warn("Found lock file with invalid PID range, removing",
			slog.String("lock_file", lockFileName),
			slog.Int("invalid_pid", pid))
		return os.Remove(lockFileName)
	}

	// Check if process is still running
	if !isProcessRunning(pid) {
		audit.Logger.Info("Found stale lock file, removing",
			slog.String("lock_file", lockFileName),
			slog.Int("stale_pid", pid))
		return os.Remove(lockFileName)
	}

	return nil // Lock is still valid
}

// createLockFile creates a lock file with current PID using atomic operations
// Enhanced to prevent race conditions and ensure atomic lock creation
func createLockFile(lockFileName string) (*os.File, error) {
	currentPID := os.Getpid()
	pidStr := strconv.Itoa(currentPID)
	
	// Create temporary lock file first to ensure atomic operation
	tmpLockFile := lockFileName + ".tmp." + pidStr
	
	// Cleanup any leftover temporary file
	os.Remove(tmpLockFile)
	
	maxRetries := 5
	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			// Small delay before retry to avoid tight loop
			time.Sleep(time.Duration(retry*50) * time.Millisecond)
		}

		// First try to cleanup any stale locks
		if err := cleanupStaleLock(lockFileName); err != nil {
			audit.Logger.Warn("Failed to cleanup stale lock",
				slog.String("lock_file", lockFileName),
				slog.String("error", err.Error()),
				slog.Int("retry", retry))
		}

		// Create temporary lock file first
		tmpFile, err := os.OpenFile(tmpLockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			if os.IsExist(err) {
				// Remove stale temp file and continue retry
				os.Remove(tmpLockFile)
				continue
			}
			return nil, fmt.Errorf("failed to create temporary lock file: %v", err)
		}

		// Write PID to temporary file
		if _, err := tmpFile.WriteString(pidStr); err != nil {
			tmpFile.Close()
			os.Remove(tmpLockFile)
			return nil, fmt.Errorf("failed to write PID to temporary lock file: %v", err)
		}

		// Ensure data is written to disk before attempting rename
		if err := tmpFile.Sync(); err != nil {
			tmpFile.Close()
			os.Remove(tmpLockFile)
			return nil, fmt.Errorf("failed to sync temporary lock file: %v", err)
		}
		tmpFile.Close()

		// Atomically rename temporary file to actual lock file
		if err := os.Rename(tmpLockFile, lockFileName); err != nil {
			os.Remove(tmpLockFile) // Cleanup temp file
			if os.IsExist(err) {
				// Lock file was created by another process, check if it's stale
				if retry < maxRetries-1 {
					audit.Logger.Debug("Lock file exists, retrying",
						slog.String("lock_file", lockFileName),
						slog.Int("retry", retry))
					continue
				}
			}
			return nil, fmt.Errorf("failed to rename temporary lock file: %v", err)
		}

		// Successfully created lock file, now open it for exclusive access
		lockFile, err := os.OpenFile(lockFileName, os.O_RDWR, 0600)
		if err != nil {
			// Clean up lock file if we can't open it
			os.Remove(lockFileName)
			return nil, fmt.Errorf("failed to open created lock file: %v", err)
		}

		// Apply exclusive file lock immediately
		if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			lockFile.Close()
			os.Remove(lockFileName)
			if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
				if retry < maxRetries-1 {
					audit.Logger.Debug("Lock file is locked by another process, retrying",
						slog.String("lock_file", lockFileName),
						slog.Int("retry", retry))
					continue
				}
				return nil, fmt.Errorf("lock file is held by another process")
			}
			return nil, fmt.Errorf("failed to acquire exclusive lock: %v", err)
		}

		audit.Logger.Debug("Lock file created and locked",
			slog.String("lock_file", lockFileName),
			slog.Int("pid", currentPID),
			slog.Int("retries", retry))

		return lockFile, nil
	}

	return nil, fmt.Errorf("failed to create lock file after %d retries", maxRetries)
}

// lockFile applies an exclusive lock to the file with timeout
// Enhanced with non-blocking option and proper error handling
func lockFile(file *os.File) error {
	// First try non-blocking lock to get immediate feedback
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			// File is locked, try with timeout using blocking call
			audit.Logger.Debug("File is locked, waiting for lock",
				slog.String("file", file.Name()))
			
			// Use blocking lock as fallback
			return unix.Flock(int(file.Fd()), unix.LOCK_EX)
		}
		return err
	}
	return nil
}

// unlockFile removes the lock from the file
func unlockFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}

// openTTYSafely safely opens TTY with availability checks
func openTTYSafely() (*os.File, error) {
	// Check if we have a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, errors.New(errors.ErrCodeSystem, "TTY not available - running in non-interactive environment")
	}

	// Attempt to open TTY
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, errors.NewFileSystemError("open", "/dev/tty", err).WithDetails("TTY not accessible")
	}

	return tty, nil
}

// getYubiKeyTimeout returns configurable timeout for YubiKey operations
func getYubiKeyTimeout() time.Duration {
	if config.Cfg.YubikeyTimeout > 0 {
		return time.Duration(config.Cfg.YubikeyTimeout) * time.Second
	}
	return 60 * time.Second // Increased default timeout
}

// CheckYubiKey checks for the availability of a YubiKey with retry mechanism.
func CheckYubiKey() error {
	return CheckYubiKeyWithRetry(3)
}

// CheckYubiKeyWithRetry checks for YubiKey availability with retry attempts
func CheckYubiKeyWithRetry(maxRetries int) error {
	audit.Logger.Info("Checking YubiKey availability", slog.Int("max_retries", maxRetries))

	// First check if the command is available
	if _, err := exec.LookPath("age-plugin-yubikey"); err != nil {
		audit.Logger.Error("age-plugin-yubikey not found in PATH")
		return errors.NewDependencyError("age-plugin-yubikey", "Please install it: https://github.com/str4d/age-plugin-yubikey")
	}

	timeout := getYubiKeyTimeout()
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		audit.Logger.Debug("YubiKey check attempt", slog.Int("attempt", attempt), slog.Int("max_retries", maxRetries))

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		cmd := exec.CommandContext(ctx, "age-plugin-yubikey", "--list")
		output, err := cmd.CombinedOutput()
		cancel()

		if err == nil {
			if strings.TrimSpace(string(output)) == "" {
				audit.Logger.Warn("No YubiKey found or no age keys on it")
				return errors.NewYubikeyNotFoundError()
			}
			audit.Logger.Info("YubiKey check completed successfully", slog.Int("attempt", attempt))
			return nil
		}

		lastErr = err
		audit.Logger.Warn("YubiKey check failed",
			slog.Int("attempt", attempt),
			slog.String("error", err.Error()),
			slog.String("output", sanitizeLogOutput(string(output))))

		if attempt < maxRetries {
			// Wait before retrying (exponential backoff)
			retryDelay := time.Duration(attempt) * 2 * time.Second
			audit.Logger.Info("Retrying YubiKey check", slog.Duration("delay", retryDelay))
			time.Sleep(retryDelay)
		}
	}

	audit.Logger.Error("YubiKey check failed after all retries", slog.Int("attempts", maxRetries))
	return errors.ParseYubiKeyError(lastErr, "Max retry attempts exceeded")
}

// createSecureBuffer creates a temporary secure buffer for sensitive operations
func createSecureBuffer(description string) *security.SecureString {
	buffer := security.NewSecureBuffer(description)
	audit.Logger.Debug("Created secure buffer", slog.String("description", description))
	return buffer
}

// sanitizeLogOutput removes sensitive information from log output with comprehensive patterns
func sanitizeLogOutput(output string) string {
	// Enhanced patterns for sensitive data detection
	sensitivePatterns := []string{
		"pin", "password", "secret", "private", "credential",
		"auth", "token", "session", "cookie", "bearer",
		"certificate", "cert", "pem", "p12", "pkcs",
		"mnemonic", "seed", "entropy", "wallet",
		"yubikey", "yubi", "piv", "oath", "fido",
		// Common key patterns
		"-----begin", "-----end", "0x", "sk_", "pk_",
		// Age-specific patterns
		"age1", "AGE-SECRET-KEY", "age-encryption",
		// Error messages that might leak info
		"failed to authenticate", "wrong pin", "invalid pin",
		"touch your yubikey", "insert your yubikey",
	}

	// Remove lines that might contain sensitive information
	lines := strings.Split(output, "\n")
	sanitized := make([]string, 0, len(lines))

	for _, line := range lines {
		lowerLine := strings.ToLower(strings.TrimSpace(line))
		
		// Skip empty lines
		if lowerLine == "" {
			sanitized = append(sanitized, line)
			continue
		}
		
		// Check for sensitive patterns
		containsSensitive := false
		for _, pattern := range sensitivePatterns {
			if strings.Contains(lowerLine, pattern) {
				containsSensitive = true
				break
			}
		}
		
		// Additional checks for hex/base64 patterns that might be keys
		if !containsSensitive {
			// Check for potential key material (long hex strings, base64)
			if len(line) > 32 && (isHexString(line) || isBase64Like(line)) {
				containsSensitive = true
			}
		}
		
		if containsSensitive {
			sanitized = append(sanitized, "[REDACTED SENSITIVE LINE]")
		} else {
			sanitized = append(sanitized, line)
		}
	}

	return strings.Join(sanitized, "\n")
}

// isHexString checks if string looks like hexadecimal key material
func isHexString(s string) bool {
	cleaned := strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), ":", "")
	if len(cleaned) < 32 {
		return false
	}
	for _, char := range cleaned {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

// isBase64Like checks if string looks like base64 encoded data
func isBase64Like(s string) bool {
	cleaned := strings.TrimSpace(s)
	if len(cleaned) < 32 {
		return false
	}
	// Check for base64 characteristics
	base64Chars := 0
	for _, char := range cleaned {
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || 
		   (char >= '0' && char <= '9') || char == '+' || char == '/' || char == '=' {
			base64Chars++
		}
	}
	return float64(base64Chars)/float64(len(cleaned)) > 0.8
}

// LoadVault decrypts and loads the vault from a file, using the specified method.
func LoadVault(details config.VaultDetails) (Vault, error) {
	// Validate the file path
	if err := config.ValidateFilePath(details.KeyFile, "keyfile"); err != nil {
		audit.Logger.Error("Failed to validate key file path",
			slog.String("key_file", details.KeyFile),
			slog.String("error", err.Error()))
		return nil, err
	}

	if _, err := os.Stat(details.KeyFile); os.IsNotExist(err) {
		// If the vault file doesn't exist, return a new, empty vault.
		audit.Logger.Info("Vault file does not exist, creating new vault",
			slog.String("key_file", details.KeyFile))
		return make(Vault), nil
	}

	audit.Logger.Info("Loading vault",
		slog.String("key_file", details.KeyFile),
		slog.String("encryption", details.Encryption))

	// Lock the file to prevent concurrent access during loading
	file, err := os.OpenFile(details.KeyFile, os.O_RDONLY, 0600)
	if err != nil {
		audit.Logger.Error("Failed to open vault file for locking",
			slog.String("key_file", details.KeyFile),
			slog.String("error", err.Error()))
		return nil, errors.NewFileSystemError("open", details.KeyFile, err)
	}
	defer file.Close()

	if err := lockFile(file); err != nil {
		audit.Logger.Error("Failed to lock vault file",
			slog.String("key_file", details.KeyFile),
			slog.String("error", err.Error()))
		return nil, errors.NewVaultLockedError(details.KeyFile)
	}

	var ageCmd *exec.Cmd

	switch details.Encryption {
	case constants.EncryptionYubiKey:
		// Check for age-plugin-yubikey availability
		if _, err := exec.LookPath("age-plugin-yubikey"); err != nil {
			return nil, errors.NewDependencyError("age-plugin-yubikey", "Please install it: https://github.com/str4d/age-plugin-yubikey")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		pluginArgs := []string{"-i"}
		if config.Cfg.YubikeySlot != "" {
			pluginArgs = append(pluginArgs, "--slot", config.Cfg.YubikeySlot)
		}
		pluginCmd := exec.CommandContext(ctx, "age-plugin-yubikey", pluginArgs...)

		tty, err := openTTYSafely()
		if err != nil {
			return nil, err
		}
		defer tty.Close()
		pluginCmd.Stdin = tty

		var stderrBuf bytes.Buffer
		pluginCmd.Stderr = &stderrBuf
		identity, err := pluginCmd.Output()
		if err != nil {
			return nil, errors.ParseYubiKeyError(err, sanitizeLogOutput(stderrBuf.String()))
		}

		// Check for age availability
		if _, err := exec.LookPath("age"); err != nil {
			return nil, errors.NewDependencyError("age", "Please install it: https://github.com/FiloSottile/age")
		}

		ageCmd = exec.CommandContext(ctx, "age", "--decrypt", "-i", "-", details.KeyFile)
		ageCmd.Stdin = bytes.NewReader(identity)

	default:
		return nil, errors.NewFormatInvalidError(details.Encryption, "unknown encryption method")
	}

	// Use SecureBuffer for sensitive decrypted data instead of bytes.Buffer
	secureBuffer := createSecureBuffer("vault_decrypt_buffer")
	defer secureBuffer.Clear() // Ensure immediate cleanup

	var stderr bytes.Buffer
	// Set up custom writer that feeds data securely into SecureString
	ageCmd.Stdout = &secureBufferWriter{buffer: secureBuffer}
	// Don't overwrite stderr if it was already set (e.g., for YubiKey error handling)
	if ageCmd.Stderr == nil {
		ageCmd.Stderr = &stderr
	}

	if err := ageCmd.Run(); err != nil {
		// SecureBuffer will be cleared by defer, no additional cleanup needed

		// Get stderr content - handle case where stderr might be set elsewhere
		var stderrContent string
		if ageCmd.Stderr == &stderr {
			stderrContent = stderr.String()
		} else {
			// If stderr was set elsewhere, we might not have direct access
			stderrContent = "stderr output not available"
		}

		// For YubiKey encryption, use ParseYubiKeyError for all errors with sanitized content
		if details.Encryption == constants.EncryptionYubiKey {
			return nil, errors.ParseYubiKeyError(err, sanitizeLogOutput(stderrContent))
		}

		audit.Logger.Error("Failed to decrypt vault",
			slog.String("key_file", details.KeyFile),
			slog.String("error", err.Error()),
			slog.String("stderr", sanitizeLogOutput(stderrContent)))
		return nil, errors.NewVaultLoadError(details.KeyFile, err).WithDetails(stderrContent)
	}

	// Data is now securely stored in secureBuffer, ready for processing
	var finalVault Vault

	// Use secure operation to process vault data
	err = secureBuffer.WithSecureOperation(func(vaultData []byte) error {
		// Detect vault format and handle accordingly
		isVersioned, err := detectVaultFormat(vaultData)
		if err != nil {
			audit.Logger.Error("Failed to detect vault format",
				slog.String("key_file", details.KeyFile),
				slog.String("error", err.Error()))
			return errors.NewVaultCorruptError(details.KeyFile, err)
		}

		if isVersioned {
			// Handle versioned format
			var header VaultHeader
			if err := json.Unmarshal(vaultData, &header); err != nil {
				audit.Logger.Error("Failed to parse versioned vault data",
					slog.String("key_file", details.KeyFile),
					slog.String("error", err.Error()))
				return errors.NewVaultCorruptError(details.KeyFile, err)
			}

			// Validate version compatibility
			if err := validateVaultVersion(header.Version); err != nil {
				audit.Logger.Error("Unsupported vault version",
					slog.String("key_file", details.KeyFile),
					slog.Int("vault_version", header.Version),
					slog.Int("supported_version", CurrentVaultVersion))
				return err
			}

			audit.Logger.Info("Loading versioned vault",
				slog.String("key_file", details.KeyFile),
				slog.Int("version", header.Version))

			finalVault = header.Data
		} else {
			// Handle legacy format
			audit.Logger.Info("Loading legacy vault format",
				slog.String("key_file", details.KeyFile))

			if err := json.Unmarshal(vaultData, &finalVault); err != nil {
				audit.Logger.Error("Failed to parse legacy vault data",
					slog.String("key_file", details.KeyFile),
					slog.String("error", err.Error()))
				return errors.NewVaultCorruptError(details.KeyFile, err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	audit.Logger.Info("Vault loaded successfully",
		slog.String("key_file", details.KeyFile),
		slog.Int("wallet_count", len(finalVault)))
	return finalVault, nil
}

// createSecureTempFile creates a temporary file with secure permissions (0600)
func createSecureTempFile(dir string) (*os.File, error) {
	tmpfile, err := os.CreateTemp(dir, "vault-tmp-*")
	if err != nil {
		return nil, err
	}

	// Immediately set secure permissions
	if err := tmpfile.Chmod(0600); err != nil {
		os.Remove(tmpfile.Name())
		return nil, err
	}

	return tmpfile, nil
}

// SaveVault encrypts and saves the vault to a file atomically.
func SaveVault(details config.VaultDetails, v Vault) error {
	audit.Logger.Info("Saving vault",
		slog.String("key_file", details.KeyFile),
		slog.String("encryption", details.Encryption),
		slog.Int("wallet_count", len(v)))

	// Validate file paths
	if err := config.ValidateFilePath(details.KeyFile, "keyfile"); err != nil {
		audit.Logger.Error("Failed to validate key file path",
			slog.String("key_file", details.KeyFile),
			slog.String("error", err.Error()))
		return err
	}

	var recipientsFile string
	if details.RecipientsFile != "" {
		if err := config.ValidateFilePath(details.RecipientsFile, "recipients file"); err != nil {
			audit.Logger.Error("Failed to validate recipients file path",
				slog.String("recipients_file", details.RecipientsFile),
				slog.String("error", err.Error()))
			return err
		}
		recipientsFile = details.RecipientsFile
	}

	// Create lock file with PID to prevent concurrent saves and handle stale locks
	lockFileName := details.KeyFile + ".lock"
	lockFile, err := createLockFile(lockFileName)
	if err != nil {
		if os.IsExist(err) {
			return errors.NewVaultLockedError(details.KeyFile)
		}
		return errors.NewFileSystemError("create", lockFileName, err)
	}
	defer func() {
		lockFile.Close()
		if removeErr := os.Remove(lockFileName); removeErr != nil {
			audit.Logger.Warn("Failed to remove lock file",
				slog.String("lock_file", lockFileName),
				slog.String("error", removeErr.Error()))
		} else {
			audit.Logger.Debug("Lock file removed", slog.String("lock_file", lockFileName))
		}
	}()

	audit.Logger.Debug("Lock file created for save operation", slog.String("lock_file", lockFileName))

	// Create versioned vault header
	vaultHeader := VaultHeader{
		Version: CurrentVaultVersion,
		Data:    v,
	}

	// Serialize versioned data securely after acquiring lock
	data, err := json.MarshalIndent(vaultHeader, "", "  ")
	if err != nil {
		return errors.New(errors.ErrCodeInternal, "failed to serialize vault data").WithContext("marshal_error", err.Error())
	}
	// Ensure serialized data is cleared from memory when function exits
	defer func() {
		security.SecureZero(data)
		data = nil
	}()

	// Create a temporary file in the same directory as the target file
	dir := filepath.Dir(details.KeyFile)
	if dir == "." {
		dir = "."
	}

	tmpfile, err := createSecureTempFile(dir)
	if err != nil {
		return errors.NewFileSystemError("create", dir, err).WithDetails("could not create temp file")
	}
	defer os.Remove(tmpfile.Name()) // clean up

	var cmd *exec.Cmd

	switch details.Encryption {
	case constants.EncryptionYubiKey:
		// Check for age availability
		if _, err := exec.LookPath("age"); err != nil {
			return errors.NewDependencyError("age", "Please install it: https://github.com/FiloSottile/age")
		}

		if recipientsFile == "" {
			return errors.NewConfigMissingError("recipients_file").WithDetails("recipients file is required for yubikey encryption")
		}
		if _, err := os.Stat(recipientsFile); os.IsNotExist(err) {
			return errors.NewFileSystemError("access", recipientsFile, err).WithDetails("recipients file not found")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		args := []string{"-a", "-R", recipientsFile, "-o", tmpfile.Name()}
		cmd = exec.CommandContext(ctx, "age", args...)
		// Use secure reader for sensitive data
		cmd.Stdin = bytes.NewReader(data)

	default:
		return errors.NewFormatInvalidError(details.Encryption, "unknown encryption method")
	}

	var stderr bytes.Buffer
	if cmd.Stderr == nil {
		cmd.Stderr = &stderr
	}

	if runErr := cmd.Run(); runErr != nil {
		// Clear any sensitive data that might remain in stderr
		stderrContent := stderr.String()
		// Sanitize stderr content before logging and error details
		sanitizedStderr := sanitizeLogOutput(stderrContent)
		audit.Logger.Error("Failed to encrypt vault",
			slog.String("key_file", details.KeyFile),
			slog.String("error", runErr.Error()),
			slog.String("stderr", sanitizedStderr))
		return errors.NewVaultSaveError(details.KeyFile, runErr).WithDetails(sanitizedStderr)
	}

	// Atomically replace the target file with our encrypted temporary file
	encryptedFile := tmpfile.Name()
	tmpfile.Close() // Close handle to allow rename

	// Atomically rename temp file to target file
	if err := os.Rename(encryptedFile, details.KeyFile); err != nil {
		audit.Logger.Error("Failed to atomically move encrypted file",
			slog.String("key_file", details.KeyFile),
			slog.String("temp_file", encryptedFile),
			slog.String("error", err.Error()))
		return errors.NewFileSystemError("rename", encryptedFile, err).WithDetails("failed to atomically move encrypted file")
	}

	// Set secure permissions for the final file
	if err := os.Chmod(details.KeyFile, 0600); err != nil {
		audit.Logger.Error("Failed to set secure permissions on final file",
			slog.String("key_file", details.KeyFile),
			slog.String("error", err.Error()))
		// Don't return error as file is already saved
	}

	audit.Logger.Info("Vault saved successfully",
		slog.String("key_file", details.KeyFile),
		slog.Int("wallet_count", len(v)))
	return nil
}
