// File: cmd/rename.go
package cmd

import (
	"fmt"

	"vault.module/internal/actions"
	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename <OLD_PREFIX> <NEW_PREFIX>",
	Short: "Safely renames a wallet in the active vault.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}

		if programmaticMode {
			return fmt.Errorf("this command is not available in programmatic mode")
		}
		oldPrefix := args[0]
		newPrefix := args[1]

		fmt.Printf("ℹ️  Active Vault: %s (Type: %s)\n", config.Cfg.ActiveVault, activeVault.Type)

		if err := actions.ValidatePrefix(newPrefix); err != nil {
			return fmt.Errorf("invalid new prefix: %w", err)
		}

		// FIX: Pass the whole activeVault struct
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf("failed to load vault: %w", err)
		}

		wallet, exists := v[oldPrefix]
		if !exists {
			return fmt.Errorf("wallet with prefix '%s' not found", oldPrefix)
		}

		if _, exists := v[newPrefix]; exists {
			return fmt.Errorf("a wallet with prefix '%s' already exists", newPrefix)
		}

		v[newPrefix] = wallet
		delete(v, oldPrefix)

		// FIX: Pass the whole activeVault struct
		if err := vault.SaveVault(activeVault, v); err != nil {
			return fmt.Errorf("failed to save vault: %w", err)
		}

		fmt.Printf("✅ Wallet '%s' successfully renamed to '%s' in vault '%s'.\n", oldPrefix, newPrefix, config.Cfg.ActiveVault)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(renameCmd)
}
