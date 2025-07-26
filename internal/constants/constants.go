// File: internal/constants/constants.go
package constants

// Vault types
const (
	VaultTypeEVM    = "evm"
	VaultTypeCosmos = "cosmos"
)

// Encryption methods
const (
	EncryptionYubiKey    = "yubikey"
	EncryptionPassphrase = "passphrase"
)

// Import formats
const (
	FormatJSON     = "json"
	FormatKeyValue = "keyvalue"
)

// Conflict resolution policies
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
