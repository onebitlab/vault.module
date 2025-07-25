// internal/tui/components/confirmation.go
package components

import (
	"strings"

	"vault.module/internal/tui/utils"

	tea "github.com/charmbracelet/bubbletea"
)

// ConfirmationType represents the type of confirmation dialog
type ConfirmationType int

const (
	ConfirmationTypeInfo ConfirmationType = iota
	ConfirmationTypeWarning
	ConfirmationTypeError
	ConfirmationTypeDanger
)

// ConfirmationComponent represents a confirmation dialog
type ConfirmationComponent struct {
	title       string
	message     string
	confirmText string
	cancelText  string
	confirmType ConfirmationType
	theme       *utils.Theme

	// State
	selectedOption int // 0 = cancel, 1 = confirm
	width          int
	height         int
	visible        bool
}

// NewConfirmationComponent creates a new confirmation component
func NewConfirmationComponent(theme *utils.Theme) *ConfirmationComponent {
	return &ConfirmationComponent{
		confirmText:    "Confirm",
		cancelText:     "Cancel",
		confirmType:    ConfirmationTypeInfo,
		theme:          theme,
		selectedOption: 0, // Default to cancel for safety
		visible:        false,
	}
}

// Show displays the confirmation dialog
func (c *ConfirmationComponent) Show(title, message string, confirmType ConfirmationType) {
	c.title = title
	c.message = message
	c.confirmType = confirmType
	c.selectedOption = 0 // Reset to cancel
	c.visible = true
}

// ShowDangerous displays a dangerous action confirmation
func (c *ConfirmationComponent) ShowDangerous(title, message string) {
	c.Show(title, message, ConfirmationTypeDanger)
	c.confirmText = "DELETE"
	c.cancelText = "Cancel"
}

// ShowWarning displays a warning confirmation
func (c *ConfirmationComponent) ShowWarning(title, message string) {
	c.Show(title, message, ConfirmationTypeWarning)
	c.confirmText = "Continue"
	c.cancelText = "Cancel"
}

// Hide hides the confirmation dialog
func (c *ConfirmationComponent) Hide() {
	c.visible = false
	c.selectedOption = 0
}

// Update handles messages
func (c *ConfirmationComponent) Update(msg tea.Msg) (*ConfirmationComponent, tea.Cmd) {
	if !c.visible {
		return c, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.width = msg.Width
		c.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			c.selectedOption = 0 // Cancel
		case "right", "l":
			c.selectedOption = 1 // Confirm
		case "tab":
			c.selectedOption = (c.selectedOption + 1) % 2
		case "enter":
			if c.selectedOption == 1 {
				c.visible = false
				return c, func() tea.Msg { return ConfirmationResultMsg{Confirmed: true} }
			} else {
				c.visible = false
				return c, func() tea.Msg { return ConfirmationResultMsg{Confirmed: false} }
			}
		case "esc":
			c.visible = false
			return c, func() tea.Msg { return ConfirmationResultMsg{Confirmed: false} }
		}
	}

	return c, nil
}

// View renders the confirmation dialog
func (c *ConfirmationComponent) View() string {
	if !c.visible {
		return ""
	}

	var content strings.Builder

	// Calculate dialog dimensions
	dialogWidth := 60
	if c.width > 0 && c.width < 80 {
		dialogWidth = c.width - 10
	}

	// Title style based on confirmation type
	var titleStyle = c.theme.Title
	var messageStyle = c.theme.InfoStyle // ❌ Исправлено: используем стиль вместо цвета

	switch c.confirmType {
	case ConfirmationTypeWarning:
		titleStyle = c.theme.WarningStyle   // ❌ Исправлено
		messageStyle = c.theme.WarningStyle // ❌ Исправлено
	case ConfirmationTypeError:
		titleStyle = c.theme.ErrorStyle   // ❌ Исправлено
		messageStyle = c.theme.ErrorStyle // ❌ Исправлено
	case ConfirmationTypeDanger:
		titleStyle = c.theme.ErrorStyle   // ❌ Исправлено
		messageStyle = c.theme.ErrorStyle // ❌ Исправлено
	}

	// Create dialog box
	content.WriteString(c.renderDialogBorder(dialogWidth))
	content.WriteString("\n")

	// Title
	content.WriteString(c.centerText(titleStyle.Render(c.title), dialogWidth))
	content.WriteString("\n")
	content.WriteString(c.renderDialogSeparator(dialogWidth))
	content.WriteString("\n")

	// Message
	messageLines := c.wrapText(c.message, dialogWidth-4)
	for _, line := range messageLines {
		content.WriteString(c.centerText(messageStyle.Render(line), dialogWidth))
		content.WriteString("\n")
	}

	content.WriteString("\n")

	// Buttons
	content.WriteString(c.renderButtons(dialogWidth))
	content.WriteString("\n")
	content.WriteString(c.renderDialogBorder(dialogWidth))

	return content.String()
}

// renderDialogBorder renders the dialog border
func (c *ConfirmationComponent) renderDialogBorder(width int) string {
	return c.theme.Navigation.Render(strings.Repeat("─", width))
}

// renderDialogSeparator renders a separator line
func (c *ConfirmationComponent) renderDialogSeparator(width int) string {
	return c.theme.Status.Render(strings.Repeat("─", width))
}

// renderButtons renders the confirmation buttons
func (c *ConfirmationComponent) renderButtons(dialogWidth int) string {
	var cancelStyle, confirmStyle = c.theme.Button, c.theme.Button

	if c.selectedOption == 0 {
		cancelStyle = c.theme.ButtonFocus
	} else {
		confirmStyle = c.theme.ButtonFocus
		if c.confirmType == ConfirmationTypeDanger {
			confirmStyle = c.theme.ErrorStyle // ❌ Исправлено
		}
	}

	cancelButton := cancelStyle.Render(" " + c.cancelText + " ")
	confirmButton := confirmStyle.Render(" " + c.confirmText + " ")

	buttons := cancelButton + "  " + confirmButton
	return c.centerText(buttons, dialogWidth)
}

// centerText centers text within the given width
func (c *ConfirmationComponent) centerText(text string, width int) string {
	textLen := len(text)
	if textLen >= width {
		return text
	}

	padding := (width - textLen) / 2
	return strings.Repeat(" ", padding) + text + strings.Repeat(" ", width-textLen-padding)
}

// wrapText wraps text to fit within the specified width
func (c *ConfirmationComponent) wrapText(text string, width int) []string {
	if len(text) <= width {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	var currentLine strings.Builder

	for _, word := range words {
		if currentLine.Len()+len(word)+1 > width {
			if currentLine.Len() > 0 {
				lines = append(lines, currentLine.String())
				currentLine.Reset()
			}
		}

		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
		}
		currentLine.WriteString(word)
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}

// IsVisible returns whether the dialog is visible
func (c *ConfirmationComponent) IsVisible() bool {
	return c.visible
}

// Message types
type ConfirmationResultMsg struct {
	Confirmed bool
}
