// File: cmd/search.go
package cmd

import (
	"fmt"
	"sort"
	"strings"

	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <QUERY>",
	Short: "Searches for matches in prefixes and notes in the active vault.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}

		if programmaticMode {
			return fmt.Errorf("this command is not available in programmatic mode")
		}
		query := strings.ToLower(args[0])

		fmt.Printf("ℹ️  Active Vault: %s (Type: %s)\n", config.Cfg.ActiveVault, activeVault.Type)

		// FIX: Pass the whole activeVault struct
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf("failed to load vault: %w", err)
		}

		var foundPrefixes []string
		for prefix, wallet := range v {
			if strings.Contains(strings.ToLower(prefix), query) || strings.Contains(strings.ToLower(wallet.Notes), query) {
				foundPrefixes = append(foundPrefixes, prefix)
			}
		}

		if len(foundPrefixes) == 0 {
			fmt.Printf("ℹ️  No wallets found for query '%s' in vault '%s'.\n", args[0], config.Cfg.ActiveVault)
			return nil
		}

		sort.Strings(foundPrefixes)

		fmt.Printf("Found wallets (%d) in '%s':\n", len(foundPrefixes), config.Cfg.ActiveVault)
		for _, prefix := range foundPrefixes {
			fmt.Printf("- %s\n", prefix)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
