// internal/tui/screens/wallet_detail.go
package screens

import (
	"fmt"
	"strings"

	"vault.module/internal/tui/utils"
	"vault.module/internal/vault"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// WalletDetailScreen represents the wallet detail screen
type WalletDetailScreen struct {
	id           string
	title        string
	vaultName    string
	vault        vault.Vault
	walletPrefix string
	wallet       *vault.Wallet

	// UI components
	addressTable table.Model
	actionList   list.Model

	// State
	currentView string // "addresses", "keys", "actions"
	showPrivate bool
	width       int
	height      int
}

// WalletAction represents an action in the wallet detail screen
type WalletAction struct {
	title       string
	description string
	action      string
}

func (i WalletAction) FilterValue() string { return i.title }
func (i WalletAction) Title() string       { return i.title }
func (i WalletAction) Description() string { return i.description }

// NewWalletDetailScreen creates a new wallet detail screen
func NewWalletDetailScreen(vaultName string, v vault.Vault, walletPrefix string) *WalletDetailScreen {
	wallet := v[walletPrefix]

	screen := &WalletDetailScreen{
		id:           "wallet_detail",
		title:        "Wallet Details",
		vaultName:    vaultName,
		vault:        v,
		walletPrefix: walletPrefix,
		wallet:       &wallet,
		currentView:  "addresses",
		showPrivate:  false,
	}

	screen.initializeComponents()
	return screen
}

// initializeComponents initializes UI components
func (s *WalletDetailScreen) initializeComponents() {
	// Initialize address table
	columns := []table.Column{
		{Title: "Index", Width: 8},
		{Title: "Address", Width: 42},
		{Title: "Type", Width: 12},
		{Title: "Balance", Width: 15},
	}

	rows := make([]table.Row, 0)
	index := 0
	for addr, details := range s.wallet.Addresses {
		addressType := "External"
		if details.IsChange {
			addressType = "Change"
		}

		rows = append(rows, table.Row{
			fmt.Sprintf("%d", index),
			addr,
			addressType,
			"0.0", // TODO: Implement balance fetching
		})
		index++
	}

	s.addressTable = table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Initialize action list
	actions := []list.Item{
		WalletAction{
			title:       "Generate New Address",
			description: "Derive a new address for this wallet",
			action:      "generate_address",
		},
		WalletAction{
			title:       "Export Wallet",
			description: "Export wallet data to file",
			action:      "export_wallet",
		},
		WalletAction{
			title:       "Show Private Keys",
			description: "Display private keys (requires confirmation)",
			action:      "show_private",
		},
		WalletAction{
			title:       "Rename Wallet",
			description: "Change wallet prefix name",
			action:      "rename_wallet",
		},
		WalletAction{
			title:       "Delete Wallet",
			description: "Permanently delete this wallet",
			action:      "delete_wallet",
		},
	}

	s.actionList = list.New(actions, list.NewDefaultDelegate(), 0, 0)
	s.actionList.Title = "Wallet Actions"
}

// ID returns the screen identifier
func (s *WalletDetailScreen) ID() string {
	return s.id
}

// Title returns the screen title
func (s *WalletDetailScreen) Title() string {
	return fmt.Sprintf("Wallet: %s", s.walletPrefix)
}

// CanGoBack determines if we can go back from this screen
func (s *WalletDetailScreen) CanGoBack() bool {
	return true
}

// Init initializes the screen
func (s *WalletDetailScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (s *WalletDetailScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height

		// Update table size
		s.addressTable.SetWidth(msg.Width - 4)
		s.addressTable.SetHeight(msg.Height - 12)

		// Update action list size
		s.actionList.SetSize(msg.Width-4, msg.Height-12)

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			return s.switchView()
		case "enter":
			if s.currentView == "actions" {
				return s.handleAction()
			}
		case "p":
			// Toggle private key view
			s.showPrivate = !s.showPrivate
			return s, nil
		case "g":
			// Quick generate address
			return s.generateAddress()
		case "e":
			// Quick export
			return s.exportWallet()
		}
	}

	// Update current view component
	switch s.currentView {
	case "addresses":
		s.addressTable, cmd = s.addressTable.Update(msg)
	case "actions":
		s.actionList, cmd = s.actionList.Update(msg)
	}

	return s, cmd
}

// View renders the screen
func (s *WalletDetailScreen) View() string {
	stateManager := utils.GetStateManager()
	theme := stateManager.GetTheme()

	var content strings.Builder

	// Header
	content.WriteString(theme.Title.Render(fmt.Sprintf("Wallet Details: %s", s.walletPrefix)))
	content.WriteString("\n\n")

	// Wallet info
	content.WriteString(s.renderWalletInfo(theme))
	content.WriteString("\n\n")

	// View tabs
	content.WriteString(s.renderViewTabs(theme))
	content.WriteString("\n\n")

	// Current view content
	switch s.currentView {
	case "addresses":
		content.WriteString(s.renderAddressView(theme))
	case "keys":
		content.WriteString(s.renderKeysView(theme))
	case "actions":
		content.WriteString(s.renderActionsView(theme))
	}

	// Help text
	content.WriteString("\n\n")
	helpText := "Tab: Switch View | Enter: Select | P: Toggle Private Keys | G: Generate | E: Export | ESC: Back"
	content.WriteString(theme.Status.Render(helpText))

	return content.String()
}

// renderWalletInfo renders wallet information
func (s *WalletDetailScreen) renderWalletInfo(theme *utils.Theme) string {
	var info strings.Builder

	info.WriteString(fmt.Sprintf("Vault: %s | ", theme.Info.Render(s.vaultName)))
	info.WriteString(fmt.Sprintf("Prefix: %s | ", theme.Info.Render(s.walletPrefix)))
	info.WriteString(fmt.Sprintf("Addresses: %s | ", theme.Success.Render(fmt.Sprintf("%d", len(s.wallet.Addresses)))))
	info.WriteString(fmt.Sprintf("Created: %s", theme.Status.Render(s.wallet.CreatedAt)))

	return info.String()
}

// renderViewTabs renders view selection tabs
func (s *WalletDetailScreen) renderViewTabs(theme *utils.Theme) string {
	views := []string{"addresses", "keys", "actions"}
	var tabs []string

	for _, view := range views {
		style := theme.Button
		if view == s.currentView {
			style = theme.ButtonFocus
		}
		tabs = append(tabs, style.Render(strings.Title(view)))
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, tabs...)
}

// renderAddressView renders the address table view
func (s *WalletDetailScreen) renderAddressView(theme *utils.Theme) string {
	return s.addressTable.View()
}

// renderKeysView renders the keys view
func (s *WalletDetailScreen) renderKeysView(theme *utils.Theme) string {
	var content strings.Builder

	if !s.showPrivate {
		content.WriteString(theme.Warning.Render("Private keys are hidden for security"))
		content.WriteString("\n\n")
		content.WriteString("Press 'P' to toggle private key visibility")
		content.WriteString("\n\n")
		content.WriteString(theme.Status.Render("Public keys and addresses are always safe to display"))
	} else {
		content.WriteString(theme.Error.Render("⚠️  PRIVATE KEYS VISIBLE - ENSURE SCREEN PRIVACY  ⚠️"))
		content.WriteString("\n\n")

		// Display keys (simplified for demo)
		for addr, details := range s.wallet.Addresses {
			content.WriteString(theme.Subtitle.Render("Address: " + addr))
			content.WriteString("\n")
			content.WriteString(fmt.Sprintf("Public Key: %s", details.PublicKey))
			content.WriteString("\n")
			if s.showPrivate {
				content.WriteString(theme.Error.Render(fmt.Sprintf("Private Key: %s", details.PrivateKey)))
			}
			content.WriteString("\n\n")
		}
	}

	return content.String()
}

// renderActionsView renders the actions list view
func (s *WalletDetailScreen) renderActionsView(theme *utils.Theme) string {
	return s.actionList.View()
}

// switchView switches between different views
func (s *WalletDetailScreen) switchView() (tea.Model, tea.Cmd) {
	views := []string{"addresses", "keys", "actions"}
	currentIndex := 0

	for i, view := range views {
		if view == s.currentView {
			currentIndex = i
			break
		}
	}

	nextIndex := (currentIndex + 1) % len(views)
	s.currentView = views[nextIndex]

	return s, nil
}

// handleAction handles action selection
func (s *WalletDetailScreen) handleAction() (tea.Model, tea.Cmd) {
	selected := s.actionList.SelectedItem()
	if action, ok := selected.(WalletAction); ok {
		switch action.action {
		case "generate_address":
			return s.generateAddress()
		case "export_wallet":
			return s.exportWallet()
		case "show_private":
			s.showPrivate = !s.showPrivate
			return s, nil
		case "rename_wallet":
			return s.renameWallet()
		case "delete_wallet":
			return s.deleteWallet()
		}
	}
	return s, nil
}

// generateAddress generates a new address
func (s *WalletDetailScreen) generateAddress() (tea.Model, tea.Cmd) {
	// TODO: Implement address generation
	return s, nil
}

// exportWallet exports wallet data
func (s *WalletDetailScreen) exportWallet() (tea.Model, tea.Cmd) {
	// TODO: Implement wallet export
	return s, nil
}

// renameWallet renames the wallet
func (s *WalletDetailScreen) renameWallet() (tea.Model, tea.Cmd) {
	// TODO: Implement wallet renaming
	return s, nil
}

// deleteWallet deletes the wallet
func (s *WalletDetailScreen) deleteWallet() (tea.Model, tea.Cmd) {
	// TODO: Implement wallet deletion with confirmation
	return s, nil
}
