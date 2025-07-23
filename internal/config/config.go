// File: internal/config/config.go
package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
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
	AuthToken   string                  `mapstructure:"authtoken"`
	YubikeySlot string                  `mapstructure:"yubikeyslot"`
	ActiveVault string                  `mapstructure:"active_vault"`
	Vaults      map[string]VaultDetails `mapstructure:"vaults"`
}

// Cfg is a global variable that holds the loaded configuration.
var Cfg Config

// GetActiveVault returns the details for the currently active vault.
func GetActiveVault() (VaultDetails, error) {
	if Cfg.ActiveVault == "" {
		return VaultDetails{}, fmt.Errorf("no active vault is set. Use 'vaults use <name>' to set one")
	}
	activeVault, ok := Cfg.Vaults[Cfg.ActiveVault]
	if !ok {
		return VaultDetails{}, fmt.Errorf("active vault '%s' not found in configuration", Cfg.ActiveVault)
	}
	if activeVault.Type == "" {
		return VaultDetails{}, fmt.Errorf("active vault '%s' has no type defined in config.json", Cfg.ActiveVault)
	}
	if activeVault.Encryption == "" {
		// Default to yubikey for backward compatibility if the field is missing.
		activeVault.Encryption = "yubikey"
	}
	return activeVault, nil
}

// LoadConfig loads the configuration from a file and environment variables.
func LoadConfig() error {
	viper.SetDefault("authtoken", "")
	viper.SetDefault("yubikeyslot", "")
	viper.SetDefault("active_vault", "")
	viper.SetDefault("vaults", map[string]VaultDetails{})
	viper.SetConfigName("config")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("VAULT")
	viper.AutomaticEnv()
	_ = viper.BindEnv("authtoken", "VAULT_AUTH_TOKEN")
	_ = viper.BindEnv("yubikeyslot", "VAULT_YUBIKEY_SLOT")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}
	return viper.Unmarshal(&Cfg)
}

// SaveConfig saves the current configuration to a file.
func SaveConfig() error {
	viper.Set("authtoken", Cfg.AuthToken)
	viper.Set("yubikeyslot", Cfg.YubikeySlot)
	viper.Set("active_vault", Cfg.ActiveVault)
	viper.Set("vaults", Cfg.Vaults)
	if err := os.MkdirAll(".", 0755); err != nil {
		return err
	}
	return viper.WriteConfigAs("config.json")
}
