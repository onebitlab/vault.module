// File: cmd/clone.go
package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vault.module/internal/actions"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var cloneYesFlag bool

var cloneCmd = &cobra.Command{
	Use:   "clone <VAULT_NAME> <PREFIXES...>",
	Short: "Creates a new, isolated vault from the active vault.",
	Long: `Creates a new, isolated vault from the active vault.

This command creates a new vault containing only the specified wallets.
The new vault will be encrypted with the same method as the source vault.
The vault file will be saved in the same directory as the source vault.
The new vault will be automatically added to config.json.

Examples:
  vault.module clone newvault A1 A2
  vault.module clone backup wallet1 wallet2
`,
	Args: cobra.MinimumNArgs(2),
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
		clonedVaultName := args[0]
		clonePrefixes := args[1:]

		if len(clonePrefixes) == 0 {
			return errors.New(colors.SafeColor(
				"at least one prefix must be specified",
				colors.Error,
			))
		}

		// Check if vault name already exists
		if _, exists := config.Cfg.Vaults[clonedVaultName]; exists {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("a vault with the name '%s' already exists", clonedVaultName),
				colors.Error,
			))
		}

		// Generate output file path in the same directory as source vault
		sourceDir := filepath.Dir(activeVault.KeyFile)
		outputFile := filepath.Join(sourceDir, clonedVaultName)

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

		// Save the cloned vault to file
		if err := vault.SaveVault(clonedVaultDetails, clonedVault); err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to save new vault to '%s': %s", outputFile, err.Error()),
				colors.Error,
			))
		}

		// Add the cloned vault to config.json
		if config.Cfg.Vaults == nil {
			config.Cfg.Vaults = make(map[string]config.VaultDetails)
		}
		config.Cfg.Vaults[clonedVaultName] = clonedVaultDetails

		if err := config.SaveConfig(); err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to save configuration: %s", err.Error()),
				colors.Error,
			))
		}

		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Isolated vault '%s' successfully created at '%s' containing %d wallets from '%s'.", clonedVaultName, outputFile, len(clonedVault), config.Cfg.ActiveVault),
			colors.Success,
		))
		fmt.Println(colors.SafeColor(
			fmt.Sprintf("💡 Use 'vault.module vaults use %s' to switch to the new vault", clonedVaultName),
			colors.Info,
		))
		return nil
	},
}

func init() {
	// Регистрация перенесена в root.go

	// Настройка флагов
	cloneCmd.Flags().BoolVar(&cloneYesFlag, "yes", false, "Overwrite without confirmation prompt")
}
