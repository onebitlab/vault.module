// File: cmd/update.go
package cmd

import (
	"errors"
	"fmt"

	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var updateIndex int

var updateCmd = &cobra.Command{
	Use:   "update <PREFIX>",
	Short: "Updates notes in the active vault.",
	Long: `Updates notes in the active vault.

This command allows you to update notes for an existing wallet.
You will be prompted to enter new values interactively.

Examples:
  vault.module update A1
  vault.module update mywallet
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

		if programmaticMode {
			return errors.New(colors.SafeColor(
				"this command is not available in programmatic mode",
				colors.Error,
			))
		}
		prefix := args[0]

		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Active Vault: %s (Type: %s)", config.Cfg.ActiveVault, activeVault.Type),
			colors.Info,
		))

		// FIX: Pass the whole activeVault struct
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to load vault: %s", err.Error()),
				colors.Error,
			))
		}

		if _, exists := v[prefix]; !exists {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("wallet with prefix '%s' not found", prefix),
				colors.Error,
			))
		}

		wallet := v[prefix]

		if updateIndex > -1 {
			var addressToUpdate *vault.Address
			for i := range wallet.Addresses {
				if wallet.Addresses[i].Index == updateIndex {
					addressToUpdate = &wallet.Addresses[i]
					break
				}
			}
			if addressToUpdate == nil {
				return errors.New(colors.SafeColor(
					fmt.Sprintf("address with index %d not found in wallet '%s'", updateIndex, prefix),
					colors.Error,
				))
			}
			// Ранее здесь обновлялся label, теперь ничего не делаем
		} else {
			newNotes, err := askForInput(fmt.Sprintf("Enter new notes for wallet '%s'", prefix))
			if err != nil {
				return errors.New(colors.SafeColor(
					fmt.Sprintf("failed to read input: %s", err.Error()),
					colors.Error,
				))
			}
			wallet.Notes = newNotes
		}

		v[prefix] = wallet
		// FIX: Pass the whole activeVault struct
		if err := vault.SaveVault(activeVault, v); err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to save vault: %s", err.Error()),
				colors.Error,
			))
		}

		fmt.Println(colors.SafeColor(
			fmt.Sprintf("Data for wallet '%s' successfully updated in vault '%s'.", prefix, config.Cfg.ActiveVault),
			colors.Success,
		))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().IntVar(&updateIndex, "index", -1, "Index of the address to update (if not specified, updates wallet notes).")
}
