// File: cmd/token.go
package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"vault.module/internal/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manages the secret token for programmatic mode.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if programmaticMode {
			return fmt.Errorf("this command is not available in programmatic mode")
		}
		return cmd.Help()
	},
}

var tokenGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generates and saves a new token.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if programmaticMode {
			return fmt.Errorf("this command is not available in programmatic mode")
		}

		// Generate 32 random bytes
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			return fmt.Errorf("failed to generate token: %w", err)
		}
		token := hex.EncodeToString(bytes)

		viper.Set("authtoken", token)
		if err := config.SaveConfig(); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		fmt.Println("✅ New token successfully generated and saved.")
		fmt.Println("   Use it to authenticate your bots and scripts:")
		fmt.Printf("   %s\n", token)
		return nil
	},
}

var tokenShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Shows the current token.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if programmaticMode {
			return fmt.Errorf("this command is not available in programmatic mode")
		}

		if config.Cfg.AuthToken == "" {
			fmt.Println("ℹ️  Token has not been generated yet. Use 'token generate'.")
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
