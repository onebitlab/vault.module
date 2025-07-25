// internal/tui/screens/audit_viewer.go
package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"vault.module/internal/tui/utils"
)

type AuditViewerScreen struct {
	id    string
	title string
}

func NewAuditViewerScreen() *AuditViewerScreen {
	return &AuditViewerScreen{
		id:    "audit_viewer",
		title: "Audit Logs",
	}
}

func (s *AuditViewerScreen) ID() string      { return s.id }
func (s *AuditViewerScreen) Title() string   { return s.title }
func (s *AuditViewerScreen) CanGoBack() bool { return true }
func (s *AuditViewerScreen) Init() tea.Cmd   { return nil }

func (s *AuditViewerScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return s, nil
}

func (s *AuditViewerScreen) View() string {
	stateManager := utils.GetStateManager()
	theme := stateManager.GetTheme()

	return theme.Title.Render("Audit Logs") + "\n\n" +
		theme.InfoStyle.Render("Audit log viewer functionality will be implemented here") + "\n\n" + // ❌ Исправлено: theme.Info -> theme.InfoStyle
		theme.Status.Render("ESC: Back")
}
