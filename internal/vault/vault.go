

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

	"golang.org/x/term"
	"golang.org/x/sys/unix"
	"vault.module/internal/audit"
	"vault.module/internal/config"
	"vault.module/internal/constants"
	"vault.module/internal/errors"
	"vault.module/internal/security"
)

const (
	CurrentVaultVersion = 1
)

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

// MigrateLegacyVaultFile migrates a legacy vault file to versioned format
func MigrateLegacyVaultFile(details config.VaultDetails) error {
	audit.Logger.Info("Starting vault format migration",
		slog.String("key_file", details.KeyFile))

	// Load vault (this will handle legacy format automatically)
	vault, err := LoadVault(details)
	if err != nil {
		return err
	}

	// Save vault (this will save in versioned format)
	if err := SaveVault(details, vault); err != nil {
		audit.Logger.Error("Failed to save migrated vault",
			slog.String("key_file", details.KeyFile),
			slog.String("error", err.Error()))
		return err
	}

	audit.Logger.Info("Vault format migration completed successfully",
		slog.String("key_file", details.KeyFile))
	return nil
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

// migrateLegacyVault converts legacy vault format to current versioned format
func migrateLegacyVault(legacyData Vault) VaultHeader {
	audit.Logger.Info("Migrating legacy vault format to versioned format",
		slog.Int("wallet_count", len(legacyData)),
		slog.Int("target_version", CurrentVaultVersion))

	return VaultHeader{
		Version: CurrentVaultVersion,
		Data:    legacyData,
	}
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
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix systems, signal 0 can be used to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// cleanupStaleLock removes lock file if the process that created it is no longer running
func cleanupStaleLock(lockFileName string) error {
	data, err := os.ReadFile(lockFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Lock file doesn't exist, nothing to clean
		}
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

	if !isProcessRunning(pid) {
		audit.Logger.Info("Found stale lock file, removing",
			slog.String("lock_file", lockFileName),
			slog.Int("stale_pid", pid))
		return os.Remove(lockFileName)
	}

	return nil // Lock is still valid
}

// createLockFile creates a lock file with current PID
func createLockFile(lockFileName string) (*os.File, error) {
	// First try to cleanup any stale locks
	if err := cleanupStaleLock(lockFileName); err != nil {
		audit.Logger.Warn("Failed to cleanup stale lock",
			slog.String("lock_file", lockFileName),
			slog.String("error", err.Error()))
	}

	// Try to create lock file
	lockFile, err := os.OpenFile(lockFileName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			// Lock exists, check if it's stale
			if cleanupErr := cleanupStaleLock(lockFileName); cleanupErr == nil {
				// Successfully cleaned up stale lock, try again
				lockFile, err = os.OpenFile(lockFileName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if err == nil {
					audit.Logger.Info("Acquired lock after cleaning stale lock",
						slog.String("lock_file", lockFileName))
				}
			}
		}
		if err != nil {
			return nil, err
		}
	}

	// Write current PID to lock file
	currentPID := os.Getpid()
	if _, err := lockFile.WriteString(strconv.Itoa(currentPID)); err != nil {
		lockFile.Close()
		os.Remove(lockFileName)
		return nil, err
	}

	audit.Logger.Debug("Lock file created",
		slog.String("lock_file", lockFileName),
		slog.Int("pid", currentPID))

	return lockFile, nil
}

// validateAndCleanPath validates and cleans the file path
func validateAndCleanPath(path string) (string, error) {
	if path == "" {
		return "", errors.NewVaultInvalidPathError(path, fmt.Errorf("path cannot be empty"))
	}

	// Clean path from relative components
	cleanPath := filepath.Clean(path)

	// Check for attempts to escape boundaries
	if strings.Contains(cleanPath, "..") {
		return "", errors.NewVaultInvalidPathError(path, fmt.Errorf("path contains invalid traversal"))
	}

	// Get absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", errors.NewVaultInvalidPathError(path, err)
	}

	// Check that path doesn't contain unsafe characters
	if strings.ContainsAny(absPath, "<>:\"|?*") {
		return "", errors.NewVaultInvalidPathError(path, fmt.Errorf("path contains invalid characters"))
	}

	return absPath, nil
}

// lockFile applies an exclusive lock to the file
func lockFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_EX)
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

// sanitizeLogOutput removes sensitive information from log output
func sanitizeLogOutput(output string) string {
	// Remove lines that might contain sensitive information
	lines := strings.Split(output, "\n")
	sanitized := make([]string, 0, len(lines))

	for _, line := range lines {
		lowerLine := strings.ToLower(line)
		// Skip lines that might contain sensitive data
		if strings.Contains(lowerLine, "pin") ||
			strings.Contains(lowerLine, "password") ||
			strings.Contains(lowerLine, "key") ||
			strings.Contains(lowerLine, "token") {
			sanitized = append(sanitized, "[REDACTED LINE]")
		} else {
			sanitized = append(sanitized, line)
		}
	}

	return strings.Join(sanitized, "\n")
}

// LoadVault decrypts and loads the vault from a file, using the specified method.
func LoadVault(details config.VaultDetails) (Vault, error) {
	// Validate and clean the file path
	cleanKeyFile, err := validateAndCleanPath(details.KeyFile)
	if err != nil {
		audit.Logger.Error("Failed to validate key file path",
			slog.String("key_file", details.KeyFile),
			slog.String("error", err.Error()))
		return nil, err
	}

	if _, err := os.Stat(cleanKeyFile); os.IsNotExist(err) {
		// If the vault file doesn't exist, return a new, empty vault.
		audit.Logger.Info("Vault file does not exist, creating new vault",
			slog.String("key_file", cleanKeyFile))
		return make(Vault), nil
	}

	audit.Logger.Info("Loading vault",
		slog.String("key_file", cleanKeyFile),
		slog.String("encryption", details.Encryption))

	// Lock the file to prevent concurrent access during loading
	file, err := os.OpenFile(cleanKeyFile, os.O_RDONLY, 0600)
	if err != nil {
		audit.Logger.Error("Failed to open vault file for locking",
			slog.String("key_file", cleanKeyFile),
			slog.String("error", err.Error()))
		return nil, errors.NewFileSystemError("open", cleanKeyFile, err)
	}
	defer file.Close()

	if err := lockFile(file); err != nil {
		audit.Logger.Error("Failed to lock vault file",
			slog.String("key_file", cleanKeyFile),
			slog.String("error", err.Error()))
		return nil, errors.NewVaultLockedError(cleanKeyFile)
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

		ageCmd = exec.CommandContext(ctx, "age", "--decrypt", "-i", "-", cleanKeyFile)
		ageCmd.Stdin = bytes.NewReader(identity)

	default:
		return nil, errors.NewFormatInvalidError(details.Encryption, "unknown encryption method")
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	ageCmd.Stdout = &out
	// Don't overwrite stderr if it was already set (e.g., for YubiKey error handling)
	if ageCmd.Stderr == nil {
		ageCmd.Stderr = &stderr
	}

	if err := ageCmd.Run(); err != nil {
		// Get stderr content - handle case where stderr might be set elsewhere
		var stderrContent string
		if ageCmd.Stderr == &stderr {
			stderrContent = stderr.String()
		} else {
			// If stderr was set elsewhere, we might not have direct access
			stderrContent = "stderr output not available"
		}
		
		// For YubiKey encryption, use ParseYubiKeyError for all errors
		if details.Encryption == constants.EncryptionYubiKey {
			return nil, errors.ParseYubiKeyError(err, stderrContent)
		}
		
		audit.Logger.Error("Failed to decrypt vault",
			slog.String("key_file", cleanKeyFile),
			slog.String("error", err.Error()),
			slog.String("stderr", sanitizeLogOutput(stderrContent)))
		return nil, errors.NewVaultLoadError(cleanKeyFile, err).WithDetails(stderrContent)
	}

	var finalVault Vault

	// Detect vault format and handle accordingly
	isVersioned, err := detectVaultFormat(out.Bytes())
	if err != nil {
		audit.Logger.Error("Failed to detect vault format",
			slog.String("key_file", cleanKeyFile),
			slog.String("error", err.Error()))
		return nil, errors.NewVaultCorruptError(cleanKeyFile, err)
	}

	if isVersioned {
		// Handle versioned format
		var header VaultHeader
		if err := json.Unmarshal(out.Bytes(), &header); err != nil {
			audit.Logger.Error("Failed to parse versioned vault data",
				slog.String("key_file", cleanKeyFile),
				slog.String("error", err.Error()))
			return nil, errors.NewVaultCorruptError(cleanKeyFile, err)
		}

		// Validate version compatibility
		if err := validateVaultVersion(header.Version); err != nil {
			audit.Logger.Error("Unsupported vault version",
				slog.String("key_file", cleanKeyFile),
				slog.Int("vault_version", header.Version),
				slog.Int("supported_version", CurrentVaultVersion))
			return nil, err
		}

		audit.Logger.Info("Loading versioned vault",
			slog.String("key_file", cleanKeyFile),
			slog.Int("version", header.Version))

		finalVault = header.Data
	} else {
		// Handle legacy format
		audit.Logger.Info("Loading legacy vault format",
			slog.String("key_file", cleanKeyFile))

		if err := json.Unmarshal(out.Bytes(), &finalVault); err != nil {
			audit.Logger.Error("Failed to parse legacy vault data",
				slog.String("key_file", cleanKeyFile),
				slog.String("error", err.Error()))
			return nil, errors.NewVaultCorruptError(cleanKeyFile, err)
		}
	}

	audit.Logger.Info("Vault loaded successfully",
		slog.String("key_file", cleanKeyFile),
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

	// Validate and clean file paths
	cleanKeyFile, err := validateAndCleanPath(details.KeyFile)
	if err != nil {
		audit.Logger.Error("Failed to validate key file path",
			slog.String("key_file", details.KeyFile),
			slog.String("error", err.Error()))
		return err
	}

	var cleanRecipientsFile string
	if details.RecipientsFile != "" {
		cleanRecipientsFile, err = validateAndCleanPath(details.RecipientsFile)
		if err != nil {
			audit.Logger.Error("Failed to validate recipients file path",
				slog.String("recipients_file", details.RecipientsFile),
				slog.String("error", err.Error()))
			return err
		}
	}

	// Create lock file with PID to prevent concurrent saves and handle stale locks
	lockFileName := cleanKeyFile + ".lock"
	lockFile, err := createLockFile(lockFileName)
	if err != nil {
		if os.IsExist(err) {
			return errors.NewVaultLockedError(cleanKeyFile)
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

	// Serialize versioned data after acquiring lock
	data, err := json.MarshalIndent(vaultHeader, "", "  ")
	if err != nil {
		return errors.New(errors.ErrCodeInternal, "failed to serialize vault data").WithContext("marshal_error", err.Error())
	}

	// Create a temporary file in the same directory as the target file
	dir := filepath.Dir(cleanKeyFile)
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

		if cleanRecipientsFile == "" {
			return errors.NewConfigMissingError("recipients_file").WithDetails("recipients file is required for yubikey encryption")
		}
		if _, err := os.Stat(cleanRecipientsFile); os.IsNotExist(err) {
			return errors.NewFileSystemError("access", cleanRecipientsFile, err).WithDetails("recipients file not found")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		args := []string{"-a", "-R", cleanRecipientsFile, "-o", tmpfile.Name()}
		cmd = exec.CommandContext(ctx, "age", args...)
		cmd.Stdin = bytes.NewReader(data)

	default:
		return errors.NewFormatInvalidError(details.Encryption, "unknown encryption method")
	}

	var stderr bytes.Buffer
	if cmd.Stderr == nil {
		cmd.Stderr = &stderr
	}

	if runErr := cmd.Run(); runErr != nil {
		audit.Logger.Error("Failed to encrypt vault",
			slog.String("key_file", cleanKeyFile),
			slog.String("error", runErr.Error()),
			slog.String("stderr", sanitizeLogOutput(stderr.String())))
		return errors.NewVaultSaveError(cleanKeyFile, runErr).WithDetails(stderr.String())
	}

	// Atomically replace the target file with our encrypted temporary file
	encryptedFile := tmpfile.Name()
	tmpfile.Close() // Close handle to allow rename
	
	// Atomically rename temp file to target file
	if err := os.Rename(encryptedFile, cleanKeyFile); err != nil {
		audit.Logger.Error("Failed to atomically move encrypted file",
			slog.String("key_file", cleanKeyFile),
			slog.String("temp_file", encryptedFile),
			slog.String("error", err.Error()))
		return errors.NewFileSystemError("rename", encryptedFile, err).WithDetails("failed to atomically move encrypted file")
	}

	// Set secure permissions for the final file
	if err := os.Chmod(cleanKeyFile, 0600); err != nil {
		audit.Logger.Error("Failed to set secure permissions on final file",
			slog.String("key_file", cleanKeyFile),
			slog.String("error", err.Error()))
		// Don't return error as file is already saved
	}

	audit.Logger.Info("Vault saved successfully",
		slog.String("key_file", cleanKeyFile),
		slog.Int("wallet_count", len(v)))
	return nil
}
