// File: cmd/utils.go
package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/security"
)

// checkVaultStatus checks the vault status and returns clear error messages
func checkVaultStatus() error {
	// Check if there is an active vault
	if config.Cfg.ActiveVault == "" {
		return errors.New(colors.SafeColor(
			"No active vault configured. Please create a vault first with 'vaults add' and activate it with 'vaults use'.",
			colors.Error,
		))
	}

	// Check if the vault exists in the configuration
	activeVault, exists := config.Cfg.Vaults[config.Cfg.ActiveVault]
	if !exists {
		return errors.New(colors.SafeColor(
			fmt.Sprintf("Active vault '%s' not found in configuration. Please check your vaults with 'vaults list' and set active vault with 'vaults use'.",
				config.Cfg.ActiveVault),
			colors.Error,
		))
	}

	// Check if the vault has a type defined
	if activeVault.Type == "" {
		return errors.New(colors.SafeColor(
			fmt.Sprintf("Active vault '%s' has no type defined. Please recreate the vault with 'vaults add'.",
				config.Cfg.ActiveVault),
			colors.Error,
		))
	}

	// Check if the vault file exists
	if _, err := os.Stat(activeVault.KeyFile); os.IsNotExist(err) {
		return errors.New(colors.SafeColor(
			fmt.Sprintf("Vault file '%s' does not exist. Please create the vault with 'vaults add'.",
				activeVault.KeyFile),
			colors.Error,
		))
	}

	return nil
}

// askForConfirmation prompts the user for a yes/no confirmation.
func askForConfirmation(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [y/N]: ", prompt)
		response, err := reader.ReadString('\n')
		if err != nil {
			return false
		}
		response = strings.ToLower(strings.TrimSpace(response))

		// FIX: Replaced if/else if chain with a switch statement for better readability.
		switch response {
		case "y", "yes":
			return true
		case "n", "no", "":
			return false
		}
	}
}

// askForInput prompts the user for data input.
func askForInput(prompt string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s: ", prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// askForSecretInput prompts the user for secret input with clipboard option
func askForSecretInput(prompt string) (string, error) {
	fmt.Printf("Choose input method for %s:\n", prompt)
	fmt.Printf("1. Type manually (input will be hidden)\n")
	fmt.Printf("2. Read from clipboard\n")
	fmt.Printf("Enter choice (1 or 2): ")

	reader := bufio.NewReader(os.Stdin)
	choice, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1":
		// Manual input with hidden characters
		fmt.Printf("Enter %s (input will be hidden): ", prompt)

		// Disable echo
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return "", err
		}
		defer term.Restore(int(os.Stdin.Fd()), oldState)

		// Read input without echo
		var input []byte
		for {
			char := make([]byte, 1)
			_, err := os.Stdin.Read(char)
			if err != nil {
				return "", err
			}

			// Handle special keys
			if char[0] == 13 { // Enter key
				fmt.Println() // Add newline
				break
			} else if char[0] == 127 { // Backspace
				if len(input) > 0 {
					input = input[:len(input)-1]
					fmt.Print("\b \b") // Clear the character
				}
			} else if char[0] >= 32 && char[0] <= 126 { // Printable characters
				input = append(input, char[0])
				fmt.Print("*") // Show asterisk instead of character
			}
		}

		return strings.TrimSpace(string(input)), nil

	case "2":
		// Read from clipboard
		fmt.Println("Reading from clipboard...")
		clipboardData, err := security.ReadClipboard()
		if err != nil {
			return "", fmt.Errorf("failed to read from clipboard: %s", err.Error())
		}
		if clipboardData == "" {
			return "", fmt.Errorf("clipboard is empty")
		}
		fmt.Println("Data read from clipboard successfully.")
		return strings.TrimSpace(clipboardData), nil

	default:
		return "", fmt.Errorf("invalid choice: %s. Please choose 1 or 2", choice)
	}
}
