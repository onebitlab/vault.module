// internal/tui/utils/navigation_stack.go
package utils

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Screen представляет интерфейс для всех экранов TUI
type Screen interface {
	tea.Model
	ID() string
	Title() string
	CanGoBack() bool
}

// NavigationStack управляет стеком экранов для навигации
type NavigationStack struct {
	screens []Screen
	current int
}

// NewNavigationStack создает новый стек навигации
func NewNavigationStack() *NavigationStack {
	return &NavigationStack{
		screens: make([]Screen, 0),
		current: -1,
	}
}

// Push добавляет новый экран в стек
func (ns *NavigationStack) Push(screen Screen) {
	// Удаляем все экраны после текущего (для случая возврата назад и нового перехода)
	if ns.current >= 0 && ns.current < len(ns.screens)-1 {
		ns.screens = ns.screens[:ns.current+1]
	}

	ns.screens = append(ns.screens, screen)
	ns.current = len(ns.screens) - 1
}

// Pop удаляет текущий экран и возвращается к предыдущему
func (ns *NavigationStack) Pop() Screen {
	if ns.current <= 0 {
		return nil
	}

	ns.current--
	return ns.Current()
}

// Current возвращает текущий экран
func (ns *NavigationStack) Current() Screen {
	if ns.current < 0 || ns.current >= len(ns.screens) {
		return nil
	}
	return ns.screens[ns.current]
}

// CanGoBack проверяет, можно ли вернуться назад
func (ns *NavigationStack) CanGoBack() bool {
	return ns.current > 0 && ns.Current() != nil && ns.Current().CanGoBack()
}

// Size возвращает количество экранов в стеке
func (ns *NavigationStack) Size() int {
	return len(ns.screens)
}

// Clear очищает весь стек
func (ns *NavigationStack) Clear() {
	ns.screens = make([]Screen, 0)
	ns.current = -1
}

// GetBreadcrumbs возвращает путь навигации для отображения
func (ns *NavigationStack) GetBreadcrumbs() []string {
	breadcrumbs := make([]string, 0, ns.current+1)
	for i := 0; i <= ns.current; i++ {
		if i < len(ns.screens) {
			breadcrumbs = append(breadcrumbs, ns.screens[i].Title())
		}
	}
	return breadcrumbs
}
