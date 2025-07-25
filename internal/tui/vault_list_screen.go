// internal/tui/vault_list_screen.go
package tui

import (
	"fmt"

	"vault.module/internal/config"
	"vault.module/internal/tui/utils"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// VaultListScreen представляет экран списка vault'ов
type VaultListScreen struct {
	id        string
	title     string
	vaultList list.Model
	width     int
	height    int
}

// NewVaultListScreen создает новый экран списка vault'ов
func NewVaultListScreen() *VaultListScreen {
	// Создаем список vault'ов (используем существующий код)
	vaultItems := make([]list.Item, 0)
	for name, details := range config.Cfg.Vaults {
		vaultItems = append(vaultItems, vaultItem{
			name:   name,
			active: name == config.Cfg.ActiveVault,
			vType:  details.Type,
		})
	}

	vaultList := list.New(vaultItems, list.NewDefaultDelegate(), 0, 0)
	vaultList.Title = "Select Vault"

	return &VaultListScreen{
		id:        "vault_list",
		title:     "Vault Manager",
		vaultList: vaultList,
	}
}

// ID возвращает идентификатор экрана
func (s *VaultListScreen) ID() string {
	return s.id
}

// Title возвращает заголовок экрана
func (s *VaultListScreen) Title() string {
	return s.title
}

// CanGoBack определяет, можно ли вернуться с этого экрана
func (s *VaultListScreen) CanGoBack() bool {
	return false // Это корневой экран
}

// Init инициализирует экран
func (s *VaultListScreen) Init() tea.Cmd {
	return nil
}

// Update обрабатывает сообщения
func (s *VaultListScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.vaultList.SetSize(msg.Width-2, msg.Height-6) // Оставляем место для навигации и статуса

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			selected := s.vaultList.SelectedItem()
			if vaultItem, ok := selected.(vaultItem); ok {
				return s, loadVault(vaultItem.name)
			}
		}

	case loadVaultMsg:
		// Vault загружен, сохраняем в state manager
		stateManager := utils.GetStateManager()
		stateManager.SetCurrentVault(msg.name, msg.vault)
		stateManager.SetAuthenticated(true)

		// Переходим к экрану управления кошельками
		return NewWalletListScreen(msg.name, msg.vault), nil

	case errorMsg:
		// TODO: Показать ошибку
		return s, nil
	}

	// Обновляем список
	s.vaultList, cmd = s.vaultList.Update(msg)
	return s, cmd
}

// View отрисовывает экран
func (s *VaultListScreen) View() string {
	stateManager := utils.GetStateManager()
	theme := stateManager.GetTheme()

	content := fmt.Sprintf(
		"%s\n\n%s\n\n%s",
		theme.Title.Render("Vault Manager"),
		s.vaultList.View(),
		theme.Status.Render("Press Enter to select, 'q' to quit"),
	)

	return content
}
