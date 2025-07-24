// File: internal/keys/keys.go
package keys

import (
	"fmt"

	"vault.module/internal/constants"
	"vault.module/internal/vault"
)

// KeyManager defines the interface for chain-specific cryptographic operations.
// This allows for easy extension to support new blockchain types.
type KeyManager interface {
	// CreateWalletFromMnemonic creates a new wallet from a mnemonic phrase.
	CreateWalletFromMnemonic(mnemonic string) (vault.Wallet, error)
	// CreateWalletFromPrivateKey creates a new wallet from a private key string.
	CreateWalletFromPrivateKey(pk string) (vault.Wallet, error)
	// DeriveNextAddress derives the next address for an existing HD wallet.
	DeriveNextAddress(wallet vault.Wallet) (vault.Wallet, vault.Address, error)
	// ValidateMnemonic checks if a mnemonic phrase is valid.
	ValidateMnemonic(mnemonic string) bool
	// ValidatePrivateKey checks the format of a private key string.
	ValidatePrivateKey(pk string) bool
}

// GetKeyManager is a factory function that returns the correct KeyManager
// implementation based on the vault's type.
func GetKeyManager(vaultType string) (KeyManager, error) {
	switch vaultType {
	case constants.VaultTypeEVM:
		return &EVMManager{}, nil
	case constants.VaultTypeCosmos:
		return &CosmosManager{}, nil
	default:
		return nil, fmt.Errorf("unsupported vault type: '%s'", vaultType)
	}
}
