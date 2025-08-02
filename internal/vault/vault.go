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
		return "", fmt.Errorf("path cannot be empty")
	}

	// Очистка пути от относительных компонентов
	cleanPath := filepath.Clean(path)

	// Проверка на попытки выхода за пределы
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("path contains invalid traversal: %s", path)
	}

	// Получение абсолютного пути
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %s", err.Error())
	}

	// Проверка что путь не содержит небезопасных символов
	if strings.ContainsAny(absPath, "<>:\"|?*") {
		return "", fmt.Errorf("path contains invalid characters: %s", path)
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

// parseYubiKeyError парses YubiKey plugin errors and returns user-friendly messages
func parseYubiKeyError(err error, stderr string) error {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return fmt.Errorf("failed to run age-plugin-yubikey: %v", err)
	}

	exitCode := exitErr.ExitCode()
	stderrStr := strings.ToLower(stderr)

	switch exitCode {
	case 1:
		if strings.Contains(stderrStr, "pin") || strings.Contains(stderrStr, "authentication") {
			return fmt.Errorf("YubiKey PIN verification failed. Please check your PIN")
		}
		if strings.Contains(stderrStr, "not found") || strings.Contains(stderrStr, "no device") {
			return fmt.Errorf("YubiKey not found. Please ensure it's connected")
		}
		return fmt.Errorf("YubiKey authentication error: %s", stderr)
	case 2:
		return fmt.Errorf("YubiKey configuration error: %s", stderr)
	default:
		return fmt.Errorf("YubiKey plugin error (exit code %d): %s", exitCode, stderr)
	}
}

// CheckYubiKey checks for the availability of a YubiKey.
func CheckYubiKey() error {
	audit.Logger.Info("Checking YubiKey availability")

	// First check if the command is available
	if _, err := exec.LookPath("age-plugin-yubikey"); err != nil {
		audit.Logger.Error("age-plugin-yubikey not found in PATH")
		return fmt.Errorf("age-plugin-yubikey is not installed or not in PATH. Please install it: https://github.com/str4d/age-plugin-yubikey")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "age-plugin-yubikey", "--list")
	output, err := cmd.CombinedOutput() // CombinedOutput gets both stdout and stderr
	if err != nil {
		audit.Logger.Error("Failed to run YubiKey check",
			slog.String("error", err.Error()),
			slog.String("output", string(output)))
		return fmt.Errorf("could not run yubikey check: %v\n%s", err, string(output))
	}
	if strings.TrimSpace(string(output)) == "" {
		audit.Logger.Warn("No YubiKey found or no age keys on it")
		return fmt.Errorf("no yubikey found or no age keys on it")
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
		return nil, fmt.Errorf("invalid key file path: %s", err.Error())
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
		return nil, fmt.Errorf("failed to open vault file: %s", err.Error())
	}
	defer file.Close()

	if err := lockFile(file); err != nil {
		audit.Logger.Error("Failed to lock vault file",
			slog.String("key_file", cleanKeyFile),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to lock vault file: %s", err.Error())
	}

	var ageCmd *exec.Cmd

	switch details.Encryption {
	case constants.EncryptionYubiKey:
		// Check for age-plugin-yubikey availability
		if _, err := exec.LookPath("age-plugin-yubikey"); err != nil {
			return nil, fmt.Errorf("age-plugin-yubikey is not installed or not in PATH. Please install it: https://github.com/str4d/age-plugin-yubikey")
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
			return nil, fmt.Errorf("could not open TTY for PIN entry: %s", err.Error())
		}
		defer tty.Close()
		pluginCmd.Stdin = tty

		var stderrBuf bytes.Buffer
		pluginCmd.Stderr = &stderrBuf
		identity, err := pluginCmd.Output()
		if err != nil {
			return nil, parseYubiKeyError(err, stderrBuf.String())
		}

		// Check for age availability
		if _, err := exec.LookPath("age"); err != nil {
			return nil, fmt.Errorf("age is not installed or not in PATH. Please install it: https://github.com/FiloSottile/age")
		}

		ageCmd = exec.CommandContext(ctx, "age", "--decrypt", "-i", "-", cleanKeyFile)
		ageCmd.Stdin = bytes.NewReader(identity)

	default:
		return nil, fmt.Errorf("unknown encryption method: %s", details.Encryption)
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
				return nil, parseYubiKeyError(err, stderrContent)
			}
		}
		
		audit.Logger.Error("Failed to decrypt vault",
			slog.String("key_file", cleanKeyFile),
			slog.String("error", err.Error()),
			slog.String("stderr", stderrContent))
		return nil, fmt.Errorf("failed to decrypt vault: %v\n%s", err, stderrContent)
	}

	var v Vault
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		audit.Logger.Error("Failed to parse vault data",
			slog.String("key_file", cleanKeyFile),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to parse vault data (file may be corrupt): %s", err.Error())
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
		return fmt.Errorf("invalid key file path: %s", err.Error())
	}

	var cleanRecipientsFile string
	if details.RecipientsFile != "" {
		cleanRecipientsFile, err = validateAndCleanPath(details.RecipientsFile)
		if err != nil {
			audit.Logger.Error("Failed to validate recipients file path",
				slog.String("recipients_file", details.RecipientsFile),
				slog.String("error", err.Error()))
			return fmt.Errorf("invalid recipients file path: %s", err.Error())
		}
	}

	// SIMPLIFIED: Create lock file to prevent concurrent saves
	lockFileName := cleanKeyFile + ".lock"
	lockFile, err := os.OpenFile(lockFileName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("vault save already in progress (lock file exists): %s", lockFileName)
		}
		return fmt.Errorf("failed to create lock file: %s", err.Error())
	}
	
	defer func() {
		lockFile.Close()
		os.Remove(lockFileName) // Always clean up lock file
	}()
	
	audit.Logger.Debug("Lock file created for save operation", slog.String("lock_file", lockFileName))

	// Serialize data after acquiring lock
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize data: %s", err.Error())
	}

	// Create a temporary file in the same directory as the target file
	dir := filepath.Dir(cleanKeyFile)
	if dir == "." {
		dir = "."
	}

	tmpfile, err := createSecureTempFile(dir)
	if err != nil {
		return fmt.Errorf("could not create temp file: %s", err.Error())
	}
	defer os.Remove(tmpfile.Name()) // clean up

	var cmd *exec.Cmd

	switch details.Encryption {
	case constants.EncryptionYubiKey:
		// Check for age availability
		if _, err := exec.LookPath("age"); err != nil {
			return fmt.Errorf("age is not installed or not in PATH. Please install it: https://github.com/FiloSottile/age")
		}

		if cleanRecipientsFile == "" {
			return fmt.Errorf("recipients file is required for yubikey encryption")
		}
		if _, err := os.Stat(cleanRecipientsFile); os.IsNotExist(err) {
			return fmt.Errorf("recipients file '%s' not found", cleanRecipientsFile)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		args := []string{"-a", "-R", cleanRecipientsFile, "-o", tmpfile.Name()}
		cmd = exec.CommandContext(ctx, "age", args...)
		cmd.Stdin = bytes.NewReader(data)

	default:
		return fmt.Errorf("unknown encryption method: %s", details.Encryption)
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
		return fmt.Errorf("failed to encrypt vault: %v\n%s", runErr, stderr.String())
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
		return fmt.Errorf("failed to atomically move encrypted file: %s", err.Error())
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
