// File: cmd/add.go
package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"vault.module/internal/actions"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/vault"
)

var addCmd = &cobra.Command{
	Use:   "add <PREFIX>",
	Short: "Adds a new wallet to the active vault.",
	Long: `Adds a new wallet to the active vault.

Examples:
  vault.module add A1
  vault.module add mywallet
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check vault status before executing the command
		if err := checkVaultStatus(); err != nil {
			return err
		}

		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}
		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Active Vault: %s (Type: %s)", config.Cfg.ActiveVault, activeVault.Type),
			colors.Info,
		))

		if programmaticMode {
			return errors.New(colors.SafeColor(
				"this command is not available in programmatic mode",
				colors.Error,
			))
		}
		prefix := args[0]
		if err := actions.ValidatePrefix(prefix); err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("invalid prefix: %s", err.Error()),
				colors.Error,
			))
		}
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to load vault: %s", err.Error()),
				colors.Error,
			))
		}
		// Ensure vault secrets are cleared when function exits
		defer func() {
			for _, wallet := range v {
				wallet.Clear()
			}
		}()
		if _, exists := v[prefix]; exists {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("wallet with prefix '%s' already exists", prefix),
				colors.Error,
			))
		}

		// The prompt is now generic and doesn't mention specific chains.
		choice, err := askForInput("Choose source: 1. Mnemonic (HD-wallet), 2. Private Key (single address)")
		if err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to read input: %s", err.Error()),
				colors.Error,
			))
		}
		if strings.TrimSpace(choice) == "" {
			return errors.New(colors.SafeColor(
				"source choice cannot be empty. Please choose 1 for mnemonic or 2 for private key",
				colors.Error,
			))
		}

		var newWallet vault.Wallet
		var finalAddress string
		switch choice {
		case "1":
			mnemonic, mnemonicErr := askForSecretInput("Enter your mnemonic phrase")
			if mnemonicErr != nil {
				return mnemonicErr
			}
			if strings.TrimSpace(mnemonic) == "" {
				return errors.New(colors.SafeColor(
					"mnemonic phrase cannot be empty",
					colors.Error,
				))
			}
			newWallet, finalAddress, err = actions.CreateWalletFromMnemonic(mnemonic, activeVault.Type)
		case "2":
			pkStr, pkErr := askForSecretInput("Enter your private key")
			if pkErr != nil {
				return pkErr
			}
			if strings.TrimSpace(pkStr) == "" {
				return errors.New(colors.SafeColor(
					"private key cannot be empty",
					colors.Error,
				))
			}
			newWallet, finalAddress, err = actions.CreateWalletFromPrivateKey(pkStr, activeVault.Type)
		default:
			return errors.New(colors.SafeColor(
				fmt.Sprintf("invalid source choice: '%s'. Please choose 1 for mnemonic or 2 for private key", choice),
				colors.Error,
			))
		}
		if err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to create wallet: %s", err.Error()),
				colors.Error,
			))
		}
		v[prefix] = newWallet
		if err := vault.SaveVault(activeVault, v); err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to save vault: %s", err.Error()),
				colors.Error,
			))
		}
		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Wallet '%s' added successfully to vault '%s'.", prefix, config.Cfg.ActiveVault),
			colors.Success,
		))
		fmt.Printf("   Address: %s\n", colors.SafeColor(finalAddress, colors.Cyan))
		return nil
	},
}

func init() {
	// Registration moved to root.go
}
