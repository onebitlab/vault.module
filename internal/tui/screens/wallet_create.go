// internal/tui/screens/wallet_create.go
package screens

import (
	"fmt"
	"strings"

	"vault.module/internal/tui/utils"
	"vault.module/internal/vault"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// WalletCreateScreen represents the wallet creation screen
type WalletCreateScreen struct {
	id        string
	title     string
	vaultName string
	vault     vault.Vault
	step      int
	maxSteps  int

	// Form fields
	prefixInput    textinput.Model
	blockchainType int
	blockchainOpts []string
	derivationPath textinput.Model
	addressCount   textinput.Model

	// State
	width  int
	height int
	err    error
}

// NewWalletCreateScreen creates a new wallet creation screen
func NewWalletCreateScreen(vaultName string, v vault.Vault) *WalletCreateScreen {
	prefixInput := textinput.New()
	prefixInput.Placeholder = "Enter wallet prefix (e.g., eth-main)"
	prefixInput.Focus()
	prefixInput.CharLimit = 30

	derivationPath := textinput.New()
	derivationPath.Placeholder = "m/44'/60'/0'/0"
	derivationPath.CharLimit = 50

	addressCount := textinput.New()
	addressCount.Placeholder = "5"
	addressCount.CharLimit = 3
	addressCount.SetValue("5")

	return &WalletCreateScreen{
		id:             "wallet_create",
		title:          "Create New Wallet",
		vaultName:      vaultName,
		vault:          v,
		step:           0,
		maxSteps:       4,
		prefixInput:    prefixInput,
		blockchainType: 0,
		blockchainOpts: []string{"Ethereum", "Bitcoin", "Cosmos"},
		derivationPath: derivationPath,
		addressCount:   addressCount,
	}
}

// ID returns the screen identifier
func (s *WalletCreateScreen) ID() string {
	return s.id
}

// Title returns the screen title
func (s *WalletCreateScreen) Title() string {
	return s.title
}

// CanGoBack determines if we can go back from this screen
func (s *WalletCreateScreen) CanGoBack() bool {
	return true
}

// Init initializes the screen
func (s *WalletCreateScreen) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages
func (s *WalletCreateScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if s.step == 1 { // Blockchain selection
				s.blockchainType = (s.blockchainType - 1 + len(s.blockchainOpts)) % len(s.blockchainOpts)
				s.updateDerivationPath()
			}
		case "down":
			if s.step == 1 { // Blockchain selection
				s.blockchainType = (s.blockchainType + 1) % len(s.blockchainOpts)
				s.updateDerivationPath()
			}
		}
	}

	// Update current step input
	switch s.step {
	case 0: // Prefix input
		s.prefixInput, cmd = s.prefixInput.Update(msg)
	case 2: // Derivation path
		s.derivationPath, cmd = s.derivationPath.Update(msg)
	case 3: // Address count
		s.addressCount, cmd = s.addressCount.Update(msg)
	}

	return s, cmd
}

// View renders the screen
func (s *WalletCreateScreen) View() string {
	stateManager := utils.GetStateManager()
	theme := stateManager.GetTheme()

	var content strings.Builder

	// Title
	content.WriteString(theme.Title.Render("Create New Wallet"))
	content.WriteString("\n\n")

	// Vault info
	content.WriteString(theme.InfoStyle.Render("Vault: " + s.vaultName))
	content.WriteString("\n\n")

	// Progress indicator
	progress := fmt.Sprintf("Step %d of %d", s.step+1, s.maxSteps)
	content.WriteString(theme.InfoStyle.Render(progress))
	content.WriteString("\n\n")

	// Step content
	switch s.step {
	case 0:
		content.WriteString(s.renderPrefixStep(theme))
	case 1:
		content.WriteString(s.renderBlockchainStep(theme))
	case 2:
		content.WriteString(s.renderDerivationStep(theme))
	case 3:
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

// renderPrefixStep renders the prefix input step
func (s *WalletCreateScreen) renderPrefixStep(theme *utils.Theme) string {
	var content strings.Builder

	content.WriteString(theme.Subtitle.Render("Wallet Prefix"))
	content.WriteString("\n\n")
	content.WriteString("Enter a unique prefix for this wallet:")
	content.WriteString("\n")
	content.WriteString(theme.Status.Render("This will be used to identify the wallet in the vault"))
	content.WriteString("\n\n")
	content.WriteString(s.prefixInput.View())

	return content.String()
}

// renderBlockchainStep renders the blockchain selection step
func (s *WalletCreateScreen) renderBlockchainStep(theme *utils.Theme) string {
	var content strings.Builder

	content.WriteString(theme.Subtitle.Render("Blockchain Type"))
	content.WriteString("\n\n")
	content.WriteString("Select the blockchain for this wallet:")
	content.WriteString("\n\n")

	for i, option := range s.blockchainOpts {
		prefix := "  "
		style := theme.ListItem

		if i == s.blockchainType {
			prefix = "> "
			style = theme.ListFocus
		}

		description := s.getBlockchainDescription(option)
		line := fmt.Sprintf("%s%s - %s", prefix, option, description)
		content.WriteString(style.Render(line))
		content.WriteString("\n")
	}

	return content.String()
}

// renderDerivationStep renders the derivation path step
func (s *WalletCreateScreen) renderDerivationStep(theme *utils.Theme) string {
	var content strings.Builder

	content.WriteString(theme.Subtitle.Render("Derivation Settings"))
	content.WriteString("\n\n")

	// Derivation path
	content.WriteString("Derivation Path:")
	content.WriteString("\n")
	content.WriteString(s.derivationPath.View())
	content.WriteString("\n\n")

	// Address count
	content.WriteString("Number of addresses to generate:")
	content.WriteString("\n")
	content.WriteString(s.addressCount.View())
	content.WriteString("\n\n")

	content.WriteString(theme.Status.Render("Standard derivation path is recommended for compatibility"))

	return content.String()
}

// renderConfirmStep renders the confirmation step
func (s *WalletCreateScreen) renderConfirmStep(theme *utils.Theme) string {
	var content strings.Builder

	content.WriteString(theme.Subtitle.Render("Confirm Wallet Creation"))
	content.WriteString("\n\n")
	content.WriteString("Please confirm the wallet details:")
	content.WriteString("\n\n")

	content.WriteString(fmt.Sprintf("Prefix: %s\n", theme.InfoStyle.Render(s.prefixInput.Value())))
	content.WriteString(fmt.Sprintf("Blockchain: %s\n", theme.InfoStyle.Render(s.blockchainOpts[s.blockchainType])))
	content.WriteString(fmt.Sprintf("Derivation Path: %s\n", theme.InfoStyle.Render(s.derivationPath.Value())))
	content.WriteString(fmt.Sprintf("Address Count: %s\n", theme.InfoStyle.Render(s.addressCount.Value())))

	content.WriteString("\n")
	content.WriteString(theme.WarningStyle.Render("Press Enter to create the wallet"))

	return content.String()
}

// getBlockchainDescription returns description for blockchain type
func (s *WalletCreateScreen) getBlockchainDescription(blockchain string) string {
	switch blockchain {
	case "Ethereum":
		return "EVM compatible addresses (0x...)"
	case "Bitcoin":
		return "Bitcoin addresses (bc1... or 1...)"
	case "Cosmos":
		return "Cosmos ecosystem addresses (cosmos...)"
	default:
		return "Unknown blockchain type"
	}
}

// updateDerivationPath updates derivation path based on blockchain
func (s *WalletCreateScreen) updateDerivationPath() {
	switch s.blockchainOpts[s.blockchainType] {
	case "Ethereum":
		s.derivationPath.SetValue("m/44'/60'/0'/0")
	case "Bitcoin":
		s.derivationPath.SetValue("m/44'/0'/0'/0")
	case "Cosmos":
		s.derivationPath.SetValue("m/44'/118'/0'/0")
	}
}

// handleEnter handles enter key press
func (s *WalletCreateScreen) handleEnter() (tea.Model, tea.Cmd) {
	switch s.step {
	case 0: // Prefix step
		if strings.TrimSpace(s.prefixInput.Value()) == "" {
			s.err = fmt.Errorf("wallet prefix cannot be empty")
			return s, nil
		}
		// Check if prefix already exists
		if _, exists := s.vault[s.prefixInput.Value()]; exists {
			s.err = fmt.Errorf("wallet with prefix '%s' already exists", s.prefixInput.Value())
			return s, nil
		}
		return s.nextStep()
	case 1: // Blockchain step
		return s.nextStep()
	case 2: // Derivation step
		if strings.TrimSpace(s.derivationPath.Value()) == "" {
			s.err = fmt.Errorf("derivation path cannot be empty")
			return s, nil
		}
		return s.nextStep()
	case 3: // Confirm step
		return s.createWallet()
	}
	return s, nil
}

// nextStep moves to the next step
func (s *WalletCreateScreen) nextStep() (tea.Model, tea.Cmd) {
	s.err = nil
	if s.step < s.maxSteps-1 {
		s.step++
		s.updateFocus()
	}
	return s, nil
}

// prevStep moves to the previous step
func (s *WalletCreateScreen) prevStep() (tea.Model, tea.Cmd) {
	s.err = nil
	if s.step > 0 {
		s.step--
		s.updateFocus()
	}
	return s, nil
}

// updateFocus updates input focus based on current step
func (s *WalletCreateScreen) updateFocus() {
	s.prefixInput.Blur()
	s.derivationPath.Blur()
	s.addressCount.Blur()

	switch s.step {
	case 0:
		s.prefixInput.Focus()
	case 2:
		s.derivationPath.Focus()
	case 3:
		s.addressCount.Focus()
	}
}

// createWallet creates the wallet
func (s *WalletCreateScreen) createWallet() (tea.Model, tea.Cmd) {
	// TODO: Implement actual wallet creation
	// For now, return to wallet manager
	return NewWalletManagerScreen(s.vaultName, s.vault), nil
}
