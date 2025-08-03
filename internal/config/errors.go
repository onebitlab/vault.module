// File: internal/config/errors.go
package config

import "fmt"

// This file is deprecated. Error handling has been moved to internal/errors package.
// Use errors.NewConfigValidationError() instead of ConfigError.

// ConfigError is deprecated. Use internal/errors package instead.
type ConfigError struct {
	Field   string
	Value   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error: field '%s', value '%s': %s", e.Field, e.Value, e.Message)
}

// NewConfigError is deprecated. Use errors.NewConfigValidationError() instead.
func NewConfigError(field, value, msg string) error {
	return &ConfigError{Field: field, Value: value, Message: msg}
}
