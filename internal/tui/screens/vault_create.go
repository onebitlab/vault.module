// internal/tui/screens/vault_create.go
package screens

import (
	"fmt"
	"strings"

	"vault.module/internal/tui/utils"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// VaultCreateScreen represents the vault creation screen
type VaultCreateScreen struct {
	id       string
	title    string
	step     int
	maxSteps int

	// Form fields
	nameInput    textinput.Model
	typeSelected int
	typeOptions  []string

	// State
	width  int
	height int
	err    error
}

// NewVaultCreateScreen creates a new vault creation screen
func NewVaultCreateScreen() *VaultCreateScreen {
	nameInput := textinput.New()
	nameInput.Placeholder = "Enter vault name"
	nameInput.Focus()
	nameInput.CharLimit = 50

	return &VaultCreateScreen{
		id:           "vault_create",
		title:        "Create New Vault",
		step:         0,
		maxSteps:     3,
		nameInput:    nameInput,
		typeSelected: 0,
		typeOptions:  []string{"mnemonic", "hardware", "file"},
	}
}

// ID returns the screen identifier
func (s *VaultCreateScreen) ID() string {
	return s.id
}

// Title returns the screen title
func (s *VaultCreateScreen) Title() string {
	return s.title
}

// CanGoBack determines if we can go back from this screen
func (s *VaultCreateScreen) CanGoBack() bool {
	return true
}

// Init initializes the screen
func (s *VaultCreateScreen) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages
func (s *VaultCreateScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return s.handleEnter()
		case "tab":
			return s.nextStep()
		case "shift+tab":
			return s.prevStep()
		case "up":
			if s.step == 1 { // Type selection step
				s.typeSelected = (s.typeSelected - 1 + len(s.typeOptions)) % len(s.typeOptions)
			}
		case "down":
			if s.step == 1 { // Type selection step
				s.typeSelected = (s.typeSelected + 1) % len(s.typeOptions)
			}
		}
	}

	// Update current step input
	switch s.step {
	case 0: // Name input
		s.nameInput, cmd = s.nameInput.Update(msg)
	}

	return s, cmd
}

// View renders the screen
func (s *VaultCreateScreen) View() string {
	stateManager := utils.GetStateManager()
	theme := stateManager.GetTheme()

	var content strings.Builder

	// Title
	content.WriteString(theme.Title.Render("Create New Vault"))
	content.WriteString("\n\n")

	// Progress indicator
	progress := fmt.Sprintf("Step %d of %d", s.step+1, s.maxSteps)
	content.WriteString(theme.InfoStyle.Render(progress))
	content.WriteString("\n\n")

	// Step content
	switch s.step {
	case 0:
		content.WriteString(s.renderNameStep(theme))
	case 1:
		content.WriteString(s.renderTypeStep(theme))
	case 2:
		content.WriteString(s.renderConfirmStep(theme))
	}

	// Error display
	if s.err != nil {
		content.WriteString("\n\n")
		content.WriteString(theme.ErrorStyle.Render("Error: " + s.err.Error()))
	}

	// Help text
	content.WriteString("\n\n")
	helpText := "Enter: Next/Confirm | Tab: Next Step | Shift+Tab: Previous Step | ESC: Back"
	content.WriteString(theme.Status.Render(helpText))

	return content.String()
}

// renderNameStep renders the name input step
func (s *VaultCreateScreen) renderNameStep(theme *utils.Theme) string {
	var content strings.Builder

	content.WriteString(theme.Subtitle.Render("Vault Name"))
	content.WriteString("\n\n")
	content.WriteString("Enter a unique name for your vault:")
	content.WriteString("\n\n")
	content.WriteString(s.nameInput.View())

	return content.String()
}

// renderTypeStep renders the type selection step
func (s *VaultCreateScreen) renderTypeStep(theme *utils.Theme) string {
	var content strings.Builder

	content.WriteString(theme.Subtitle.Render("Vault Type"))
	content.WriteString("\n\n")
	content.WriteString("Select the type of vault to create:")
	content.WriteString("\n\n")

	for i, option := range s.typeOptions {
		prefix := "  "
		style := theme.ListItem

		if i == s.typeSelected {
			prefix = "> "
			style = theme.ListFocus
		}

		description := s.getTypeDescription(option)
		line := fmt.Sprintf("%s%s - %s", prefix, option, description)
		content.WriteString(style.Render(line))
		content.WriteString("\n")
	}

	return content.String()
}

// renderConfirmStep renders the confirmation step
func (s *VaultCreateScreen) renderConfirmStep(theme *utils.Theme) string {
	var content strings.Builder

	content.WriteString(theme.Subtitle.Render("Confirm Creation"))
	content.WriteString("\n\n")
	content.WriteString("Please confirm the vault details:")
	content.WriteString("\n\n")

	content.WriteString(fmt.Sprintf("Name: %s\n", theme.InfoStyle.Render(s.nameInput.Value())))
	content.WriteString(fmt.Sprintf("Type: %s\n", theme.InfoStyle.Render(s.typeOptions[s.typeSelected])))
	content.WriteString(fmt.Sprintf("Description: %s\n", theme.Status.Render(s.getTypeDescription(s.typeOptions[s.typeSelected]))))

	content.WriteString("\n")
	content.WriteString(theme.WarningStyle.Render("Press Enter to create the vault"))

	return content.String()
}

// getTypeDescription returns description for vault type
func (s *VaultCreateScreen) getTypeDescription(vaultType string) string {
	switch vaultType {
	case "mnemonic":
		return "BIP39 mnemonic phrase based vault"
	case "hardware":
		return "Hardware device (YubiKey) based vault"
	case "file":
		return "File-based encrypted vault"
	default:
		return "Unknown vault type"
	}
}

// handleEnter handles enter key press
func (s *VaultCreateScreen) handleEnter() (tea.Model, tea.Cmd) {
	switch s.step {
	case 0: // Name step
		if strings.TrimSpace(s.nameInput.Value()) == "" {
			s.err = fmt.Errorf("vault name cannot be empty")
			return s, nil
		}
		return s.nextStep()
	case 1: // Type step
		return s.nextStep()
	case 2: // Confirm step
		return s.createVault()
	}
	return s, nil
}

// nextStep moves to the next step
func (s *VaultCreateScreen) nextStep() (tea.Model, tea.Cmd) {
	s.err = nil
	if s.step < s.maxSteps-1 {
		s.step++
		if s.step == 0 {
			s.nameInput.Focus()
		} else {
			s.nameInput.Blur()
		}
	}
	return s, nil
}

// prevStep moves to the previous step
func (s *VaultCreateScreen) prevStep() (tea.Model, tea.Cmd) {
	s.err = nil
	if s.step > 0 {
		s.step--
		if s.step == 0 {
			s.nameInput.Focus()
		} else {
			s.nameInput.Blur()
		}
	}
	return s, nil
}

// createVault creates the vault
func (s *VaultCreateScreen) createVault() (tea.Model, tea.Cmd) {
	// TODO: Implement actual vault creation
	// For now, just return to vault manager
	return NewVaultManagerScreen(), nil
}
