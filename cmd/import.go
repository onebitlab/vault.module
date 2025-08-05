// File: cmd/import.go
package cmd

import (
	"fmt"
	"os"

	"vault.module/internal/actions"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/constants"
	"vault.module/internal/errors"
	"vault.module/internal/security"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var importFormat string
var importConflict string

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
		return errors.WrapCommand(func() error {
			// Check vault status before executing the command
			if err := checkVaultStatus(); err != nil {
				return err
			}

			// Check if shutdown is in progress
			if security.IsShuttingDown() {
				return errors.New(errors.ErrCodeSystem, "system is shutting down, cannot process new commands")
			}

			activeVault, err := config.GetActiveVault()
			if err != nil {
				return err
			}

			if programmaticMode {
				return errors.NewProgrammaticModeError("import")
			}

			filePath := args[0]

			fmt.Println(colors.SafeColor(
				fmt.Sprintf("Active Vault: %s (Type: %s)", config.Cfg.ActiveVault, activeVault.Type),
				colors.Info,
			))

			v, err := vault.LoadVault(activeVault)
			if err != nil {
				return errors.NewVaultLoadError(activeVault.KeyFile, err)
			}

			// Ensure vault secrets are cleared when function exits
			defer func() {
				for _, wallet := range v {
					wallet.Clear()
				}
			}()

			content, err := os.ReadFile(filePath)
			if err != nil {
				return errors.NewFileSystemError("read", filePath, err)
			}

			// Register file content for secure cleanup if it contains sensitive data
			if len(content) > 0 {
				security.RegisterTempFileGlobal(filePath, fmt.Sprintf("import file: %s", filePath))
			}

			// Pass the vault type to the action to use the correct key manager.
			updatedVault, report, err := actions.ImportWallets(v, content, importFormat, importConflict, activeVault.Type)
			if err != nil {
				return err
			}

			if err := vault.SaveVault(activeVault, updatedVault); err != nil {
				return errors.NewVaultSaveError(activeVault.KeyFile, err)
			}

			fmt.Println(colors.SafeColor(report, colors.Success))
			return nil
		})
	},
}

func init() {
	importCmd.Flags().StringVar(&importFormat, "format", constants.FormatJSON, "File format (json or key-value).")
	importCmd.Flags().StringVar(&importConflict, "on-conflict", constants.ConflictPolicySkip, "Behavior on conflict (skip, overwrite, fail).")
}
