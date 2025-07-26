// File: cmd/derive.go
package cmd

import (
	"errors"
	"fmt"

	"vault.module/internal/actions"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var deriveCmd = &cobra.Command{
	Use:   "derive <PREFIX>",
	Short: "Derives and adds the next address for a wallet in the active vault.",
	Long: `Derives and adds the next address for a wallet in the active vault.

This command is only available for HD wallets (created from mnemonic).
It will derive the next address using the wallet's derivation path.

Examples:
  vault.module derive A1
  vault.module derive myhdwallet
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Проверяем состояние vault перед выполнением команды
		if err := checkVaultStatus(); err != nil {
			return err
		}

		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}

		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Active Vault: %s (Type: %s)", config.Cfg.ActiveVault, activeVault.Type),
			colors.Info,
		))

		if programmaticMode {
			return errors.New(colors.SafeColor(
				"this command is not available in programmatic mode",
				colors.Error,
			))
		}
		prefix := args[0]

		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to load vault: %s", err.Error()),
				colors.Error,
			))
		}

		wallet, exists := v[prefix]
		if !exists {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("wallet with prefix '%s' not found", prefix),
				colors.Error,
			))
		}

		// Pass the vault type to the action to use the correct key manager.
		updatedWallet, newAddr, err := actions.DeriveNextAddress(wallet, activeVault.Type)
		if err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("derivation error: %s", err.Error()),
				colors.Error,
			))
		}

		v[prefix] = updatedWallet

		if err := vault.SaveVault(activeVault, v); err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to save vault: %s", err.Error()),
				colors.Error,
			))
		}

		fmt.Println(colors.SafeColor(
			fmt.Sprintf("New address (index %d) successfully derived for wallet '%s'.", newAddr.Index, prefix),
			colors.Success,
		))
		fmt.Printf("   Address: %s\n", colors.SafeColor(newAddr.Address, colors.Cyan))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deriveCmd)
}
