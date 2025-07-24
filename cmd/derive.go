// File: cmd/derive.go
package cmd

import (
	"fmt"

	"vault.module/internal/actions"
	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var deriveCmd = &cobra.Command{
	Use:   "derive <PREFIX>",
	Short: "Derives and adds the next address for a wallet in the active vault.",
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

		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf("failed to load vault: %w", err)
		}

		wallet, exists := v[prefix]
		if !exists {
			return fmt.Errorf("wallet with prefix '%s' not found", prefix)
		}

		// Pass the vault type to the action to use the correct key manager.
		updatedWallet, newAddr, err := actions.DeriveNextAddress(wallet, activeVault.Type)
		if err != nil {
			return fmt.Errorf("derivation error: %w", err)
		}

		v[prefix] = updatedWallet

		if err := vault.SaveVault(activeVault, v); err != nil {
			return fmt.Errorf("failed to save vault: %w", err)
		}

		fmt.Printf("✅ New address (index %d) successfully derived for wallet '%s'.\n", newAddr.Index, prefix)
		fmt.Printf("   Address: %s\n", newAddr.Address)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deriveCmd)
}
