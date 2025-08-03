// File: main.go
package main

import (
	"fmt"
	"os"

	"vault.module/cmd"
	"vault.module/internal/errors"
)

func main() {
	// Execute the root command and check for errors.
	if err := cmd.Execute(); err != nil {
		// Use centralized error handling
		if errors.DefaultHandler != nil {
			errorMsg := errors.FormatForUser(err)
			fmt.Fprintln(os.Stderr, "Error:", errorMsg)
		} else {
			// Fallback if error handler not initialized
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}
