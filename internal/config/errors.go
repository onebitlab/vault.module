package config

import "fmt"

type ConfigError struct {
	Field   string
	Value   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error: field '%s', value '%s': %s", e.Field, e.Value, e.Message)
}

func NewConfigError(field, value, msg string) error {
	return &ConfigError{Field: field, Value: value, Message: msg}
}
