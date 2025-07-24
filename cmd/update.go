// File: cmd/update.go
package cmd

import (
	"fmt"

	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var updateIndex int

var updateCmd = &cobra.Command{
	Use:   "update <PREFIX>",
	Short: "Updates metadata (notes or label) in the active vault.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}

		if programmaticMode {
			return fmt.Errorf("this command is not available in programmatic mode")
		}
		prefix := args[0]

		fmt.Printf("ℹ️  Active Vault: %s (Type: %s)\n", config.Cfg.ActiveVault, activeVault.Type)

		// FIX: Pass the whole activeVault struct
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf("failed to load vault: %w", err)
		}

		wallet, exists := v[prefix]
		if !exists {
			return fmt.Errorf("wallet with prefix '%s' not found", prefix)
		}

		if updateIndex > -1 {
			var addressToUpdate *vault.Address
			for i := range wallet.Addresses {
				if wallet.Addresses[i].Index == updateIndex {
					addressToUpdate = &wallet.Addresses[i]
					break
				}
			}
			if addressToUpdate == nil {
				return fmt.Errorf("address with index %d not found in wallet '%s'", updateIndex, prefix)
			}
			// Ранее здесь обновлялся label, теперь ничего не делаем
		} else {
			newNotes, err := askForInput(fmt.Sprintf("Enter new notes for wallet '%s'", prefix))
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			wallet.Notes = newNotes
		}

		v[prefix] = wallet
		// FIX: Pass the whole activeVault struct
		if err := vault.SaveVault(activeVault, v); err != nil {
			return fmt.Errorf("failed to save vault: %w", err)
		}

		fmt.Printf("✅ Data for wallet '%s' successfully updated in vault '%s'.\n", prefix, config.Cfg.ActiveVault)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().IntVar(&updateIndex, "index", -1, "Index of the address to update label for (if not set, updates wallet notes).")
}
