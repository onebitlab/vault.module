// File: internal/config/config.go
package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
	"vault.module/internal/errors"
)

// VaultDetails holds the paths and type for a single vault.
type VaultDetails struct {
	KeyFile        string `mapstructure:"keyfile"`
	RecipientsFile string `mapstructure:"recipientsfile"`
	Type           string `mapstructure:"type"`
	Encryption     string `mapstructure:"encryption"` // <-- NEW FIELD
}

// Config defines the new structure of the configuration file.
type Config struct {
	AuthToken           string                  `mapstructure:"authtoken"`
	YubikeySlot         string                  `mapstructure:"yubikeyslot"`
	YubikeyTimeout      int                     `mapstructure:"yubikey_timeout"`    // Timeout in seconds for YubiKey operations
	ActiveVault         string                  `mapstructure:"active_vault"`
	ClipboardTimeout    int                     `mapstructure:"clipboard_timeout"`    // Timeout in seconds for clipboard clearing
	Vaults              map[string]VaultDetails `mapstructure:"vaults"`
}

// Cfg is a global variable that holds the loaded configuration.
var Cfg Config

// GetActiveVault returns the details for the currently active vault.
func GetActiveVault() (VaultDetails, error) {
	if Cfg.ActiveVault == "" {
		return VaultDetails{}, errors.NewActiveVaultNotSetError()
	}
	activeVault, ok := Cfg.Vaults[Cfg.ActiveVault]
	if !ok {
		return VaultDetails{}, errors.NewVaultNotFoundError(Cfg.ActiveVault)
	}
	if activeVault.Type == "" {
		return VaultDetails{}, errors.NewConfigValidationError("type", "", fmt.Sprintf("active vault '%s' has no type defined in config.json", Cfg.ActiveVault))
	}
	if activeVault.Encryption == "" {
		return VaultDetails{}, errors.NewConfigValidationError("encryption", "", fmt.Sprintf("active vault '%s' has no encryption method defined in config.json", Cfg.ActiveVault))
	}
	return activeVault, nil
}

// LoadConfig loads the configuration from a file and environment variables.
func LoadConfig() error {
	viper.SetDefault("authtoken", "")
	viper.SetDefault("yubikeyslot", "")
	viper.SetDefault("yubikey_timeout", 60) // Default 60 seconds for YubiKey operations
	viper.SetDefault("active_vault", "")
	viper.SetDefault("clipboard_timeout", 30) // Default 30 seconds
	viper.SetDefault("vaults", map[string]VaultDetails{})
	viper.SetConfigName("config")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("VAULT")
	viper.AutomaticEnv()
	_ = viper.BindEnv("authtoken", "VAULT_AUTH_TOKEN")
	_ = viper.BindEnv("yubikeyslot", "VAULT_YUBIKEY_SLOT")
	_ = viper.BindEnv("yubikey_timeout", "VAULT_YUBIKEY_TIMEOUT")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return errors.NewConfigLoadError("config.json", err)
		}
	}
	return viper.Unmarshal(&Cfg)
}

// GetClipboardTimeout returns the clipboard timeout value from configuration.
// If not set or invalid, returns the default value of 30 seconds.
func GetClipboardTimeout() int {
	if Cfg.ClipboardTimeout <= 0 {
		return 30 // Default fallback
	}
	return Cfg.ClipboardTimeout
}

// SaveConfig saves the current configuration to a file.
func SaveConfig() error {
	viper.Set("authtoken", Cfg.AuthToken)
	viper.Set("yubikeyslot", Cfg.YubikeySlot)
	viper.Set("yubikey_timeout", Cfg.YubikeyTimeout)
	viper.Set("active_vault", Cfg.ActiveVault)
	viper.Set("clipboard_timeout", Cfg.ClipboardTimeout)
	viper.Set("vaults", Cfg.Vaults)
	if err := os.MkdirAll(".", 0755); err != nil {
		return errors.FromOSError(err, ".")
	}
	if err := viper.WriteConfigAs("config.json"); err != nil {
		return errors.NewConfigSaveError("config.json", err)
	}
	return nil
}
