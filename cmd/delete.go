// File: cmd/delete.go
package cmd

import (
	"fmt"
	"log/slog"

	"vault.module/internal/audit"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var deleteYes bool

var deleteCmd = &cobra.Command{
	Use:   "delete <PREFIX>",
	Short: "Deletes a wallet from the active vault.",
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
		prefix := args[0]

		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Active Vault: %s (Type: %s)", config.Cfg.ActiveVault, activeVault.Type),
			colors.Info,
		))

		// FIX: Pass the whole activeVault struct
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("failed to load vault: %w", err),
				colors.Error,
			))
		}

		if _, exists := v[prefix]; !exists {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("wallet with prefix '%s' not found", prefix),
				colors.Error,
			))
		}

		if !deleteYes {
			prompt := fmt.Sprintf("Are you sure you want to delete wallet '%s' from vault '%s'? This action is irreversible.", prefix, config.Cfg.ActiveVault)
			if !askForConfirmation(colors.SafeColor(prompt, colors.Warning)) {
				fmt.Println(colors.SafeColor("Cancelled.", colors.Info))
				return nil
			}
		}

		audit.Logger.Warn("Attempting wallet deletion",
			slog.String("command", "delete"),
			slog.String("vault", config.Cfg.ActiveVault),
			slog.String("prefix", prefix),
		)

		delete(v, prefix)

		// FIX: Pass the whole activeVault struct
		if err := vault.SaveVault(activeVault, v); err != nil {
			audit.Logger.Error("Failed to save vault after deletion", "error", err.Error(), "prefix", prefix)
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("failed to save vault: %w", err),
				colors.Error,
			))
		}

		audit.Logger.Info("Wallet deleted successfully", "prefix", prefix, "vault", config.Cfg.ActiveVault)
		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Wallet '%s' successfully deleted from vault '%s'.", prefix, config.Cfg.ActiveVault),
			colors.Success,
		))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().BoolVar(&deleteYes, "yes", false, "Skip confirmation prompt.")
}
