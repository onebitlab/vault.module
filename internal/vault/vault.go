// File: internal/vault/vault.go
package vault

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"vault.module/internal/config"
	"vault.module/internal/constants"
)

// Address defines the structure for a single address.
type Address struct {
	Index      int    `json:"index"`
	Path       string `json:"path"`
	Address    string `json:"address"`
	PrivateKey string `json:"privateKey"`
}

// Wallet defines the structure for a wallet, which can be HD or a single key.
type Wallet struct {
	Mnemonic       string    `json:"mnemonic,omitempty"`
	DerivationPath string    `json:"derivationPath,omitempty"`
	Addresses      []Address `json:"addresses"`
	Notes          string    `json:"notes"`
	Tags           []string  `json:"tags"`
}

// Sanitize creates a "clean" copy of the wallet for safe display.
func (w Wallet) Sanitize() Wallet {
	sanitizedWallet := w
	if sanitizedWallet.Mnemonic != "" {
		sanitizedWallet.Mnemonic = "[REDACTED]"
	}

	sanitizedAddresses := make([]Address, len(w.Addresses))
	for i, addr := range w.Addresses {
		sanitizedAddresses[i] = addr
		sanitizedAddresses[i].PrivateKey = "[REDACTED]"
	}
	sanitizedWallet.Addresses = sanitizedAddresses
	return sanitizedWallet
}

// Vault is the root structure of our vault (the JSON file).
type Vault map[string]Wallet

// New creates an empty vault.
func New() Vault {
	return make(Vault)
}

// CheckYubiKey checks for the availability of a YubiKey.
func CheckYubiKey() error {
	cmd := exec.Command("age-plugin-yubikey", "--list")
	output, err := cmd.CombinedOutput() // CombinedOutput gets both stdout and stderr
	if err != nil {
		return fmt.Errorf("could not run yubikey check: %v\n%s", err, string(output))
	}
	if strings.TrimSpace(string(output)) == "" {
		return fmt.Errorf("no yubikey found or no age keys on it")
	}
	return nil
}

// LoadVault decrypts and loads the vault from a file, using the specified method.
func LoadVault(details config.VaultDetails) (Vault, error) {
	if _, err := os.Stat(details.KeyFile); os.IsNotExist(err) {
		// If the vault file doesn't exist, return a new, empty vault.
		return make(Vault), nil
	}

	var ageCmd *exec.Cmd

	switch details.Encryption {
	case constants.EncryptionYubiKey:
		pluginArgs := []string{"-i"}
		if config.Cfg.YubikeySlot != "" {
			pluginArgs = append(pluginArgs, "--slot", config.Cfg.YubikeySlot)
		}
		pluginCmd := exec.Command("age-plugin-yubikey", pluginArgs...)

		tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			return nil, fmt.Errorf("could not open TTY for PIN entry: %w", err)
		}
		defer tty.Close()
		pluginCmd.Stdin = tty
		pluginCmd.Stderr = tty

		identity, err := pluginCmd.Output()
		if err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				return nil, fmt.Errorf("yubikey plugin error (maybe wrong PIN?): %v", err)
			}
			return nil, fmt.Errorf("failed to run age-plugin-yubikey: %v", err)
		}

		ageCmd = exec.Command("age", "--decrypt", "-i", "-", details.KeyFile)
		ageCmd.Stdin = bytes.NewReader(identity)

	case constants.EncryptionPassphrase:
		ageCmd = exec.Command("age", "--decrypt", "--passphrase", details.KeyFile)
		tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			return nil, fmt.Errorf("could not open TTY for passphrase entry: %w", err)
		}
		defer tty.Close()
		ageCmd.Stdin = tty
		ageCmd.Stderr = os.Stderr // Show prompt to user

	default:
		return nil, fmt.Errorf("unknown encryption method: %s", details.Encryption)
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	ageCmd.Stdout = &out
	if ageCmd.Stderr == nil {
		ageCmd.Stderr = &stderr
	}

	if err := ageCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to decrypt vault: %v\n%s", err, stderr.String())
	}

	var v Vault
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		return nil, fmt.Errorf("failed to parse vault data (file may be corrupt): %w", err)
	}
	return v, nil
}

// SaveVault encrypts and saves the vault to a file.
func SaveVault(details config.VaultDetails, v Vault) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize data: %w", err)
	}

	var cmd *exec.Cmd

	switch details.Encryption {
	case constants.EncryptionYubiKey:
		if details.RecipientsFile == "" {
			return fmt.Errorf("recipients file is required for yubikey encryption")
		}
		if _, err := os.Stat(details.RecipientsFile); os.IsNotExist(err) {
			return fmt.Errorf("recipients file '%s' not found", details.RecipientsFile)
		}
		args := []string{"-a", "-r", details.RecipientsFile, "-o", details.KeyFile}
		cmd = exec.Command("age", args...)
		cmd.Stdin = bytes.NewReader(data)

	case constants.EncryptionPassphrase:
		tmpfile, err := os.CreateTemp("", "vault-*.json")
		if err != nil {
			return fmt.Errorf("could not create temp file: %w", err)
		}
		defer os.Remove(tmpfile.Name()) // clean up

		if _, err := tmpfile.Write(data); err != nil {
			return fmt.Errorf("could not write to temp file: %w", err)
		}
		if err := tmpfile.Close(); err != nil {
			return fmt.Errorf("could not close temp file: %w", err)
		}

		cmd = exec.Command("age", "-p", "-o", details.KeyFile, tmpfile.Name())
		tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			return fmt.Errorf("could not open TTY for passphrase entry: %w", err)
		}
		defer tty.Close()
		cmd.Stdin = tty
		cmd.Stderr = tty

	default:
		return fmt.Errorf("unknown encryption method: %s", details.Encryption)
	}

	var stderr bytes.Buffer
	if cmd.Stderr == nil {
		cmd.Stderr = &stderr
	}

	if runErr := cmd.Run(); runErr != nil {
		return fmt.Errorf("failed to encrypt vault: %v\n%s", runErr, stderr.String())
	}

	return nil
}
