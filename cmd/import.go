// File: cmd/import.go
package cmd

import (
	"errors"
	"fmt"
	"os"

	"vault.module/internal/actions"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/constants"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var importFormat string
var importConflict string
var importYesFlag bool

var importCmd = &cobra.Command{
	Use:   "import <INPUT_FILE>",
	Short: "Bulk imports accounts from a file into the active vault.",
	Long: `Bulk imports accounts from a file into the active vault.

Supported formats:
  - JSON: Standard wallet export format
  - Key-Value: Simple key=value format

The command will prompt for conflict resolution if wallets with same names exist.

Examples:
  vault.module import wallets.json
  vault.module import backup.txt --format keyvalue
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
		filePath := args[0]

		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Active Vault: %s (Type: %s)", config.Cfg.ActiveVault, activeVault.Type),
			colors.Info,
		))

		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to load vault: %s", err.Error()),
				colors.Error,
			))
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to read file '%s': %s", filePath, err.Error()),
				colors.Error,
			))
		}

		// Pass the vault type to the action to use the correct key manager.
		updatedVault, report, err := actions.ImportWallets(v, content, importFormat, importConflict, activeVault.Type)
		if err != nil {
			return err
		}

		if err := vault.SaveVault(activeVault, updatedVault); err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to save vault: %s", err.Error()),
				colors.Error,
			))
		}

		fmt.Println(colors.SafeColor(report, colors.Success))
		return nil
	},
}

func init() {
	// Регистрация перенесена в root.go

	// Настройка флагов
	importCmd.Flags().StringVar(&importFormat, "format", constants.FormatJSON, "File format (json or key-value).")
	importCmd.Flags().StringVar(&importConflict, "on-conflict", constants.ConflictPolicySkip, "Behavior on conflict (skip, overwrite, fail).")
	importCmd.Flags().BoolVar(&importYesFlag, "yes", false, "Import and overwrite without confirmation prompt")
}
