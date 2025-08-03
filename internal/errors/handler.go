// File: internal/errors/handler.go
package errors

import (
	"log/slog"
	"os"

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

// logError logs the error with appropriate level based on severity
func (h *Handler) logError(vErr *VaultError) {
	attrs := vErr.ToSlogAttrs()

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
