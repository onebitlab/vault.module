// File: internal/errors/types.go
package errors

import (
	"fmt"
	"log/slog"
)

// ErrorCode represents standardized error codes
type ErrorCode string

const (
	// Configuration errors
	ErrCodeConfigLoad        ErrorCode = "CONFIG_LOAD_FAILED"
	ErrCodeConfigSave        ErrorCode = "CONFIG_SAVE_FAILED"
	ErrCodeConfigValidation  ErrorCode = "CONFIG_VALIDATION_FAILED"
	ErrCodeConfigMissing     ErrorCode = "CONFIG_MISSING"

	// Vault errors
	ErrCodeVaultLoad         ErrorCode = "VAULT_LOAD_FAILED"
	ErrCodeVaultSave         ErrorCode = "VAULT_SAVE_FAILED"
	ErrCodeVaultExists       ErrorCode = "VAULT_EXISTS"
	ErrCodeVaultLocked       ErrorCode = "VAULT_LOCKED"
	ErrCodeVaultCorrupt      ErrorCode = "VAULT_CORRUPT"
	ErrCodeVaultNotFound     ErrorCode = "VAULT_NOT_FOUND"
	ErrCodeVaultInvalidPath  ErrorCode = "VAULT_INVALID_PATH"

	// Authentication errors
	ErrCodeAuthFailed        ErrorCode = "AUTH_FAILED"
	ErrCodeYubikeyNotFound   ErrorCode = "YUBIKEY_NOT_FOUND"
	ErrCodeYubikeyAuth       ErrorCode = "YUBIKEY_AUTH_FAILED"
	ErrCodeYubikeyConfig     ErrorCode = "YUBIKEY_CONFIG_ERROR"

	// Wallet errors
	ErrCodeWalletNotFound    ErrorCode = "WALLET_NOT_FOUND"
	ErrCodeWalletExists      ErrorCode = "WALLET_EXISTS"
	ErrCodeWalletInvalid     ErrorCode = "WALLET_INVALID"
	ErrCodeAddressNotFound   ErrorCode = "ADDRESS_NOT_FOUND"

	// Input validation errors
	ErrCodeInvalidInput      ErrorCode = "INVALID_INPUT"
	ErrCodeInvalidPrefix     ErrorCode = "INVALID_PREFIX"
	ErrCodeInvalidKey        ErrorCode = "INVALID_KEY"
	ErrCodeInvalidMnemonic   ErrorCode = "INVALID_MNEMONIC"

	// System errors
	ErrCodeSystem            ErrorCode = "SYSTEM_ERROR"
	ErrCodeFileSystem        ErrorCode = "FILESYSTEM_ERROR"
	ErrCodePermission        ErrorCode = "PERMISSION_DENIED"
	ErrCodeDependency        ErrorCode = "DEPENDENCY_MISSING"
	ErrCodeClipboard         ErrorCode = "CLIPBOARD_ERROR"
	ErrCodeTimeout           ErrorCode = "TIMEOUT"

	// Import/Export errors
	ErrCodeImportFailed      ErrorCode = "IMPORT_FAILED"
	ErrCodeExportFailed      ErrorCode = "EXPORT_FAILED"
	ErrCodeFormatInvalid     ErrorCode = "FORMAT_INVALID"

	// Generic errors
	ErrCodeInternal          ErrorCode = "INTERNAL_ERROR"
	ErrCodeNotImplemented    ErrorCode = "NOT_IMPLEMENTED"
	ErrCodeUnavailable       ErrorCode = "SERVICE_UNAVAILABLE"
)

// ErrorSeverity represents the severity level of an error
type ErrorSeverity string

const (
	SeverityInfo    ErrorSeverity = "INFO"
	SeverityWarning ErrorSeverity = "WARNING"
	SeverityError   ErrorSeverity = "ERROR"
	SeverityCritical ErrorSeverity = "CRITICAL"
)

// VaultError represents a standardized error structure
type VaultError struct {
	Code      ErrorCode     `json:"code"`
	Message   string        `json:"message"`
	Details   string        `json:"details,omitempty"`
	Severity  ErrorSeverity `json:"severity"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Cause     error         `json:"-"` // Don't serialize the underlying error
}

// Error implements the error interface
func (e *VaultError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for error wrapping
func (e *VaultError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches a specific code
func (e *VaultError) Is(target error) bool {
	if targetErr, ok := target.(*VaultError); ok {
		return e.Code == targetErr.Code
	}
	return false
}

// WithContext adds context information to the error
func (e *VaultError) WithContext(key string, value interface{}) *VaultError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// ToSlogAttrs converts error context to slog attributes
func (e *VaultError) ToSlogAttrs() []slog.Attr {
	attrs := []slog.Attr{
		slog.String("error_code", string(e.Code)),
		slog.String("error_message", e.Message),
		slog.String("severity", string(e.Severity)),
	}

	if e.Details != "" {
		attrs = append(attrs, slog.String("details", e.Details))
	}

	if e.Cause != nil {
		attrs = append(attrs, slog.String("cause", e.Cause.Error()))
	}

	// Add context as individual attributes
	for key, value := range e.Context {
		attrs = append(attrs, slog.Any(fmt.Sprintf("ctx_%s", key), value))
	}

	return attrs
}

// New creates a new VaultError
func New(code ErrorCode, message string) *VaultError {
	return &VaultError{
		Code:     code,
		Message:  message,
		Severity: SeverityError,
		Context:  make(map[string]interface{}),
	}
}

// Newf creates a new VaultError with formatted message
func Newf(code ErrorCode, format string, args ...interface{}) *VaultError {
	return &VaultError{
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
		Severity: SeverityError,
		Context:  make(map[string]interface{}),
	}
}

// Wrap wraps an existing error with VaultError
func Wrap(code ErrorCode, message string, cause error) *VaultError {
	return &VaultError{
		Code:     code,
		Message:  message,
		Severity: SeverityError,
		Context:  make(map[string]interface{}),
		Cause:    cause,
	}
}

// Wrapf wraps an existing error with formatted message
func Wrapf(code ErrorCode, cause error, format string, args ...interface{}) *VaultError {
	return &VaultError{
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
		Severity: SeverityError,
		Context:  make(map[string]interface{}),
		Cause:    cause,
	}
}

// WithSeverity sets the severity level
func (e *VaultError) WithSeverity(severity ErrorSeverity) *VaultError {
	e.Severity = severity
	return e
}

// WithDetails adds detailed information
func (e *VaultError) WithDetails(details string) *VaultError {
	e.Details = details
	return e
}

// IsCode checks if error has specific code
func IsCode(err error, code ErrorCode) bool {
	if vErr, ok := err.(*VaultError); ok {
		return vErr.Code == code
	}
	return false
}

// GetCode extracts error code from error
func GetCode(err error) ErrorCode {
	if vErr, ok := err.(*VaultError); ok {
		return vErr.Code
	}
	return ErrCodeInternal
}

// GetSeverity extracts severity from error
func GetSeverity(err error) ErrorSeverity {
	if vErr, ok := err.(*VaultError); ok {
		return vErr.Severity
	}
	return SeverityError
}
