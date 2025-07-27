// File: cmd/config.go
package cmd // <-- THIS MUST BE 'package cmd'

import (
	"errors"
	"fmt"
	"strings"

	"vault.module/internal/colors"
	"vault.module/internal/config"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manages the application settings.",
	Long: `Manages the application settings.

This command allows you to view and modify configuration settings.
Use subcommands to list or set specific configuration values.

Available global configuration keys:
  yubikeyslot    - YubiKey slot number (e.g., "1", "2", or empty for default)
  authtoken      - Authentication token
  active_vault   - Currently active vault name

Examples:
  vault.module config list                    # Show all configuration keys and values
  vault.module config set yubikeyslot 1      # Set specific key value
`,
}

var configSetCmd = &cobra.Command{
	Use:   "set <KEY> <VALUE>",
	Short: "Sets a value for a configuration key.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := strings.ToLower(args[0])
		value := args[1]

		// Update the global config structure
		switch key {
		case "yubikeyslot":
			config.Cfg.YubikeySlot = value
		case "authtoken":
			config.Cfg.AuthToken = value
		case "active_vault":
			config.Cfg.ActiveVault = value
		default:
			return errors.New(colors.SafeColor(
				fmt.Sprintf("unknown configuration key: %s", args[0]),
				colors.Error,
			))
		}

		// Save the updated configuration
		if err := config.SaveConfig(); err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to save configuration: %s", err.Error()),
				colors.Error,
			))
		}
		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Configuration updated: %s = %s", args[0], value),
			colors.Success,
		))
		return nil
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "Shows all configuration keys and their values.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(colors.SafeColor("Global Configuration:", colors.Bold))
		fmt.Printf("  %s: %s\n", colors.SafeColor("yubikeyslot", colors.Cyan), config.Cfg.YubikeySlot)
		fmt.Printf("  %s: %s\n", colors.SafeColor("authtoken", colors.Cyan), config.Cfg.AuthToken)
		fmt.Printf("  %s: %s\n", colors.SafeColor("active_vault", colors.Cyan), config.Cfg.ActiveVault)
		return nil
	},
}

func init() {
	// Регистрация перенесена в root.go

	// Настройка подкоманд
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configSetCmd)
}
