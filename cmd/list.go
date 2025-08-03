// File: cmd/list.go
package cmd

import (
	"encoding/json"
	"fmt"
	"sort"

	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/errors"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var listJson bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Shows a list of all saved wallets in the active vault.",
	Long: `Shows a list of all saved wallets in the active vault.

Displays:
  - Wallet names (prefixes)
  - Number of addresses per wallet
  - Public addresses for each wallet

Examples:
  vault.module list
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.WrapCommand(func() error {
			if err := checkVaultStatus(); err != nil {
				return err
			}

			activeVault, err := config.GetActiveVault()
			if err != nil {
				return err
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

			if len(v) == 0 {
				fmt.Println(colors.SafeColor(
					fmt.Sprintf("Vault '%s' is empty.", config.Cfg.ActiveVault),
					colors.Info,
				))
				return nil
			}

			filteredPrefixes := make([]string, 0, len(v))
			for prefix := range v {
				filteredPrefixes = append(filteredPrefixes, prefix)
			}

			if len(filteredPrefixes) == 0 {
				fmt.Println(colors.SafeColor(
					"No wallets found matching your filters.",
					colors.Warning,
				))
				return nil
			}

			sort.Strings(filteredPrefixes)

			if listJson {
				outputVault := make(vault.Vault)
				for _, prefix := range filteredPrefixes {
					wallet := v[prefix]
					if !programmaticMode {
						outputVault[prefix] = wallet.Sanitize()
					} else {
						outputVault[prefix] = wallet
					}
				}
				jsonData, err := json.MarshalIndent(outputVault, "", "  ")
				if err != nil {
					return errors.New(errors.ErrCodeInternal, "failed to generate JSON").WithContext("marshal_error", err.Error())
				}
				fmt.Println(string(jsonData))
			} else {
				fmt.Println(colors.SafeColor(
					fmt.Sprintf("Saved wallets in '%s' (Type: %s):", config.Cfg.ActiveVault, activeVault.Type),
					colors.Bold,
				))
				for _, prefix := range filteredPrefixes {
					wallet := v[prefix]

					// Determine wallet source and format display
					var sourceInfo string
					if wallet.Mnemonic != nil {
						mnemonicHint := wallet.GetMnemonicHint()
						if mnemonicHint != "" {
							sourceInfo = fmt.Sprintf("HD from: %s", mnemonicHint)
						} else {
							sourceInfo = "HD wallet (mnemonic cleared)"
						}
					} else {
						// Single key wallet - private keys are not saved to JSON for security
						sourceInfo = "Wallet from private key (imported)"
					}

					fmt.Printf("- %s (%s)\n", colors.SafeColor(prefix, colors.White), colors.SafeColor(sourceInfo, colors.Yellow))

					// Show addresses with index and private key hint
					for _, addr := range wallet.Addresses {
						fmt.Printf("  [%d] %s", addr.Index, colors.SafeColor(addr.Address, colors.Cyan))

						// Show private key hint if available
						if addr.PrivateKey != nil && addr.PrivateKey.String() != "" {
							privateKeyStr := addr.PrivateKey.String()
							if len(privateKeyStr) >= 6 {
								hint := fmt.Sprintf("%s...%s", privateKeyStr[:3], privateKeyStr[len(privateKeyStr)-3:])
								fmt.Printf(" (private key: %s)", colors.SafeColor(hint, colors.Dim))
							}
						}
						fmt.Println()
					}

					// Show notes if present (after addresses)
					if wallet.Notes != "" {
						fmt.Printf("  Notes: %s\n", colors.SafeColor(wallet.Notes, colors.Dim))
					}
				}
			}
			return nil
		})
	},
}

func init() {
	listCmd.Flags().BoolVar(&listJson, "json", false, "Output the list in JSON format.")
}
