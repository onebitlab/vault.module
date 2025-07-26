// File: cmd/utils.go
package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"vault.module/internal/colors"
	"vault.module/internal/config"
)

// checkVaultStatus проверяет состояние vault и возвращает понятные сообщения об ошибках
func checkVaultStatus() error {
	// Проверяем, есть ли активный vault
	if config.Cfg.ActiveVault == "" {
		return errors.New(colors.SafeColor(
			"No active vault configured. Please create a vault first with 'vaults add' and activate it with 'vaults use'.",
			colors.Error,
		))
	}

	// Проверяем, существует ли vault в конфигурации
	activeVault, exists := config.Cfg.Vaults[config.Cfg.ActiveVault]
	if !exists {
		return errors.New(colors.SafeColor(
			fmt.Sprintf("Active vault '%s' not found in configuration. Please check your vaults with 'vaults list' and set active vault with 'vaults use'.",
				config.Cfg.ActiveVault),
			colors.Error,
		))
	}

	// Проверяем, есть ли тип vault
	if activeVault.Type == "" {
		return errors.New(colors.SafeColor(
			fmt.Sprintf("Active vault '%s' has no type defined. Please recreate the vault with 'vaults add'.",
				config.Cfg.ActiveVault),
			colors.Error,
		))
	}

	// Проверяем, существует ли файл vault
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
		fmt.Printf("%s [y/n]: ", prompt)
		response, err := reader.ReadString('\n')
		if err != nil {
			return false
		}
		response = strings.ToLower(strings.TrimSpace(response))

		// FIX: Replaced if/else if chain with a switch statement for better readability.
		switch response {
		case "y", "yes":
			return true
		case "n", "no":
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


