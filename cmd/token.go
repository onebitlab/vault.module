// File: cmd/token.go
package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"vault.module/internal/colors"
	"vault.module/internal/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manages the secret token for programmatic mode.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if programmaticMode {
			return fmt.Errorf(colors.SafeColor(
				"this command is not available in programmatic mode",
				colors.Error,
			))
		}
		return cmd.Help()
	},
}

var tokenGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generates and saves a new token.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if programmaticMode {
			return fmt.Errorf(colors.SafeColor(
				"this command is not available in programmatic mode",
				colors.Error,
			))
		}

		// Generate 32 random bytes
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("failed to generate token: %w", err),
				colors.Error,
			))
		}
		token := hex.EncodeToString(bytes)

		viper.Set("authtoken", token)
		if err := config.SaveConfig(); err != nil {
			return fmt.Errorf(colors.SafeColor(
				fmt.Sprintf("failed to save configuration: %w", err),
				colors.Error,
			))
		}

		fmt.Println(colors.SafeColor(
			"New token successfully generated and saved.",
			colors.Success,
		))
		fmt.Println("   Use it to authenticate your bots and scripts:")
		fmt.Printf("   %s\n", colors.SafeColor(token, colors.Cyan))
		return nil
	},
}

var tokenShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Shows the current token.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if programmaticMode {
			return fmt.Errorf(colors.SafeColor(
				"this command is not available in programmatic mode",
				colors.Error,
			))
		}

		if config.Cfg.AuthToken == "" {
			fmt.Println(colors.SafeColor(
				"Token has not been generated yet. Use 'token generate'.",
				colors.Info,
			))
			return nil
		}
		fmt.Println(config.Cfg.AuthToken)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tokenCmd)
	tokenCmd.AddCommand(tokenGenerateCmd)
	tokenCmd.AddCommand(tokenShowCmd)
}
