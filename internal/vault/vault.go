// File: internal/vault/vault.go
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
	"strings"
	"time"

	"golang.org/x/sys/unix"
	"vault.module/internal/audit"
	"vault.module/internal/config"
	"vault.module/internal/constants"
	"vault.module/internal/errors"
	"vault.module/internal/security"
)

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

// Vault is the root structure of our vault (the JSON file).
type Vault map[string]Wallet

// New creates an empty vault.
func New() Vault {
	return make(Vault)
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

// CheckYubiKey checks for the availability of a YubiKey.
func CheckYubiKey() error {
	audit.Logger.Info("Checking YubiKey availability")

	// First check if the command is available
	if _, err := exec.LookPath("age-plugin-yubikey"); err != nil {
		audit.Logger.Error("age-plugin-yubikey not found in PATH")
		return errors.NewDependencyError("age-plugin-yubikey", "Please install it: https://github.com/str4d/age-plugin-yubikey")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "age-plugin-yubikey", "--list")
	output, err := cmd.CombinedOutput() // CombinedOutput gets both stdout and stderr
	if err != nil {
		audit.Logger.Error("Failed to run YubiKey check",
			slog.String("error", err.Error()),
			slog.String("output", string(output)))
		return errors.ParseYubiKeyError(err, string(output))
	}
	if strings.TrimSpace(string(output)) == "" {
		audit.Logger.Warn("No YubiKey found or no age keys on it")
		return errors.NewYubikeyNotFoundError()
	}

	audit.Logger.Info("YubiKey check completed successfully")
	return nil
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

		tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			return nil, errors.NewFileSystemError("open", "/dev/tty", err).
				WithDetails("could not open TTY for PIN entry")
		}
		defer tty.Close()
		pluginCmd.Stdin = tty

		var stderrBuf bytes.Buffer
		pluginCmd.Stderr = &stderrBuf
		identity, err := pluginCmd.Output()
		if err != nil {
			return nil, errors.ParseYubiKeyError(err, stderrBuf.String())
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
		
		// For YubiKey encryption, provide more specific error handling
		if details.Encryption == constants.EncryptionYubiKey {
			// Check if this is a YubiKey-related error during decryption
			if strings.Contains(strings.ToLower(stderrContent), "yubikey") || 
			   strings.Contains(strings.ToLower(stderrContent), "pin") ||
			   strings.Contains(strings.ToLower(stderrContent), "authentication") {
				return nil, errors.ParseYubiKeyError(err, stderrContent)
			}
		}
		
		audit.Logger.Error("Failed to decrypt vault",
			slog.String("key_file", cleanKeyFile),
			slog.String("error", err.Error()),
			slog.String("stderr", stderrContent))
		return nil, errors.NewVaultLoadError(cleanKeyFile, err).WithDetails(stderrContent)
	}

	var v Vault
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		audit.Logger.Error("Failed to parse vault data",
			slog.String("key_file", cleanKeyFile),
			slog.String("error", err.Error()))
		return nil, errors.NewVaultCorruptError(cleanKeyFile, err)
	}

	audit.Logger.Info("Vault loaded successfully",
		slog.String("key_file", cleanKeyFile),
		slog.Int("wallet_count", len(v)))
	return v, nil
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

	// SIMPLIFIED: Create lock file to prevent concurrent saves
	lockFileName := cleanKeyFile + ".lock"
	lockFile, err := os.OpenFile(lockFileName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			return errors.NewVaultLockedError(cleanKeyFile)
		}
		return errors.NewFileSystemError("create", lockFileName, err)
	}
	
	defer func() {
		lockFile.Close()
		os.Remove(lockFileName) // Always clean up lock file
	}()
	
	audit.Logger.Debug("Lock file created for save operation", slog.String("lock_file", lockFileName))

	// Serialize data after acquiring lock
	data, err := json.MarshalIndent(v, "", "  ")
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
			slog.String("stderr", stderr.String()))
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
