// internal/tui/screens/key_operations.go
package screens

import (
	"fmt"
	"strings"

	"vault.module/internal/tui/utils"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// KeyOperation represents a key operation
type KeyOperation struct {
	title       string
	description string
	action      string
}

func (i KeyOperation) FilterValue() string { return i.title }
func (i KeyOperation) Title() string       { return i.title }
func (i KeyOperation) Description() string { return i.description }

// KeyOperationsScreen represents the key operations screen
type KeyOperationsScreen struct {
	id            string
	title         string
	operationList list.Model

	// Operation forms
	currentOperation string
	pathInput        textinput.Model
	countInput       textinput.Model
	resultText       string

	// State
	width    int
	height   int
	showForm bool
	err      error
}

// NewKeyOperationsScreen creates a new key operations screen
func NewKeyOperationsScreen() *KeyOperationsScreen {
	operations := []list.Item{
		KeyOperation{
			title:       "Generate Mnemonic",
			description: "Generate a new BIP39 mnemonic phrase",
			action:      "generate_mnemonic",
		},
		KeyOperation{
			title:       "Derive Addresses",
			description: "Derive addresses from current vault",
			action:      "derive_addresses",
		},
		KeyOperation{
			title:       "Generate Private Key",
			description: "Generate a standalone private key",
			action:      "generate_private_key",
		},
		KeyOperation{
			title:       "Validate Mnemonic",
			description: "Validate a BIP39 mnemonic phrase",
			action:      "validate_mnemonic",
		},
		KeyOperation{
			title:       "Address from Public Key",
			description: "Generate address from public key",
			action:      "address_from_pubkey",
		},
	}

	operationList := list.New(operations, list.NewDefaultDelegate(), 0, 0)
	operationList.Title = "Key Operations"

	pathInput := textinput.New()
	pathInput.Placeholder = "m/44'/60'/0'/0"
	pathInput.CharLimit = 50

	countInput := textinput.New()
	countInput.Placeholder = "5"
	countInput.CharLimit = 3
	countInput.SetValue("5")

	return &KeyOperationsScreen{
		id:            "key_operations",
		title:         "Key Operations",
		operationList: operationList,
		pathInput:     pathInput,
		countInput:    countInput,
		showForm:      false,
	}
}

// ID returns the screen identifier
func (s *KeyOperationsScreen) ID() string {
	return s.id
}

// Title returns the screen title
func (s *KeyOperationsScreen) Title() string {
	return s.title
}

// CanGoBack determines if we can go back from this screen
func (s *KeyOperationsScreen) CanGoBack() bool {
	return true
}

// Init initializes the screen
func (s *KeyOperationsScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (s *KeyOperationsScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.operationList.SetSize(msg.Width-4, msg.Height-10)

	case tea.KeyMsg:
		if s.showForm {
			switch msg.String() {
			case "esc":
				s.showForm = false
				s.resultText = ""
				s.err = nil
				return s, nil
			case "enter":
				return s.executeOperation()
			case "tab":
				if s.pathInput.Focused() {
					s.pathInput.Blur()
					s.countInput.Focus()
				} else {
					s.countInput.Blur()
					s.pathInput.Focus()
				}
			}

			// Update form inputs
			if s.pathInput.Focused() {
				s.pathInput, cmd = s.pathInput.Update(msg)
			} else {
				s.countInput, cmd = s.countInput.Update(msg)
			}
		} else {
			switch msg.String() {
			case "enter":
				return s.handleSelection()
			}

			// Update operation list
			s.operationList, cmd = s.operationList.Update(msg)
		}
	}

	return s, cmd
}

// View renders the screen
func (s *KeyOperationsScreen) View() string {
	stateManager := utils.GetStateManager()
	theme := stateManager.GetTheme()

	var content strings.Builder

	// Title
	content.WriteString(theme.Title.Render("Key Operations"))
	content.WriteString("\n\n")

	// Current vault info
	vaultName, _ := stateManager.GetCurrentVault()
	if vaultName != "" {
		content.WriteString(theme.InfoStyle.Render("Current vault: " + vaultName))
	} else {
		content.WriteString(theme.WarningStyle.Render("No vault selected - some operations may be unavailable"))
	}
	content.WriteString("\n\n")

	if s.showForm {
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

// renderOperationForm renders the operation form
func (s *KeyOperationsScreen) renderOperationForm(theme *utils.Theme) string {
	var content strings.Builder

	content.WriteString(theme.Subtitle.Render(s.getOperationTitle(s.currentOperation)))
	content.WriteString("\n\n")

	switch s.currentOperation {
	case "derive_addresses":
		content.WriteString("Derivation Path:")
		content.WriteString("\n")
		content.WriteString(s.pathInput.View())
		content.WriteString("\n\n")

		content.WriteString("Number of addresses:")
		content.WriteString("\n")
		content.WriteString(s.countInput.View())
		content.WriteString("\n\n")

	case "generate_mnemonic":
		content.WriteString("Word count:")
		content.WriteString("\n")
		content.WriteString(s.countInput.View())
		content.WriteString("\n")
		content.WriteString(theme.Status.Render("Standard options: 12, 15, 18, 21, 24 words"))
		content.WriteString("\n\n")
	}

	// Result display
	if s.resultText != "" {
		content.WriteString(theme.Subtitle.Render("Result:"))
		content.WriteString("\n\n")
		content.WriteString(theme.InfoStyle.Render(s.resultText))
		content.WriteString("\n\n")
	}

	content.WriteString(theme.Status.Render("Enter: Execute | Tab: Next Field | ESC: Back"))

	return content.String()
}

// getOperationTitle returns the title for an operation
func (s *KeyOperationsScreen) getOperationTitle(operation string) string {
	switch operation {
	case "generate_mnemonic":
		return "Generate Mnemonic Phrase"
	case "derive_addresses":
		return "Derive Addresses"
	case "generate_private_key":
		return "Generate Private Key"
	case "validate_mnemonic":
		return "Validate Mnemonic"
	case "address_from_pubkey":
		return "Address from Public Key"
	default:
		return "Key Operation"
	}
}

// handleSelection handles operation selection
func (s *KeyOperationsScreen) handleSelection() (tea.Model, tea.Cmd) {
	selected := s.operationList.SelectedItem()
	if operation, ok := selected.(KeyOperation); ok {
		s.currentOperation = operation.action
		s.showForm = true
		s.resultText = ""
		s.err = nil

		// Set up form for specific operations
		switch operation.action {
		case "derive_addresses":
			s.pathInput.Focus()
		case "generate_mnemonic":
			s.countInput.SetValue("12")
			s.countInput.Focus()
		}
	}
	return s, nil
}

// executeOperation executes the selected operation
func (s *KeyOperationsScreen) executeOperation() (tea.Model, tea.Cmd) {
	switch s.currentOperation {
	case "generate_mnemonic":
		return s.generateMnemonic()
	case "derive_addresses":
		return s.deriveAddresses()
	case "generate_private_key":
		return s.generatePrivateKey()
	case "validate_mnemonic":
		return s.validateMnemonic()
	case "address_from_pubkey":
		return s.addressFromPubkey()
	}
	return s, nil
}

// generateMnemonic generates a new mnemonic phrase
func (s *KeyOperationsScreen) generateMnemonic() (tea.Model, tea.Cmd) {
	// TODO: Implement actual mnemonic generation
	s.resultText = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	return s, nil
}

// deriveAddresses derives addresses from current vault
func (s *KeyOperationsScreen) deriveAddresses() (tea.Model, tea.Cmd) {
	stateManager := utils.GetStateManager()
	vaultName, _ := stateManager.GetCurrentVault()

	if vaultName == "" {
		s.err = fmt.Errorf("no vault selected")
		return s, nil
	}

	// TODO: Implement actual address derivation
	path := s.pathInput.Value()
	count := s.countInput.Value()

	s.resultText = fmt.Sprintf("Derived %s addresses using path %s:\n", count, path)
	s.resultText += "0x1234567890123456789012345678901234567890\n"
	s.resultText += "0x2345678901234567890123456789012345678901\n"
	s.resultText += "..."

	return s, nil
}

// generatePrivateKey generates a standalone private key
func (s *KeyOperationsScreen) generatePrivateKey() (tea.Model, tea.Cmd) {
	// TODO: Implement actual private key generation
	s.resultText = "Private Key: 0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef12"
	return s, nil
}

// validateMnemonic validates a mnemonic phrase
func (s *KeyOperationsScreen) validateMnemonic() (tea.Model, tea.Cmd) {
	// TODO: Implement actual mnemonic validation
	s.resultText = "Mnemonic validation functionality not yet implemented"
	return s, nil
}

// addressFromPubkey generates address from public key
func (s *KeyOperationsScreen) addressFromPubkey() (tea.Model, tea.Cmd) {
	// TODO: Implement actual address generation from public key
	s.resultText = "Address from public key functionality not yet implemented"
	return s, nil
}
