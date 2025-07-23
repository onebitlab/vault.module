// File: main.go
package main

import (
	"fmt"
	"os"

	"vault.module/cmd"
)

func main() {
	// Execute the root command and check for errors.
	if err := cmd.Execute(); err != nil {
		// Print the error to standard error for visibility in scripts and logs.
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
