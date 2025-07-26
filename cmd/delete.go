// File: cmd/delete.go
package cmd

import (
	"errors"
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
	Long: `Deletes a wallet from the active vault.

This command will permanently remove the specified wallet and all its data.
You will be prompted for confirmation unless --yes flag is used.

Examples:
  vault.module delete A1
  vault.module delete mywallet --yes
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Проверяем состояние vault перед выполнением команды
		if err := checkVaultStatus(); err != nil {
			return err
		}

		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}

		if programmaticMode {
			return errors.New(colors.SafeColor(
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
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to load vault: %s", err.Error()),
				colors.Error,
			))
		}

		if _, exists := v[prefix]; !exists {
			return errors.New(colors.SafeColor(
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
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to save vault: %s", err.Error()),
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
	// Регистрация перенесена в root.go

	// Настройка флагов
	deleteCmd.Flags().BoolVar(&deleteYes, "yes", false, "Delete without confirmation prompt")
}
