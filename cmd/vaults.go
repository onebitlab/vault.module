// File: cmd/vaults.go
package cmd

import (
	"bufio"
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
var vaultsRemoveYesFlag bool

// validateAndCleanPath проверяет и очищает путь к файлу
func validateAndCleanPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Очищаем путь от лишних символов
	cleanPath := filepath.Clean(path)

	// Проверяем, что путь не содержит подозрительные символы
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("path contains invalid characters: %s", path)
	}

	// Проверяем, что путь не является абсолютным путем к системным директориям
	if filepath.IsAbs(cleanPath) {
		// Проверяем, что путь не указывает на системные директории
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
Use subcommands to add, list, remove, or switch between vaults.

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

		// Валидируем и очищаем пути к файлам
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

		newVault := config.VaultDetails{
			KeyFile:        absKeyFile,
			RecipientsFile: absRecipientsFile,
			Type:           normalizedVaultType,
			Encryption:     constants.EncryptionYubiKey,
		}

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

		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Vault '%s' added successfully", name),
			colors.Success,
		))
		if config.Cfg.ActiveVault == name {
			fmt.Println(colors.SafeColor(
				fmt.Sprintf("✅ Vault '%s' is now active", name),
				colors.Success,
			))
		}

		// Автоматически создаем физический файл vault
		fmt.Println(colors.SafeColor(
			"Creating vault file...",
			colors.Info,
		))

		// Создаем пустой vault
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
		fmt.Println(colors.SafeColor(
			fmt.Sprintf("✅ Vault file created successfully at '%s'", absKeyFile),
			colors.Success,
		))
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

// vaultsRemoveCmd removes a vault from the configuration.
var vaultsRemoveCmd = &cobra.Command{
	Use:   "remove <NAME>",
	Short: "Removes a vault from the configuration.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if _, exists := config.Cfg.Vaults[name]; !exists {
			return fmt.Errorf("no vault with the name '%s' found", name)
		}
		if !vaultsRemoveYesFlag {
			fmt.Printf("Are you sure you want to remove vault '%s'? [y/N]: ", name)
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}
		delete(config.Cfg.Vaults, name)
		if config.Cfg.ActiveVault == name {
			config.Cfg.ActiveVault = ""
			fmt.Printf("Removed active vault '%s'. No vault is active now.\n", name)
		} else {
			fmt.Printf("Removed vault '%s'.\n", name)
		}
		return config.SaveConfig()
	},
}

func init() {
	// Регистрация перенесена в root.go

	// Настройка флагов
	vaultsAddCmd.Flags().StringVar(&keyFile, "keyfile", "", "Path to the encrypted key file for the new vault (required)")
	vaultsAddCmd.Flags().StringVar(&recipientsFile, "recipientsfile", "", "Path to the recipients file (required for yubikey encryption)")
	vaultsAddCmd.Flags().StringVar(&vaultType, "type", "", "Type of the vault, e.g., EVM (required)")

	_ = vaultsAddCmd.MarkFlagRequired("keyfile")
	_ = vaultsAddCmd.MarkFlagRequired("type")
	vaultsRemoveCmd.Flags().BoolVar(&vaultsRemoveYesFlag, "yes", false, "Remove without confirmation prompt")
}
