// File: cmd/clone.go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vault.module/internal/actions"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/errors"
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
		return errors.WrapCommand(func() error {
			activeVault, err := config.GetActiveVault()
			if err != nil {
				return err
			}

			if programmaticMode {
				return errors.NewProgrammaticModeError("clone")
			}
			
			clonedVaultName := args[0]
			clonePrefixes := args[1:]

			if len(clonePrefixes) == 0 {
				return errors.NewInvalidInputError("", "at least one prefix must be specified")
			}

			// Check if vault name already exists
			if _, exists := config.Cfg.Vaults[clonedVaultName]; exists {
				return errors.NewVaultExistsError(clonedVaultName)
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

			clonedVault, err := actions.CloneVault(v, clonePrefixes)
			if err != nil {
				return err
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
				return errors.NewVaultSaveError(outputFile, err)
			}

			// Add the cloned vault to config.json
			if config.Cfg.Vaults == nil {
				config.Cfg.Vaults = make(map[string]config.VaultDetails)
			}
			config.Cfg.Vaults[clonedVaultName] = clonedVaultDetails

			if err := config.SaveConfig(); err != nil {
				return errors.NewConfigSaveError("config.json", err)
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
		})
	},
}

func init() {
	cloneCmd.Flags().BoolVar(&cloneYesFlag, "yes", false, "Overwrite without confirmation prompt")
}
