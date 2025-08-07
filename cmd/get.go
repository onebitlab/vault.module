// File: cmd/get.go
package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"vault.module/internal/audit"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/errors"
	"vault.module/internal/security"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

const (
	defaultClipboardTimeout = 30
	maxClipboardTimeout     = 3600 // 1 hour maximum
	minClipboardTimeout     = 1    // 1 second minimum
	// Input validation constants
	maxPrefixLength         = 32   // Maximum prefix length
	maxFieldLength          = 32   // Maximum field length
	maxIndexValue           = 999  // Maximum index value
)

var getIndex int
var getJson bool
var getCopy bool
var getClipboardTimeout int // New flag for configurable timeout

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
		return errors.WrapCommand(func() error {
		// Validate command arguments first
		if err := validateGetCommandArgs(args); err != nil {
		return err
		}

		// Validate input parameters
			if err := validateGetCommandInputs(); err != nil {
				return err
			}

			// Check if shutdown is in progress
			if security.IsShuttingDown() {
				return errors.New(errors.ErrCodeSystem, "system is shutting down, cannot process new commands")
			}

			// Check vault status before executing the command
			if err := checkVaultStatus(); err != nil {
				return err
			}

			activeVault, err := config.GetActiveVault()
			if err != nil {
				return err
			}

			prefix := args[0]
			field := strings.ToLower(args[1])

			// Load vault
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

			wallet, exists := v[prefix]
			if !exists {
				return errors.NewWalletNotFoundError(prefix, config.Cfg.ActiveVault)
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
					return errors.New(errors.ErrCodeInternal, "failed to generate JSON").WithContext("marshal_error", err.Error())
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
					return errors.NewWalletInvalidError(prefix, "wallet does not have a mnemonic phrase")
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
					return errors.NewAddressNotFoundError(prefix, getIndex)
				}

				switch field {
				case "address":
					audit.Logger.Info("Public data accessed", slog.String("command", "get"), slog.String("vault", config.Cfg.ActiveVault), slog.String("prefix", prefix), slog.Int("index", getIndex), slog.String("field", "address"))
					result = addressData.Address
				case "privatekey":
					audit.Logger.Warn("Secret data accessed", slog.String("command", "get"), slog.String("vault", config.Cfg.ActiveVault), slog.String("prefix", prefix), slog.Int("index", getIndex), slog.String("field", "privateKey"))
					if addressData.PrivateKey == nil {
						return errors.NewAddressNotFoundError(prefix, getIndex).WithDetails("address does not have a private key")
					}
					result = addressData.PrivateKey.String()
					isSecret = true
				case "notes":
					audit.Logger.Info("Notes accessed", slog.String("command", "get"), slog.String("vault", config.Cfg.ActiveVault), slog.String("prefix", prefix), slog.String("field", "notes"))
					if wallet.Notes != "" {
						result = wallet.Notes
					} else {
						return errors.NewWalletInvalidError(prefix, "wallet does not have notes")
					}
				default:
					return errors.NewInvalidInputError(args[1], fmt.Sprintf("unknown field '%s'. Available fields: address, privatekey, mnemonic, notes", args[1]))
				}
			}

			// --- Main logic for choosing the output mode ---
			if programmaticMode {
				fmt.Print(result)
			} else {
				if isSecret {
					// Register clipboard for cleanup with shutdown manager
					security.RegisterClipboardGlobal(fmt.Sprintf("clipboard for %s.%s", prefix, field))

					// Copy to clipboard with configurable timeout
					if err := security.GetClipboard().WriteAllWithCustomTimeout(result, getClipboardTimeout); err != nil {
						return errors.NewClipboardError(err)
					}
					fmt.Println(colors.SafeColor(
						fmt.Sprintf("Secret copied to clipboard. Independent process will clear it in %d seconds.", getClipboardTimeout),
						colors.Success,
					))
				} else {
					// For non-secret data, we can also copy to clipboard if --copy flag is specified
					if getCopy {
						if err := security.CopyToClipboard(result); err != nil {
							return errors.NewClipboardError(err)
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
		})
	},
}

// validateGetCommandInputs validates input parameters for the get command
func validateGetCommandInputs() error {
	// Validate clipboard timeout range with overflow protection
	if getClipboardTimeout < minClipboardTimeout {
		return errors.NewInvalidInputError(
			fmt.Sprintf("%d", getClipboardTimeout),
			fmt.Sprintf("clipboard timeout must be at least %d second(s)", minClipboardTimeout),
		)
	}
	if getClipboardTimeout > maxClipboardTimeout {
		return errors.NewInvalidInputError(
			fmt.Sprintf("%d", getClipboardTimeout),
			fmt.Sprintf("clipboard timeout must be at most %d seconds (1 hour)", maxClipboardTimeout),
		)
	}

	// Additional overflow protection for timeout value
	if getClipboardTimeout < 0 {
		return errors.NewInvalidInputError(
			fmt.Sprintf("%d", getClipboardTimeout),
			"clipboard timeout cannot be negative (potential overflow)",
		)
	}

	// Validate address index (must be non-negative and within reasonable range)
	if getIndex < 0 {
		return errors.NewInvalidInputError(
			fmt.Sprintf("%d", getIndex),
			"address index must be non-negative",
		)
	}
	if getIndex > maxIndexValue {
		return errors.NewInvalidInputError(
			fmt.Sprintf("%d", getIndex),
			fmt.Sprintf("address index must be at most %d", maxIndexValue),
		)
	}

	return nil
}

// validateGetCommandArgs validates command line arguments
func validateGetCommandArgs(args []string) error {
	if len(args) != 2 {
		return errors.NewInvalidInputError(
			fmt.Sprintf("%d arguments", len(args)),
			"exactly 2 arguments required: <PREFIX> <FIELD>",
		)
	}

	prefix := args[0]
	field := args[1]

	// Validate prefix length and content
	if len(prefix) == 0 {
		return errors.NewInvalidInputError(prefix, "prefix cannot be empty")
	}
	if len(prefix) > maxPrefixLength {
		return errors.NewInvalidInputError(
			prefix,
			fmt.Sprintf("prefix length must be at most %d characters", maxPrefixLength),
		)
	}

	// Validate prefix content (alphanumeric and basic symbols only)
	for _, char := range prefix {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || 
			(char >= '0' && char <= '9') || char == '_' || char == '-') {
			return errors.NewInvalidInputError(
				prefix,
				"prefix can only contain alphanumeric characters, underscores, and hyphens",
			)
		}
	}

	// Validate field length and content
	if len(field) == 0 {
		return errors.NewInvalidInputError(field, "field cannot be empty")
	}
	if len(field) > maxFieldLength {
		return errors.NewInvalidInputError(
			field,
			fmt.Sprintf("field length must be at most %d characters", maxFieldLength),
		)
	}

	// Validate field content (lowercase letters only)
	for _, char := range strings.ToLower(field) {
		if !((char >= 'a' && char <= 'z')) {
			return errors.NewInvalidInputError(
				field,
				"field can only contain alphabetic characters",
			)
		}
	}

	// Validate field is one of allowed values
	allowedFields := []string{"address", "privatekey", "mnemonic", "notes"}
	fieldLower := strings.ToLower(field)
	validField := false
	for _, allowed := range allowedFields {
		if fieldLower == allowed {
			validField = true
			break
		}
	}
	if !validField {
		return errors.NewInvalidInputError(
			field,
			fmt.Sprintf("unknown field '%s'. Available fields: %s", field, strings.Join(allowedFields, ", ")),
		)
	}

	return nil
}

func init() {
	getCmd.Flags().IntVar(&getIndex, "index", 0, "Index of the address within an HD wallet.")
	getCmd.Flags().BoolVar(&getJson, "json", false, "Output all wallet data in JSON format.")
	getCmd.Flags().BoolVarP(&getCopy, "copy", "c", false, "Copy data to clipboard (applies to non-secret data).")
	getCmd.Flags().IntVar(&getClipboardTimeout, "clipboard-timeout", defaultClipboardTimeout, fmt.Sprintf("Seconds after which clipboard will be cleared (range: %d-%d, default: %d).", minClipboardTimeout, maxClipboardTimeout, defaultClipboardTimeout))
}
