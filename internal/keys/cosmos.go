// File: internal/keys/cosmos.go
package keys

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/go-bip39"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"vault.module/internal/security"
	"vault.module/internal/vault"
)

const (
	// CosmosDerivationPath is a standard derivation path for Cosmos.
	CosmosDerivationPath = "m/44'/118'/0'/0"
)

// CosmosManager implements the KeyManager interface for Cosmos-based chains.
type CosmosManager struct{}

// CreateWalletFromMnemonic creates a Cosmos wallet from a mnemonic.
func (m *CosmosManager) CreateWalletFromMnemonic(mnemonic string) (vault.Wallet, error) {
	if !m.ValidateMnemonic(mnemonic) {
		return vault.Wallet{}, fmt.Errorf("the provided mnemonic phrase is invalid")
	}

	path := fmt.Sprintf("%s/0", CosmosDerivationPath)
	privKey, err := deriveCosmosPrivateKey(mnemonic, path)
	if err != nil {
		return vault.Wallet{}, err
	}

	address := privKey.PubKey().Address().String()

	return vault.Wallet{
		Mnemonic:       security.NewSecureString(mnemonic),
		DerivationPath: CosmosDerivationPath,
		Addresses: []vault.Address{
			{
				Index:      0,
				Path:       path,
				Address:    address,
				PrivateKey: security.NewSecureString(fmt.Sprintf("%X", privKey.Bytes())),
			},
		},
	}, nil
}

// CreateWalletFromPrivateKey is not supported for Cosmos in this implementation
// as it's less common and secure than using mnemonics.
func (m *CosmosManager) CreateWalletFromPrivateKey(pk string) (vault.Wallet, error) {
	return vault.Wallet{}, fmt.Errorf("creating from a raw private key is not supported for Cosmos wallets; please use a mnemonic")
}

// DeriveNextAddress derives the next address for a Cosmos HD wallet.
func (m *CosmosManager) DeriveNextAddress(wallet vault.Wallet) (vault.Wallet, vault.Address, error) {
	if wallet.Mnemonic == nil || wallet.Mnemonic.String() == "" {
		return wallet, vault.Address{}, fmt.Errorf("derivation is only possible for HD wallets (with a mnemonic)")
	}

	nextIndex := len(wallet.Addresses)
	path := fmt.Sprintf("%s/%d", wallet.DerivationPath, nextIndex)

	privKey, err := deriveCosmosPrivateKey(wallet.Mnemonic.String(), path)
	if err != nil {
		return wallet, vault.Address{}, err
	}

	address := privKey.PubKey().Address().String()

	newAddress := vault.Address{
		Index:      nextIndex,
		Path:       path,
		Address:    address,
		PrivateKey: security.NewSecureString(fmt.Sprintf("%X", privKey.Bytes())),
	}

	wallet.Addresses = append(wallet.Addresses, newAddress)
	return wallet, newAddress, nil
}

// ValidateMnemonic checks if a mnemonic phrase is valid.
func (m *CosmosManager) ValidateMnemonic(mnemonic string) bool {
	return bip39.IsMnemonicValid(mnemonic)
}

// ValidatePrivateKey is not applicable for Cosmos in this context.
func (m *CosmosManager) ValidatePrivateKey(pk string) bool {
	return false
}

// --- Cosmos Helper Functions ---

func deriveCosmosPrivateKey(mnemonic, path string) (secp256k1.PrivKey, error) {
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		return nil, err
	}
	master, ch := hd.ComputeMastersFromSeed(seed)
	derived, err := hd.DerivePrivateKeyForPath(master, ch, path)
	if err != nil {
		return nil, err
	}
	return secp256k1.PrivKey(derived), nil
}
