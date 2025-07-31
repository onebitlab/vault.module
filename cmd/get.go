// File: cmd/get.go
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"vault.module/internal/audit"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/security"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

const defaultClipboardTimeout = 30

var getIndex int
var getJson bool
var getCopy bool
var getClipboardTimeout int // Новый флаг для настройки времени очистки

var getCmd = &cobra.Command{
	Use:   "get <PREFIX> <FIELD>",
	Short: "Gets data from the active vault.",
	Long: `Gets data from the active vault.

Available fields (FIELD):
  address      - public address (default --index 0)
  privatekey   - private key (default --index 0)
  mnemonic     - mnemonic phrase (if present)
  notes        - notes (if present)

Examples:
  vault.module get A1 address
  vault.module get A1 privatekey --index 0
  vault.module get A1 mnemonic
  vault.module get A1 --json
  vault.module get A1 privatekey --clipboard-timeout 60  # Clear after 60 seconds
`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Проверяем состояние vault перед выполнением команды
		if err := checkVaultStatus(); err != nil {
			return err
		}

		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}

		prefix := args[0]
		field := strings.ToLower(args[1])

		// FIX: Pass the whole activeVault struct
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("failed to load vault: %s", err.Error()),
				colors.Error,
			))
		}
		// Ensure vault secrets are cleared when function exits
		defer func() {
			for _, wallet := range v {
				wallet.Clear()
			}
		}()

		wallet, exists := v[prefix]
		if !exists {
			return errors.New(colors.SafeColor(
				fmt.Sprintf("wallet with prefix '%s' not found in active vault '%s'", prefix, config.Cfg.ActiveVault),
				colors.Error,
			))
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
				return errors.New(colors.SafeColor(
					fmt.Sprintf("failed to generate JSON: %s", err.Error()),
					colors.Error,
				))
			}
			fmt.Println(string(jsonData))
			return nil
		}

		// --- Logic for getting individual fields ---
		var result string
		isSecret := false
		if field == "mnemonic" {
			audit.Logger.Warn("Secret data accessed", slog.String("command", "get"), slog.String("vault", config.Cfg.ActiveVault), slog.String("prefix", prefix), slog.String("field", "mnemonic"))
			if wallet.Mnemonic == nil || wallet.Mnemonic.String() == "" {
				return errors.New(colors.SafeColor(
					fmt.Sprintf("wallet '%s' does not have a mnemonic phrase", prefix),
					colors.Error,
				))
			}
			result = wallet.Mnemonic.String()
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
				return errors.New(colors.SafeColor(
					fmt.Sprintf("address with index %d not found in wallet '%s'", getIndex, prefix),
					colors.Error,
				))
			}

			switch field {
			case "address":
				audit.Logger.Info("Public data accessed", slog.String("command", "get"), slog.String("vault", config.Cfg.ActiveVault), slog.String("prefix", prefix), slog.Int("index", getIndex), slog.String("field", "address"))
				result = addressData.Address
			case "privatekey":
				audit.Logger.Warn("Secret data accessed", slog.String("command", "get"), slog.String("vault", config.Cfg.ActiveVault), slog.String("prefix", prefix), slog.Int("index", getIndex), slog.String("field", "privateKey"))
				if addressData.PrivateKey == nil {
					return errors.New(colors.SafeColor(
						fmt.Sprintf("address with index %d does not have a private key", getIndex),
						colors.Error,
					))
				}
				result = addressData.PrivateKey.String()
				isSecret = true
			case "notes":
				audit.Logger.Info("Notes accessed", slog.String("command", "get"), slog.String("vault", config.Cfg.ActiveVault), slog.String("prefix", prefix), slog.String("field", "notes"))
				if wallet.Notes != "" {
					result = wallet.Notes
				} else {
					return errors.New(colors.SafeColor(
						fmt.Sprintf("wallet '%s' does not have notes", prefix),
						colors.Error,
					))
				}
			default:
				return errors.New(colors.SafeColor(
					fmt.Sprintf("unknown field '%s'. Available fields: address, privatekey, mnemonic, notes", args[1]),
					colors.Error,
				))
			}
		}

		// --- Main logic for choosing the output mode ---
		if programmaticMode {
			fmt.Print(result)
		} else {
			if isSecret {
				// Копируем в clipboard с настраиваемым таймаутом
				if err := security.GetClipboard().WriteAllWithCustomTimeout(result, getClipboardTimeout); err != nil {
					return errors.New(colors.SafeColor(
						fmt.Sprintf("failed to copy to clipboard: %s", err.Error()),
						colors.Error,
					))
				}
				fmt.Println(colors.SafeColor(
					fmt.Sprintf("Secret copied to clipboard. Independent process will clear it in %d seconds.", getClipboardTimeout),
					colors.Success,
				))
			} else {
				// Для несекретных данных можем тоже копировать в clipboard если указан флаг --copy
				if getCopy {
					if err := security.CopyToClipboard(result); err != nil {
						return errors.New(colors.SafeColor(
							fmt.Sprintf("failed to copy to clipboard: %s", err.Error()),
							colors.Error,
						))
					}
					fmt.Println(colors.SafeColor(
						fmt.Sprintf("Data copied to clipboard: %s", result),
						colors.Success,
					))
				} else {
					fmt.Println(result)
				}
			}
		}
		return nil
	},
}



func init() {
	getCmd.Flags().IntVar(&getIndex, "index", 0, "Index of the address within an HD wallet.")
	getCmd.Flags().BoolVar(&getJson, "json", false, "Output all wallet data in JSON format.")
	getCmd.Flags().BoolVarP(&getCopy, "copy", "c", false, "Copy data to clipboard (applies to non-secret data).")
	getCmd.Flags().IntVar(&getClipboardTimeout, "clipboard-timeout", defaultClipboardTimeout, "Seconds after which clipboard will be cleared (default: 30).")
}
