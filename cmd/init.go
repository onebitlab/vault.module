// File: cmd/init.go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"vault.module/internal/config"
	"vault.module/internal/vault"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initializes the active vault file.",
	RunE: func(cmd *cobra.Command, args []string) error {
		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}

		fmt.Printf("ℹ️  Initializing Vault: %s (Type: %s, Encryption: %s)\n", config.Cfg.ActiveVault, activeVault.Type, activeVault.Encryption)

		if _, err := os.Stat(activeVault.KeyFile); err == nil {
			fmt.Printf("⚠️  Warning: Vault file '%s' for active vault '%s' already exists.\n", activeVault.KeyFile, config.Cfg.ActiveVault)
			if !askForConfirmation("Are you sure you want to overwrite it? ALL DATA WILL BE LOST!") {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		emptyVault := make(vault.Vault)
		if err := vault.SaveVault(activeVault, emptyVault); err != nil {
			return fmt.Errorf("failed to create vault: %w", err)
		}

		fmt.Printf("✅ Vault '%s' successfully initialized at '%s'.\n", config.Cfg.ActiveVault, activeVault.KeyFile)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
