// File: internal/constants/constants.go
package constants

// Vault Types
const (
	VaultTypeEVM    = "EVM"
	VaultTypeCosmos = "COSMOS"
)

// Encryption Methods
const (
	EncryptionYubiKey    = "yubikey"
	EncryptionPassphrase = "passphrase"
)

// Import/Export Formats
const (
	FormatJSON     = "json"
	FormatKeyValue = "key-value"
)

// Import Conflict Policies
const (
	ConflictPolicySkip      = "skip"
	ConflictPolicyOverwrite = "overwrite"
	ConflictPolicyFail      = "fail"
)

// Copyable Fields
const (
	FieldAddress    = "address"
	FieldPrivateKey = "privatekey"
	FieldMnemonic   = "mnemonic"
)
