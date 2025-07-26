// File: cmd/clone.go
package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"vault.module/internal/actions"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var clonePrefixes []string
var cloneYesFlag bool

var cloneCmd = &cobra.Command{
	Use:   "clone <PREFIXES...>",
	Short: "Creates a new, isolated vault from the active vault.",
	Long: `Creates a new, isolated vault from the active vault.

This command creates a new vault containing only the specified wallets.
The new vault will be encrypted with the same method as the source vault.

Examples:
  vault.module clone A1 A2
  vault.module clone wallet1 wallet2 --prefix newvault
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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

		if len(clonePrefixes) == 0 {
			return errors.New(colors.SafeColor(
				"at least one prefix must be specified using the --prefix flag",
				colors.Error,
			))
		}

		if outputFile != "" {
			if _, err := os.Stat(outputFile); err == nil && !cloneYesFlag {
				fmt.Printf("File '%s' already exists. Overwrite? [y/N]: ", outputFile)
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					fmt.Println("Cancelled.")
					return nil
				}
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
				fmt.Sprintf("failed to load active vault '%s': %s", config.Cfg.ActiveVault, err.Error()),
				colors.Error,
			))
		}

		clonedVault, err := actions.CloneVault(v, clonePrefixes)
		if err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("cloning error: %s", err.Error()),
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
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to save new vault to '%s': %s", outputFile, err.Error()),
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
	// Регистрация перенесена в root.go

	// Настройка флагов
	cloneCmd.Flags().StringSliceVar(&clonePrefixes, "prefix", []string{}, "Prefixes of wallets to include in the cloned vault (can be specified multiple times).")
	cloneCmd.Flags().BoolVar(&cloneYesFlag, "yes", false, "Overwrite without confirmation prompt")
}
