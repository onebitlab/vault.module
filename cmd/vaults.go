// File: cmd/vaults.go
package cmd

import (
	"fmt"
	"path/filepath"
	"sort"

	"vault.module/internal/config"

	"github.com/spf13/cobra"
)

var keyFile, recipientsFile, vaultType, encryptionMethod string

// vaultsCmd represents the base command for vault management.
var vaultsCmd = &cobra.Command{
	Use:     "vaults",
	Short:   "Manage multiple vault configurations.",
	Aliases: []string{"vault"},
}

// vaultsListCmd lists all configured vaults.
var vaultsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists all configured vaults.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(config.Cfg.Vaults) == 0 {
			fmt.Println("No vaults configured. Add one with 'vaults add <name>'.")
			return nil
		}

		names := make([]string, 0, len(config.Cfg.Vaults))
		for name := range config.Cfg.Vaults {
			names = append(names, name)
		}
		sort.Strings(names)

		fmt.Println("Configured Vaults:")
		for _, name := range names {
			details := config.Cfg.Vaults[name]
			if name == config.Cfg.ActiveVault {
				fmt.Printf(" * %s (active, type: %s, encryption: %s)\n", name, details.Type, details.Encryption)
			} else {
				fmt.Printf("   %s (type: %s, encryption: %s)\n", name, details.Type, details.Encryption)
			}
			fmt.Printf("     - Key File: %s\n", details.KeyFile)
			if details.Encryption == "yubikey" {
				fmt.Printf("     - Recipients File: %s\n", details.RecipientsFile)
			}
		}
		return nil
	},
}

// vaultsAddCmd adds a new vault to the configuration.
var vaultsAddCmd = &cobra.Command{
	Use:   "add <NAME>",
	Short: "Adds a new vault to the configuration.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if _, exists := config.Cfg.Vaults[name]; exists {
			return fmt.Errorf("a vault with the name '%s' already exists", name)
		}

		if encryptionMethod != "yubikey" && encryptionMethod != "passphrase" {
			return fmt.Errorf("invalid encryption method '%s', must be 'yubikey' or 'passphrase'", encryptionMethod)
		}
		if encryptionMethod == "yubikey" && recipientsFile == "" {
			return fmt.Errorf("--recipientsfile is required for yubikey encryption")
		}

		absKeyFile, err := filepath.Abs(keyFile)
		if err != nil {
			return fmt.Errorf("invalid key file path: %w", err)
		}

		var absRecipientsFile string
		if recipientsFile != "" {
			absRecipientsFile, err = filepath.Abs(recipientsFile)
			if err != nil {
				return fmt.Errorf("invalid recipients file path: %w", err)
			}
		}

		newVault := config.VaultDetails{
			KeyFile:        absKeyFile,
			RecipientsFile: absRecipientsFile,
			Type:           vaultType,
			Encryption:     encryptionMethod,
		}

		if config.Cfg.Vaults == nil {
			config.Cfg.Vaults = make(map[string]config.VaultDetails)
		}
		config.Cfg.Vaults[name] = newVault

		if config.Cfg.ActiveVault == "" {
			config.Cfg.ActiveVault = name
			fmt.Printf("Vault '%s' (type: %s, encryption: %s) added and set as active.\n", name, vaultType, encryptionMethod)
		} else {
			fmt.Printf("Vault '%s' (type: %s, encryption: %s) added.\n", name, vaultType, encryptionMethod)
		}

		return config.SaveConfig()
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
	rootCmd.AddCommand(vaultsCmd)
	vaultsCmd.AddCommand(vaultsListCmd)
	vaultsCmd.AddCommand(vaultsAddCmd)
	vaultsCmd.AddCommand(vaultsUseCmd)
	vaultsCmd.AddCommand(vaultsRemoveCmd)

	vaultsAddCmd.Flags().StringVar(&keyFile, "keyfile", "", "Path to the encrypted key file for the new vault (required)")
	vaultsAddCmd.Flags().StringVar(&recipientsFile, "recipientsfile", "", "Path to the recipients file (required for yubikey encryption)")
	vaultsAddCmd.Flags().StringVar(&vaultType, "type", "", "Type of the vault, e.g., EVM (required)")
	vaultsAddCmd.Flags().StringVar(&encryptionMethod, "encryption", "yubikey", "Encryption method: 'yubikey' or 'passphrase'")
	_ = vaultsAddCmd.MarkFlagRequired("keyfile")
	_ = vaultsAddCmd.MarkFlagRequired("type")
}
