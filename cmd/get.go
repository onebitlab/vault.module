// File: cmd/get.go
package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"vault.module/internal/audit"
	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
)

var getIndex int
var getJson bool
var getCopy bool

var getCmd = &cobra.Command{
	Use:   "get <PREFIX> <FIELD>",
	Short: "Gets data from the active vault.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}

		prefix := args[0]
		field := strings.ToLower(args[1])

		// FIX: Pass the whole activeVault struct
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return fmt.Errorf("failed to load vault: %w", err)
		}

		wallet, exists := v[prefix]
		if !exists {
			return fmt.Errorf("wallet with prefix '%s' not found in active vault '%s'", prefix, config.Cfg.ActiveVault)
		}

		// --- Logic for the --json flag ---
		if getJson {
			audit.Logger.Info("Wallet data accessed", slog.String("command", "get"), slog.String("vault", config.Cfg.ActiveVault), slog.String("prefix", prefix), slog.Bool("json", true))
			var dataToMarshal interface{}
			if programmaticMode {
				dataToMarshal = wallet
			} else {
				dataToMarshal = wallet.Sanitize()
			}
			jsonData, err := json.MarshalIndent(dataToMarshal, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to generate JSON: %w", err)
			}
			fmt.Println(string(jsonData))
			return nil
		}

		// --- Logic for getting individual fields ---
		var result string
		isSecret := false
		if field == "mnemonic" {
			audit.Logger.Warn("Secret data accessed", slog.String("command", "get"), slog.String("vault", config.Cfg.ActiveVault), slog.String("prefix", prefix), slog.String("field", "mnemonic"))
			if wallet.Mnemonic == "" {
				return fmt.Errorf("wallet '%s' does not have a mnemonic phrase", prefix)
			}
			result = wallet.Mnemonic
			isSecret = true
		} else {
			var addressData *vault.Address
			for i := range wallet.Addresses {
				if wallet.Addresses[i].Index == getIndex {
					addressData = &wallet.Addresses[i]
					break
				}
			}

			if addressData == nil {
				return fmt.Errorf("address with index %d not found in wallet '%s'", getIndex, prefix)
			}

			switch field {
			case "address":
				audit.Logger.Info("Public data accessed", slog.String("command", "get"), slog.String("vault", config.Cfg.ActiveVault), slog.String("prefix", prefix), slog.Int("index", getIndex), slog.String("field", "address"))
				result = addressData.Address
			case "privatekey":
				audit.Logger.Warn("Secret data accessed", slog.String("command", "get"), slog.String("vault", config.Cfg.ActiveVault), slog.String("prefix", prefix), slog.Int("index", getIndex), slog.String("field", "privateKey"))
				result = addressData.PrivateKey
				isSecret = true
			default:
				return fmt.Errorf("unknown field '%s'", args[1])
			}
		}

		// --- Main logic for choosing the output mode ---
		if programmaticMode {
			fmt.Print(result)
		} else {
			if isSecret {
				if err := clipboard.WriteAll(result); err != nil {
					return fmt.Errorf("failed to copy to clipboard: %w", err)
				}
				fmt.Println("✅ Secret copied to clipboard. It will be cleared in 30 seconds.")
				go func() {
					time.Sleep(30 * time.Second)
					currentClipboard, _ := clipboard.ReadAll()
					if currentClipboard == result {
						clipboard.WriteAll("")
					}
				}()
			} else {
				fmt.Println(result)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
	getCmd.Flags().IntVar(&getIndex, "index", 0, "Index of the address within an HD wallet.")
	getCmd.Flags().BoolVar(&getJson, "json", false, "Output all wallet data in JSON format.")
	getCmd.Flags().BoolVarP(&getCopy, "copy", "c", false, "Copy secret to clipboard (now default behavior for secrets in interactive mode).")
}
