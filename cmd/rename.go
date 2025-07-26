// File: cmd/rename.go
package cmd

import (
	"fmt"

	"vault.module/internal/actions"
	"vault.module/internal/colors"
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
			return fmt.Errorf(colors.SafeColor(
				"this command is not available in programmatic mode",
				colors.Error,
			))
		}
		oldPrefix := args[0]
		newPrefix := args[1]

		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Active Vault: %s (Type: %s)", config.Cfg.ActiveVault, activeVault.Type),
			colors.Info,
		))

		if err := actions.ValidatePrefix(newPrefix); err != nil {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("invalid new prefix: %w", err),
				colors.Error,
			))
		}

		// FIX: Pass the whole activeVault struct
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("failed to load vault: %w", err),
				colors.Error,
			))
		}

		wallet, exists := v[oldPrefix]
		if !exists {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("wallet with prefix '%s' not found", oldPrefix),
				colors.Error,
			))
		}

		if _, exists := v[newPrefix]; exists {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("a wallet with prefix '%s' already exists", newPrefix),
				colors.Error,
			))
		}

		v[newPrefix] = wallet
		delete(v, oldPrefix)

		// FIX: Pass the whole activeVault struct
		if err := vault.SaveVault(activeVault, v); err != nil {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("failed to save vault: %w", err),
				colors.Error,
			))
		}

		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Wallet '%s' successfully renamed to '%s' in vault '%s'.", oldPrefix, newPrefix, config.Cfg.ActiveVault),
			colors.Success,
		))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(renameCmd)
}
