// internal/tui/screens/import_export.go
package screens

import (
	"fmt"
	"strings"

	"vault.module/internal/tui/utils"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ImportExportOperation represents an import/export operation
type ImportExportOperation struct {
	title       string
	description string
	action      string
	category    string // "import" or "export"
}

func (i ImportExportOperation) FilterValue() string { return i.title }
func (i ImportExportOperation) Title() string       { return i.title }
func (i ImportExportOperation) Description() string { return i.description }

// ImportExportScreen represents the import/export screen
type ImportExportScreen struct {
	id            string
	title         string
	operationList list.Model
	filePicker    filepicker.Model

	// Form inputs
	filePathInput textinput.Model
	passwordInput textinput.Model

	// State
	currentOperation string
	showForm         bool
	showFilePicker   bool
	resultText       string
	width            int
	height           int
	err              error
}

// NewImportExportScreen creates a new import/export screen
func NewImportExportScreen() *ImportExportScreen {
	operations := []list.Item{
		ImportExportOperation{
			title:       "Export Vault",
			description: "Export entire vault to encrypted file",
			action:      "export_vault",
			category:    "export",
		},
		ImportExportOperation{
			title:       "Import Vault",
			description: "Import vault from encrypted file",
			action:      "import_vault",
			category:    "import",
		},
		ImportExportOperation{
			title:       "Export Wallet",
			description: "Export specific wallet to file",
			action:      "export_wallet",
			category:    "export",
		},
		ImportExportOperation{
			title:       "Import Wallet",
			description: "Import wallet from file",
			action:      "import_wallet",
			category:    "import",
		},
		ImportExportOperation{
			title:       "Export Mnemonic",
			description: "Export mnemonic phrase to secure file",
			action:      "export_mnemonic",
			category:    "export",
		},
		ImportExportOperation{
			title:       "Import Mnemonic",
			description: "Import mnemonic phrase from file",
			action:      "import_mnemonic",
			category:    "import",
		},
		ImportExportOperation{
			title:       "Backup Configuration",
			description: "Backup application configuration",
			action:      "backup_config",
			category:    "export",
		},
		ImportExportOperation{
			title:       "Restore Configuration",
			description: "Restore application configuration",
			action:      "restore_config",
			category:    "import",
		},
	}

	operationList := list.New(operations, list.NewDefaultDelegate(), 0, 0)
	operationList.Title = "Import/Export Operations"

	fp := filepicker.New()
	fp.AllowedTypes = []string{".json", ".vault", ".backup"}
	fp.CurrentDirectory = "."

	filePathInput := textinput.New()
	filePathInput.Placeholder = "/path/to/file"
	filePathInput.CharLimit = 200

	passwordInput := textinput.New()
	passwordInput.Placeholder = "Enter password for encryption"
	passwordInput.EchoMode = textinput.EchoPassword
	passwordInput.CharLimit = 100

	return &ImportExportScreen{
		id:             "import_export",
		title:          "Import/Export",
		operationList:  operationList,
		filePicker:     fp,
		filePathInput:  filePathInput,
		passwordInput:  passwordInput,
		showForm:       false,
		showFilePicker: false,
	}
}

// ID returns the screen identifier
func (s *ImportExportScreen) ID() string {
	return s.id
}

// Title returns the screen title
func (s *ImportExportScreen) Title() string {
	return s.title
}

// CanGoBack determines if we can go back from this screen
func (s *ImportExportScreen) CanGoBack() bool {
	return true
}

// Init initializes the screen
func (s *ImportExportScreen) Init() tea.Cmd {
	return s.filePicker.Init()
}

// Update handles messages
func (s *ImportExportScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.operationList.SetSize(msg.Width-4, msg.Height-10)

	case tea.KeyMsg:
		if s.showFilePicker {
			switch msg.String() {
			case "esc":
				s.showFilePicker = false
				return s, nil
			case "enter":
				if selectedFile, path := s.filePicker.DidSelectFile(msg); selectedFile {
					s.filePathInput.SetValue(path)
					s.showFilePicker = false
					return s, nil
				}
			}

			s.filePicker, cmd = s.filePicker.Update(msg)
			return s, cmd

		} else if s.showForm {
			switch msg.String() {
			case "esc":
				s.showForm = false
				s.resultText = ""
				s.err = nil
				return s, nil
			case "enter":
				return s.executeOperation()
			case "tab":
				if s.filePathInput.Focused() {
					s.filePathInput.Blur()
					s.passwordInput.Focus()
				} else {
					s.passwordInput.Blur()
					s.filePathInput.Focus()
				}
			case "ctrl+f":
				s.showFilePicker = true
				return s, s.filePicker.Init()
			}

			// Update form inputs
			if s.filePathInput.Focused() {
				s.filePathInput, cmd = s.filePathInput.Update(msg)
			} else {
				s.passwordInput, cmd = s.passwordInput.Update(msg)
			}
		} else {
			switch msg.String() {
			case "enter":
				return s.handleSelection()
			}

			s.operationList, cmd = s.operationList.Update(msg)
		}
	}

	return s, cmd
}

// View renders the screen
func (s *ImportExportScreen) View() string {
	stateManager := utils.GetStateManager()
	theme := stateManager.GetTheme()

	var content strings.Builder

	// Title
	content.WriteString(theme.Title.Render("Import/Export Operations"))
	content.WriteString("\n\n")

	// Current vault info
	vaultName, _ := stateManager.GetCurrentVault()
	if vaultName != "" {
		content.WriteString(theme.InfoStyle.Render("Current vault: " + vaultName))
	} else {
		content.WriteString(theme.WarningStyle.Render("No vault selected"))
	}
	content.WriteString("\n\n")

	if s.showFilePicker {
		content.WriteString(s.renderFilePicker(theme))
	} else if s.showForm {
		content.WriteString(s.renderOperationForm(theme))
	} else {
		content.WriteString(s.operationList.View())
		content.WriteString("\n\n")
		content.WriteString(theme.Status.Render("Press Enter to select operation, ESC to go back"))
	}

	// Error display
	if s.err != nil {
		content.WriteString("\n\n")
		content.WriteString(theme.ErrorStyle.Render("Error: " + s.err.Error()))
	}

	return content.String()
}

// renderFilePicker renders the file picker
func (s *ImportExportScreen) renderFilePicker(theme *utils.Theme) string {
	var content strings.Builder

	content.WriteString(theme.Subtitle.Render("Select File"))
	content.WriteString("\n\n")
	content.WriteString(s.filePicker.View())
	content.WriteString("\n\n")
	content.WriteString(theme.Status.Render("Enter: Select | ESC: Cancel"))

	return content.String()
}

// renderOperationForm renders the operation form
func (s *ImportExportScreen) renderOperationForm(theme *utils.Theme) string {
	var content strings.Builder

	content.WriteString(theme.Subtitle.Render(s.getOperationTitle(s.currentOperation)))
	content.WriteString("\n\n")

	// File path input
	content.WriteString("File Path:")
	content.WriteString("\n")
	content.WriteString(s.filePathInput.View())
	content.WriteString("\n")
	content.WriteString(theme.Status.Render("Press Ctrl+F to open file browser"))
	content.WriteString("\n\n")

	// Password input (for encrypted operations)
	if s.requiresPassword(s.currentOperation) {
		content.WriteString("Password:")
		content.WriteString("\n")
		content.WriteString(s.passwordInput.View())
		content.WriteString("\n\n")
	}

	// Operation-specific information
	content.WriteString(s.getOperationInfo(s.currentOperation, theme))
	content.WriteString("\n\n")

	// Result display
	if s.resultText != "" {
		content.WriteString(theme.Subtitle.Render("Result:"))
		content.WriteString("\n\n")
		content.WriteString(theme.SuccessStyle.Render(s.resultText))
		content.WriteString("\n\n")
	}

	content.WriteString(theme.Status.Render("Enter: Execute | Tab: Next Field | Ctrl+F: File Browser | ESC: Back"))

	return content.String()
}

// getOperationTitle returns the title for an operation
func (s *ImportExportScreen) getOperationTitle(operation string) string {
	switch operation {
	case "export_vault":
		return "Export Vault"
	case "import_vault":
		return "Import Vault"
	case "export_wallet":
		return "Export Wallet"
	case "import_wallet":
		return "Import Wallet"
	case "export_mnemonic":
		return "Export Mnemonic"
	case "import_mnemonic":
		return "Import Mnemonic"
	case "backup_config":
		return "Backup Configuration"
	case "restore_config":
		return "Restore Configuration"
	default:
		return "Import/Export Operation"
	}
}

// getOperationInfo returns information about the operation
func (s *ImportExportScreen) getOperationInfo(operation string, theme *utils.Theme) string {
	switch operation {
	case "export_vault":
		return theme.InfoStyle.Render("This will export the entire vault including all wallets and keys to an encrypted file.")
	case "import_vault":
		return theme.WarningStyle.Render("This will import a vault from file. Existing vault with same name will be overwritten.")
	case "export_wallet":
		return theme.InfoStyle.Render("Export a specific wallet from the current vault.")
	case "import_wallet":
		return theme.InfoStyle.Render("Import a wallet into the current vault.")
	case "export_mnemonic":
		return theme.WarningStyle.Render("⚠️  Mnemonic phrases are highly sensitive. Ensure secure storage.")
	case "import_mnemonic":
		return theme.WarningStyle.Render("⚠️  Only import mnemonic phrases from trusted sources.")
	case "backup_config":
		return theme.InfoStyle.Render("Backup application settings and configuration.")
	case "restore_config":
		return theme.WarningStyle.Render("This will overwrite current application configuration.")
	default:
		return ""
	}
}

// requiresPassword checks if operation requires password
func (s *ImportExportScreen) requiresPassword(operation string) bool {
	switch operation {
	case "export_vault", "import_vault", "export_mnemonic", "import_mnemonic":
		return true
	default:
		return false
	}
}

// handleSelection handles operation selection
func (s *ImportExportScreen) handleSelection() (tea.Model, tea.Cmd) {
	selected := s.operationList.SelectedItem()
	if operation, ok := selected.(ImportExportOperation); ok {
		s.currentOperation = operation.action
		s.showForm = true
		s.resultText = ""
		s.err = nil

		// Set default file extension based on operation
		switch operation.action {
		case "export_vault", "import_vault":
			s.filePathInput.SetValue("vault_backup.vault")
		case "export_wallet", "import_wallet":
			s.filePathInput.SetValue("wallet_export.json")
		case "export_mnemonic", "import_mnemonic":
			s.filePathInput.SetValue("mnemonic.txt")
		case "backup_config", "restore_config":
			s.filePathInput.SetValue("config_backup.json")
		}

		s.filePathInput.Focus()
	}
	return s, nil
}

// executeOperation executes the selected operation
func (s *ImportExportScreen) executeOperation() (tea.Model, tea.Cmd) {
	filePath := strings.TrimSpace(s.filePathInput.Value())
	if filePath == "" {
		s.err = fmt.Errorf("file path cannot be empty")
		return s, nil
	}

	password := s.passwordInput.Value()
	if s.requiresPassword(s.currentOperation) && password == "" {
		s.err = fmt.Errorf("password is required for this operation")
		return s, nil
	}

	switch s.currentOperation {
	case "export_vault":
		return s.exportVault(filePath, password)
	case "import_vault":
		return s.importVault(filePath, password)
	case "export_wallet":
		return s.exportWallet(filePath)
	case "import_wallet":
		return s.importWallet(filePath)
	case "export_mnemonic":
		return s.exportMnemonic(filePath, password)
	case "import_mnemonic":
		return s.importMnemonic(filePath, password)
	case "backup_config":
		return s.backupConfig(filePath)
	case "restore_config":
		return s.restoreConfig(filePath)
	}

	return s, nil
}

// exportVault exports the current vault
func (s *ImportExportScreen) exportVault(filePath, password string) (tea.Model, tea.Cmd) {
	// TODO: Implement actual vault export
	s.resultText = fmt.Sprintf("Vault exported successfully to: %s", filePath)
	return s, nil
}

// importVault imports a vault from file
func (s *ImportExportScreen) importVault(filePath, password string) (tea.Model, tea.Cmd) {
	// TODO: Implement actual vault import
	s.resultText = fmt.Sprintf("Vault imported successfully from: %s", filePath)
	return s, nil
}

// exportWallet exports a wallet
func (s *ImportExportScreen) exportWallet(filePath string) (tea.Model, tea.Cmd) {
	// TODO: Implement actual wallet export
	s.resultText = fmt.Sprintf("Wallet exported successfully to: %s", filePath)
	return s, nil
}

// importWallet imports a wallet
func (s *ImportExportScreen) importWallet(filePath string) (tea.Model, tea.Cmd) {
	// TODO: Implement actual wallet import
	s.resultText = fmt.Sprintf("Wallet imported successfully from: %s", filePath)
	return s, nil
}

// exportMnemonic exports mnemonic phrase
func (s *ImportExportScreen) exportMnemonic(filePath, password string) (tea.Model, tea.Cmd) {
	// TODO: Implement actual mnemonic export
	s.resultText = fmt.Sprintf("Mnemonic exported successfully to: %s", filePath)
	return s, nil
}

// importMnemonic imports mnemonic phrase
func (s *ImportExportScreen) importMnemonic(filePath, password string) (tea.Model, tea.Cmd) {
	// TODO: Implement actual mnemonic import
	s.resultText = fmt.Sprintf("Mnemonic imported successfully from: %s", filePath)
	return s, nil
}

// backupConfig backs up configuration
func (s *ImportExportScreen) backupConfig(filePath string) (tea.Model, tea.Cmd) {
	// TODO: Implement actual config backup
	s.resultText = fmt.Sprintf("Configuration backed up successfully to: %s", filePath)
	return s, nil
}

// restoreConfig restores configuration
func (s *ImportExportScreen) restoreConfig(filePath string) (tea.Model, tea.Cmd) {
	// TODO: Implement actual config restore
	s.resultText = fmt.Sprintf("Configuration restored successfully from: %s", filePath)
	return s, nil
}
