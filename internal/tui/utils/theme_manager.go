// internal/tui/utils/theme_manager.go
package utils

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme contains all styles for TUI
type Theme struct {
	// Basic colors
	Primary    lipgloss.Color
	Secondary  lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
	Info       lipgloss.Color
	Background lipgloss.Color
	Foreground lipgloss.Color

	// Component styles
	Title        lipgloss.Style
	Subtitle     lipgloss.Style
	Status       lipgloss.Style
	ErrorStyle   lipgloss.Style // ❌ Исправлено: было Error
	SuccessStyle lipgloss.Style // ❌ Исправлено: было Success
	WarningStyle lipgloss.Style // ❌ Исправлено: было Warning
	InfoStyle    lipgloss.Style // ❌ Исправлено: было Info

	// Navigation styles
	Breadcrumb lipgloss.Style
	Navigation lipgloss.Style

	// Form styles
	Input       lipgloss.Style
	InputFocus  lipgloss.Style
	Button      lipgloss.Style
	ButtonFocus lipgloss.Style

	// List styles
	ListItem  lipgloss.Style
	ListFocus lipgloss.Style
	ListTitle lipgloss.Style

	// Table styles
	TableHeader lipgloss.Style
	TableRow    lipgloss.Style
	TableFocus  lipgloss.Style
}

// GetDefaultTheme returns the default theme
func GetDefaultTheme() *Theme {
	// Define colors
	primary := lipgloss.Color("#25A065")
	secondary := lipgloss.Color("#04B575")
	success := lipgloss.Color("#00FF00")
	warning := lipgloss.Color("#FFFF00")
	errorColor := lipgloss.Color("#FF0000")
	info := lipgloss.Color("#0080FF")
	background := lipgloss.Color("#000000")
	foreground := lipgloss.Color("#FFFFFF")

	theme := &Theme{
		Primary:    primary,
		Secondary:  secondary,
		Success:    success,
		Warning:    warning,
		Error:      errorColor,
		Info:       info,
		Background: background,
		Foreground: foreground,
	}

	// Basic styles
	theme.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(primary).
		Padding(0, 1).
		Bold(true)

	theme.Subtitle = lipgloss.NewStyle().
		Foreground(secondary).
		Bold(true)

	theme.Status = lipgloss.NewStyle().
		Foreground(secondary)

	theme.ErrorStyle = lipgloss.NewStyle().
		Foreground(errorColor).
		Bold(true)

	theme.SuccessStyle = lipgloss.NewStyle().
		Foreground(success).
		Bold(true)

	theme.WarningStyle = lipgloss.NewStyle().
		Foreground(warning).
		Bold(true)

	theme.InfoStyle = lipgloss.NewStyle().
		Foreground(info)

	// Navigation
	theme.Breadcrumb = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)

	theme.Navigation = lipgloss.NewStyle().
		Foreground(secondary).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primary).
		Padding(0, 1)

	// Forms
	theme.Input = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#555555")).
		Padding(0, 1)

	theme.InputFocus = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primary).
		Padding(0, 1)

	theme.Button = lipgloss.NewStyle().
		Background(lipgloss.Color("#333333")).
		Foreground(foreground).
		Padding(0, 2).
		Border(lipgloss.RoundedBorder())

	theme.ButtonFocus = lipgloss.NewStyle().
		Background(primary).
		Foreground(lipgloss.Color("#FFFDF5")).
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		Bold(true)

	// Lists
	theme.ListItem = lipgloss.NewStyle().
		Foreground(foreground).
		Padding(0, 2)

	theme.ListFocus = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(primary).
		Padding(0, 2).
		Bold(true)

	theme.ListTitle = lipgloss.NewStyle().
		Foreground(secondary).
		Bold(true).
		Underline(true)

	// Tables
	theme.TableHeader = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(primary).
		Padding(0, 1).
		Bold(true)

	theme.TableRow = lipgloss.NewStyle().
		Foreground(foreground).
		Padding(0, 1)

	theme.TableFocus = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(secondary).
		Padding(0, 1).
		Bold(true)

	return theme
}

// GetDarkTheme returns a dark theme
func GetDarkTheme() *Theme {
	theme := GetDefaultTheme()

	// Modify colors for dark theme
	theme.Background = lipgloss.Color("#1a1a1a")
	theme.Foreground = lipgloss.Color("#e0e0e0")

	return theme
}

// GetLightTheme returns a light theme
func GetLightTheme() *Theme {
	theme := GetDefaultTheme()

	// Modify colors for light theme
	theme.Background = lipgloss.Color("#ffffff")
	theme.Foreground = lipgloss.Color("#000000")
	theme.Primary = lipgloss.Color("#2d5a3d")
	theme.Secondary = lipgloss.Color("#3d7a5d")

	return theme
}
