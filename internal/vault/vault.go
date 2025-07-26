// File: internal/vault/vault.go
package vault

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	PrivateKey *security.SecureString `json:"-"`
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

// Vault is the root structure of our vault (the JSON file).
type Vault map[string]Wallet

// New creates an empty vault.
func New() Vault {
	return make(Vault)
}

// validateAndCleanPath проверяет и очищает путь к файлу
func validateAndCleanPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Очищаем путь от лишних символов
	cleanPath := filepath.Clean(path)

	// Проверяем, что путь не содержит подозрительные символы
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("path contains invalid characters: %s", path)
	}

	// Проверяем, что путь не является абсолютным путем к системным директориям
	if filepath.IsAbs(cleanPath) {
		// Проверяем, что путь не указывает на системные директории
		base := filepath.Base(cleanPath)
		if base == "" || base == "." || base == ".." {
			return "", fmt.Errorf("invalid path: %s", path)
		}
	}

	return cleanPath, nil
}

// CheckYubiKey checks for the availability of a YubiKey.
func CheckYubiKey() error {
	audit.Logger.Info("Checking YubiKey availability")

	// Сначала проверяем, что команда доступна
	if _, err := exec.LookPath("age-plugin-yubikey"); err != nil {
		audit.Logger.Error("age-plugin-yubikey not found in PATH")
		return fmt.Errorf("age-plugin-yubikey is not installed or not in PATH. Please install it: https://github.com/str4d/age-plugin-yubikey")
	}

	cmd := exec.Command("age-plugin-yubikey", "--list")
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
	// Валидируем и очищаем путь к файлу
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

	var ageCmd *exec.Cmd

	switch details.Encryption {
	case constants.EncryptionYubiKey:
		// Проверяем наличие age-plugin-yubikey
		if _, err := exec.LookPath("age-plugin-yubikey"); err != nil {
			return nil, fmt.Errorf("age-plugin-yubikey is not installed or not in PATH. Please install it: https://github.com/str4d/age-plugin-yubikey")
		}

		pluginArgs := []string{"-i"}
		if config.Cfg.YubikeySlot != "" {
			pluginArgs = append(pluginArgs, "--slot", config.Cfg.YubikeySlot)
		}
		pluginCmd := exec.Command("age-plugin-yubikey", pluginArgs...)

		tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			return nil, fmt.Errorf("could not open TTY for PIN entry: %s", err.Error())
		}
		defer tty.Close()
		pluginCmd.Stdin = tty
		pluginCmd.Stderr = tty

		identity, err := pluginCmd.Output()
		if err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				return nil, fmt.Errorf("yubikey plugin error (maybe wrong PIN?): %v", err)
			}
			return nil, fmt.Errorf("failed to run age-plugin-yubikey: %v", err)
		}

		// Проверяем наличие age
		if _, err := exec.LookPath("age"); err != nil {
			return nil, fmt.Errorf("age is not installed or not in PATH. Please install it: https://github.com/FiloSottile/age")
		}

		ageCmd = exec.Command("age", "--decrypt", "-i", "-", cleanKeyFile)
		ageCmd.Stdin = bytes.NewReader(identity)

	default:
		return nil, fmt.Errorf("unknown encryption method: %s", details.Encryption)
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	ageCmd.Stdout = &out
	if ageCmd.Stderr == nil {
		ageCmd.Stderr = &stderr
	}

	if err := ageCmd.Run(); err != nil {
		audit.Logger.Error("Failed to decrypt vault",
			slog.String("key_file", cleanKeyFile),
			slog.String("error", err.Error()),
			slog.String("stderr", stderr.String()))
		return nil, fmt.Errorf("failed to decrypt vault: %v\n%s", err, stderr.String())
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

// SaveVault encrypts and saves the vault to a file atomically.
func SaveVault(details config.VaultDetails, v Vault) error {
	audit.Logger.Info("Saving vault",
		slog.String("key_file", details.KeyFile),
		slog.String("encryption", details.Encryption),
		slog.Int("wallet_count", len(v)))

	// Валидируем и очищаем пути к файлам
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

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize data: %s", err.Error())
	}

	// Создаем временный файл в той же директории что и целевой файл
	dir := filepath.Dir(cleanKeyFile)
	if dir == "." {
		dir = "."
	}

	tmpfile, err := os.CreateTemp(dir, "vault-tmp-*")
	if err != nil {
		return fmt.Errorf("could not create temp file: %s", err.Error())
	}
	defer os.Remove(tmpfile.Name()) // clean up

	var cmd *exec.Cmd

	switch details.Encryption {
	case constants.EncryptionYubiKey:
		// Проверяем наличие age
		if _, err := exec.LookPath("age"); err != nil {
			return fmt.Errorf("age is not installed or not in PATH. Please install it: https://github.com/FiloSottile/age")
		}

		if cleanRecipientsFile == "" {
			return fmt.Errorf("recipients file is required for yubikey encryption")
		}
		if _, err := os.Stat(cleanRecipientsFile); os.IsNotExist(err) {
			return fmt.Errorf("recipients file '%s' not found", cleanRecipientsFile)
		}
		args := []string{"-a", "-R", cleanRecipientsFile, "-o", tmpfile.Name()}
		cmd = exec.Command("age", args...)
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

	// Атомарно перемещаем зашифрованный файл на место
	encryptedFile := tmpfile.Name()

	if err := os.Rename(encryptedFile, cleanKeyFile); err != nil {
		audit.Logger.Error("Failed to atomically move encrypted file",
			slog.String("key_file", cleanKeyFile),
			slog.String("temp_file", encryptedFile),
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to atomically move encrypted file: %s", err.Error())
	}

	audit.Logger.Info("Vault saved successfully",
		slog.String("key_file", cleanKeyFile),
		slog.Int("wallet_count", len(v)))
	return nil
}
