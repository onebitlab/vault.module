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
)

const (
	// EVMDerivationPath is the standard derivation path for EVM.
	EVMDerivationPath = "m/44'/60'/0'/0"
)

// ValidateMnemonic checks if a mnemonic phrase is valid according to the BIP-39 standard.
func ValidateMnemonic(mnemonic string) bool {
	return bip39.IsMnemonicValid(mnemonic)
}

// ValidatePrivateKey checks the format of an EVM private key.
func ValidatePrivateKey(pk string) bool {
	match, _ := regexp.MatchString(`^(0x)?[0-9a-fA-F]{64}$`, pk)
	return match
}

// PrivateKeyFromString converts a hex string into a private key object.
func PrivateKeyFromString(pk string) (*ecdsa.PrivateKey, error) {
	if !ValidatePrivateKey(pk) {
		return nil, fmt.Errorf("invalid private key format")
	}

	// Trim the 0x prefix if it exists to work with the raw hex.
	cleanPk := strings.TrimPrefix(pk, "0x")

	privateKeyBytes, err := hex.DecodeString(cleanPk)
	if err != nil {
		return nil, err
	}
	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

// CreateWalletFromMnemonic creates an HD wallet from a mnemonic phrase.
func CreateWalletFromMnemonic(mnemonic string) (*hdwallet.Wallet, error) {
	if !ValidateMnemonic(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic phrase")
	}
	seed := bip39.NewSeed(mnemonic, "")
	return hdwallet.NewFromSeed(seed)
}

// DerivePrivateKey gets the private key for a specific derivation path.
func DerivePrivateKey(wallet *hdwallet.Wallet, path string) (*ecdsa.PrivateKey, error) {
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

// PrivateKeyToAddress converts a private key to its public address.
func PrivateKeyToAddress(privateKey *ecdsa.PrivateKey) (string, error) {
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("error casting public key to ECDSA")
	}
	return crypto.PubkeyToAddress(*publicKeyECDSA).Hex(), nil
}

// PrivateKeyToString converts a private key to its string representation.
func PrivateKeyToString(privateKey *ecdsa.PrivateKey) string {
	privateKeyBytes := crypto.FromECDSA(privateKey)
	// Return the raw hex string without the 0x prefix.
	return hex.EncodeToString(privateKeyBytes)
}
