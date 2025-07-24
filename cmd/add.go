// File: cmd/add.go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"vault.module/internal/actions"
	"vault.module/internal/config"
	"vault.module/internal/vault"
)

var addCmd = &cobra.Command{
	Use:   "add <PREFIX>",
	Short: "Adds a new wallet to the active vault.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}
		fmt.Printf("ℹ️  Active Vault: %s (Type: %s)\n", config.Cfg.ActiveVault, activeVault.Type)

		if programmaticMode {
			return fmt.Errorf("this command is not available in programmatic mode")
		}
		prefix := args[0]
		if err := actions.ValidatePrefix(prefix); err != nil {
			return fmt.Errorf("invalid prefix: %w", err)
		}
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf("failed to load vault: %w", err)
		}
		if _, exists := v[prefix]; exists {
			return fmt.Errorf("wallet with prefix '%s' already exists", prefix)
		}

		// The prompt is now generic and doesn't mention specific chains.
		choice, err := askForInput("Choose source: 1. Mnemonic (HD-wallet), 2. Private Key (single address)")
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		var newWallet vault.Wallet
		var finalAddress string
		switch choice {
		case "1":
			mnemonic, err := askForSecretInput("Enter your mnemonic phrase")
			if err != nil {
				return err
			}
			// Pass the vault type to the action.
			newWallet, finalAddress, err = actions.CreateWalletFromMnemonic(mnemonic, activeVault.Type)
		case "2":
			pkStr, err := askForSecretInput("Enter your private key")
			if err != nil {
				return err
			}
			// Pass the vault type to the action.
			newWallet, finalAddress, err = actions.CreateWalletFromPrivateKey(pkStr, activeVault.Type)
		default:
			return fmt.Errorf("invalid choice")
		}
		if err != nil {
			return fmt.Errorf("failed to create wallet: %w", err)
		}
		v[prefix] = newWallet
		if err := vault.SaveVault(activeVault, v); err != nil {
			return fmt.Errorf("failed to save vault: %w", err)
		}
		fmt.Printf("✅ Wallet '%s' added successfully to vault '%s'.\n", prefix, config.Cfg.ActiveVault)
		fmt.Printf("   Address: %s\n", finalAddress)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}
