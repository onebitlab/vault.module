// internal/tui/components/status_bar.go
package components

import (
	"fmt"
	"strings" // ❌ Добавлен недостающий импорт

	"vault.module/internal/tui/utils"
)

// StatusBar represents the status bar
type StatusBar struct {
	theme      *utils.Theme
	width      int
	leftText   string
	rightText  string
	centerText string
}

// NewStatusBar creates a new status bar
func NewStatusBar(theme *utils.Theme) *StatusBar {
	return &StatusBar{
		theme: theme,
		width: 80,
	}
}

// SetWidth sets the width of the status bar
func (sb *StatusBar) SetWidth(width int) {
	sb.width = width
}

// SetLeftText sets the left text
func (sb *StatusBar) SetLeftText(text string) {
	sb.leftText = text
}

// SetRightText sets the right text
func (sb *StatusBar) SetRightText(text string) {
	sb.rightText = text
}

// SetCenterText sets the center text
func (sb *StatusBar) SetCenterText(text string) {
	sb.centerText = text
}

// Render renders the status bar
func (sb *StatusBar) Render() string {
	leftLen := len(sb.leftText)
	rightLen := len(sb.rightText)
	centerLen := len(sb.centerText)

	// Calculate available space
	availableWidth := sb.width - leftLen - rightLen

	var result string

	if centerLen > 0 && availableWidth >= centerLen+2 {
		// There's room for center text
		leftPadding := (availableWidth - centerLen) / 2
		rightPadding := availableWidth - centerLen - leftPadding

		result = sb.leftText +
			strings.Repeat(" ", leftPadding) +
			sb.centerText +
			strings.Repeat(" ", rightPadding) +
			sb.rightText
	} else {
		// Only left and right text
		padding := availableWidth
		if padding < 0 {
			padding = 0
		}

		result = sb.leftText +
			strings.Repeat(" ", padding) +
			sb.rightText
	}

	return sb.theme.Status.Render(result)
}

// SetVaultInfo sets information about the current vault
func (sb *StatusBar) SetVaultInfo(vaultName string, isAuthenticated bool) {
	if vaultName != "" {
		authStatus := "🔒"
		if isAuthenticated {
			authStatus = "🔓"
		}
		sb.SetLeftText(fmt.Sprintf("Vault: %s %s", vaultName, authStatus))
	} else {
		sb.SetLeftText("No vault selected")
	}
}

// SetHelpText sets the help text
func (sb *StatusBar) SetHelpText(text string) {
	sb.SetRightText(text)
}
