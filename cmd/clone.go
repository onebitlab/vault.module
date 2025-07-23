// File: cmd/clone.go
package cmd

import (
	"fmt"
	"vault.module/internal/actions"
	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var clonePrefixes []string

var cloneCmd = &cobra.Command{
	Use:   "clone <OUTPUT_FILE_PATH>",
	Short: "Creates a new, isolated vault from the active vault.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}

		if programmaticMode {
			return fmt.Errorf("this command is not available in programmatic mode")
		}
		outputFile := args[0]

		if len(clonePrefixes) == 0 {
			return fmt.Errorf("at least one prefix must be specified using the --prefix flag")
		}

		fmt.Printf("ℹ️  Active Vault: %s (Type: %s)\n", config.Cfg.ActiveVault, activeVault.Type)

		// FIX: Pass the whole activeVault struct
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf("failed to load active vault '%s': %w", config.Cfg.ActiveVault, err)
		}

		clonedVault, err := actions.CloneVault(v, clonePrefixes)
		if err != nil {
			return fmt.Errorf("cloning error: %w", err)
		}

		// Create a temporary VaultDetails for the new file, inheriting the active vault's properties.
		clonedVaultDetails := config.VaultDetails{
			KeyFile:        outputFile,
			RecipientsFile: activeVault.RecipientsFile,
			Encryption:     activeVault.Encryption,
			Type:           activeVault.Type,
		}

		// FIX: Pass the new details struct and the cloned vault data
		if err := vault.SaveVault(clonedVaultDetails, clonedVault); err != nil {
			return fmt.Errorf("failed to save new vault to '%s': %w", outputFile, err)
		}

		fmt.Printf("✅ Isolated vault successfully created at '%s' containing %d wallets from '%s'.\n", outputFile, len(clonedVault), config.Cfg.ActiveVault)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cloneCmd)
	cloneCmd.Flags().StringSliceVar(&clonePrefixes, "prefix", []string{}, "Prefix of a wallet to add to the new vault (can be specified multiple times).")
}