// File: cmd/init.go
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/vault"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initializes the active vault file.",
	RunE: func(cmd *cobra.Command, args []string) error {
		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}

		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Initializing Vault: %s (Type: %s, Encryption: %s)", config.Cfg.ActiveVault, activeVault.Type, activeVault.Encryption),
			colors.Info,
		))

		if _, err := os.Stat(activeVault.KeyFile); err == nil {
			fmt.Println(colors.SafeColor(
				fmt.Sprintf("Warning: Vault file '%s' for active vault '%s' already exists.", activeVault.KeyFile, config.Cfg.ActiveVault),
				colors.Warning,
			))
			if !askForConfirmation(colors.SafeColor(
				"Are you sure you want to overwrite it? ALL DATA WILL BE LOST!",
				colors.Warning,
			)) {
				fmt.Println(colors.SafeColor("Cancelled.", colors.Info))
				return nil
			}
		}

		emptyVault := make(vault.Vault)
		if err := vault.SaveVault(activeVault, emptyVault); err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to create vault: %s", err.Error()),
				colors.Error,
			))
		}

		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Vault '%s' successfully initialized at '%s'.", config.Cfg.ActiveVault, activeVault.KeyFile),
			colors.Success,
		))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
