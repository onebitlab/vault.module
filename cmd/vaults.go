// File: cmd/vaults.go
package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"vault.module/internal/audit"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/constants"
	"vault.module/internal/errors"
	"vault.module/internal/vault"
)

var keyFile, recipientsFile, vaultType string
var vaultsDeleteYesFlag bool

// vaultsCmd represents the base command for vault management.
var vaultsCmd = &cobra.Command{
	Use:   "vaults",
	Short: "Manage vaults",
	Long: `Manage multiple vault configurations.

This command allows you to manage multiple vault configurations.
Use subcommands to add, list, delete, or switch between vaults.

Examples:
  vault.module vaults list
  vault.module vaults add myvault --type evm
  vault.module vaults use myvault
`,
	Aliases: []string{"vault"},
}

// vaultsListCmd lists all configured vaults.
var vaultsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists all configured vaults.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.WrapCommand(func() error {
			if len(config.Cfg.Vaults) == 0 {
			fmt.Println(colors.SafeColor(
				"No vaults configured. Add one with 'vaults add <name>'.",
				colors.Warning,
			))
				return nil
			}

			names := make([]string, 0, len(config.Cfg.Vaults))
			for name := range config.Cfg.Vaults {
				names = append(names, name)
			}
			sort.Strings(names)

			fmt.Println(colors.SafeColor("Configured Vaults:", colors.Bold))
			for _, name := range names {
				details := config.Cfg.Vaults[name]
				if name == config.Cfg.ActiveVault {
					fmt.Printf(" %s %s %s\n",
						colors.SafeColor("*", colors.Success),
						colors.SafeColor(name, colors.Cyan),
						colors.SafeColor(fmt.Sprintf("(active, type: %s, encryption: %s)", details.Type, details.Encryption), colors.Dim),
					)
				} else {
					fmt.Printf("   %s %s\n",
						colors.SafeColor(name, colors.Bold),
						colors.SafeColor(fmt.Sprintf("(type: %s, encryption: %s)", details.Type, details.Encryption), colors.Dim),
					)
				}
				fmt.Printf("     - Key File: %s\n", colors.SafeColor(details.KeyFile, colors.Yellow))
				if details.Encryption == constants.EncryptionYubiKey {
					fmt.Printf("     - Recipients File: %s\n", colors.SafeColor(details.RecipientsFile, colors.Yellow))
				}
			}
			return nil
		})
	},
}

// vaultsAddCmd adds a new vault to the configuration.
var vaultsAddCmd = &cobra.Command{
	Use:   "add <NAME>",
	Short: "Adds a new vault to the configuration and creates the vault file.",
	Long: `Adds a new vault to the configuration and automatically creates the vault file.

This command:
  1. Creates a vault configuration entry
  2. Sets the vault as active (if no active vault exists)
  3. Automatically creates the encrypted vault file

Examples:
  vault.module vaults add myvault --type evm --keyfile myvault.key --recipientsfile recipients.txt
  vault.module vaults add myvault --type cosmos --keyfile myvault.key --recipientsfile recipients.txt
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.WrapCommand(func() error {
			name := args[0]
			if _, exists := config.Cfg.Vaults[name]; exists {
				return errors.NewVaultExistsError(name)
			}

			if recipientsFile == "" {
				return errors.NewInvalidInputError("recipientsfile", "--recipientsfile is required for yubikey encryption")
			}

			// Normalize vault type to lowercase
			normalizedVaultType := strings.ToLower(strings.TrimSpace(vaultType))

			// Validate file paths using secure validation
			if err := config.ValidateFilePath(keyFile, "keyfile"); err != nil {
				return errors.NewVaultInvalidPathError(keyFile, fmt.Errorf("keyfile validation failed: %w", err))
			}

			absKeyFile, err := filepath.Abs(filepath.Clean(keyFile))
			if err != nil {
				return errors.NewVaultInvalidPathError(keyFile, err)
			}

			var absRecipientsFile string
			if recipientsFile != "" {
				if err := config.ValidateFilePath(recipientsFile, "recipients file"); err != nil {
					return errors.NewVaultInvalidPathError(recipientsFile, fmt.Errorf("recipients file validation failed: %w", err))
				}

				absRecipientsFile, err = filepath.Abs(filepath.Clean(recipientsFile))
				if err != nil {
					return errors.NewVaultInvalidPathError(recipientsFile, err)
				}
			}

			// Prepare vault details for creation
			newVault := config.VaultDetails{
				KeyFile:        absKeyFile,
				RecipientsFile: absRecipientsFile,
				Type:           normalizedVaultType,
				Encryption:     constants.EncryptionYubiKey,
			}

			// Automatically create the physical vault file first
			fmt.Println(colors.SafeColor(
				"Creating vault file...",
				colors.Info,
			))

			// Create an empty vault
			emptyVault := make(vault.Vault)
			if err := vault.SaveVault(newVault, emptyVault); err != nil {
				return errors.NewVaultSaveError(absKeyFile, err)
			}

			// Only add to config.json after successful vault file creation
			if config.Cfg.Vaults == nil {
				config.Cfg.Vaults = make(map[string]config.VaultDetails)
			}
			config.Cfg.Vaults[name] = newVault

			if config.Cfg.ActiveVault == "" {
				config.Cfg.ActiveVault = name
			}

			if err := config.SaveConfig(); err != nil {
				return errors.NewConfigSaveError("config.json", err)
			}

			audit.Logger.Info("Vault configuration added",
				slog.String("vault_name", name),
				slog.String("vault_type", normalizedVaultType),
				slog.String("key_file", absKeyFile),
				slog.Bool("is_active", config.Cfg.ActiveVault == name))

			if config.Cfg.ActiveVault == name {
				fmt.Println(colors.SafeColor(
					fmt.Sprintf("Vault '%s' created successfully at '%s' and is now active", name, absKeyFile),
					colors.Success,
				))
			} else {
				fmt.Println(colors.SafeColor(
					fmt.Sprintf("Vault '%s' created successfully at '%s'", name, absKeyFile),
					colors.Success,
				))
			}
			fmt.Println(colors.SafeColor(
				"💡 Next step: Run 'vault.module add <wallet>' to add wallets",
				colors.Info,
			))

			return nil
		})
	},
}

// vaultsUseCmd sets a vault as the active one.
var vaultsUseCmd = &cobra.Command{
	Use:   "use <NAME>",
	Short: "Sets the active vault.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.WrapCommand(func() error {
			name := args[0]
			if _, exists := config.Cfg.Vaults[name]; !exists {
				return errors.NewVaultNotFoundError(name)
			}

			config.Cfg.ActiveVault = name
			if err := config.SaveConfig(); err != nil {
				return errors.NewConfigSaveError("config.json", err)
			}
			fmt.Printf("Switched to vault '%s'.\n", name)
			return nil
		})
	},
}

// vaultsDeleteCmd deletes a vault from the configuration and deletes the vault file.
var vaultsDeleteCmd = &cobra.Command{
	Use:   "delete <NAME>",
	Short: "Deletes a vault from the configuration and deletes the vault file.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.WrapCommand(func() error {
			name := args[0]
			vaultDetails, exists := config.Cfg.Vaults[name]
			if !exists {
				return errors.NewVaultNotFoundError(name)
			}

			if !vaultsDeleteYesFlag {
				prompt := fmt.Sprintf("Are you sure you want to delete vault '%s' and delete its file at '%s'? This action is irreversible.", name, vaultDetails.KeyFile)
				if !askForConfirmation(colors.SafeColor(prompt, colors.Warning)) {
					fmt.Println(colors.SafeColor("Cancelled.", colors.Info))
					return nil
				}
			}

			// Delete the vault file first
			if err := os.Remove(vaultDetails.KeyFile); err != nil {
				if !os.IsNotExist(err) {
					audit.Logger.Error("Failed to delete vault file",
						slog.String("vault_name", name),
						slog.String("key_file", vaultDetails.KeyFile),
						slog.String("error", err.Error()))
					return errors.NewFileSystemError("delete", vaultDetails.KeyFile, err)
				}
				// File doesn't exist, which is fine
				audit.Logger.Warn("Vault file does not exist",
					slog.String("vault_name", name),
					slog.String("key_file", vaultDetails.KeyFile))
			} else {
				audit.Logger.Info("Vault file deleted",
					slog.String("vault_name", name),
					slog.String("key_file", vaultDetails.KeyFile))
			}

			// Delete from configuration
			delete(config.Cfg.Vaults, name)
			if config.Cfg.ActiveVault == name {
				config.Cfg.ActiveVault = ""
				fmt.Printf("Deleted active vault '%s' and deleted its file. No vault is active now.\n", name)
			} else {
				fmt.Printf("Deleted vault '%s' and deleted its file.\n", name)
			}

			if err := config.SaveConfig(); err != nil {
				return errors.NewConfigSaveError("config.json", err)
			}

			return nil
		})
	},
}

func init() {
	vaultsAddCmd.Flags().StringVar(&keyFile, "keyfile", "", "Path to the encrypted key file for the new vault (required)")
	vaultsAddCmd.Flags().StringVar(&recipientsFile, "recipientsfile", "", "Path to the recipients file (required for yubikey encryption)")
	vaultsAddCmd.Flags().StringVar(&vaultType, "type", "", "Type of the vault, e.g., EVM (required)")

	_ = vaultsAddCmd.MarkFlagRequired("keyfile")
	_ = vaultsAddCmd.MarkFlagRequired("type")
	vaultsDeleteCmd.Flags().BoolVar(&vaultsDeleteYesFlag, "yes", false, "Delete without confirmation prompt")
}
