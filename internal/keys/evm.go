// File: internal/keys/evm.go
package keys

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/tyler-smith/go-bip39"
	"vault.module/internal/security"
	"vault.module/internal/vault"
)

const (
	// EVMDerivationPath is the standard derivation path for EVM.
	EVMDerivationPath = "m/44'/60'/0'/0"
)

// EVMManager implements the KeyManager interface for EVM-compatible chains.
type EVMManager struct{}

// CreateWalletFromMnemonic creates a wallet from a mnemonic.
func (m *EVMManager) CreateWalletFromMnemonic(mnemonic string) (vault.Wallet, error) {
	if !m.ValidateMnemonic(mnemonic) {
		return vault.Wallet{}, fmt.Errorf("the provided mnemonic phrase is invalid")
	}

	hdWallet, err := createEVMWalletFromMnemonic(mnemonic)
	if err != nil {
		return vault.Wallet{}, fmt.Errorf("failed to create wallet: %s", err.Error())
	}

	path := fmt.Sprintf("%s/0", EVMDerivationPath)
	privateKey, err := deriveEVMPrivateKey(hdWallet, path)
	if err != nil {
		return vault.Wallet{}, fmt.Errorf("failed to derive private key: %s", err.Error())
	}

	address, err := privateKeyToEVMAddress(privateKey)
	if err != nil {
		return vault.Wallet{}, fmt.Errorf("failed to generate address: %s", err.Error())
	}

	// Create SecureString for private key
	privateKeyStr := privateKeyToEVMString(privateKey)
	privateKeySecure := security.NewSecureString(privateKeyStr)

	// SECURE memory clearing using proper security function
	privateKeyStrBytes := []byte(privateKeyStr)
	security.SecureClearBytes(privateKeyStrBytes) // Use secure multi-pass clearing
	privateKeyStr = "" // Clear string reference
	
	// Clear sensitive data from memory immediately
	privateKeyBytes := crypto.FromECDSA(privateKey)
	for i := range privateKeyBytes {
		privateKeyBytes[i] = 0
	}

	// Create wallet structure
	wallet := vault.Wallet{
		Mnemonic:       security.NewSecureString(mnemonic),
		DerivationPath: EVMDerivationPath,
		Addresses: []vault.Address{
			{
				Index:      0,
				Path:       path,
				Address:    address,
				PrivateKey: privateKeySecure,
			},
		},
	}

	// Set up cleanup in case of future errors (defer not needed here as we return immediately)
	return wallet, nil
}

// CreateWalletFromPrivateKey creates a wallet from a private key.
func (m *EVMManager) CreateWalletFromPrivateKey(pkStr string) (vault.Wallet, error) {
	if !m.ValidatePrivateKey(pkStr) {
		return vault.Wallet{}, fmt.Errorf("the provided private key is invalid")
	}

	privateKey, err := privateKeyFromEVMString(pkStr)
	if err != nil {
		return vault.Wallet{}, fmt.Errorf("failed to process private key: %s", err.Error())
	}

	address, err := privateKeyToEVMAddress(privateKey)
	if err != nil {
		return vault.Wallet{}, fmt.Errorf("failed to generate address: %s", err.Error())
	}

	// Create SecureString for private key
	privateKeyStr := privateKeyToEVMString(privateKey)
	privateKeySecure := security.NewSecureString(privateKeyStr)

	// SECURE memory clearing using proper security function
	privateKeyStrBytes := []byte(privateKeyStr)
	security.SecureClearBytes(privateKeyStrBytes) // Use secure multi-pass clearing
	privateKeyStr = "" // Clear string reference
	
	// Clear sensitive data from memory immediately
	privateKeyBytes := crypto.FromECDSA(privateKey)
	for i := range privateKeyBytes {
		privateKeyBytes[i] = 0
	}

	// Create wallet structure
	wallet := vault.Wallet{
		Addresses: []vault.Address{
			{
				Index:      0,
				Path:       "imported",
				Address:    address,
				PrivateKey: privateKeySecure,
			},
		},
	}

	// Set up cleanup in case of future errors (defer not needed here as we return immediately)
	return wallet, nil
}

// DeriveNextAddress derives the next address for an HD wallet.
func (m *EVMManager) DeriveNextAddress(wallet vault.Wallet) (vault.Wallet, vault.Address, error) {
	if wallet.Mnemonic == nil || wallet.Mnemonic.String() == "" {
		return wallet, vault.Address{}, fmt.Errorf("derivation is only possible for HD wallets (with a mnemonic)")
	}

	nextIndex := len(wallet.Addresses)

	// Use WithValue to safely access mnemonic
	var hdWallet *hdwallet.Wallet
	var err error
	err = wallet.Mnemonic.WithValue(func(mnemonicStr string) error {
		hdWallet, err = createEVMWalletFromMnemonic(mnemonicStr)
		return err
	})
	if err != nil {
		return wallet, vault.Address{}, fmt.Errorf("failed to create wallet from mnemonic: %s", err.Error())
	}

	path := fmt.Sprintf("%s/%d", wallet.DerivationPath, nextIndex)
	privateKey, err := deriveEVMPrivateKey(hdWallet, path)
	if err != nil {
		return wallet, vault.Address{}, fmt.Errorf("failed to derive private key: %s", err.Error())
	}

	address, err := privateKeyToEVMAddress(privateKey)
	if err != nil {
		return wallet, vault.Address{}, fmt.Errorf("failed to generate address: %s", err.Error())
	}

	// Create SecureString for private key
	privateKeyStr := privateKeyToEVMString(privateKey)
	privateKeySecure := security.NewSecureString(privateKeyStr)

	// SECURE memory clearing using proper security function
	privateKeyStrBytes := []byte(privateKeyStr)
	security.SecureClearBytes(privateKeyStrBytes) // Use secure multi-pass clearing
	privateKeyStr = "" // Clear string reference
	
	// Clear sensitive data from memory immediately
	privateKeyBytes := crypto.FromECDSA(privateKey)
	for i := range privateKeyBytes {
		privateKeyBytes[i] = 0
	}

	// Create new address structure
	newAddress := vault.Address{
		Index:      nextIndex,
		Path:       path,
		Address:    address,
		PrivateKey: privateKeySecure,
	}

	// Add cleanup for error cases
	defer func() {
		if err != nil {
			// Clean up secrets if an error occurs after this point
			newAddress.PrivateKey.Clear()
		}
	}()

	wallet.Addresses = append(wallet.Addresses, newAddress)
	return wallet, newAddress, nil
}

// ValidateMnemonic checks if a mnemonic phrase is valid according to the BIP-39 standard.
func (m *EVMManager) ValidateMnemonic(mnemonic string) bool {
	return bip39.IsMnemonicValid(mnemonic)
}

// ValidatePrivateKey checks the format of an EVM private key.
func (m *EVMManager) ValidatePrivateKey(pk string) bool {
	match, _ := regexp.MatchString(`^(0x)?[0-9a-fA-F]{64}$`, pk)
	return match
}

// --- EVM Helper Functions ---

func privateKeyFromEVMString(pk string) (*ecdsa.PrivateKey, error) {
	cleanPk := strings.TrimPrefix(pk, "0x")
	privateKeyBytes, err := hex.DecodeString(cleanPk)
	if err != nil {
		return nil, err
	}
	return crypto.ToECDSA(privateKeyBytes)
}

func createEVMWalletFromMnemonic(mnemonic string) (*hdwallet.Wallet, error) {
	seed := bip39.NewSeed(mnemonic, "")
	return hdwallet.NewFromSeed(seed)
}

func deriveEVMPrivateKey(wallet *hdwallet.Wallet, path string) (*ecdsa.PrivateKey, error) {
	derivationPath, err := hdwallet.ParseDerivationPath(path)
	if err != nil {
		return nil, err
	}
	account, err := wallet.Derive(derivationPath, false)
	if err != nil {
		return nil, err
	}
	return wallet.PrivateKey(account)
}

func privateKeyToEVMAddress(privateKey *ecdsa.PrivateKey) (string, error) {
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("error casting public key to ECDSA")
	}
	return crypto.PubkeyToAddress(*publicKeyECDSA).Hex(), nil
}

func privateKeyToEVMString(privateKey *ecdsa.PrivateKey) string {
	privateKeyBytes := crypto.FromECDSA(privateKey)
	return hex.EncodeToString(privateKeyBytes)
}
