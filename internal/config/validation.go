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

// ValidateVaultDetails проверяет детали конкретного vault'а с улучшенной валидацией путей
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
	
	// Enhanced keyfile validation with symlink checking
	if err := ValidateFilePath(details.KeyFile, "keyfile"); err != nil {
		return NewConfigError("keyfile", details.KeyFile, err.Error())
	}
	
	// Validate keyfile directory with enhanced security
	keyDir := filepath.Dir(details.KeyFile)
	if err := ValidateDirectoryPath(keyDir, "keyfile directory"); err != nil {
		return NewConfigError("keyfile_dir", keyDir, err.Error())
	}
	
	// Enhanced recipients file validation for YubiKey encryption
	if details.Encryption == constants.EncryptionYubiKey {
		if details.RecipientsFile == "" {
			return NewConfigError("recipients_file", "", "required for yubikey encryption")
		}
		
		if err := ValidateFilePath(details.RecipientsFile, "recipients file"); err != nil {
			return NewConfigError("recipients_file", details.RecipientsFile, err.Error())
		}
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

// ValidateFilePath validates file paths with security checks including symlink resolution
func ValidateFilePath(filePath string, description string) error {
	if filePath == "" {
		return fmt.Errorf("%s path cannot be empty", description)
	}
	
	// Clean the path to resolve any . and .. elements
	cleanPath := filepath.Clean(filePath)
	
	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("%s path contains invalid path traversal elements", description)
	}
	
	// Resolve symlinks to get the actual path
	realPath, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		// If EvalSymlinks fails, the file might not exist yet, so we check the directory
		dirPath := filepath.Dir(cleanPath)
		if _, dirErr := os.Stat(dirPath); os.IsNotExist(dirErr) {
			return fmt.Errorf("%s directory does not exist: %s", description, dirPath)
		}
		// If directory exists but file doesn't, that's acceptable for new files
		return nil
	}
	
	// Check if the resolved path is different from original (indicates symlink)
	if realPath != cleanPath {
		// Additional security check: ensure symlink doesn't point outside allowed areas
		if err := validateSymlinkSecurity(cleanPath, realPath, description); err != nil {
			return err
		}
	}
	
	// Check file permissions and accessibility
	if err := validateFileAccess(realPath, description); err != nil {
		return err
	}
	
	return nil
}

// ValidateDirectoryPath validates directory paths with security checks including symlink resolution
func ValidateDirectoryPath(dirPath string, description string) error {
	if dirPath == "" {
		return fmt.Errorf("%s directory path cannot be empty", description)
	}
	
	// Clean the path
	cleanPath := filepath.Clean(dirPath)
	
	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("%s directory path contains invalid path traversal elements", description)
	}
	
	// Resolve symlinks
	realPath, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve %s directory path: %v", description, err)
	}
	
	// Check if resolved path is different (indicates symlink)
	if realPath != cleanPath {
		if err := validateSymlinkSecurity(cleanPath, realPath, description+" directory"); err != nil {
			return err
		}
	}
	
	// Verify it's actually a directory
	stat, err := os.Stat(realPath)
	if err != nil {
		return fmt.Errorf("%s directory does not exist: %s", description, realPath)
	}
	
	if !stat.IsDir() {
		return fmt.Errorf("%s path is not a directory: %s", description, realPath)
	}
	
	// Check directory permissions
	if err := validateDirectoryAccess(realPath, description); err != nil {
		return err
	}
	
	return nil
}

// validateSymlinkSecurity checks if symlink is safe to use
func validateSymlinkSecurity(originalPath, realPath, description string) error {
	// Get absolute paths for comparison
	absOriginal, err := filepath.Abs(originalPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %v", description, err)
	}
	
	absReal, err := filepath.Abs(realPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute real path for %s: %v", description, err)
	}
	
	// Basic security: ensure symlink doesn't point to system directories
	systemDirs := []string{"/etc", "/sys", "/proc", "/dev", "/boot", "/root"}
	for _, sysDir := range systemDirs {
		if strings.HasPrefix(absReal, sysDir) {
			return fmt.Errorf("%s symlink points to restricted system directory: %s", description, absReal)
		}
	}
	
	// Log symlink usage for audit purposes
	fmt.Printf("Warning: %s uses symbolic link: %s -> %s\n", description, absOriginal, absReal)
	
	return nil
}

// validateFileAccess checks file permissions and accessibility
func validateFileAccess(filePath, description string) error {
	stat, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("cannot access %s file: %v", description, err)
	}
	
	// Check if it's actually a file
	if stat.IsDir() {
		return fmt.Errorf("%s path points to a directory, not a file: %s", description, filePath)
	}
	
	// Check read permissions
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("cannot read %s file: %v", description, err)
	}
	file.Close()
	
	return nil
}

// validateDirectoryAccess checks directory permissions
func validateDirectoryAccess(dirPath, description string) error {
	// Test read access
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("cannot read %s directory: %v", description, err)
	}
	
	// Check write access by trying to create a temporary file
	tempFile := filepath.Join(dirPath, ".vault_test_write")
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("cannot write to %s directory: %v", description, err)
	}
	file.Close()
	os.Remove(tempFile) // Clean up
	
	_ = entries // Avoid unused variable warning
	return nil
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
