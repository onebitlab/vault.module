// File: cmd/export.go
package cmd

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"vault.module/internal/actions"
	"vault.module/internal/audit"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/errors"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var exportYes bool

var exportCmd = &cobra.Command{
	Use:   "export [OUTPUT_FILE]",
	Short: "Exports all accounts from the active vault to an unencrypted JSON file.",
	Long: `Exports all accounts from the active vault to an unencrypted JSON file.

This command exports all wallets and their data to a JSON file.
The exported file will be unencrypted, so handle it with care.
If no output file is specified, it will create a file in the vault directory.

Examples:
  vault.module export                    # Export to vault_directory/export.json
  vault.module export wallets.json       # Export to specific file
  vault.module export backup.json --yes  # Export with confirmation skip
`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.WrapCommand(func() error {
			// Check vault status before executing the command
			if err := checkVaultStatus(); err != nil {
				return err
			}

			activeVault, err := config.GetActiveVault()
			if err != nil {
				return err
			}

			if programmaticMode {
				return errors.NewProgrammaticModeError("export")
			}

			// Determine output file
			var outputFile string
			if len(args) > 0 {
				outputFile = args[0]
			} else {
				// Generate default filename in vault directory
				vaultDir := filepath.Dir(activeVault.KeyFile)
				outputFile = filepath.Join(vaultDir, "export.json")
			}

			if _, err := os.Stat(outputFile); err == nil && !exportYes {
				fmt.Printf("File '%s' already exists. Overwrite? [y/N]: ", outputFile)
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

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

			if len(v) == 0 {
				fmt.Println(colors.SafeColor(
					fmt.Sprintf("Vault '%s' is empty. Nothing to export.", config.Cfg.ActiveVault),
					colors.Info,
				))
				return nil
			}

			if !exportYes {
				if !askForConfirmation(colors.SafeColor(
					"WARNING: You are about to create an unencrypted copy of all secrets from the active vault. Are you sure?",
					colors.Warning,
				)) {
					fmt.Println(colors.SafeColor("Cancelled.", colors.Info))
					return nil
				}
			}

			audit.Logger.Error("Executing plaintext export of an entire vault",
				slog.String("command", "export"),
				slog.String("vault", config.Cfg.ActiveVault),
				slog.String("destination_file", outputFile),
			)

			jsonData, err := actions.ExportVault(v)
			if err != nil {
				return errors.NewExportFailedError("json", "failed to generate JSON for export", err)
			}

			if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
				return errors.NewFileSystemError("write", outputFile, err)
			}

			audit.Logger.Info("Plaintext export completed successfully", "destination_file", outputFile)
			fmt.Println(colors.SafeColor(
				fmt.Sprintf("All wallets (%d) from vault '%s' successfully exported to '%s'.", len(v), config.Cfg.ActiveVault, outputFile),
				colors.Success,
			))
			return nil
		})
	},
}

func init() {
	exportCmd.Flags().BoolVar(&exportYes, "yes", false, "Skip confirmation prompt.")
}
