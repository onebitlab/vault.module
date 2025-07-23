// File: cmd/tag.go
package cmd

import (
	"fmt"

	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manages wallet tags.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if programmaticMode {
			return fmt.Errorf("this command is not available in programmatic mode")
		}
		return cmd.Help()
	},
}

var tagAddCmd = &cobra.Command{
	Use:   "add <PREFIX> <TAG>",
	Short: "Adds a tag to a wallet in the active vault.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}
		prefix := args[0]
		tagToAdd := args[1]

		fmt.Printf("ℹ️  Active Vault: %s (Type: %s)\n", config.Cfg.ActiveVault, activeVault.Type)

		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf("failed to load vault: %w", err)
		}
		wallet, exists := v[prefix]
		if !exists {
			return fmt.Errorf("wallet with prefix '%s' not found", prefix)
		}
		for _, tag := range wallet.Tags {
			if tag == tagToAdd {
				fmt.Printf("ℹ️  Tag '%s' already exists on wallet '%s'.\n", tagToAdd, prefix)
				return nil
			}
		}
		wallet.Tags = append(wallet.Tags, tagToAdd)
		v[prefix] = wallet
		if err := vault.SaveVault(activeVault, v); err != nil {
			return fmt.Errorf("failed to save vault: %w", err)
		}
		fmt.Printf("✅ Tag '%s' successfully added to wallet '%s'.\n", tagToAdd, prefix)
		return nil
	},
}

var tagRemoveCmd = &cobra.Command{
	Use:   "remove <PREFIX> <TAG>",
	Short: "Removes a tag from a wallet in the active vault.",
	Args:  cobra.ExactArgs(2),
	// FIX: Removed the stray hyphen from cobra.Command
	RunE: func(cmd *cobra.Command, args []string) error {
		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}
		prefix := args[0]
		tagToRemove := args[1]

		fmt.Printf("ℹ️  Active Vault: %s (Type: %s)\n", config.Cfg.ActiveVault, activeVault.Type)

		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf("failed to load vault: %w", err)
		}
		wallet, exists := v[prefix]
		if !exists {
			return fmt.Errorf("wallet with prefix '%s' not found", prefix)
		}
		found := false
		newTags := []string{}
		for _, tag := range wallet.Tags {
			if tag == tagToRemove {
				found = true
			} else {
				newTags = append(newTags, tag)
			}
		}
		if !found {
			fmt.Printf("ℹ️  Tag '%s' not found on wallet '%s'.\n", tagToRemove, prefix)
			return nil
		}
		wallet.Tags = newTags
		v[prefix] = wallet
		if err := vault.SaveVault(activeVault, v); err != nil {
			return fmt.Errorf("failed to save vault: %w", err)
		}
		fmt.Printf("✅ Tag '%s' successfully removed from wallet '%s'.\n", tagToRemove, prefix)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tagCmd)
	tagCmd.AddCommand(tagAddCmd)
	tagCmd.AddCommand(tagRemoveCmd)
}
