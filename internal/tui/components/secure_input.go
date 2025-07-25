// internal/tui/components/secure_input.go
package components

import (
	"strings"
	"time"

	"vault.module/internal/tui/utils"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// SecureInputType represents the type of secure input
type SecureInputType int

const (
	SecureInputPassword SecureInputType = iota
	SecureInputPIN
	SecureInputMnemonic
	SecureInputPrivateKey
)

// SecureInputComponent represents a secure input component
type SecureInputComponent struct {
	input     textinput.Model
	inputType SecureInputType
	theme     *utils.Theme

	// Security features
	showValue     bool
	autoHideDelay time.Duration
	lastShowTime  time.Time
	validator     func(string) error

	// State
	width    int
	height   int
	errorMsg string
}

// NewSecureInputComponent creates a new secure input component
func NewSecureInputComponent(theme *utils.Theme, inputType SecureInputType) *SecureInputComponent {
	input := textinput.New()
	input.EchoMode = textinput.EchoPassword

	component := &SecureInputComponent{
		input:         input,
		inputType:     inputType,
		theme:         theme,
		showValue:     false,
		autoHideDelay: 5 * time.Second,
	}

	// Configure based on input type
	switch inputType {
	case SecureInputPassword:
		input.Placeholder = "Enter password"
		input.CharLimit = 100
	case SecureInputPIN:
		input.Placeholder = "Enter PIN"
		input.CharLimit = 8
	case SecureInputMnemonic:
		input.Placeholder = "Enter mnemonic phrase"
		input.CharLimit = 500
	case SecureInputPrivateKey:
		input.Placeholder = "Enter private key"
		input.CharLimit = 200
	}

	return component
}

// SetValidator sets the input validator
func (c *SecureInputComponent) SetValidator(validator func(string) error) {
	c.validator = validator
}

// SetPlaceholder sets the input placeholder
func (c *SecureInputComponent) SetPlaceholder(placeholder string) {
	c.input.Placeholder = placeholder
}

// Focus focuses the input
func (c *SecureInputComponent) Focus() {
	c.input.Focus()
}

// Blur blurs the input
func (c *SecureInputComponent) Blur() {
	c.input.Blur()
	c.showValue = false
}

// Value returns the input value
func (c *SecureInputComponent) Value() string {
	return c.input.Value()
}

// SetValue sets the input value
func (c *SecureInputComponent) SetValue(value string) {
	c.input.SetValue(value)
}

// Update handles messages
func (c *SecureInputComponent) Update(msg tea.Msg) (*SecureInputComponent, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.width = msg.Width
		c.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+h":
			// Toggle visibility
			c.toggleVisibility()
			return c, nil
		case "ctrl+c":
			// Clear input
			c.input.SetValue("")
			c.errorMsg = ""
			return c, nil
		}

		// Update input
		c.input, cmd = c.input.Update(msg)

		// Validate input if validator is set
		if c.validator != nil {
			if err := c.validator(c.input.Value()); err != nil {
				c.errorMsg = err.Error()
			} else {
				c.errorMsg = ""
			}
		}

		// Update activity for security manager
		utils.GetSecurityManager().UpdateActivity()

	case time.Time:
		// Auto-hide timer
		if c.showValue && time.Since(c.lastShowTime) > c.autoHideDelay {
			c.showValue = false
		}
	}

	return c, cmd
}

// View renders the secure input
func (c *SecureInputComponent) View() string {
	var content strings.Builder

	// Render input with appropriate echo mode
	if c.showValue {
		c.input.EchoMode = textinput.EchoNormal
	} else {
		c.input.EchoMode = textinput.EchoPassword
	}

	content.WriteString(c.input.View())

	// Show visibility indicator
	if c.showValue {
		content.WriteString(" ")
		content.WriteString(c.theme.WarningStyle.Render("👁"))
	}

	// Show error message
	if c.errorMsg != "" {
		content.WriteString("\n")
		content.WriteString(c.theme.ErrorStyle.Render("Error: " + c.errorMsg))
	}

	// Show help text
	content.WriteString("\n")
	helpText := c.getHelpText()
	content.WriteString(c.theme.Status.Render(helpText))

	return content.String()
}

// toggleVisibility toggles the visibility of the input value
func (c *SecureInputComponent) toggleVisibility() {
	c.showValue = !c.showValue
	if c.showValue {
		c.lastShowTime = time.Now()
	}
}

// getHelpText returns help text based on input type
func (c *SecureInputComponent) getHelpText() string {
	baseHelp := "Ctrl+H: Toggle visibility | Ctrl+C: Clear"

	switch c.inputType {
	case SecureInputPassword:
		return baseHelp + " | Use strong password"
	case SecureInputPIN:
		return baseHelp + " | 4-8 digits recommended"
	case SecureInputMnemonic:
		return baseHelp + " | 12/24 words separated by spaces"
	case SecureInputPrivateKey:
		return baseHelp + " | Hex format (0x...)"
	default:
		return baseHelp
	}
}

// Validate validates the current input
func (c *SecureInputComponent) Validate() error {
	if c.validator != nil {
		return c.validator(c.input.Value())
	}
	return nil
}

// IsValid returns whether the current input is valid
func (c *SecureInputComponent) IsValid() bool {
	return c.errorMsg == ""
}

// Clear clears the input and error
func (c *SecureInputComponent) Clear() {
	c.input.SetValue("")
	c.errorMsg = ""
	c.showValue = false
}
