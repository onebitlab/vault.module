// File: cmd/add.go
package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"vault.module/internal/actions"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/errors"
	"vault.module/internal/security"
	"vault.module/internal/vault"
)

var addCmd = &cobra.Command{
	Use:   "add <PREFIX>",
	Short: "Adds a new wallet to the active vault.",
	Long: `Adds a new wallet to the active vault.

Examples:
  vault.module add A1
  vault.module add mywallet
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.WrapCommand(func() error {
			// Check vault status before executing the command
			if err := checkVaultStatus(); err != nil {
				return err
			}

			// Check if shutdown is in progress
			if security.IsShuttingDown() {
				return errors.New(errors.ErrCodeSystem, "system is shutting down, cannot process new commands")
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
				return errors.NewProgrammaticModeError("add")
			}

			prefix := args[0]
			if err := actions.ValidatePrefix(prefix); err != nil {
				return errors.NewInvalidPrefixError(prefix, err.Error())
			}

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

			if _, exists := v[prefix]; exists {
				return errors.NewWalletExistsError(prefix)
			}

			// The prompt is now generic and doesn't mention specific chains.
			choice, err := askForInput("Choose source: 1. Mnemonic (HD-wallet), 2. Private Key (single address)")
			if err != nil {
				return err
			}
			if strings.TrimSpace(choice) == "" {
				return errors.NewInvalidInputError(choice, "source choice cannot be empty. Please choose 1 for mnemonic or 2 for private key")
			}

			var newWallet vault.Wallet
			var finalAddress string
			switch choice {
			case "1":
				mnemonic, mnemonicErr := askForSecretInputWithCleanup("Enter your mnemonic phrase")
				if mnemonicErr != nil {
					return mnemonicErr
				}
				if strings.TrimSpace(mnemonic) == "" {
					return errors.NewInvalidMnemonicError("mnemonic phrase cannot be empty")
				}
				newWallet, finalAddress, err = actions.CreateWalletFromMnemonic(mnemonic, activeVault.Type)
			case "2":
				pkStr, pkErr := askForSecretInputWithCleanup("Enter your private key")
				if pkErr != nil {
					return pkErr
				}
				if strings.TrimSpace(pkStr) == "" {
					return errors.NewInvalidKeyError("private", "private key cannot be empty")
				}
				newWallet, finalAddress, err = actions.CreateWalletFromPrivateKey(pkStr, activeVault.Type)
			default:
				return errors.NewInvalidInputError(choice, fmt.Sprintf("invalid source choice: '%s'. Please choose 1 for mnemonic or 2 for private key", choice))
			}

			if err != nil {
				return errors.NewWalletInvalidError(prefix, err.Error())
			}

			v[prefix] = newWallet
			if err := vault.SaveVault(activeVault, v); err != nil {
				return errors.NewVaultSaveError(activeVault.KeyFile, err)
			}

			fmt.Println(colors.SafeColor(
				fmt.Sprintf("Wallet '%s' added successfully to vault '%s'.", prefix, config.Cfg.ActiveVault),
				colors.Success,
			))
			fmt.Printf("   Address: %s\n", colors.SafeColor(finalAddress, colors.Cyan))
			return nil
		})
	},
}

func init() {
	// Registration moved to root.go
}
