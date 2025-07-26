// File: cmd/export.go
package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"vault.module/internal/actions"
	"vault.module/internal/audit"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var exportYes bool

var exportCmd = &cobra.Command{
	Use:   "export <OUTPUT_FILE>",
	Short: "Exports all accounts from the active vault to an unencrypted JSON file.",
	Long: `Exports all accounts from the active vault to an unencrypted JSON file.

This command exports all wallets and their data to a JSON file.
The exported file will be unencrypted, so handle it with care.

Examples:
  vault.module export wallets.json
  vault.module export backup.json --yes
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
		outputFile := args[0]

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

		// FIX: Pass the whole activeVault struct
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to load vault: %s", err.Error()),
				colors.Error,
			))
		}

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
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to generate JSON for export: %s", err.Error()),
				colors.Error,
			))
		}

		if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to write to file '%s': %s", outputFile, err.Error()),
				colors.Error,
			))
		}

		audit.Logger.Info("Plaintext export completed successfully", "destination_file", outputFile)
		fmt.Println(colors.SafeColor(
			fmt.Sprintf("All wallets (%d) from vault '%s' successfully exported to '%s'.", len(v), config.Cfg.ActiveVault, outputFile),
			colors.Success,
		))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().BoolVar(&exportYes, "yes", false, "Skip confirmation prompt.")
}
