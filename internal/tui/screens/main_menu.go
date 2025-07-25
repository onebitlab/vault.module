// internal/tui/screens/main_menu.go
package screens

import (
	"vault.module/internal/tui/utils"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// MenuItem represents a menu item
type MenuItem struct {
	title       string
	description string
	action      string
}

func (i MenuItem) FilterValue() string { return i.title }
func (i MenuItem) Title() string       { return i.title }
func (i MenuItem) Description() string { return i.description }

// MainMenuScreen represents the main menu screen
type MainMenuScreen struct {
	id       string
	title    string
	menuList list.Model
	width    int
	height   int
}

// NewMainMenuScreen creates a new main menu screen
func NewMainMenuScreen() *MainMenuScreen {
	// Create menu items
	items := []list.Item{
		MenuItem{
			title:       "Vault Manager",
			description: "Create, select, and manage vaults",
			action:      "vault_manager",
		},
		MenuItem{
			title:       "Wallet Manager",
			description: "Manage wallets in current vault",
			action:      "wallet_manager",
		},
		MenuItem{
			title:       "Key Operations",
			description: "Generate keys and derive addresses",
			action:      "key_operations",
		},
		MenuItem{
			title:       "Import/Export",
			description: "Import and export vault data",
			action:      "import_export",
		},
		MenuItem{
			title:       "Settings",
			description: "Configure application settings",
			action:      "settings",
		},
		MenuItem{
			title:       "Audit Logs",
			description: "View security and operation logs",
			action:      "audit_logs",
		},
		MenuItem{
			title:       "Help",
			description: "View help and documentation",
			action:      "help",
		},
	}

	menuList := list.New(items, list.NewDefaultDelegate(), 0, 0)
	menuList.Title = "Main Menu"

	return &MainMenuScreen{
		id:       "main_menu",
		title:    "Main Menu",
		menuList: menuList,
	}
}

// ID returns the screen identifier
func (s *MainMenuScreen) ID() string {
	return s.id
}

// Title returns the screen title
func (s *MainMenuScreen) Title() string {
	return s.title
}

// CanGoBack determines if we can go back from this screen
func (s *MainMenuScreen) CanGoBack() bool {
	return false // This is the root screen
}

// Init initializes the screen
func (s *MainMenuScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (s *MainMenuScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.menuList.SetSize(msg.Width-2, msg.Height-6)

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			selected := s.menuList.SelectedItem()
			if menuItem, ok := selected.(MenuItem); ok {
				return s.handleMenuSelection(menuItem.action)
			}
		}
	}

	// Update the list
	s.menuList, cmd = s.menuList.Update(msg)
	return s, cmd
}

// View renders the screen
func (s *MainMenuScreen) View() string {
	stateManager := utils.GetStateManager()
	theme := stateManager.GetTheme()

	// Get current vault info for display
	vaultName, _ := stateManager.GetCurrentVault()
	var vaultInfo string
	if vaultName != "" {
		vaultInfo = theme.Info.Render("Current vault: " + vaultName)
	} else {
		vaultInfo = theme.Warning.Render("No vault selected")
	}

	return theme.Title.Render("Vault Module - Main Menu") + "\n\n" +
		vaultInfo + "\n\n" +
		s.menuList.View() + "\n\n" +
		theme.Status.Render("Press Enter to select, Ctrl+C to quit")
}

// handleMenuSelection handles menu item selection
func (s *MainMenuScreen) handleMenuSelection(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "vault_manager":
		return NewVaultManagerScreen(), nil
	case "wallet_manager":
		// Check if vault is selected
		stateManager := utils.GetStateManager()
		vaultName, vault := stateManager.GetCurrentVault()
		if vaultName == "" {
			// No vault selected, go to vault manager first
			return NewVaultManagerScreen(), nil
		}
		return NewWalletManagerScreen(vaultName, vault), nil
	case "key_operations":
		return NewKeyOperationsScreen(), nil
	case "import_export":
		return NewImportExportScreen(), nil
	case "settings":
		return NewSettingsScreen(), nil
	case "audit_logs":
		return NewAuditViewerScreen(), nil
	case "help":
		return NewHelpScreen(), nil
	default:
		return s, nil
	}
}
