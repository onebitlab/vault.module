// File: internal/keys/keys.go
package keys

import (
	"fmt"
	"strings"

	"vault.module/internal/constants"
	"vault.module/internal/vault"
)

// KeyManager defines the interface for key management operations.
type KeyManager interface {
	CreateWalletFromMnemonic(mnemonic string) (vault.Wallet, error)
	CreateWalletFromPrivateKey(pk string) (vault.Wallet, error)
	DeriveNextAddress(wallet vault.Wallet) (vault.Wallet, vault.Address, error)
	ValidateMnemonic(mnemonic string) bool
	ValidatePrivateKey(pk string) bool
}

// GetKeyManager returns the appropriate key manager for the given vault type.
func GetKeyManager(vaultType string) (KeyManager, error) {
	normalized := strings.ToLower(strings.TrimSpace(vaultType))
	switch normalized {
	case constants.VaultTypeEVM:
		return &EVMManager{}, nil
	case constants.VaultTypeCosmos:
		return &CosmosManager{}, nil
	default:
		return nil, fmt.Errorf("unsupported vault type: %s (supported: %s, %s)",
			vaultType, constants.VaultTypeEVM, constants.VaultTypeCosmos)
	}
}
