// internal/tui/components/status_bar.go
package components

import (
	"fmt"
	"strings"

	"vault.module/internal/tui/utils"
)

// StatusBar представляет статусную строку
type StatusBar struct {
	theme      *utils.Theme
	width      int
	leftText   string
	rightText  string
	centerText string
}

// NewStatusBar создает новую статусную строку
func NewStatusBar(theme *utils.Theme) *StatusBar {
	return &StatusBar{
		theme: theme,
		width: 80,
	}
}

// SetWidth устанавливает ширину статусной строки
func (sb *StatusBar) SetWidth(width int) {
	sb.width = width
}

// SetLeftText устанавливает текст слева
func (sb *StatusBar) SetLeftText(text string) {
	sb.leftText = text
}

// SetRightText устанавливает текст справа
func (sb *StatusBar) SetRightText(text string) {
	sb.rightText = text
}

// SetCenterText устанавливает текст по центру
func (sb *StatusBar) SetCenterText(text string) {
	sb.centerText = text
}

// Render отрисовывает статусную строку
func (sb *StatusBar) Render() string {
	leftLen := len(sb.leftText)
	rightLen := len(sb.rightText)
	centerLen := len(sb.centerText)

	// Вычисляем доступное пространство
	availableWidth := sb.width - leftLen - rightLen

	var result string

	if centerLen > 0 && availableWidth >= centerLen+2 {
		// Есть место для центрального текста
		leftPadding := (availableWidth - centerLen) / 2
		rightPadding := availableWidth - centerLen - leftPadding

		result = sb.leftText +
			strings.Repeat(" ", leftPadding) +
			sb.centerText +
			strings.Repeat(" ", rightPadding) +
			sb.rightText
	} else {
		// Только левый и правый текст
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

// SetVaultInfo устанавливает информацию о текущем vault'е
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

// SetHelpText устанавливает текст справки
func (sb *StatusBar) SetHelpText(text string) {
	sb.SetRightText(text)
}
