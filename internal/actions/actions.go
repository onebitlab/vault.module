// File: internal/actions/actions.go
package actions

import (
	"bufio"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"vault.module/internal/constants"
	"vault.module/internal/errors"
	"vault.module/internal/keys"
	"vault.module/internal/vault"
)

// CreateWalletFromMnemonic creates a wallet from a mnemonic for a specific vault type.
func CreateWalletFromMnemonic(mnemonic, vaultType string) (vault.Wallet, string, error) {
	manager, err := keys.GetKeyManager(vaultType)
	if err != nil {
		return vault.Wallet{}, "", err
	}

	newWallet, err := manager.CreateWalletFromMnemonic(mnemonic)
	if err != nil {
		return vault.Wallet{}, "", err
	}

	// Ensure we clear the wallet secrets if there's an error later
	defer func() {
		if err != nil {
			// Clear all secrets in the wallet
			if newWallet.Mnemonic != nil {
				newWallet.Mnemonic.Clear()
			}
			for i := range newWallet.Addresses {
				if newWallet.Addresses[i].PrivateKey != nil {
					newWallet.Addresses[i].PrivateKey.Clear()
				}
			}
		}
	}()

	// The first address is always created.
	finalAddress := newWallet.Addresses[0].Address
	return newWallet, finalAddress, nil
}

// CreateWalletFromPrivateKey creates a wallet from a private key for a specific vault type.
func CreateWalletFromPrivateKey(pkStr, vaultType string) (vault.Wallet, string, error) {
	manager, err := keys.GetKeyManager(vaultType)
	if err != nil {
		return vault.Wallet{}, "", err
	}

	newWallet, err := manager.CreateWalletFromPrivateKey(pkStr)
	if err != nil {
		return vault.Wallet{}, "", err
	}

	// Ensure we clear the wallet secrets if there's an error later
	defer func() {
		if err != nil {
			// Clear all secrets in the wallet
			if newWallet.Mnemonic != nil {
				newWallet.Mnemonic.Clear()
			}
			for i := range newWallet.Addresses {
				if newWallet.Addresses[i].PrivateKey != nil {
					newWallet.Addresses[i].PrivateKey.Clear()
				}
			}
		}
	}()

	finalAddress := newWallet.Addresses[0].Address
	return newWallet, finalAddress, nil
}

// ValidatePrefix checks if a prefix follows the naming rules with enhanced security.
func ValidatePrefix(prefix string) error {
	if prefix == "" {
		return errors.NewInvalidPrefixError(prefix, "prefix cannot be empty")
	}
	
	// Check maximum length (32 characters - optimal balance)
	if len(prefix) > 32 {
		return errors.NewInvalidPrefixError(prefix, fmt.Sprintf("prefix too long (max 32 characters, got %d)", len(prefix)))
	}
	
	// Check for valid characters (latin letters, numbers, underscore)
	match, _ := regexp.MatchString("^[a-zA-Z0-9_]+$", prefix)
	if !match {
		return errors.NewInvalidPrefixError(prefix, "prefix can only contain latin letters, numbers and '_' symbols")
	}
	
	// Check that prefix doesn't start with number or underscore
	match, _ = regexp.MatchString("^[0-9_]", prefix)
	if match {
		return errors.NewInvalidPrefixError(prefix, "prefix cannot start with number or '_'")
	}
	
	// Check for reserved prefixes that might cause conflicts
	reservedPrefixes := []string{"system", "config", "admin", "root", "vault", "temp", "tmp"}
	lowerPrefix := strings.ToLower(prefix)
	for _, reserved := range reservedPrefixes {
		if lowerPrefix == reserved {
			return errors.NewInvalidPrefixError(prefix, fmt.Sprintf("prefix '%s' is reserved and cannot be used", prefix))
		}
	}
	
	return nil
}

// DeriveNextAddress derives the next address using the appropriate key manager.
func DeriveNextAddress(wallet vault.Wallet, vaultType string) (vault.Wallet, vault.Address, error) {
	manager, err := keys.GetKeyManager(vaultType)
	if err != nil {
		return wallet, vault.Address{}, err
	}
	return manager.DeriveNextAddress(wallet)
}

// CloneVault creates a new vault containing only the specified wallets.
func CloneVault(sourceVault vault.Vault, prefixesToClone []string) (vault.Vault, error) {
	clonedVault := make(vault.Vault)
	for _, prefix := range prefixesToClone {
		wallet, exists := sourceVault[prefix]
		if !exists {
			continue
		}
		clonedVault[prefix] = wallet
	}
	if len(clonedVault) == 0 {
		return nil, errors.New(errors.ErrCodeWalletNotFound, "none of the specified wallets were found")
	}
	return clonedVault, nil
}

// ExportVault converts the vault to JSON for exporting.
func ExportVault(v vault.Vault) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// ImportWallets imports wallets into an existing vault.
func ImportWallets(v vault.Vault, content []byte, format, conflictPolicy, vaultType string) (vault.Vault, string, error) {
	var walletsToImport map[string]vault.Wallet
	var err error

	switch format {
	case constants.FormatJSON:
		walletsToImport, err = parseJsonImport(content)
	case constants.FormatKeyValue:
		walletsToImport, err = parseKeyValueImport(content, vaultType)
	default:
		return v, "", errors.NewFormatInvalidError(format, "unknown format")
	}

	if err != nil {
		return v, "", errors.NewImportFailedError(format, "error parsing import file", err)
	}

	addedCount := 0
	skippedCount := 0
	overwrittenCount := 0

	for prefix, newWalletData := range walletsToImport {
		if oldWallet, exists := v[prefix]; exists {
			switch conflictPolicy {
			case constants.ConflictPolicySkip:
				skippedCount++
				continue
			case constants.ConflictPolicyOverwrite:
				overwrittenCount++
				oldWallet.Clear() // clear secrets from old wallet
			case constants.ConflictPolicyFail:
				return v, "", errors.NewWalletExistsError(prefix)
			}
		} else {
			addedCount++
		}
		v[prefix] = newWalletData
	}

	report := fmt.Sprintf("Import complete. Added: %d, Overwritten: %d, Skipped: %d", addedCount, overwrittenCount, skippedCount)
	return v, report, nil
}

func parseJsonImport(content []byte) (map[string]vault.Wallet, error) {
	var importedVault vault.Vault
	if err := json.Unmarshal(content, &importedVault); err != nil {
		return nil, err
	}
	return importedVault, nil
}

func parseKeyValueImport(content []byte, vaultType string) (map[string]vault.Wallet, error) {
	wallets := make(map[string]vault.Wallet)
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	re := regexp.MustCompile(`[:=]`)

	manager, err := keys.GetKeyManager(vaultType)
	if err != nil {
		return nil, err
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := re.Split(line, 2)
		if len(parts) != 2 {
			continue
		}
		prefix := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"")

		if err := ValidatePrefix(prefix); err != nil {
			continue
		}

		var newWallet vault.Wallet
		var creationErr error

		if manager.ValidateMnemonic(value) {
			newWallet, creationErr = manager.CreateWalletFromMnemonic(value)
		} else if manager.ValidatePrivateKey(value) {
			newWallet, creationErr = manager.CreateWalletFromPrivateKey(value)
		} else {
			continue
		}

		if creationErr != nil {
			continue
		}
		wallets[prefix] = newWallet
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return wallets, nil
}
