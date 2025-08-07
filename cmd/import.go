// File: cmd/import.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vault.module/internal/actions"
	"vault.module/internal/colors"
	"vault.module/internal/config"
	"vault.module/internal/constants"
	"vault.module/internal/errors"
	"vault.module/internal/security"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var importFormat string
var importConflict string

const (
	// File validation constants
	maxFileSize     = 10 * 1024 * 1024 // 10MB maximum file size
	maxPathLength   = 255              // Maximum file path length
	allowedFileExts = ".json,.txt,.csv"  // Allowed file extensions
)

var importCmd = &cobra.Command{
	Use:   "import <INPUT_FILE>",
	Short: "Bulk imports accounts from a file into the active vault.",
	Long: `Bulk imports accounts from a file into the active vault.

Supported formats:
  - JSON: Standard wallet export format
  - Key-Value: Simple key=value format

The command will prompt for conflict resolution if wallets with same names exist.

Examples:
  vault.module import wallets.json
  vault.module import backup.txt --format keyvalue
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.WrapCommand(func() error {
			// Validate command arguments first
			if err := validateImportCommandArgs(args); err != nil {
				return err
			}

			// Validate flags and parameters
			if err := validateImportCommandInputs(); err != nil {
				return err
			}

			// Check vault status before executing the command
			if err := checkVaultStatus(); err != nil {
				return err
			}

			// Check if shutdown is in progress
			if security.IsShuttingDown() {
				return errors.New(errors.ErrCodeSystem, "system is shutting down, cannot process new commands")
			}

			activeVault, err := config.GetActiveVault()
			if err != nil {
				return err
			}

			if programmaticMode {
				return errors.NewProgrammaticModeError("import")
			}

			filePath := args[0]

			// Additional file validation before processing
			if err := validateFileForImport(filePath); err != nil {
				return err
			}

			fmt.Println(colors.SafeColor(
				fmt.Sprintf("Active Vault: %s (Type: %s)", config.Cfg.ActiveVault, activeVault.Type),
				colors.Info,
			))

			v, err := vault.LoadVault(activeVault)
			if err != nil {
				return errors.NewVaultLoadError(activeVault.KeyFile, err)
			}

			// Ensure vault secrets are cleared when function exits
			defer func() {
				for _, wallet := range v {
					wallet.Clear()
				}
			}()

			content, err := os.ReadFile(filePath)
			if err != nil {
				return errors.NewFileSystemError("read", filePath, err)
			}

			// Register file content for secure cleanup if it contains sensitive data
			if len(content) > 0 {
				security.RegisterTempFileGlobal(filePath, fmt.Sprintf("import file: %s", filePath))
			}

			// Pass the vault type to the action to use the correct key manager.
			updatedVault, report, err := actions.ImportWallets(v, content, importFormat, importConflict, activeVault.Type)
			if err != nil {
				return err
			}

			if err := vault.SaveVault(activeVault, updatedVault); err != nil {
				return errors.NewVaultSaveError(activeVault.KeyFile, err)
			}

			fmt.Println(colors.SafeColor(report, colors.Success))
			return nil
		})
	},
}

// validateImportCommandArgs validates command line arguments
func validateImportCommandArgs(args []string) error {
	if len(args) != 1 {
		return errors.NewInvalidInputError(
			fmt.Sprintf("%d arguments", len(args)),
			"exactly 1 argument required: <INPUT_FILE>",
		)
	}

	filePath := args[0]

	// Validate file path length
	if len(filePath) == 0 {
		return errors.NewInvalidInputError(filePath, "file path cannot be empty")
	}
	if len(filePath) > maxPathLength {
		return errors.NewInvalidInputError(
			filePath,
			fmt.Sprintf("file path length must be at most %d characters", maxPathLength),
		)
	}

	// Validate file path doesn't contain dangerous characters
	dangerousChars := []string{"<", ">", "|", "?", "*", "\x00"}
	for _, char := range dangerousChars {
		if strings.Contains(filePath, char) {
			return errors.NewInvalidInputError(
				filePath,
				fmt.Sprintf("file path contains dangerous character: %s", char),
			)
		}
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	allowedExts := strings.Split(allowedFileExts, ",")
	validExt := false
	for _, allowedExt := range allowedExts {
		if ext == allowedExt {
			validExt = true
			break
		}
	}
	if !validExt {
		return errors.NewInvalidInputError(
			filePath,
			fmt.Sprintf("unsupported file extension '%s'. Allowed extensions: %s", ext, allowedFileExts),
		)
	}

	return nil
}

// validateImportCommandInputs validates input parameters for the import command
func validateImportCommandInputs() error {
	// Validate format parameter
	allowedFormats := []string{constants.FormatJSON, "key-value", "keyvalue"}
	validFormat := false
	for _, allowed := range allowedFormats {
		if strings.EqualFold(importFormat, allowed) {
			validFormat = true
			break
		}
	}
	if !validFormat {
		return errors.NewInvalidInputError(
			importFormat,
			fmt.Sprintf("invalid format '%s'. Allowed formats: %s", importFormat, strings.Join(allowedFormats, ", ")),
		)
	}

	// Validate conflict policy parameter
	allowedPolicies := []string{constants.ConflictPolicySkip, constants.ConflictPolicyOverwrite, constants.ConflictPolicyFail}
	validPolicy := false
	for _, allowed := range allowedPolicies {
		if strings.EqualFold(importConflict, allowed) {
			validPolicy = true
			break
		}
	}
	if !validPolicy {
		return errors.NewInvalidInputError(
			importConflict,
			fmt.Sprintf("invalid conflict policy '%s'. Allowed policies: %s", importConflict, strings.Join(allowedPolicies, ", ")),
		)
	}

	return nil
}

// validateFileForImport performs additional file validation before processing
func validateFileForImport(filePath string) error {
	// Check if file exists and get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.NewInvalidInputError(filePath, "file does not exist")
		}
		return errors.NewFileSystemError("stat", filePath, err)
	}

	// Check if it's a regular file
	if !fileInfo.Mode().IsRegular() {
		return errors.NewInvalidInputError(filePath, "path is not a regular file")
	}

	// Check file size to prevent memory exhaustion
	if fileInfo.Size() > maxFileSize {
		return errors.NewInvalidInputError(
			filePath,
			fmt.Sprintf("file size (%d bytes) exceeds maximum allowed size (%d bytes)", fileInfo.Size(), maxFileSize),
		)
	}

	// Check file permissions (must be readable)
	if fileInfo.Mode().Perm()&0400 == 0 {
		return errors.NewInvalidInputError(filePath, "file is not readable")
	}

	// Validate file is not empty
	if fileInfo.Size() == 0 {
		return errors.NewInvalidInputError(filePath, "file is empty")
	}

	return nil
}

func init() {
	importCmd.Flags().StringVar(&importFormat, "format", constants.FormatJSON, "File format (json or key-value).")
	importCmd.Flags().StringVar(&importConflict, "on-conflict", constants.ConflictPolicySkip, "Behavior on conflict (skip, overwrite, fail).")
}
