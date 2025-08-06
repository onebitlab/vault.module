// File: internal/errors/builders.go
package errors

import (
	"fmt"
	"os"
	"strings"
)

// Configuration Error Builders
func NewConfigLoadError(path string, cause error) *VaultError {
	return Wrap(ErrCodeConfigLoad, "failed to load configuration", cause).
		WithContext("config_path", path).
		WithSeverity(SeverityError)
}

func NewConfigSaveError(path string, cause error) *VaultError {
	return Wrap(ErrCodeConfigSave, "failed to save configuration", cause).
		WithContext("config_path", path).
		WithSeverity(SeverityError)
}

func NewConfigValidationError(field, value, message string) *VaultError {
	return Newf(ErrCodeConfigValidation, "configuration validation failed").
		WithDetails(fmt.Sprintf("field '%s' with value '%s': %s", field, value, message)).
		WithContext("field", field).
		WithContext("value", value).
		WithSeverity(SeverityError)
}

func NewConfigMissingError(field string) *VaultError {
	return Newf(ErrCodeConfigMissing, "required configuration field missing: %s", field).
		WithContext("field", field).
		WithSeverity(SeverityError)
}

// Vault Error Builders
func NewVaultLoadError(path string, cause error) *VaultError {
	return Wrap(ErrCodeVaultLoad, "failed to load vault", cause).
		WithContext("vault_path", path).
		WithSeverity(SeverityError)
}

func NewVaultSaveError(path string, cause error) *VaultError {
	return Wrap(ErrCodeVaultSave, "failed to save vault", cause).
		WithContext("vault_path", path).
		WithSeverity(SeverityError)
}

func NewVaultExistsError(name string) *VaultError {
	return Newf(ErrCodeVaultExists, "vault '%s' already exists", name).
		WithContext("vault_name", name).
		WithSeverity(SeverityError)
}

func NewVaultNotFoundError(name string) *VaultError {
	return Newf(ErrCodeVaultNotFound, "vault '%s' not found", name).
		WithContext("vault_name", name).
		WithSeverity(SeverityError)
}

func NewVaultLockedError(path string) *VaultError {
	return Newf(ErrCodeVaultLocked, "vault is locked by another process").
		WithContext("vault_path", path).
		WithSeverity(SeverityWarning)
}

func NewVaultCorruptError(path string, cause error) *VaultError {
	return Wrap(ErrCodeVaultCorrupt, "vault data is corrupted", cause).
		WithContext("vault_path", path).
		WithSeverity(SeverityCritical)
}

func NewVaultInvalidPathError(path string, cause error) *VaultError {
	return Wrap(ErrCodeVaultInvalidPath, "invalid vault path", cause).
		WithContext("path", path).
		WithSeverity(SeverityError)
}

// Authentication Error Builders
func NewAuthFailedError(details string) *VaultError {
	return New(ErrCodeAuthFailed, "authentication failed").
		WithDetails(details).
		WithSeverity(SeverityError)
}

func NewYubikeyNotFoundError() *VaultError {
	return New(ErrCodeYubikeyNotFound, "YubiKey not found or not connected").
		WithDetails("Please ensure your YubiKey is connected and recognized by the system").
		WithSeverity(SeverityError)
}

func NewYubikeyAuthError(details string) *VaultError {
	return New(ErrCodeYubikeyAuth, "YubiKey authentication failed").
		WithDetails(details).
		WithSeverity(SeverityError)
}

func NewYubikeyConfigError(details string) *VaultError {
	return New(ErrCodeYubikeyConfig, "YubiKey configuration error").
		WithDetails(details).
		WithSeverity(SeverityError)
}

// Wallet Error Builders
func NewWalletNotFoundError(prefix, vaultName string) *VaultError {
	return Newf(ErrCodeWalletNotFound, "wallet '%s' not found in vault '%s'", prefix, vaultName).
		WithContext("wallet_prefix", prefix).
		WithContext("vault_name", vaultName).
		WithSeverity(SeverityError)
}

func NewWalletExistsError(prefix string) *VaultError {
	return Newf(ErrCodeWalletExists, "wallet '%s' already exists", prefix).
		WithContext("wallet_prefix", prefix).
		WithSeverity(SeverityError)
}

func NewWalletInvalidError(prefix, reason string) *VaultError {
	return Newf(ErrCodeWalletInvalid, "wallet '%s' is invalid", prefix).
		WithDetails(reason).
		WithContext("wallet_prefix", prefix).
		WithSeverity(SeverityError)
}

func NewAddressNotFoundError(prefix string, index int) *VaultError {
	return Newf(ErrCodeAddressNotFound, "address with index %d not found in wallet '%s'", index, prefix).
		WithContext("wallet_prefix", prefix).
		WithContext("address_index", index).
		WithSeverity(SeverityError)
}

// Input Validation Error Builders
func NewInvalidInputError(input, reason string) *VaultError {
	return New(ErrCodeInvalidInput, "invalid input provided").
		WithDetails(reason).
		WithContext("input", input).
		WithSeverity(SeverityError)
}

func NewInvalidPrefixError(prefix, reason string) *VaultError {
	return Newf(ErrCodeInvalidPrefix, "invalid prefix '%s'", prefix).
		WithDetails(reason).
		WithContext("prefix", prefix).
		WithSeverity(SeverityError)
}

func NewInvalidKeyError(keyType, reason string) *VaultError {
	return Newf(ErrCodeInvalidKey, "invalid %s key", keyType).
		WithDetails(reason).
		WithContext("key_type", keyType).
		WithSeverity(SeverityError)
}

func NewInvalidMnemonicError(reason string) *VaultError {
	return New(ErrCodeInvalidMnemonic, "invalid mnemonic phrase").
		WithDetails(reason).
		WithSeverity(SeverityError)
}

// System Error Builders
func NewFileSystemError(operation, path string, cause error) *VaultError {
	return Wrap(ErrCodeFileSystem, fmt.Sprintf("filesystem operation '%s' failed", operation), cause).
		WithContext("operation", operation).
		WithContext("path", path).
		WithSeverity(SeverityError)
}

func NewPermissionError(path string, cause error) *VaultError {
	return Wrap(ErrCodePermission, "permission denied", cause).
		WithContext("path", path).
		WithSeverity(SeverityError)
}

func NewDependencyError(dependency, details string) *VaultError {
	return Newf(ErrCodeDependency, "required dependency '%s' is missing or not working", dependency).
		WithDetails(details).
		WithContext("dependency", dependency).
		WithSeverity(SeverityCritical)
}

func NewClipboardError(cause error) *VaultError {
	return Wrap(ErrCodeClipboard, "clipboard operation failed", cause).
		WithSeverity(SeverityWarning)
}

func NewTimeoutError(operation string, duration string) *VaultError {
	return Newf(ErrCodeTimeout, "operation '%s' timed out", operation).
		WithDetails(fmt.Sprintf("timeout after %s", duration)).
		WithContext("operation", operation).
		WithContext("timeout", duration).
		WithSeverity(SeverityError)
}

// Import/Export Error Builders
func NewImportFailedError(format, reason string, cause error) *VaultError {
	return Wrap(ErrCodeImportFailed, fmt.Sprintf("import failed for format '%s'", format), cause).
		WithDetails(reason).
		WithContext("format", format).
		WithSeverity(SeverityError)
}

func NewExportFailedError(format, reason string, cause error) *VaultError {
	return Wrap(ErrCodeExportFailed, fmt.Sprintf("export failed for format '%s'", format), cause).
		WithDetails(reason).
		WithContext("format", format).
		WithSeverity(SeverityError)
}

func NewFormatInvalidError(format, details string) *VaultError {
	return Newf(ErrCodeFormatInvalid, "invalid format '%s'", format).
		WithDetails(details).
		WithContext("format", format).
		WithSeverity(SeverityError)
}

// Helper functions for common error scenarios
func NewActiveVaultNotSetError() *VaultError {
	return New(ErrCodeConfigMissing, "no active vault is set").
		WithDetails("Use 'vault.module vaults use <n>' to set an active vault").
		WithSeverity(SeverityError)
}

func NewProgrammaticModeError(command string) *VaultError {
	return Newf(ErrCodeUnavailable, "command '%s' is not available in programmatic mode", command).
		WithContext("command", command).
		WithSeverity(SeverityError)
}

// Error conversion helpers
func FromOSError(err error, path string) *VaultError {
	if err == nil {
		return nil
	}

	if os.IsNotExist(err) {
		return NewFileSystemError("access", path, err).
			WithDetails("file or directory does not exist")
	}
	
	if os.IsPermission(err) {
		return NewPermissionError(path, err)
	}

	return NewFileSystemError("unknown", path, err)
}

func FromValidationError(field, value, message string) *VaultError {
	return NewConfigValidationError(field, value, message)
}

// ParseYubiKeyError converts YubiKey plugin errors to VaultError with sanitized output
func ParseYubiKeyError(cause error, stderr string) *VaultError {
	stderrStr := strings.ToLower(stderr)

	if strings.Contains(stderrStr, "pin") || strings.Contains(stderrStr, "authentication") {
		return NewYubikeyAuthError("PIN verification failed. Please check your PIN")
	}

	if strings.Contains(stderrStr, "not found") || strings.Contains(stderrStr, "no device") {
		return NewYubikeyNotFoundError()
	}

	// Always sanitize stderr content before including in error details
	// Note: stderr should already be sanitized by caller, but double-check for safety
	sanitizedStderr := sanitizeYubikeyErrorOutput(stderr)
	return NewYubikeyConfigError(sanitizedStderr)
}

// sanitizeYubikeyErrorOutput provides additional sanitization specifically for YubiKey errors
func sanitizeYubikeyErrorOutput(output string) string {
	// YubiKey-specific sensitive patterns
	sensitivePatterns := []string{
		"pin", "piv", "oath", "fido", "certificate",
		"private key", "public key", "secret", "credential",
		"age1", "yubikey identity", "slot",
		"touch", "user presence", "authenticate",
	}
	
	lines := strings.Split(output, "\n")
	sanitized := make([]string, 0, len(lines))
	
	for _, line := range lines {
		lowerLine := strings.ToLower(strings.TrimSpace(line))
		
		if lowerLine == "" {
			sanitized = append(sanitized, line)
			continue
		}
		
		containsSensitive := false
		for _, pattern := range sensitivePatterns {
			if strings.Contains(lowerLine, pattern) {
				containsSensitive = true
				break
			}
		}
		
		if containsSensitive {
			sanitized = append(sanitized, "[REDACTED YUBIKEY INFO]")
		} else {
			// Keep general error messages that don't contain sensitive info
			sanitized = append(sanitized, line)
		}
	}
	
	return strings.Join(sanitized, "\n")
}
