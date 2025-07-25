// internal/tui/screens/settings.go
package screens

import (
	"fmt"
	"strings"

	"vault.module/internal/tui/utils"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// SettingItem represents a setting item
type SettingItem struct {
	title       string
	description string
	value       string
	settingType string // "text", "select", "toggle", "number"
	options     []string
	action      string
}

func (i SettingItem) FilterValue() string { return i.title }
func (i SettingItem) Title() string       { return i.title }
func (i SettingItem) Description() string {
	return fmt.Sprintf("%s (Current: %s)", i.description, i.value)
}

// SettingsScreen represents the settings screen
type SettingsScreen struct {
	id           string
	title        string
	settingsList list.Model

	// Edit form
	editInput   textinput.Model
	editSelect  int
	editOptions []string

	// State
	currentSetting *SettingItem
	editMode       bool
	width          int
	height         int
	err            error
}

// NewSettingsScreen creates a new settings screen
func NewSettingsScreen() *SettingsScreen {
	settings := []list.Item{
		SettingItem{
			title:       "Theme",
			description: "Application color theme",
			value:       "Default",
			settingType: "select",
			options:     []string{"Default", "Dark", "Light"},
			action:      "theme",
		},
		SettingItem{
			title:       "Session Timeout",
			description: "Auto-logout timeout in minutes",
			value:       "60",
			settingType: "number",
			action:      "session_timeout",
		},
		SettingItem{
			title:       "Auto-save",
			description: "Automatically save changes",
			value:       "Enabled",
			settingType: "toggle",
			options:     []string{"Enabled", "Disabled"},
			action:      "auto_save",
		},
		SettingItem{
			title:       "Default Derivation Path",
			description: "Default derivation path for new wallets",
			value:       "m/44'/60'/0'/0",
			settingType: "text",
			action:      "default_derivation",
		},
		SettingItem{
			title:       "Log Level",
			description: "Application logging level",
			value:       "Info",
			settingType: "select",
			options:     []string{"Debug", "Info", "Warning", "Error"},
			action:      "log_level",
		},
		SettingItem{
			title:       "Backup Directory",
			description: "Default directory for backups",
			value:       "./backups",
			settingType: "text",
			action:      "backup_dir",
		},
		SettingItem{
			title:       "YubiKey Support",
			description: "Enable YubiKey hardware authentication",
			value:       "Enabled",
			settingType: "toggle",
			options:     []string{"Enabled", "Disabled"},
			action:      "yubikey_support",
		},
		SettingItem{
			title:       "Terminal Width",
			description: "Preferred terminal width",
			value:       "120",
			settingType: "number",
			action:      "terminal_width",
		},
	}

	settingsList := list.New(settings, list.NewDefaultDelegate(), 0, 0)
	settingsList.Title = "Application Settings"

	editInput := textinput.New()
	editInput.CharLimit = 100

	return &SettingsScreen{
		id:           "settings",
		title:        "Settings",
		settingsList: settingsList,
		editInput:    editInput,
		editMode:     false,
	}
}

// ID returns the screen identifier
func (s *SettingsScreen) ID() string {
	return s.id
}

// Title returns the screen title
func (s *SettingsScreen) Title() string {
	return s.title
}

// CanGoBack determines if we can go back from this screen
func (s *SettingsScreen) CanGoBack() bool {
	return true
}

// Init initializes the screen
func (s *SettingsScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (s *SettingsScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.settingsList.SetSize(msg.Width-4, msg.Height-10)

	case tea.KeyMsg:
		if s.editMode {
			switch msg.String() {
			case "esc":
				s.editMode = false
				s.currentSetting = nil
				s.err = nil
				return s, nil
			case "enter":
				return s.saveSetting()
			case "up":
				if s.currentSetting.settingType == "select" || s.currentSetting.settingType == "toggle" {
					s.editSelect = (s.editSelect - 1 + len(s.editOptions)) % len(s.editOptions)
				}
			case "down":
				if s.currentSetting.settingType == "select" || s.currentSetting.settingType == "toggle" {
					s.editSelect = (s.editSelect + 1) % len(s.editOptions)
				}
			}

			// Update text input for text/number types
			if s.currentSetting.settingType == "text" || s.currentSetting.settingType == "number" {
				s.editInput, cmd = s.editInput.Update(msg)
			}
		} else {
			switch msg.String() {
			case "enter":
				return s.editSetting()
			case "r":
				// Reset to defaults
				return s.resetToDefaults()
			case "s":
				// Save all settings
				return s.saveAllSettings()
			}

			s.settingsList, cmd = s.settingsList.Update(msg)
		}
	}

	return s, cmd
}

// View renders the screen
func (s *SettingsScreen) View() string {
	stateManager := utils.GetStateManager()
	theme := stateManager.GetTheme()

	var content strings.Builder

	// Title
	content.WriteString(theme.Title.Render("Application Settings"))
	content.WriteString("\n\n")

	if s.editMode && s.currentSetting != nil {
		content.WriteString(s.renderEditForm(theme))
	} else {
		content.WriteString(s.settingsList.View())
		content.WriteString("\n\n")
		content.WriteString(theme.Status.Render("Enter: Edit | R: Reset to Defaults | S: Save All | ESC: Back"))
	}

	// Error display
	if s.err != nil {
		content.WriteString("\n\n")
		content.WriteString(theme.Error.Render("Error: " + s.err.Error()))
	}

	return content.String()
}

// renderEditForm renders the setting edit form
func (s *SettingsScreen) renderEditForm(theme *utils.Theme) string {
	var content strings.Builder

	content.WriteString(theme.Subtitle.Render("Edit Setting: " + s.currentSetting.title))
	content.WriteString("\n\n")
	content.WriteString(theme.Info.Render(s.currentSetting.description))
	content.WriteString("\n\n")

	switch s.currentSetting.settingType {
	case "text", "number":
		content.WriteString("Value:")
		content.WriteString("\n")
		content.WriteString(s.editInput.View())
		content.WriteString("\n\n")

		if s.currentSetting.settingType == "number" {
			content.WriteString(theme.Status.Render("Enter a numeric value"))
		}

	case "select", "toggle":
		content.WriteString("Select option:")
		content.WriteString("\n\n")

		for i, option := range s.editOptions {
			prefix := "  "
			style := theme.ListItem

			if i == s.editSelect {
				prefix = "> "
				style = theme.ListFocus
			}

			line := prefix + option
			content.WriteString(style.Render(line))
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")
	content.WriteString(theme.Status.Render("Enter: Save | Up/Down: Navigate | ESC: Cancel"))

	return content.String()
}

// editSetting starts editing a setting
func (s *SettingsScreen) editSetting() (tea.Model, tea.Cmd) {
	selected := s.settingsList.SelectedItem()
	if setting, ok := selected.(SettingItem); ok {
		s.currentSetting = &setting
		s.editMode = true
		s.err = nil

		switch setting.settingType {
		case "text", "number":
			s.editInput.SetValue(setting.value)
			s.editInput.Focus()
		case "select", "toggle":
			s.editOptions = setting.options
			// Find current value index
			for i, option := range s.editOptions {
				if option == setting.value {
					s.editSelect = i
					break
				}
			}
		}
	}
	return s, nil
}

// saveSetting saves the current setting
func (s *SettingsScreen) saveSetting() (tea.Model, tea.Cmd) {
	if s.currentSetting == nil {
		return s, nil
	}

	var newValue string

	switch s.currentSetting.settingType {
	case "text":
		newValue = strings.TrimSpace(s.editInput.Value())
		if newValue == "" {
			s.err = fmt.Errorf("value cannot be empty")
			return s, nil
		}
	case "number":
		newValue = strings.TrimSpace(s.editInput.Value())
		// TODO: Add number validation
	case "select", "toggle":
		if s.editSelect >= 0 && s.editSelect < len(s.editOptions) {
			newValue = s.editOptions[s.editSelect]
		}
	}

	// Apply the setting
	s.applySetting(s.currentSetting.action, newValue)

	// Update the list item
	s.updateSettingInList(s.currentSetting.action, newValue)

	s.editMode = false
	s.currentSetting = nil

	return s, nil
}

// applySetting applies a setting change
func (s *SettingsScreen) applySetting(action, value string) {
	stateManager := utils.GetStateManager()

	switch action {
	case "theme":
		var theme *utils.Theme
		switch value {
		case "Dark":
			theme = utils.GetDarkTheme()
		case "Light":
			theme = utils.GetLightTheme()
		default:
			theme = utils.GetDefaultTheme()
		}
		stateManager.SetTheme(theme)

	case "session_timeout":
		// TODO: Apply session timeout setting

	case "auto_save":
		// TODO: Apply auto-save setting

	case "default_derivation":
		// TODO: Apply default derivation path setting

	case "log_level":
		// TODO: Apply log level setting

	case "backup_dir":
		// TODO: Apply backup directory setting

	case "yubikey_support":
		// TODO: Apply YubiKey support setting

	case "terminal_width":
		// TODO: Apply terminal width setting
	}
}

// updateSettingInList updates a setting value in the list
func (s *SettingsScreen) updateSettingInList(action, newValue string) {
	items := s.settingsList.Items()
	for i, item := range items {
		if setting, ok := item.(SettingItem); ok && setting.action == action {
			setting.value = newValue
			items[i] = setting
			break
		}
	}
	s.settingsList.SetItems(items)
}

// resetToDefaults resets all settings to default values
func (s *SettingsScreen) resetToDefaults() (tea.Model, tea.Cmd) {
	// TODO: Implement reset to defaults
	return s, nil
}

// saveAllSettings saves all current settings
func (s *SettingsScreen) saveAllSettings() (tea.Model, tea.Cmd) {
	// TODO: Implement save all settings
	return s, nil
}
