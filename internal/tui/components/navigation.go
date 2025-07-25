// internal/tui/components/navigation.go
package components

import (
	"strings"

	"vault.module/internal/tui/utils"
)

// NavigationBar представляет навигационную панель
type NavigationBar struct {
	breadcrumbs []string
	theme       *utils.Theme
	width       int
}

// NewNavigationBar создает новую навигационную панель
func NewNavigationBar(theme *utils.Theme) *NavigationBar {
	return &NavigationBar{
		theme: theme,
		width: 80,
	}
}

// SetBreadcrumbs устанавливает путь навигации
func (nb *NavigationBar) SetBreadcrumbs(breadcrumbs []string) {
	nb.breadcrumbs = breadcrumbs
}

// SetWidth устанавливает ширину панели
func (nb *NavigationBar) SetWidth(width int) {
	nb.width = width
}

// Render отрисовывает навигационную панель
func (nb *NavigationBar) Render() string {
	if len(nb.breadcrumbs) == 0 {
		return ""
	}

	// Создаем строку навигации
	breadcrumbStr := strings.Join(nb.breadcrumbs, " > ")

	// Обрезаем если слишком длинная
	if len(breadcrumbStr) > nb.width-4 {
		breadcrumbStr = "..." + breadcrumbStr[len(breadcrumbStr)-(nb.width-7):]
	}

	return nb.theme.Breadcrumb.Render(breadcrumbStr)
}
