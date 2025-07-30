// File: internal/audit/audit.go
package audit

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger

// InitLogger initializes the logger for auditing purposes.
func InitLogger() error {
	// Open or create the log file for appending.
	logFile, err := os.OpenFile("audit.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	// Create a logger that writes JSON to the specified file.
	Logger = slog.New(slog.NewJSONHandler(logFile, nil))
	return nil
}
