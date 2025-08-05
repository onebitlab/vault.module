// File: main.go
package main

import (
	"fmt"
	"os"

	"vault.module/cmd"
	"vault.module/internal"
	"vault.module/internal/errors"
	"vault.module/internal/shutdown"
)

func main() {
	// Initialize package integration (security <-> shutdown)
	internal.InitializeIntegration()
	
	// Initialize the graceful shutdown manager
	shutdownManager := shutdown.GetManager()
	
	// Defer shutdown to ensure cleanup happens even on normal exit
	defer func() {
		if !shutdownManager.IsShutdown() {
			shutdownManager.Shutdown()
		}
	}()
	
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
		
		// Ensure cleanup happens before exit
		if !shutdownManager.IsShutdown() {
			shutdownManager.Shutdown()
		}
		
		os.Exit(1)
	}
}
