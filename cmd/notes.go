// File: cmd/notes.go
package cmd

import (
	"fmt"

	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/errors"
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
		return errors.WrapCommand(func() error {
			// Check vault status before executing the command
			if err := checkVaultStatus(); err != nil {
				return err
			}

			activeVault, err := config.GetActiveVault()
			if err != nil {
				return err
			}

			if programmaticMode {
				return errors.NewProgrammaticModeError("notes")
			}
			
			prefix := args[0]

			fmt.Println(colors.SafeColor(
				fmt.Sprintf("Active Vault: %s (Type: %s)", config.Cfg.ActiveVault, activeVault.Type),
				colors.Info,
			))

			v, err := vault.LoadVault(activeVault)
			if err != nil {
				return errors.NewVaultLoadError(activeVault.KeyFile, err)
			}
			
			// Ensure vault secrets are cleared when function exits
			defer func() {
				for _, wallet := range v {
					wallet.Clear()
				}
			}()

			if _, exists := v[prefix]; !exists {
				return errors.NewWalletNotFoundError(prefix, config.Cfg.ActiveVault)
			}

			wallet := v[prefix]

			newNotes, err := askForInput(fmt.Sprintf("Enter new notes for wallet '%s'", prefix))
			if err != nil {
				return errors.NewInvalidInputError("", fmt.Sprintf("failed to read input: %s", err.Error()))
			}
			wallet.Notes = newNotes

			v[prefix] = wallet
			if err := vault.SaveVault(activeVault, v); err != nil {
				return errors.NewVaultSaveError(activeVault.KeyFile, err)
			}

			fmt.Println(colors.SafeColor(
				fmt.Sprintf("Data for wallet '%s' successfully updated in vault '%s'.", prefix, config.Cfg.ActiveVault),
				colors.Success,
			))
			return nil
		})
	},
}

func init() {
}
