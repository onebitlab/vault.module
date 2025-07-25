// internal/tui/components/yubikey_auth.go
package components

import (
	"fmt"
	"strings"
	"time"

	"vault.module/internal/tui/utils"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// YubiKeyAuthState represents the authentication state
type YubiKeyAuthState int

const (
	YubiKeyStateIdle YubiKeyAuthState = iota
	YubiKeyStateWaiting
	YubiKeyStateAuthenticating
	YubiKeyStateSuccess
	YubiKeyStateError
)

// YubiKeyAuthComponent represents YubiKey authentication component
type YubiKeyAuthComponent struct {
	state    YubiKeyAuthState
	spinner  spinner.Model
	pinInput textinput.Model
	theme    *utils.Theme

	// Authentication data
	challenge string
	response  string
	errorMsg  string
	timeout   time.Duration
	startTime time.Time

	// Configuration
	requirePIN bool
	width      int
	height     int
}

// NewYubiKeyAuthComponent creates a new YubiKey authentication component
func NewYubiKeyAuthComponent(theme *utils.Theme, requirePIN bool) *YubiKeyAuthComponent {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = theme.Info

	pinInput := textinput.New()
	pinInput.Placeholder = "Enter YubiKey PIN"
	pinInput.EchoMode = textinput.EchoPassword
	pinInput.CharLimit = 8

	return &YubiKeyAuthComponent{
		state:      YubiKeyStateIdle,
		spinner:    s,
		pinInput:   pinInput,
		theme:      theme,
		requirePIN: requirePIN,
		timeout:    30 * time.Second,
	}
}

// Init initializes the component
func (c *YubiKeyAuthComponent) Init() tea.Cmd {
	return c.spinner.Tick
}

// Update handles messages
func (c *YubiKeyAuthComponent) Update(msg tea.Msg) (*YubiKeyAuthComponent, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.width = msg.Width
		c.height = msg.Height

	case tea.KeyMsg:
		switch c.state {
		case YubiKeyStateIdle:
			switch msg.String() {
			case "enter":
				return c.startAuthentication()
			case "esc":
				return c, nil
			}
		case YubiKeyStateWaiting:
			if c.requirePIN {
				switch msg.String() {
				case "enter":
					if strings.TrimSpace(c.pinInput.Value()) != "" {
						return c.authenticateWithPIN()
					}
				case "esc":
					c.state = YubiKeyStateIdle
					c.pinInput.SetValue("")
					return c, nil
				}
				c.pinInput, cmd = c.pinInput.Update(msg)
			} else {
				switch msg.String() {
				case "enter":
					return c.authenticateWithoutPIN()
				case "esc":
					c.state = YubiKeyStateIdle
					return c, nil
				}
			}
		case YubiKeyStateSuccess, YubiKeyStateError:
			switch msg.String() {
			case "enter", "esc":
				c.state = YubiKeyStateIdle
				c.errorMsg = ""
				c.pinInput.SetValue("")
				return c, nil
			}
		}

	case spinner.TickMsg:
		if c.state == YubiKeyStateAuthenticating {
			c.spinner, cmd = c.spinner.Update(msg)

			// Check for timeout
			if time.Since(c.startTime) > c.timeout {
				c.state = YubiKeyStateError
				c.errorMsg = "Authentication timeout"
				return c, nil
			}
		}

	case yubiKeyAuthResultMsg:
		if msg.success {
			c.state = YubiKeyStateSuccess
			c.response = msg.response
		} else {
			c.state = YubiKeyStateError
			c.errorMsg = msg.error
		}
		return c, nil
	}

	return c, cmd
}

// View renders the component
func (c *YubiKeyAuthComponent) View() string {
	var content strings.Builder

	content.WriteString(c.theme.Title.Render("YubiKey Authentication"))
	content.WriteString("\n\n")

	switch c.state {
	case YubiKeyStateIdle:
		content.WriteString(c.renderIdleState())
	case YubiKeyStateWaiting:
		content.WriteString(c.renderWaitingState())
	case YubiKeyStateAuthenticating:
		content.WriteString(c.renderAuthenticatingState())
	case YubiKeyStateSuccess:
		content.WriteString(c.renderSuccessState())
	case YubiKeyStateError:
		content.WriteString(c.renderErrorState())
	}

	return content.String()
}

// renderIdleState renders the idle state
func (c *YubiKeyAuthComponent) renderIdleState() string {
	var content strings.Builder

	content.WriteString(c.theme.Info.Render("🔐 YubiKey authentication required"))
	content.WriteString("\n\n")
	content.WriteString("Please ensure your YubiKey is connected")
	content.WriteString("\n\n")
	content.WriteString(c.theme.Status.Render("Press Enter to start authentication, ESC to cancel"))

	return content.String()
}

// renderWaitingState renders the waiting state
func (c *YubiKeyAuthComponent) renderWaitingState() string {
	var content strings.Builder

	if c.requirePIN {
		content.WriteString(c.theme.Subtitle.Render("Enter YubiKey PIN"))
		content.WriteString("\n\n")
		content.WriteString(c.pinInput.View())
		content.WriteString("\n\n")
		content.WriteString(c.theme.Status.Render("Enter: Authenticate | ESC: Cancel"))
	} else {
		content.WriteString(c.theme.Info.Render("🔑 Touch your YubiKey to authenticate"))
		content.WriteString("\n\n")
		content.WriteString(c.theme.Status.Render("Enter: Continue | ESC: Cancel"))
	}

	return content.String()
}

// renderAuthenticatingState renders the authenticating state
func (c *YubiKeyAuthComponent) renderAuthenticatingState() string {
	var content strings.Builder

	content.WriteString(c.spinner.View())
	content.WriteString(" ")
	content.WriteString(c.theme.Info.Render("Authenticating with YubiKey..."))
	content.WriteString("\n\n")

	elapsed := time.Since(c.startTime)
	remaining := c.timeout - elapsed
	content.WriteString(c.theme.Status.Render(fmt.Sprintf("Timeout in: %.0f seconds", remaining.Seconds())))

	return content.String()
}

// renderSuccessState renders the success state
func (c *YubiKeyAuthComponent) renderSuccessState() string {
	var content strings.Builder

	content.WriteString(c.theme.Success.Render("✅ Authentication successful!"))
	content.WriteString("\n\n")
	content.WriteString(c.theme.Info.Render("YubiKey authentication completed"))
	content.WriteString("\n\n")
	content.WriteString(c.theme.Status.Render("Press Enter to continue"))

	return content.String()
}

// renderErrorState renders the error state
func (c *YubiKeyAuthComponent) renderErrorState() string {
	var content strings.Builder

	content.WriteString(c.theme.Error.Render("❌ Authentication failed"))
	content.WriteString("\n\n")
	content.WriteString(c.theme.Error.Render("Error: " + c.errorMsg))
	content.WriteString("\n\n")
	content.WriteString(c.theme.Status.Render("Press Enter to retry, ESC to cancel"))

	return content.String()
}

// startAuthentication starts the authentication process
func (c *YubiKeyAuthComponent) startAuthentication() (*YubiKeyAuthComponent, tea.Cmd) {
	c.state = YubiKeyStateWaiting
	c.errorMsg = ""

	if c.requirePIN {
		c.pinInput.Focus()
	}

	return c, nil
}

// authenticateWithPIN authenticates with PIN
func (c *YubiKeyAuthComponent) authenticateWithPIN() (*YubiKeyAuthComponent, tea.Cmd) {
	c.state = YubiKeyStateAuthenticating
	c.startTime = time.Now()
	c.pinInput.Blur()

	pin := c.pinInput.Value()
	return c, tea.Batch(
		c.spinner.Tick,
		c.performYubiKeyAuth(pin),
	)
}

// authenticateWithoutPIN authenticates without PIN
func (c *YubiKeyAuthComponent) authenticateWithoutPIN() (*YubiKeyAuthComponent, tea.Cmd) {
	c.state = YubiKeyStateAuthenticating
	c.startTime = time.Now()

	return c, tea.Batch(
		c.spinner.Tick,
		c.performYubiKeyAuth(""),
	)
}

// performYubiKeyAuth performs the actual YubiKey authentication
func (c *YubiKeyAuthComponent) performYubiKeyAuth(pin string) tea.Cmd {
	return func() tea.Msg {
		// TODO: Implement actual YubiKey authentication
		// This is a placeholder implementation
		time.Sleep(2 * time.Second) // Simulate authentication delay

		// Simulate success/failure
		if pin != "wrong" { // Simple test condition
			return yubiKeyAuthResultMsg{
				success:  true,
				response: "auth_token_12345",
			}
		} else {
			return yubiKeyAuthResultMsg{
				success: false,
				error:   "Invalid PIN or YubiKey not found",
			}
		}
	}
}

// IsAuthenticated returns whether authentication was successful
func (c *YubiKeyAuthComponent) IsAuthenticated() bool {
	return c.state == YubiKeyStateSuccess
}

// GetAuthResponse returns the authentication response
func (c *YubiKeyAuthComponent) GetAuthResponse() string {
	return c.response
}

// Reset resets the component to idle state
func (c *YubiKeyAuthComponent) Reset() {
	c.state = YubiKeyStateIdle
	c.errorMsg = ""
	c.response = ""
	c.pinInput.SetValue("")
}

// Message types
type yubiKeyAuthResultMsg struct {
	success  bool
	response string
	error    string
}
