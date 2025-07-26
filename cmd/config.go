// File: cmd/config.go
package cmd // <-- THIS MUST BE 'package cmd'

import (
	"errors"
	"fmt"
	"strings"

	"vault.module/internal/colors"
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

var configGetCmd = &cobra.Command{
	Use:   "get <KEY>",
	Short: "Shows the value of a configuration key.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := strings.ToLower(args[0])

		if !viper.IsSet(key) {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("key '%s' not found in configuration", args[0]),
				colors.Error,
			))
		}
		value := viper.Get(key)
		fmt.Printf("%s: %v\n", colors.SafeColor(args[0], colors.Cyan), value)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
}
