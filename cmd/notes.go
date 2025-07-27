// File: cmd/notes.go
package cmd

import (
	"errors"
	"fmt"

	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var notesCmd = &cobra.Command{
	Use:   "notes <PREFIX>",
	Short: "Updates wallet notes in the active vault.",
	Long: `Updates wallet notes in the active vault.

This command allows you to update notes for an existing wallet.
Notes are stored at the wallet level and apply to all addresses in the wallet.
You will be prompted to enter new values interactively.

Examples:
  vault.module notes A1
  vault.module notes mywallet
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

		newNotes, err := askForInput(fmt.Sprintf("Enter new notes for wallet '%s'", prefix))
		if err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to read input: %s", err.Error()),
				colors.Error,
			))
		}
		wallet.Notes = newNotes

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
	// Регистрация перенесена в root.go
}
