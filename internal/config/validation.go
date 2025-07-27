package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vault.module/internal/constants"
)

// NormalizeVaultType converts vault type to lowercase for case-insensitive comparison
func NormalizeVaultType(vaultType string) string {
	return strings.ToLower(strings.TrimSpace(vaultType))
}

// ValidateVaultType checks if the vault type is supported
func ValidateVaultType(vaultType string) error {
	normalized := NormalizeVaultType(vaultType)
	switch normalized {
	case constants.VaultTypeEVM, constants.VaultTypeCosmos:
		return nil
	default:
		return fmt.Errorf("unsupported vault type: %s (supported: %s, %s)",
			vaultType, constants.VaultTypeEVM, constants.VaultTypeCosmos)
	}
}

// ValidateConfig проверяет корректность конфигурации
func ValidateConfig(cfg *Config) error {
	// Проверяем активный vault
	if cfg.ActiveVault != "" {
		if _, exists := cfg.Vaults[cfg.ActiveVault]; !exists {
			return NewConfigError("active_vault", cfg.ActiveVault, "not found in configuration")
		}
	}
	// Проверяем каждый vault
	for name, details := range cfg.Vaults {
		if err := ValidateVaultDetails(name, details); err != nil {
			return fmt.Errorf("vault '%s': %s", name, err.Error())
		}
	}
	return nil
}

// ValidateVaultDetails проверяет детали конкретного vault'а
// Может вернуть *ConfigError
func ValidateVaultDetails(name string, details VaultDetails) error {
	if err := ValidateVaultName(name); err != nil {
		return NewConfigError("vault_name", name, err.Error())
	}
	if !isValidVaultType(details.Type) {
		return NewConfigError("type", details.Type, "must be one of: "+strings.Join(getAllVaultTypes(), ", "))
	}
	if !isValidEncryptionMethod(details.Encryption) {
		return NewConfigError("encryption", details.Encryption, "must be one of: "+strings.Join(getAllEncryptionMethods(), ", "))
	}
	if details.KeyFile == "" {
		return NewConfigError("keyfile", "", "cannot be empty")
	}
	keyDir := filepath.Dir(details.KeyFile)
	stat, err := os.Stat(keyDir)
	if err != nil || !stat.IsDir() {
		return NewConfigError("keyfile_dir", keyDir, "directory does not exist")
	}
	if details.Encryption == constants.EncryptionYubiKey {
		if details.RecipientsFile == "" {
			return NewConfigError("recipients_file", "", "required for yubikey encryption")
		}
		stat, err := os.Stat(details.RecipientsFile)
		if err != nil || stat.IsDir() {
			return NewConfigError("recipients_file", details.RecipientsFile, "file does not exist or is a directory")
		}
		f, err := os.Open(details.RecipientsFile)
		if err != nil {
			return NewConfigError("recipients_file", details.RecipientsFile, "cannot read file")
		}
		f.Close()
	}
	return nil
}

// ValidateVaultName проверяет имя vault'а на корректность
// Может вернуть *ConfigError
func ValidateVaultName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("vault name cannot be empty")
	}
	if len(name) > 50 {
		return fmt.Errorf("vault name too long (max 50 characters)")
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_') {
			return fmt.Errorf("vault name contains invalid character: %c", r)
		}
	}
	if name[0] >= '0' && name[0] <= '9' || name[0] == '-' || name[0] == '_' {
		return fmt.Errorf("vault name cannot start with number or special character")
	}
	return nil
}

func isValidVaultType(vaultType string) bool {
	validTypes := getAllVaultTypes()
	for _, t := range validTypes {
		if t == vaultType {
			return true
		}
	}
	return false
}

func isValidEncryptionMethod(method string) bool {
	validMethods := getAllEncryptionMethods()
	for _, m := range validMethods {
		if m == method {
			return true
		}
	}
	return false
}

func getAllVaultTypes() []string {
	return []string{
		constants.VaultTypeEVM,
		constants.VaultTypeCosmos,
	}
}

func getAllEncryptionMethods() []string {
	return []string{
		constants.EncryptionYubiKey,
	}
}

func LoadConfigWithValidation() error {
	if err := LoadConfig(); err != nil {
		return NewConfigError("load", "", err.Error())
	}
	if err := ValidateConfig(&Cfg); err != nil {
		return NewConfigError("validate", "", err.Error())
	}
	return nil
}
