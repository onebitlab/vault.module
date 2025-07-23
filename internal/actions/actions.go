// File: internal/actions/actions.go
package actions

import (
	"bufio"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"vault.module/internal/keys"
	"vault.module/internal/vault"
)

// CreateWalletFromMnemonic creates a wallet from a mnemonic.
// It no longer needs walletType as it's determined by the vault context.
func CreateWalletFromMnemonic(mnemonic string) (vault.Wallet, string, error) {
	if !keys.ValidateMnemonic(mnemonic) {
		return vault.Wallet{}, "", fmt.Errorf("the provided mnemonic phrase is invalid")
	}

	hdWallet, err := keys.CreateWalletFromMnemonic(mnemonic)
	if err != nil {
		return vault.Wallet{}, "", fmt.Errorf("failed to create wallet: %w", err)
	}

	path := fmt.Sprintf("%s/0", keys.EVMDerivationPath)
	privateKey, err := keys.DerivePrivateKey(hdWallet, path)
	if err != nil {
		return vault.Wallet{}, "", fmt.Errorf("failed to derive private key: %w", err)
	}

	address, err := keys.PrivateKeyToAddress(privateKey)
	if err != nil {
		return vault.Wallet{}, "", fmt.Errorf("failed to generate address: %w", err)
	}

	newWallet := vault.Wallet{
		Mnemonic:       mnemonic,
		DerivationPath: keys.EVMDerivationPath,
		Addresses: []vault.Address{
			{
				Index:      0,
				Path:       path,
				Address:    address,
				PrivateKey: keys.PrivateKeyToString(privateKey),
				Label:      "Primary",
			},
		},
	}
	return newWallet, address, nil
}

// CreateWalletFromPrivateKey creates a wallet from a private key.
// It no longer needs walletType as it's determined by the vault context.
func CreateWalletFromPrivateKey(pkStr string) (vault.Wallet, string, error) {
	if !keys.ValidatePrivateKey(pkStr) {
		return vault.Wallet{}, "", fmt.Errorf("the provided private key is invalid")
	}

	privateKey, err := keys.PrivateKeyFromString(pkStr)
	if err != nil {
		return vault.Wallet{}, "", fmt.Errorf("failed to process private key: %w", err)
	}

	address, err := keys.PrivateKeyToAddress(privateKey)
	if err != nil {
		return vault.Wallet{}, "", fmt.Errorf("failed to generate address: %w", err)
	}

	newWallet := vault.Wallet{
		Addresses: []vault.Address{
			{
				Index:      0,
				Path:       "imported",
				Address:    address,
				PrivateKey: keys.PrivateKeyToString(privateKey),
				Label:      "Imported",
			},
		},
	}
	return newWallet, address, nil
}

// ValidatePrefix checks if a prefix follows the naming rules.
func ValidatePrefix(prefix string) error {
	if prefix == "" {
		return fmt.Errorf("prefix cannot be empty")
	}
	match, _ := regexp.MatchString("^[a-zA-Z0-9_]+$", prefix)
	if !match {
		return fmt.Errorf("prefix can only contain latin letters, numbers, and the '_' symbol")
	}
	match, _ = regexp.MatchString("^[0-9]", prefix)
	if match {
		return fmt.Errorf("prefix cannot start with a number")
	}
	return nil
}

// DeriveNextAddress derives the next address for an HD wallet.
func DeriveNextAddress(wallet vault.Wallet) (vault.Wallet, vault.Address, error) {
	if wallet.Mnemonic == "" {
		return wallet, vault.Address{}, fmt.Errorf("derivation is only possible for HD wallets (with a mnemonic)")
	}

	nextIndex := len(wallet.Addresses)

	hdWallet, err := keys.CreateWalletFromMnemonic(wallet.Mnemonic)
	if err != nil {
		return wallet, vault.Address{}, fmt.Errorf("failed to create wallet from mnemonic: %w", err)
	}

	path := fmt.Sprintf("%s/%d", wallet.DerivationPath, nextIndex)
	privateKey, err := keys.DerivePrivateKey(hdWallet, path)
	if err != nil {
		return wallet, vault.Address{}, fmt.Errorf("failed to derive private key: %w", err)
	}

	address, err := keys.PrivateKeyToAddress(privateKey)
	if err != nil {
		return wallet, vault.Address{}, fmt.Errorf("failed to generate address: %w", err)
	}

	newAddress := vault.Address{
		Index:      nextIndex,
		Path:       path,
		Address:    address,
		PrivateKey: keys.PrivateKeyToString(privateKey),
		Label:      fmt.Sprintf("Address #%d", nextIndex),
	}

	wallet.Addresses = append(wallet.Addresses, newAddress)
	return wallet, newAddress, nil
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
		return nil, fmt.Errorf("none of the specified wallets were found")
	}
	return clonedVault, nil
}

// ExportVault converts the vault to JSON for exporting.
func ExportVault(v vault.Vault) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// ImportWallets imports wallets into an existing vault.
func ImportWallets(v vault.Vault, content []byte, format, conflictPolicy string) (vault.Vault, string, error) {
	var walletsToImport map[string]vault.Wallet
	var err error

	switch format {
	case "json":
		walletsToImport, err = parseJsonImport(content)
	case "key-value":
		walletsToImport, err = parseKeyValueImport(content)
	default:
		return v, "", fmt.Errorf("unknown format: %s", format)
	}

	if err != nil {
		return v, "", fmt.Errorf("error parsing import file: %w", err)
	}

	addedCount := 0
	skippedCount := 0
	overwrittenCount := 0

	for prefix, newWalletData := range walletsToImport {
		if _, exists := v[prefix]; exists {
			switch conflictPolicy {
			case "skip":
				skippedCount++
				continue
			case "overwrite":
				overwrittenCount++
			case "fail":
				return v, "", fmt.Errorf("wallet '%s' already exists", prefix)
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

func parseKeyValueImport(content []byte) (map[string]vault.Wallet, error) {
	wallets := make(map[string]vault.Wallet)
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	lineNumber := 0

	re := regexp.MustCompile(`[:=]`)

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := re.Split(line, 2)
		if len(parts) != 2 {
			continue // Silently ignore invalid lines
		}
		prefix := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"")

		if err := ValidatePrefix(prefix); err != nil {
			continue // Silently ignore invalid prefixes
		}

		var newWallet vault.Wallet
		var err error

		if keys.ValidateMnemonic(value) {
			newWallet, _, err = CreateWalletFromMnemonic(value)
		} else if keys.ValidatePrivateKey(value) {
			newWallet, _, err = CreateWalletFromPrivateKey(value)
		} else {
			continue // Silently ignore lines with invalid keys/mnemonics
		}

		if err != nil {
			continue // Silently ignore creation errors
		}
		wallets[prefix] = newWallet
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return wallets, nil
}
