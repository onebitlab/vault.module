// internal/tui/tui.go
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"vault.module/internal/config"
	"vault.module/internal/vault"
)

// Определяем стили
var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000"))
)

// Состояния интерфейса
type state int

const (
	stateVaultList state = iota
	stateWalletList
	stateWalletDetail
	stateAddWallet
	stateError
)

// Главная модель TUI
type Model struct {
	state        state
	err          error
	vaultList    list.Model
	walletList   list.Model
	textInput    textinput.Model
	currentVault string
	vault        vault.Vault
	width        int
	height       int
}

// Элементы списка
type vaultItem struct {
	name   string
	active bool
	vType  string
}

func (i vaultItem) FilterValue() string { return i.name }
func (i vaultItem) Title() string {
	if i.active {
		return fmt.Sprintf("● %s (active)", i.name)
	}
	return i.name
}
func (i vaultItem) Description() string { return fmt.Sprintf("Type: %s", i.vType) }

type walletItem struct {
	prefix    string
	addresses int
}

func (i walletItem) FilterValue() string { return i.prefix }
func (i walletItem) Title() string       { return i.prefix }
func (i walletItem) Description() string {
	return fmt.Sprintf("%d address(es)", i.addresses)
}

// Инициализация модели
func initialModel() *Model {
	// Создаем список vault'ов
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

	walletList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	walletList.Title = "Wallets"

	textInput := textinput.New()
	textInput.Placeholder = "Enter command..."

	return &Model{
		state:      stateVaultList,
		vaultList:  vaultList,
		walletList: walletList,
		textInput:  textInput,
	}
}

// Команды для асинхронных операций
type loadVaultMsg struct {
	vault vault.Vault
	name  string
}

type errorMsg struct {
	err error
}

func loadVault(name string) tea.Cmd {
	return func() tea.Msg {
		details, exists := config.Cfg.Vaults[name]
		if !exists {
			return errorMsg{fmt.Errorf("vault '%s' not found", name)}
		}

		v, err := vault.LoadVault(details)
		if err != nil {
			return errorMsg{fmt.Errorf("failed to load vault: %w", err)}
		}

		return loadVaultMsg{vault: v, name: name}
	}
}

// Обработка сообщений
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.vaultList.SetSize(msg.Width-2, msg.Height-2)
		m.walletList.SetSize(msg.Width-2, msg.Height-2)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "esc":
			if m.state == stateWalletList {
				m.state = stateVaultList
				return m, nil
			}

		case "enter":
			switch m.state {
			case stateVaultList:
				selected := m.vaultList.SelectedItem()
				if vaultItem, ok := selected.(vaultItem); ok {
					m.state = stateWalletList
					return m, loadVault(vaultItem.name)
				}
			}
		}

	case loadVaultMsg:
		m.vault = msg.vault
		m.currentVault = msg.name

		// Создаем список кошельков
		walletItems := make([]list.Item, 0, len(msg.vault))
		for prefix, wallet := range msg.vault {
			walletItems = append(walletItems, walletItem{
				prefix:    prefix,
				addresses: len(wallet.Addresses),
			})
		}

		m.walletList.SetItems(walletItems)
		m.walletList.Title = fmt.Sprintf("Wallets in %s", msg.name)

	case errorMsg:
		m.err = msg.err
		m.state = stateError
		return m, nil
	}

	// Обновляем активные компоненты
	switch m.state {
	case stateVaultList:
		m.vaultList, cmd = m.vaultList.Update(msg)
	case stateWalletList:
		m.walletList, cmd = m.walletList.Update(msg)
	case stateAddWallet:
		m.textInput, cmd = m.textInput.Update(msg)
	}

	return m, cmd
}

func (m *Model) Init() tea.Cmd {
	return nil
}

// Отрисовка интерфейса
func (m *Model) View() string {
	switch m.state {
	case stateError:
		return fmt.Sprintf(
			"%s\n\n%s\n\nPress 'q' to quit",
			titleStyle.Render("Error"),
			errorStyle.Render(m.err.Error()),
		)

	case stateVaultList:
		return fmt.Sprintf(
			"%s\n\n%s\n\n%s",
			titleStyle.Render("Vault Manager"),
			m.vaultList.View(),
			statusStyle.Render("Press Enter to select, 'q' to quit"),
		)

	case stateWalletList:
		return fmt.Sprintf(
			"%s\n\n%s\n\n%s",
			titleStyle.Render(fmt.Sprintf("Vault: %s", m.currentVault)),
			m.walletList.View(),
			statusStyle.Render("Press Esc to go back, 'q' to quit"),
		)

	default:
		return "Loading..."
	}
}

// StartTUI запускает интерактивный интерфейс
func StartTUI() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
	}
}
