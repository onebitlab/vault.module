// File: cmd/vaults.go
package cmd

import (
	"errors"
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
	"vault.module/internal/vault"
)

var keyFile, recipientsFile, vaultType string
var vaultsDeleteYesFlag bool

// validateAndCleanPath validates and cleans the file path
func validateAndCleanPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Clean the path from extra characters
	cleanPath := filepath.Clean(path)

	// Check that the path doesn't contain suspicious characters
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("path contains invalid characters: %s", path)
	}

	// Check that the path is not an absolute path to system directories
	if filepath.IsAbs(cleanPath) {
		// Check that the path doesn't point to system directories
		base := filepath.Base(cleanPath)
		if base == "" || base == "." || base == ".." {
			return "", fmt.Errorf("invalid path: %s", path)
		}
	}

	return cleanPath, nil
}

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
		name := args[0]
		if _, exists := config.Cfg.Vaults[name]; exists {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("a vault with the name '%s' already exists", name),
				colors.Error,
			))
		}

		if recipientsFile == "" {
			return errors.New(colors.SafeColor(
				"--recipientsfile is required for yubikey encryption",
				colors.Error,
			))
		}

		// Normalize vault type to lowercase
		normalizedVaultType := strings.ToLower(strings.TrimSpace(vaultType))

		// Validate and clean file paths
		cleanKeyFile, err := validateAndCleanPath(keyFile)
		if err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("invalid key file path: %s", err.Error()),
				colors.Error,
			))
		}

		absKeyFile, err := filepath.Abs(cleanKeyFile)
		if err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("invalid key file path: %s", err.Error()),
				colors.Error,
			))
		}

		var absRecipientsFile string
		if recipientsFile != "" {
			cleanRecipientsFile, err := validateAndCleanPath(recipientsFile)
			if err != nil {
				return errors.New(colors.SafeColor(
					fmt.Sprintf("invalid recipients file path: %s", err.Error()),
					colors.Error,
				))
			}

			absRecipientsFile, err = filepath.Abs(cleanRecipientsFile)
			if err != nil {
				return errors.New(colors.SafeColor(
					fmt.Sprintf("invalid recipients file path: %s", err.Error()),
					colors.Error,
				))
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
			audit.Logger.Error("Failed to create vault file",
				slog.String("vault_name", name),
				slog.String("key_file", absKeyFile),
				slog.String("error", err.Error()))
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to create vault file: %s", err.Error()),
				colors.Error,
			))
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
			audit.Logger.Error("Failed to save configuration",
				slog.String("vault_name", name),
				slog.String("error", err.Error()))
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to save configuration: %s", err.Error()),
				colors.Error,
			))
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
	},
}

// vaultsUseCmd sets a vault as the active one.
var vaultsUseCmd = &cobra.Command{
	Use:   "use <NAME>",
	Short: "Sets the active vault.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if _, exists := config.Cfg.Vaults[name]; !exists {
			return fmt.Errorf("no vault with the name '%s' found", name)
		}

		config.Cfg.ActiveVault = name
		if err := config.SaveConfig(); err != nil {
			return err
		}
		fmt.Printf("Switched to vault '%s'.\n", name)
		return nil
	},
}

// vaultsDeleteCmd deletes a vault from the configuration and deletes the vault file.
var vaultsDeleteCmd = &cobra.Command{
	Use:   "delete <NAME>",
	Short: "Deletes a vault from the configuration and deletes the vault file.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		vaultDetails, exists := config.Cfg.Vaults[name]
		if !exists {
			return fmt.Errorf("no vault with the name '%s' found", name)
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
				return errors.New(colors.SafeColor(
					fmt.Sprintf("failed to delete vault file '%s': %s", vaultDetails.KeyFile, err.Error()),
					colors.Error,
				))
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
			audit.Logger.Error("Failed to save configuration after vault removal",
				slog.String("vault_name", name),
				slog.String("error", err.Error()))
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to save configuration: %s", err.Error()),
				colors.Error,
			))
		}

		return nil
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
