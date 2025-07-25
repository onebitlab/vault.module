// internal/tui/utils/error_handler.go
package utils

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// ErrorLevel represents the severity of an error
type ErrorLevel int

const (
	ErrorLevelInfo ErrorLevel = iota
	ErrorLevelWarning
	ErrorLevelError
	ErrorLevelCritical
)

// AppError represents an application error with context
type AppError struct {
	Level     ErrorLevel
	Message   string
	Details   string
	Timestamp time.Time
	Context   map[string]interface{}
	Cause     error
}

// Error implements the error interface
func (e *AppError) Error() string {
	return e.Message
}

// ErrorHandler manages error handling and logging
type ErrorHandler struct {
	errors    []AppError
	maxErrors int
}

var (
	errorHandler *ErrorHandler
	errorOnce    sync.Once
)

// GetErrorHandler returns the singleton error handler
func GetErrorHandler() *ErrorHandler {
	errorOnce.Do(func() {
		errorHandler = &ErrorHandler{
			errors:    make([]AppError, 0),
			maxErrors: 100,
		}
	})
	return errorHandler
}

// HandleError handles an error with the specified level
func (eh *ErrorHandler) HandleError(level ErrorLevel, message string, err error) *AppError {
	appError := &AppError{
		Level:     level,
		Message:   message,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
		Cause:     err,
	}

	if err != nil {
		appError.Details = err.Error()
	}

	// Add to error list
	eh.addError(*appError)

	// Log the error
	eh.logError(*appError)

	return appError
}

// HandleErrorWithContext handles an error with additional context
func (eh *ErrorHandler) HandleErrorWithContext(level ErrorLevel, message string, err error, context map[string]interface{}) *AppError {
	appError := eh.HandleError(level, message, err)
	appError.Context = context
	return appError
}

// addError adds an error to the list
func (eh *ErrorHandler) addError(err AppError) {
	eh.errors = append(eh.errors, err)

	// Keep only the last maxErrors
	if len(eh.errors) > eh.maxErrors {
		eh.errors = eh.errors[1:]
	}
}

// logError logs an error
func (eh *ErrorHandler) logError(err AppError) {
	levelStr := eh.getLevelString(err.Level)
	logMsg := fmt.Sprintf("[%s] %s: %s", levelStr, err.Timestamp.Format("2006-01-02 15:04:05"), err.Message)

	if err.Details != "" {
		logMsg += " - " + err.Details
	}

	// Log to appropriate destination based on level
	switch err.Level {
	case ErrorLevelCritical, ErrorLevelError:
		log.Printf("ERROR: %s", logMsg)
	case ErrorLevelWarning:
		log.Printf("WARNING: %s", logMsg)
	case ErrorLevelInfo:
		log.Printf("INFO: %s", logMsg)
	}
}

// getLevelString returns the string representation of an error level
func (eh *ErrorHandler) getLevelString(level ErrorLevel) string {
	switch level {
	case ErrorLevelInfo:
		return "INFO"
	case ErrorLevelWarning:
		return "WARNING"
	case ErrorLevelError:
		return "ERROR"
	case ErrorLevelCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// GetRecentErrors returns recent errors
func (eh *ErrorHandler) GetRecentErrors(count int) []AppError {
	if count <= 0 || count > len(eh.errors) {
		count = len(eh.errors)
	}

	start := len(eh.errors) - count
	return eh.errors[start:]
}

// GetErrorsByLevel returns errors of a specific level
func (eh *ErrorHandler) GetErrorsByLevel(level ErrorLevel) []AppError {
	var filtered []AppError
	for _, err := range eh.errors {
		if err.Level == level {
			filtered = append(filtered, err)
		}
	}
	return filtered
}

// ClearErrors clears all stored errors
func (eh *ErrorHandler) ClearErrors() {
	eh.errors = make([]AppError, 0)
}

// FormatErrorForDisplay formats an error for display in TUI
func (eh *ErrorHandler) FormatErrorForDisplay(err AppError) string {
	var parts []string

	// Add timestamp
	parts = append(parts, err.Timestamp.Format("15:04:05"))

	// Add level
	parts = append(parts, eh.getLevelString(err.Level))

	// Add message
	parts = append(parts, err.Message)

	return strings.Join(parts, " | ")
}

// Convenience functions for different error levels
func HandleInfo(message string) *AppError {
	return GetErrorHandler().HandleError(ErrorLevelInfo, message, nil)
}

func HandleWarning(message string, err error) *AppError {
	return GetErrorHandler().HandleError(ErrorLevelWarning, message, err)
}

func HandleError(message string, err error) *AppError {
	return GetErrorHandler().HandleError(ErrorLevelError, message, err)
}

func HandleCritical(message string, err error) *AppError {
	return GetErrorHandler().HandleError(ErrorLevelCritical, message, err)
}

// Validation error helpers
func NewValidationError(field, message string) *AppError {
	return &AppError{
		Level:     ErrorLevelError,
		Message:   fmt.Sprintf("Validation error in %s: %s", field, message),
		Timestamp: time.Now(),
		Context:   map[string]interface{}{"field": field},
	}
}

func NewSecurityError(message string) *AppError {
	return &AppError{
		Level:     ErrorLevelCritical,
		Message:   "Security error: " + message,
		Timestamp: time.Now(),
		Context:   map[string]interface{}{"type": "security"},
	}
}
