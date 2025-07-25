// internal/tui/screens/help.go
package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"vault.module/internal/tui/utils"
)

type HelpScreen struct {
	id    string
	title string
}

func NewHelpScreen() *HelpScreen {
	return &HelpScreen{
		id:    "help",
		title: "Help",
	}
}

func (s *HelpScreen) ID() string      { return s.id }
func (s *HelpScreen) Title() string   { return s.title }
func (s *HelpScreen) CanGoBack() bool { return true }
func (s *HelpScreen) Init() tea.Cmd   { return nil }

func (s *HelpScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return s, nil
}

func (s *HelpScreen) View() string {
	stateManager := utils.GetStateManager()
	theme := stateManager.GetTheme()

	helpText := `
Vault Module - Crypto Key Manager

NAVIGATION:
- Use arrow keys to navigate lists
- Enter: Select/Confirm
- ESC: Go back
- Ctrl+C: Quit application
- Tab: Navigate between form fields

MAIN FEATURES:
- Vault Manager: Create and manage vaults
- Wallet Manager: Manage wallets within vaults
- Key Operations: Generate keys and derive addresses
- Import/Export: Backup and restore data
- Settings: Configure application preferences

SECURITY:
- All sensitive data is encrypted
- YubiKey support for hardware authentication
- Session timeouts for security
- Audit logging for all operations

For more information, visit: https://github.com/onebitlab/vault.module
`

	return theme.Title.Render("Help & Documentation") + "\n\n" +
		theme.InfoStyle.Render(helpText) + "\n\n" + // ❌ Исправлено: theme.Info -> theme.InfoStyle
		theme.Status.Render("ESC: Back")
}
