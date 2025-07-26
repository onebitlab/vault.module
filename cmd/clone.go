// File: cmd/clone.go
package cmd

import (
	"fmt"

	"vault.module/internal/actions"
	"vault.module/internal/colors"
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
			return fmt.Errorf(colors.SafeColor(
				"this command is not available in programmatic mode",
				colors.Error,
			))
		}
		outputFile := args[0]

		if len(clonePrefixes) == 0 {
			return fmt.Errorf(colors.SafeColor(
				"at least one prefix must be specified using the --prefix flag",
				colors.Error,
			))
		}

		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Active Vault: %s (Type: %s)", config.Cfg.ActiveVault, activeVault.Type),
			colors.Info,
		))

		// FIX: Pass the whole activeVault struct
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("failed to load active vault '%s': %w", config.Cfg.ActiveVault, err),
				colors.Error,
			))
		}

		clonedVault, err := actions.CloneVault(v, clonePrefixes)
		if err != nil {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("cloning error: %w", err),
				colors.Error,
			))
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
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("failed to save new vault to '%s': %w", outputFile, err),
				colors.Error,
			))
		}

		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Isolated vault successfully created at '%s' containing %d wallets from '%s'.", outputFile, len(clonedVault), config.Cfg.ActiveVault),
			colors.Success,
		))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cloneCmd)
	cloneCmd.Flags().StringSliceVar(&clonePrefixes, "prefix", []string{}, "Prefixes of wallets to include in the cloned vault (can be specified multiple times).")
}
