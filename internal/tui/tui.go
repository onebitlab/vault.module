// internal/tui/tui.go
package tui

import (
	"fmt"
	"time"

	"vault.module/internal/tui/components"
	"vault.module/internal/tui/screens"
	"vault.module/internal/tui/utils"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MainModel represents the main TUI model with navigation and security
type MainModel struct {
	navigationStack *utils.NavigationStack
	stateManager    *utils.StateManager
	securityManager *utils.SecurityManager
	errorHandler    *utils.ErrorHandler

	// UI components
	navigationBar *components.NavigationBar
	statusBar     *components.StatusBar
	confirmation  *components.ConfirmationComponent
	yubiKeyAuth   *components.YubiKeyAuthComponent

	// State
	theme        *utils.Theme
	width        int
	height       int
	initialized  bool
	sessionTimer *time.Timer

	// Security state
	requiresAuth bool
	isLocked     bool
}

// NewMainModel creates a new main model with security features
func NewMainModel() *MainModel {
	stateManager := utils.GetStateManager()
	securityManager := utils.GetSecurityManager()
	errorHandler := utils.GetErrorHandler()
	theme := stateManager.GetTheme()

	return &MainModel{
		navigationStack: utils.NewNavigationStack(),
		stateManager:    stateManager,
		securityManager: securityManager,
		errorHandler:    errorHandler,
		navigationBar:   components.NewNavigationBar(theme),
		statusBar:       components.NewStatusBar(theme),
		confirmation:    components.NewConfirmationComponent(theme),
		yubiKeyAuth:     components.NewYubiKeyAuthComponent(theme, true),
		theme:           theme,
		initialized:     false,
		requiresAuth:    false,
		isLocked:        false,
	}
}

// Init initializes the model
func (m *MainModel) Init() tea.Cmd {
	// Check if system is locked
	if m.securityManager.IsLocked() {
		m.isLocked = true
		return m.showLockScreen()
	}

	// Check if session expired
	if m.securityManager.IsSessionExpired() {
		m.requiresAuth = true
		return m.showAuthScreen()
	}

	// Load the main menu as the initial screen
	initialScreen := screens.NewMainMenuScreen()
	m.navigationStack.Push(initialScreen)

	m.initialized = true
	m.startSessionTimer()

	return tea.Batch(
		initialScreen.Init(),
		m.yubiKeyAuth.Init(),
	)
}

// Update handles messages with security checks
func (m *MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Handle system lock state
	if m.isLocked {
		return m.handleLockedState(msg)
	}

	// Handle authentication requirement
	if m.requiresAuth {
		return m.handleAuthState(msg)
	}

	// Update security manager activity
	m.securityManager.UpdateActivity()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.stateManager.SetTerminalSize(msg.Width, msg.Height)
		m.navigationBar.SetWidth(msg.Width)
		m.statusBar.SetWidth(msg.Width)

		// Pass size to current screen
		if current := m.navigationStack.Current(); current != nil {
			current.Update(msg)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+l":
			// Manual lock
			m.lockSystem()
			return m, nil
		case "esc":
			if m.confirmation.IsVisible() {
				m.confirmation.Hide()
				return m, nil
			}

			if m.navigationStack.CanGoBack() {
				if prevScreen := m.navigationStack.Pop(); prevScreen != nil {
					m.updateNavigation()
					return m, nil
				}
			}
			return m, tea.Quit
		}

	case components.ConfirmationResultMsg:
		// Handle confirmation result
		if msg.Confirmed {
			// Execute confirmed action
			return m.executeConfirmedAction()
		}
		return m, nil

	case sessionTimeoutMsg:
		// Session timeout
		m.requiresAuth = true
		m.securityManager.ClearSession()
		return m, m.showAuthScreen()

	case time.Time:
		// Check for session expiration
		if m.securityManager.IsSessionExpired() {
			m.requiresAuth = true
			return m, m.showAuthScreen()
		}

		// Check for system lock
		if m.securityManager.IsLocked() {
			m.isLocked = true
			return m, m.showLockScreen()
		}
	}

	// Update confirmation component
	if m.confirmation.IsVisible() {
		m.confirmation, cmd = m.confirmation.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	// Update current screen
	if current := m.navigationStack.Current(); current != nil {
		updatedScreen, cmd := current.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

		if updatedScreen != current {
			// Screen changed, update stack
			m.navigationStack.Push(updatedScreen.(utils.Screen))
			m.updateNavigation()
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the interface with security overlays
func (m *MainModel) View() string {
	if m.isLocked {
		return m.renderLockScreen()
	}

	if m.requiresAuth {
		return m.renderAuthScreen()
	}

	if !m.initialized {
		return "Initializing secure session..."
	}

	var content string

	// Get current screen content
	if current := m.navigationStack.Current(); current != nil {
		content = current.View()
	} else {
		content = "No screen available"
	}

	// Update navigation and status
	m.updateNavigation()
	m.updateStatus()

	// Assemble final view
	var sections []string

	// Navigation bar
	if nav := m.navigationBar.Render(); nav != "" {
		sections = append(sections, nav)
	}

	// Main content
	sections = append(sections, content)

	// Status bar
	if status := m.statusBar.Render(); status != "" {
		sections = append(sections, status)
	}

	result := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Overlay confirmation dialog if visible
	if m.confirmation.IsVisible() {
		confirmationView := m.confirmation.View()
		// Center the confirmation dialog
		result = m.overlayDialog(result, confirmationView)
	}

	return result
}

// handleLockedState handles the locked state
func (m *MainModel) handleLockedState(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			// Check if lockout period has expired
			if !m.securityManager.IsLocked() {
				m.isLocked = false
				m.requiresAuth = true
				return m, m.showAuthScreen()
			}
		}
	}
	return m, nil
}

// handleAuthState handles the authentication state
func (m *MainModel) handleAuthState(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	m.yubiKeyAuth, cmd = m.yubiKeyAuth.Update(msg)

	if m.yubiKeyAuth.IsAuthenticated() {
		// Authentication successful
		m.requiresAuth = false
		m.securityManager.RecordSuccessfulAuth()
		m.yubiKeyAuth.Reset()

		// Load main menu
		if m.navigationStack.Size() == 0 {
			initialScreen := screens.NewMainMenuScreen()
			m.navigationStack.Push(initialScreen)
			return m, initialScreen.Init()
		}
	}

	return m, cmd
}

// renderLockScreen renders the lock screen
func (m *MainModel) renderLockScreen() string {
	remaining := m.securityManager.GetLockoutTimeRemaining()

	content := m.theme.Error.Render("🔒 SYSTEM LOCKED") + "\n\n"
	content += m.theme.Warning.Render("Too many failed authentication attempts") + "\n\n"

	if remaining > 0 {
		content += m.theme.Info.Render(fmt.Sprintf("Lockout expires in: %v", remaining.Round(time.Second))) + "\n\n"
		content += m.theme.Status.Render("Please wait for lockout period to expire")
	} else {
		content += m.theme.Success.Render("Lockout period expired") + "\n\n"
		content += m.theme.Status.Render("Press Enter to continue to authentication")
	}

	return content
}

// renderAuthScreen renders the authentication screen
func (m *MainModel) renderAuthScreen() string {
	return m.yubiKeyAuth.View()
}

// showLockScreen shows the lock screen
func (m *MainModel) showLockScreen() tea.Cmd {
	return func() tea.Msg {
		return time.Now()
	}
}

// showAuthScreen shows the authentication screen
func (m *MainModel) showAuthScreen() tea.Cmd {
	return m.yubiKeyAuth.Init()
}

// lockSystem locks the system
func (m *MainModel) lockSystem() {
	m.isLocked = true
	m.securityManager.ClearSession()
	m.stateManager.ClearSession()
}

// startSessionTimer starts the session timeout timer
func (m *MainModel) startSessionTimer() {
	if m.sessionTimer != nil {
		m.sessionTimer.Stop()
	}

	m.sessionTimer = time.AfterFunc(30*time.Minute, func() {
		// This will be handled in the Update method
	})
}

// updateNavigation updates the navigation bar
func (m *MainModel) updateNavigation() {
	breadcrumbs := m.navigationStack.GetBreadcrumbs()
	m.navigationBar.SetBreadcrumbs(breadcrumbs)
}

// updateStatus updates the status bar with security info
func (m *MainModel) updateStatus() {
	// Get current vault info
	vaultName, _ := m.stateManager.GetCurrentVault()
	isAuthenticated := m.stateManager.IsAuthenticated()

	m.statusBar.SetVaultInfo(vaultName, isAuthenticated)

	// Set help based on context
	helpText := "ESC: Back | Ctrl+C: Quit | Ctrl+L: Lock"
	if m.navigationStack.CanGoBack() {
		m.statusBar.SetHelpText(helpText)
	} else {
		m.statusBar.SetHelpText("Ctrl+C: Quit | Ctrl+L: Lock")
	}
}

// overlayDialog overlays a dialog on top of content
func (m *MainModel) overlayDialog(content, dialog string) string {
	// Simple overlay implementation
	return content + "\n\n" + dialog
}

// executeConfirmedAction executes the action that was confirmed
func (m *MainModel) executeConfirmedAction() (tea.Model, tea.Cmd) {
	// TODO: Implement confirmed action execution
	return m, nil
}

// Message types
type sessionTimeoutMsg struct{}

// StartTUI starts the enhanced secure interactive interface
func StartTUI() {
	model := NewMainModel()
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
	}
}
