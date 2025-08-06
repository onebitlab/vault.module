// File: internal/errors/handler.go
package errors

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"vault.module/internal/audit"
	"vault.module/internal/colors"
)

// Handler provides centralized error handling functionality
type Handler struct {
	logger *slog.Logger
}

// DefaultHandler is the global error handler instance
var DefaultHandler *Handler

// InitHandler initializes the global error handler
func InitHandler(logger *slog.Logger) {
	DefaultHandler = &Handler{
		logger: logger,
	}
}

// Handle processes an error with logging and optional context
func (h *Handler) Handle(err error) {
	if err == nil {
		return
	}

	var vErr *VaultError
	if !AsVaultError(err, &vErr) {
		// Convert non-VaultError to VaultError
		vErr = Wrap(ErrCodeInternal, "unexpected error occurred", err)
	}

	// Log the error with structured logging
	h.logError(vErr)
}

// HandleWithExit processes an error and exits the program if severity is critical
func (h *Handler) HandleWithExit(err error) {
	if err == nil {
		return
	}

	h.Handle(err)

	if vErr, ok := err.(*VaultError); ok && vErr.Severity == SeverityCritical {
		os.Exit(1)
	}
}

// logError logs the error with appropriate level based on severity and sanitizes sensitive content
func (h *Handler) logError(vErr *VaultError) {
	// Sanitize error attributes before logging for sensitive error types
	attrs := h.sanitizeErrorAttributes(vErr)

	switch vErr.Severity {
	case SeverityInfo:
		h.logger.LogAttrs(nil, slog.LevelInfo, "Operation info", attrs...)
	case SeverityWarning:
		h.logger.LogAttrs(nil, slog.LevelWarn, "Operation warning", attrs...)
	case SeverityError:
		h.logger.LogAttrs(nil, slog.LevelError, "Operation error", attrs...)
	case SeverityCritical:
		h.logger.LogAttrs(nil, slog.LevelError, "Critical error", attrs...)
	default:
		h.logger.LogAttrs(nil, slog.LevelError, "Unknown severity error", attrs...)
	}
}

// sanitizeErrorAttributes sanitizes sensitive information in error attributes before logging
func (h *Handler) sanitizeErrorAttributes(vErr *VaultError) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("error_code", string(vErr.Code)),
		slog.String("error_message", vErr.Message),
		slog.String("severity", string(vErr.Severity)),
	}

	// Sanitize details for sensitive error types
	if vErr.Details != "" {
		sanitizedDetails := vErr.Details
		// Apply extra sanitization for YubiKey-related errors
		if isYubiKeyRelatedError(vErr.Code) {
			sanitizedDetails = h.sanitizeYubiKeyDetails(vErr.Details)
		}
		attrs = append(attrs, slog.String("details", sanitizedDetails))
	}

	if vErr.Cause != nil {
		// Sanitize the cause error message as well
		causeMsg := vErr.Cause.Error()
		if isYubiKeyRelatedError(vErr.Code) {
			causeMsg = h.sanitizeYubiKeyDetails(causeMsg)
		}
		attrs = append(attrs, slog.String("cause", causeMsg))
	}

	// Add context as individual attributes with sanitization
	for key, value := range vErr.Context {
		// Convert value to string for sanitization if needed
		valueStr := fmt.Sprintf("%v", value)
		if isYubiKeyRelatedError(vErr.Code) && isSensitiveContextKey(key) {
			valueStr = "[REDACTED]"
		}
		attrs = append(attrs, slog.Any(fmt.Sprintf("ctx_%s", key), valueStr))
	}

	return attrs
}

// isYubiKeyRelatedError checks if error code is related to YubiKey operations
func isYubiKeyRelatedError(code ErrorCode) bool {
	return code == ErrCodeYubikeyNotFound ||
		code == ErrCodeYubikeyAuth ||
		code == ErrCodeYubikeyConfig ||
		code == ErrCodeAuthFailed
}

// isSensitiveContextKey checks if context key might contain sensitive information
func isSensitiveContextKey(key string) bool {
	lowerKey := strings.ToLower(key)
	sensitiveKeys := []string{
		"pin", "password", "key", "secret", "token",
		"credential", "auth", "session", "identity",
		"stderr", "output", "response",
	}
	
	for _, sensitiveKey := range sensitiveKeys {
		if strings.Contains(lowerKey, sensitiveKey) {
			return true
		}
	}
	return false
}

// sanitizeYubiKeyDetails applies YubiKey-specific sanitization to error details
func (h *Handler) sanitizeYubiKeyDetails(details string) string {
	// Use similar logic to sanitizeYubikeyErrorOutput
	sensitivePatterns := []string{
		"pin", "piv", "oath", "fido", "certificate",
		"private key", "public key", "secret", "credential",
		"age1", "yubikey identity", "slot",
		"touch", "user presence", "authenticate",
		"-----begin", "-----end",
	}
	
	lowerDetails := strings.ToLower(details)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerDetails, pattern) {
			return "[REDACTED YUBIKEY ERROR DETAILS]"
		}
	}
	
	return details
}

// FormatForUser formats error for user display
func (h *Handler) FormatForUser(err error) string {
	if err == nil {
		return ""
	}

	var vErr *VaultError
	if !AsVaultError(err, &vErr) {
		return colors.SafeColor("An unexpected error occurred", colors.Error)
	}

	// Format based on severity
	var colorFunc func(string) string
	switch vErr.Severity {
	case SeverityInfo:
		colorFunc = colors.Info
	case SeverityWarning:
		colorFunc = colors.Warning
	case SeverityError, SeverityCritical:
		colorFunc = colors.Error
	default:
		colorFunc = colors.Error
	}

	message := vErr.Message
	if vErr.Details != "" {
		message += " (" + vErr.Details + ")"
	}

	return colors.SafeColor(message, colorFunc)
}

// HandleAndFormat handles error and returns formatted message for user
func (h *Handler) HandleAndFormat(err error) string {
	if err == nil {
		return ""
	}

	h.Handle(err)
	return h.FormatForUser(err)
}

// Global convenience functions
func Handle(err error) {
	if DefaultHandler != nil {
		DefaultHandler.Handle(err)
	}
}

func HandleWithExit(err error) {
	if DefaultHandler != nil {
		DefaultHandler.HandleWithExit(err)
	}
}

func FormatForUser(err error) string {
	if DefaultHandler != nil {
		return DefaultHandler.FormatForUser(err)
	}
	return colors.SafeColor("Error handler not initialized", colors.Error)
}

func HandleAndFormat(err error) string {
	if DefaultHandler != nil {
		return DefaultHandler.HandleAndFormat(err)
	}
	return colors.SafeColor("Error handler not initialized", colors.Error)
}

// AsVaultError checks if error can be converted to VaultError
func AsVaultError(err error, target **VaultError) bool {
	if vErr, ok := err.(*VaultError); ok {
		*target = vErr
		return true
	}
	return false
}

// Command wrapper for consistent error handling
type CommandResult struct {
	Error error
	Data  interface{}
}

// WrapCommand wraps command execution with consistent error handling
func WrapCommand(fn func() error) error {
	defer func() {
		if r := recover(); r != nil {
			// Convert panic to VaultError
			vErr := New(ErrCodeInternal, "unexpected panic occurred").
				WithSeverity(SeverityCritical).
				WithDetails("panic recovered in command execution")

			Handle(vErr)
		}
	}()

	if err := fn(); err != nil {
		Handle(err)
		return err
	}

	return nil
}

// InitWithAuditLogger initializes error handler with audit logger
func InitWithAuditLogger() error {
	if audit.Logger == nil {
		return New(ErrCodeInternal, "audit logger not initialized")
	}

	InitHandler(audit.Logger)
	return nil
}
