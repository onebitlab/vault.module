// File: cmd/list.go
package cmd

import (
	"encoding/json"
	"fmt"
	"sort"

	"vault.module/internal/colors"
	"vault.module/internal/config"
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
		// Проверяем состояние vault перед выполнением команды
		if err := checkVaultStatus(); err != nil {
			return err
		}

		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}

		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf("failed to load vault: %s", err.Error())
		}

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
				return fmt.Errorf("failed to generate JSON: %s", err.Error())
			}
			fmt.Println(string(jsonData))
		} else {
			fmt.Println(colors.SafeColor(
				fmt.Sprintf("Saved wallets in '%s' (Type: %s):", config.Cfg.ActiveVault, activeVault.Type),
				colors.Bold,
			))
			for name, wallet := range v {
				fmt.Printf("- %s (Addresses: %d)\n", name, len(wallet.Addresses))
				for _, addr := range wallet.Addresses {
					fmt.Printf("    %s\n", colors.SafeColor(addr.Address, colors.Cyan))
				}
			}
		}
		return nil
	},
}

func init() {
	listCmd.Flags().BoolVar(&listJson, "json", false, "Output the list in JSON format.")
}
