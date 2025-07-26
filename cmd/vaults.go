// File: cmd/vaults.go
package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/constants"
	"vault.module/internal/vault"
)

var keyFile, recipientsFile, vaultType, encryptionMethod string
var vaultsRemoveYesFlag bool

// vaultsCmd represents the base command for vault management.
var vaultsCmd = &cobra.Command{
	Use:   "vaults",
	Short: "Manage vaults configuration.",
	Long: `Manage multiple vault configurations.

This command allows you to manage multiple vault configurations.
Use subcommands to add, list, remove, or switch between vaults.

Examples:
  vault.module vaults list
  vault.module vaults add myvault --type evm --encryption yubikey
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
  vault.module vaults add myvault --type evm --encryption yubikey --keyfile myvault.key --recipientsfile recipients.txt
  vault.module vaults add myvault --type cosmos --encryption passphrase --keyfile myvault.key
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

		if encryptionMethod != constants.EncryptionYubiKey && encryptionMethod != constants.EncryptionPassphrase {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("invalid encryption method '%s', must be 'yubikey' or 'passphrase'", encryptionMethod),
				colors.Error,
			))
		}
		if encryptionMethod == constants.EncryptionYubiKey && recipientsFile == "" {
			return errors.New(colors.SafeColor(
				"--recipientsfile is required for yubikey encryption",
				colors.Error,
			))
		}

		// Normalize vault type to lowercase
		normalizedVaultType := strings.ToLower(strings.TrimSpace(vaultType))

		absKeyFile, err := filepath.Abs(keyFile)
		if err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("invalid key file path: %s", err.Error()),
				colors.Error,
			))
		}

		var absRecipientsFile string
		if recipientsFile != "" {
			absRecipientsFile, err = filepath.Abs(recipientsFile)
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
			Encryption:     encryptionMethod,
		}

		if config.Cfg.Vaults == nil {
			config.Cfg.Vaults = make(map[string]config.VaultDetails)
		}
		config.Cfg.Vaults[name] = newVault

		if config.Cfg.ActiveVault == "" {
			config.Cfg.ActiveVault = name
		}

		if err := config.SaveConfig(); err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to save configuration: %s", err.Error()),
				colors.Error,
			))
		}

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

		if newVault.Encryption == constants.EncryptionPassphrase {
			fmt.Println(colors.SafeColor(
				"Creating vault file with passphrase encryption...",
				colors.Info,
			))

			// Запрашиваем passphrase у пользователя
			passphrase, err := askForSecretInput("Enter passphrase for vault encryption")
			if err != nil {
				return errors.New(colors.SafeColor(
					fmt.Sprintf("failed to read passphrase: %s", err.Error()),
					colors.Error,
				))
			}

			// Создаем пустой vault
			emptyVault := make(vault.Vault)
			data, err := json.MarshalIndent(emptyVault, "", "  ")
			if err != nil {
				return errors.New(colors.SafeColor(
					fmt.Sprintf("failed to serialize vault data: %s", err.Error()),
					colors.Error,
				))
			}

			// Создаем временный файл
			tmpfile, err := os.CreateTemp("", "vault-*.json")
			if err != nil {
				return errors.New(colors.SafeColor(
					fmt.Sprintf("could not create temp file: %s", err.Error()),
					colors.Error,
				))
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write(data); err != nil {
				return errors.New(colors.SafeColor(
					fmt.Sprintf("could not write to temp file: %s", err.Error()),
					colors.Error,
				))
			}
			if err := tmpfile.Close(); err != nil {
				return errors.New(colors.SafeColor(
					fmt.Sprintf("could not close temp file: %s", err.Error()),
					colors.Error,
				))
			}

			// Запускаем age с переданным passphrase
			cmd := exec.Command("age", "-p", "-o", absKeyFile, tmpfile.Name())
			cmd.Stdin = strings.NewReader(passphrase + "\n")
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				return errors.New(colors.SafeColor(
					fmt.Sprintf("failed to encrypt vault: %s", err.Error()),
					colors.Error,
				))
			}

			fmt.Println(colors.SafeColor(
				"✅ Vault file created successfully with passphrase encryption",
				colors.Success,
			))
		} else {
			// Для YubiKey используем стандартный SaveVault
			emptyVault := make(vault.Vault)
			if err := vault.SaveVault(newVault, emptyVault); err != nil {
				return errors.New(colors.SafeColor(
					fmt.Sprintf("failed to create vault file: %s", err.Error()),
					colors.Error,
				))
			}
			fmt.Println(colors.SafeColor(
				fmt.Sprintf("✅ Vault file created successfully at '%s'", absKeyFile),
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
	vaultsAddCmd.Flags().StringVar(&encryptionMethod, "encryption", constants.EncryptionYubiKey, "Encryption method: 'yubikey' or 'passphrase'")
	_ = vaultsAddCmd.MarkFlagRequired("keyfile")
	_ = vaultsAddCmd.MarkFlagRequired("type")
	vaultsRemoveCmd.Flags().BoolVar(&vaultsRemoveYesFlag, "yes", false, "Remove without confirmation prompt")
}
