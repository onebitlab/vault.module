// internal/tui/screens/wallet_manager.go
package screens

import (
	"fmt"

	"vault.module/internal/tui/utils"
	"vault.module/internal/vault"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// WalletManagerItem represents a wallet in the manager
type WalletManagerItem struct {
	prefix      string
	addresses   int
	blockchain  string
	description string
}

func (i WalletManagerItem) FilterValue() string { return i.prefix }
func (i WalletManagerItem) Title() string       { return i.prefix }
func (i WalletManagerItem) Description() string {
	return fmt.Sprintf("%s | %d addresses | %s", i.blockchain, i.addresses, i.description)
}

// WalletManagerScreen represents the wallet manager screen
type WalletManagerScreen struct {
	id         string
	title      string
	vaultName  string
	vault      vault.Vault
	walletList list.Model
	width      int
	height     int
}

// NewWalletManagerScreen creates a new wallet manager screen
func NewWalletManagerScreen(vaultName string, v vault.Vault) *WalletManagerScreen {
	screen := &WalletManagerScreen{
		id:        "wallet_manager",
		title:     "Wallet Manager",
		vaultName: vaultName,
		vault:     v,
	}

	screen.refreshWalletList()
	return screen
}

// refreshWalletList refreshes the wallet list from vault
func (s *WalletManagerScreen) refreshWalletList() {
	items := make([]list.Item, 0)

	// Add "Create New Wallet" option
	items = append(items, WalletManagerItem{
		prefix:      "[Create New Wallet]",
		blockchain:  "action",
		description: "Create a new wallet",
	})

	// Add existing wallets
	for prefix, wallet := range s.vault {
		// Determine blockchain type from addresses
		blockchain := "Unknown"
		if len(wallet.Addresses) > 0 {
			// Simple heuristic based on address format
			firstAddr := ""
			for _, addr := range wallet.Addresses {
				firstAddr = addr
				break
			}
			if len(firstAddr) > 0 {
				switch {
				case firstAddr[:2] == "0x":
					blockchain = "Ethereum"
				case firstAddr[:4] == "cosmos":
					blockchain = "Cosmos"
				case firstAddr[:2] == "bc" || firstAddr[:2] == "tb":
					blockchain = "Bitcoin"
				}
			}
		}

		items = append(items, WalletManagerItem{
			prefix:      prefix,
			addresses:   len(wallet.Addresses),
			blockchain:  blockchain,
			description: fmt.Sprintf("Created: %s", wallet.CreatedAt),
		})
	}

	s.walletList = list.New(items, list.NewDefaultDelegate(), 0, 0)
	s.walletList.Title = fmt.Sprintf("Wallets in %s", s.vaultName)
}

// ID returns the screen identifier
func (s *WalletManagerScreen) ID() string {
	return s.id
}

// Title returns the screen title
func (s *WalletManagerScreen) Title() string {
	return fmt.Sprintf("Wallets - %s", s.vaultName)
}

// CanGoBack determines if we can go back from this screen
func (s *WalletManagerScreen) CanGoBack() bool {
	return true
}

// Init initializes the screen
func (s *WalletManagerScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (s *WalletManagerScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.walletList.SetSize(msg.Width-2, msg.Height-8)

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return s.handleSelection()
		case "n":
			// Quick shortcut to create new wallet
			return NewWalletCreateScreen(s.vaultName, s.vault), nil
		case "d":
			// Delete selected wallet
			return s.handleDelete()
		case "r":
			// Refresh wallet list
			s.refreshWalletList()
			return s, nil
		}
	}

	// Update the list
	s.walletList, cmd = s.walletList.Update(msg)
	return s, cmd
}

// View renders the screen
func (s *WalletManagerScreen) View() string {
	stateManager := utils.GetStateManager()
	theme := stateManager.GetTheme()

	vaultInfo := theme.Info.Render("Vault: " + s.vaultName)
	walletCount := theme.Status.Render(fmt.Sprintf("Total wallets: %d", len(s.vault)))

	helpText := theme.Status.Render(
		"Enter: Select/View | N: New Wallet | D: Delete | R: Refresh | ESC: Back",
	)

	return theme.Title.Render("Wallet Manager") + "\n\n" +
		vaultInfo + " | " + walletCount + "\n\n" +
		s.walletList.View() + "\n\n" +
		helpText
}

// handleSelection handles wallet selection
func (s *WalletManagerScreen) handleSelection() (tea.Model, tea.Cmd) {
	selected := s.walletList.SelectedItem()
	if item, ok := selected.(WalletManagerItem); ok {
		if item.prefix == "[Create New Wallet]" {
			return NewWalletCreateScreen(s.vaultName, s.vault), nil
		}

		// View wallet details
		return NewWalletDetailScreen(s.vaultName, s.vault, item.prefix), nil
	}
	return s, nil
}

// handleDelete handles wallet deletion
func (s *WalletManagerScreen) handleDelete() (tea.Model, tea.Cmd) {
	selected := s.walletList.SelectedItem()
	if item, ok := selected.(WalletManagerItem); ok {
		if item.prefix != "[Create New Wallet]" {
			// TODO: Show confirmation dialog
			// For now, just return to avoid accidental deletion
			return s, nil
		}
	}
	return s, nil
}
