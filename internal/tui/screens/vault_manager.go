// internal/tui/screens/vault_manager.go
package screens

import (
	"fmt"

	"vault.module/internal/config"
	"vault.module/internal/tui/utils"
	"vault.module/internal/vault"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// VaultManagerItem represents a vault in the manager
type VaultManagerItem struct {
	name        string
	vaultType   string
	isActive    bool
	isLoaded    bool
	description string
}

func (i VaultManagerItem) FilterValue() string { return i.name }
func (i VaultManagerItem) Title() string {
	status := ""
	if i.isActive {
		status = " (active)"
	}
	if i.isLoaded {
		status += " ✓"
	}
	return i.name + status
}
func (i VaultManagerItem) Description() string {
	return fmt.Sprintf("Type: %s | %s", i.vaultType, i.description)
}

// VaultManagerScreen represents the vault manager screen
type VaultManagerScreen struct {
	id        string
	title     string
	vaultList list.Model
	width     int
	height    int
}

// NewVaultManagerScreen creates a new vault manager screen
func NewVaultManagerScreen() *VaultManagerScreen {
	screen := &VaultManagerScreen{
		id:    "vault_manager",
		title: "Vault Manager",
	}

	screen.refreshVaultList()
	return screen
}

// refreshVaultList refreshes the vault list from config
func (s *VaultManagerScreen) refreshVaultList() {
	stateManager := utils.GetStateManager()
	currentVaultName, _ := stateManager.GetCurrentVault()

	items := make([]list.Item, 0)

	// Add "Create New Vault" option
	items = append(items, VaultManagerItem{
		name:        "[Create New Vault]",
		vaultType:   "action",
		description: "Create a new vault",
	})

	// Add existing vaults
	for name, details := range config.Cfg.Vaults {
		items = append(items, VaultManagerItem{
			name:        name,
			vaultType:   details.Type,
			isActive:    name == config.Cfg.ActiveVault,
			isLoaded:    name == currentVaultName,
			description: "Vault configuration",
		})
	}

	s.vaultList = list.New(items, list.NewDefaultDelegate(), 0, 0)
	s.vaultList.Title = "Available Vaults"
}

// ID returns the screen identifier
func (s *VaultManagerScreen) ID() string {
	return s.id
}

// Title returns the screen title
func (s *VaultManagerScreen) Title() string {
	return s.title
}

// CanGoBack determines if we can go back from this screen
func (s *VaultManagerScreen) CanGoBack() bool {
	return true
}

// Init initializes the screen
func (s *VaultManagerScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (s *VaultManagerScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.vaultList.SetSize(msg.Width-2, msg.Height-8)

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return s.handleSelection()
		case "n":
			// Quick shortcut to create new vault
			return NewVaultCreateScreen(), nil
		case "d":
			// Delete selected vault
			return s.handleDelete()
		case "r":
			// Refresh vault list
			s.refreshVaultList()
			return s, nil
		}

	case loadVaultMsg:
		// Vault loaded successfully
		stateManager := utils.GetStateManager()
		stateManager.SetCurrentVault(msg.name, msg.vault)
		stateManager.SetAuthenticated(true)
		s.refreshVaultList()
		return s, nil

	case errorMsg:
		// TODO: Show error message
		return s, nil
	}

	// Update the list
	s.vaultList, cmd = s.vaultList.Update(msg)
	return s, cmd
}

// View renders the screen
func (s *VaultManagerScreen) View() string {
	stateManager := utils.GetStateManager()
	theme := stateManager.GetTheme()

	// Get current vault info
	currentVaultName, _ := stateManager.GetCurrentVault()
	var statusInfo string
	if currentVaultName != "" {
		statusInfo = theme.SuccessStyle.Render("Current vault: " + currentVaultName)
	} else {
		statusInfo = theme.WarningStyle.Render("No vault loaded")
	}

	helpText := theme.Status.Render(
		"Enter: Select/Load | N: New Vault | D: Delete | R: Refresh | ESC: Back",
	)

	return theme.Title.Render("Vault Manager") + "\n\n" +
		statusInfo + "\n\n" +
		s.vaultList.View() + "\n\n" +
		helpText
}

// handleSelection handles vault selection
func (s *VaultManagerScreen) handleSelection() (tea.Model, tea.Cmd) {
	selected := s.vaultList.SelectedItem()
	if item, ok := selected.(VaultManagerItem); ok {
		if item.name == "[Create New Vault]" {
			return NewVaultCreateScreen(), nil
		}

		// Load the selected vault
		return s, loadVault(item.name)
	}
	return s, nil
}

// handleDelete handles vault deletion
func (s *VaultManagerScreen) handleDelete() (tea.Model, tea.Cmd) {
	selected := s.vaultList.SelectedItem()
	if item, ok := selected.(VaultManagerItem); ok {
		if item.name != "[Create New Vault]" {
			// TODO: Show confirmation dialog
			// For now, just return to avoid accidental deletion
			return s, nil
		}
	}
	return s, nil
}

// loadVault command for loading a vault
func loadVault(name string) tea.Cmd {
	return func() tea.Msg {
		details, exists := config.Cfg.Vaults[name]
		if !exists {
			return errorMsg{fmt.Errorf("vault '%s' not found", name)}
		}

		v, err := vault.LoadVault(details)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to load vault: %w", err)}
		}

		return loadVaultMsg{vault: v, name: name}
	}
}

// Message types
type loadVaultMsg struct {
	vault vault.Vault
	name  string
}

type errorMsg struct {
	err error
}
