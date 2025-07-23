// File: cmd/config.go
package cmd // <-- THIS MUST BE 'package cmd'

import (
	"fmt"
	"strings"

	"vault.module/internal/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manages the application settings.",
}

var configSetCmd = &cobra.Command{
	Use:   "set <KEY> <VALUE>",
	Short: "Sets a value for a configuration key.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := strings.ToLower(args[0])
		value := args[1]

		viper.Set(key, value)
		if err := config.SaveConfig(); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}
		fmt.Printf("✅ Configuration updated: %s = %s\n", args[0], value)
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <KEY>",
	Short: "Shows the value of a configuration key.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := strings.ToLower(args[0])

		if !viper.IsSet(key) {
			return fmt.Errorf("key '%s' not found in configuration", args[0])
		}
		value := viper.Get(key)
		fmt.Printf("%s: %v\n", args[0], value)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
}
