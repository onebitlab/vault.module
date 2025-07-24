// File: cmd/list.go
package cmd

import (
	"encoding/json"
	"fmt"
	"sort"

	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var listJson bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Shows a list of all saved wallets in the active vault.",
	RunE: func(cmd *cobra.Command, args []string) error {
		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}

		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf("failed to load vault: %w", err)
		}

		if len(v) == 0 {
			fmt.Printf("ℹ️  Vault '%s' is empty.\n", config.Cfg.ActiveVault)
			return nil
		}

		filteredPrefixes := make([]string, 0, len(v))
		for prefix := range v {
			filteredPrefixes = append(filteredPrefixes, prefix)
		}

		if len(filteredPrefixes) == 0 {
			fmt.Println("ℹ️  No wallets found matching your filters.")
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
				return fmt.Errorf("failed to generate JSON: %w", err)
			}
			fmt.Println(string(jsonData))
		} else {
			fmt.Printf("Saved wallets in '%s' (Type: %s):\n", config.Cfg.ActiveVault, activeVault.Type)
			for _, prefix := range filteredPrefixes {
				fmt.Printf("- %s (Addresses: %d)\n", prefix, len(v[prefix].Addresses))
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVar(&listJson, "json", false, "Output the list in JSON format.")
}
