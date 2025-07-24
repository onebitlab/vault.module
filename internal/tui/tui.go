// File: internal/tui/tui.go
package tui

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"vault.module/internal/actions"
	"vault.module/internal/audit"
	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Constants ---
const clipboardClearTimeout = 30 * time.Second

// --- Styles ---
type Styles struct {
	App, Title, Status, Error, Help, Bordered lipgloss.Style
	Selected, Normal                          lipgloss.Style
	TableHeader, TableRow, SelectedTableRow   lipgloss.Style
}

func newStyles() *Styles {
	s := &Styles{}
	s.App = lipgloss.NewStyle().Margin(0, 2)
	s.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5")).Background(lipgloss.Color("#25A065")).Padding(0, 1)
	s.Status = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"})
	s.Error = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5733")).Bold(true)
	s.Help = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	s.Bordered = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(1)
	s.Selected = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	s.Normal = lipgloss.NewStyle()
	s.TableHeader = lipgloss.NewStyle().Bold(true)
	s.TableRow = lipgloss.NewStyle()
	s.SelectedTableRow = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	return s
}

// --- View Stack Management ---
type view interface {
	tea.Model
	Help() []key.Binding
	Title() string
}

// --- Messages ---
type vaultLoadedMsg struct {
	v   vault.Vault
	err error
}
type deriveCompletedMsg struct {
	w      vault.Wallet
	newAdr vault.Address
	err    error
}
type clipboardClearedMsg struct{}
type statusMsg struct {
	text     string
	isError  bool
	duration time.Duration
}
type popViewMsg struct{}
type popToRootMsg struct{}

// --- Keymaps ---
var globalKeys = []key.Binding{
	key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
}

type keyMap struct {
	bindings []key.Binding
}

func (k keyMap) ShortHelp() []key.Binding  { return k.bindings }
func (k keyMap) FullHelp() [][]key.Binding { return [][]key.Binding{k.bindings} }

// --- List Items ---
type vaultItem struct{ name, desc string }

func (i vaultItem) Title() string       { return i.name }
func (i vaultItem) Description() string { return i.desc }
func (i vaultItem) FilterValue() string { return i.name }

type walletItem struct{ prefix, desc string }

func (i walletItem) Title() string       { return i.prefix }
func (i walletItem) Description() string { return i.desc }
func (i walletItem) FilterValue() string { return i.prefix }

type choiceItem struct{ id, title, desc string }

func (i choiceItem) Title() string       { return i.title }
func (i choiceItem) Description() string { return i.desc }
func (i choiceItem) FilterValue() string { return i.title }

// --- Main Model ---
type model struct {
	styles        *Styles
	help          help.Model
	spinner       spinner.Model
	viewStack     []view
	loadedVault   *vault.Vault
	activeVault   config.VaultDetails
	statusMsg     string
	statusIsError bool
	width, height int
}

func newModel() *model {
	styles := newStyles()
	spinner := spinner.New(spinner.WithStyle(styles.Status))
	m := &model{
		styles:  styles,
		help:    help.New(),
		spinner: spinner,
	}
	m.help.ShowAll = true
	return m
}

func (m *model) pushView(v view) { m.viewStack = append(m.viewStack, v) }
func (m *model) popView() {
	if len(m.viewStack) > 1 {
		m.viewStack = m.viewStack[:len(m.viewStack)-1]
	}
}
func (m *model) popToRoot() {
	if len(m.viewStack) > 1 {
		m.viewStack = m.viewStack[:1]
	}
}
func (m *model) currentView() view {
	if len(m.viewStack) == 0 {
		return nil
	}
	return m.viewStack[len(m.viewStack)-1]
}
func (m *model) saveActiveVault() tea.Cmd {
	if m.loadedVault == nil {
		return func() tea.Msg { return statusMsg{text: "Error: no vault loaded to save", isError: true} }
	}
	err := vault.SaveVault(m.activeVault, *m.loadedVault)
	if err != nil {
		return func() tea.Msg { return statusMsg{text: fmt.Sprintf("Save error: %v", err), isError: true} }
	}
	return nil
}
func loadVaultCmd(details config.VaultDetails) tea.Cmd {
	return func() tea.Msg {
		v, err := vault.LoadVault(details)
		return vaultLoadedMsg{v: v, err: err}
	}
}

func (m *model) Init() tea.Cmd {
	activeVault, err := config.GetActiveVault()
	if err != nil || len(config.Cfg.Vaults) == 0 {
		vaultListView := newVaultListView(m)
		m.pushView(vaultListView)
		return vaultListView.Init()
	}
	m.activeVault = activeVault
	loadingView := newLoadingView(m, fmt.Sprintf("Loading vault '%s'...", config.Cfg.ActiveVault))
	m.pushView(loadingView)
	return tea.Batch(m.spinner.Tick, loadVaultCmd(activeVault))
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Глобальная обработка хоткеев для vaultListView
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if v, ok := m.currentView().(*vaultListView); ok {
			switch keyMsg.String() {
			case "a":
				return m, v.showAddVaultForm()
			case "t":
				return m, v.showTokenMenu()
			case "s":
				return m, v.showConfigForm()
			case "c":
				return m, v.showCloneVaultForm()
			case "o":
				return m, v.showAutocompleteForm()
			case "i":
				return m, v.showInitVaultConfirm()
			case "d", "x":
				if selected, ok := v.list.SelectedItem().(vaultItem); ok {
					prompt := fmt.Sprintf("Remove vault '%s' from config? (File will NOT be deleted)", selected.name)
					onConfirm := func() tea.Msg {
						delete(config.Cfg.Vaults, selected.name)
						if config.Cfg.ActiveVault == selected.name {
							config.Cfg.ActiveVault = ""
						}
						if err := config.SaveConfig(); err != nil {
							return statusMsg{text: fmt.Sprintf("Delete error: %v", err), isError: true}
						}
						v.refreshVaultList()
						return statusMsg{text: fmt.Sprintf("Vault '%s' removed.", selected.name), isError: false, duration: 3 * time.Second}
					}
					v.parent.pushView(newConfirmView(v.parent, prompt, onConfirm))
					return m, nil
				}
			case "enter":
				if selected, ok := v.list.SelectedItem().(vaultItem); ok {
					config.Cfg.ActiveVault = selected.name
					if err := config.SaveConfig(); err != nil {
						return m, func() tea.Msg { return statusMsg{text: "Failed to save config", isError: true} }
					}
					details := config.Cfg.Vaults[selected.name]
					loading := newLoadingView(m, fmt.Sprintf("Loading vault '%s'...", selected.name))
					v.parent.pushView(loading)
					return m, tea.Batch(loading.Init(), loadVaultCmd(details))
				}
			}
		}
	}
	// Обычная логика
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case statusMsg:
		m.statusMsg = msg.text
		m.statusIsError = msg.isError
		if msg.duration > 0 {
			cmds = append(cmds, tea.Tick(msg.duration, func(t time.Time) tea.Msg {
				if m.statusMsg == msg.text {
					return statusMsg{text: ""}
				}
				return nil
			}))
		}
		return m, tea.Batch(cmds...)
	case popViewMsg:
		m.popView()
		if v := m.currentView(); v != nil {
			return m, v.Init()
		}
	case popToRootMsg:
		m.popToRoot()
		if v := m.currentView(); v != nil {
			return m, v.Init()
		}
	case vaultLoadedMsg:
		m.popView() // Pop loading view
		if msg.err != nil {
			cmds = append(cmds, func() tea.Msg { return statusMsg{text: fmt.Sprintf("Error: %v", msg.err), isError: true} })
			vaultListView := newVaultListView(m)
			m.pushView(vaultListView)
			cmds = append(cmds, vaultListView.Init())
			return m, tea.Batch(cmds...)
		}
		v := msg.v
		m.loadedVault = &v
		walletListView := newWalletListView(m)
		m.pushView(walletListView)
		return m, walletListView.Init()
	case clipboardClearedMsg:
		return m, func() tea.Msg {
			return statusMsg{text: "Clipboard cleared.", isError: false, duration: 3 * time.Second}
		}
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, globalKeys[0]): // Quit
			return m, tea.Quit
		case key.Matches(msg, globalKeys[1]): // Back
			if len(m.viewStack) > 1 {
				m.popView()
				if v := m.currentView(); v != nil {
					return m, v.Init()
				}
			}
		}
	}
	if v := m.currentView(); v != nil {
		newView, cmd := v.Update(msg)
		m.viewStack[len(m.viewStack)-1] = newView.(view)
		cmds = append(cmds, cmd)
	}
	var spinnerCmd tea.Cmd
	m.spinner, spinnerCmd = m.spinner.Update(msg)
	cmds = append(cmds, spinnerCmd)
	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}
	var mainContent string
	if v := m.currentView(); v != nil {
		mainContent = v.View()
	} else {
		mainContent = "No view to display."
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.headerView(), m.styles.App.Render(mainContent), m.footerView())
}

func (m *model) headerView() string {
	title := "Vault"
	if v := m.currentView(); v != nil {
		title = v.Title()
	}
	return m.styles.Title.Render(title)
}

func (m *model) footerView() string {
	var helpKeys []key.Binding
	if v := m.currentView(); v != nil {
		helpKeys = v.Help()
	}
	km := keyMap{bindings: append(helpKeys, globalKeys...)}
	statusStyle := m.styles.Status
	if m.statusIsError {
		statusStyle = m.styles.Error
	}
	return lipgloss.JoinVertical(lipgloss.Left, statusStyle.Render(m.statusMsg), m.styles.Help.Render(m.help.View(km)))
}

// --- Entry Point ---
func StartTUI() {
	if err := audit.InitLogger(); err != nil {
		fmt.Println("Failed to initialize audit logger:", err)
		os.Exit(1)
	}
	if err := config.LoadConfig(); err != nil {
		fmt.Println("Failed to load configuration:", err)
		os.Exit(1)
	}
	audit.Logger.Info("Starting interactive TUI mode")
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error starting TUI:", err)
		os.Exit(1)
	}
}

// =======================================================================//
//                           VIEW IMPLEMENTATIONS                         //
// =======================================================================//

// --- Loading View ---
type loadingView struct {
	parent *model
	msg    string
}

func newLoadingView(parent *model, msg string) *loadingView {
	return &loadingView{parent: parent, msg: msg}
}
func (v *loadingView) Init() tea.Cmd                           { return v.parent.spinner.Tick }
func (v *loadingView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return v, nil }
func (v *loadingView) View() string {
	return fmt.Sprintf("\n   %s %s\n", v.parent.spinner.View(), v.msg)
}
func (v *loadingView) Title() string       { return "Loading..." }
func (v *loadingView) Help() []key.Binding { return nil }

// --- Confirm View ---
type confirmView struct {
	parent    *model
	prompt    string
	onConfirm tea.Cmd
}

func newConfirmView(parent *model, prompt string, onConfirm tea.Cmd) *confirmView {
	return &confirmView{parent: parent, prompt: prompt, onConfirm: onConfirm}
}
func (v *confirmView) Init() tea.Cmd { return nil }
func (v *confirmView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "y", "Y":
			return v, tea.Sequence(func() tea.Msg { return popViewMsg{} }, v.onConfirm)
		case "n", "N", "esc":
			return v, func() tea.Msg { return popViewMsg{} }
		}
	}
	return v, nil
}
func (v *confirmView) View() string {
	return v.parent.styles.Bordered.Render(fmt.Sprintf("%s\n\n(y/n)", v.prompt))
}
func (v *confirmView) Title() string { return "Confirmation" }
func (v *confirmView) Help() []key.Binding {
	return []key.Binding{key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yes")), key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "no"))}
}

// --- Choice View ---
type choiceView struct {
	parent   *model
	list     list.Model
	onChoose func(choiceID string) tea.Cmd
}

func newChoiceView(parent *model, title string, choices []choiceItem, onChoose func(string) tea.Cmd) *choiceView {
	listItems := make([]list.Item, len(choices))
	for i, choice := range choices {
		listItems[i] = choice
	}
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = parent.styles.Selected
	delegate.Styles.NormalTitle = parent.styles.Normal
	l := list.New(listItems, delegate, 0, 0)
	l.Title = title
	l.SetShowHelp(false)
	return &choiceView{parent: parent, list: l, onChoose: onChoose}
}
func (v *choiceView) Init() tea.Cmd {
	v.list.SetSize(v.parent.width-4, v.parent.height-lipgloss.Height(v.parent.headerView())-lipgloss.Height(v.parent.footerView())-2)
	return nil
}
func (v *choiceView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			if i, ok := v.list.SelectedItem().(choiceItem); ok {
				return v, v.onChoose(i.id)
			}
		}
	}
	var cmd tea.Cmd
	v.list, cmd = v.list.Update(msg)
	return v, cmd
}
func (v *choiceView) View() string  { return v.list.View() }
func (v *choiceView) Title() string { return v.list.Title }
func (v *choiceView) Help() []key.Binding {
	return []key.Binding{key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select"))}
}

// --- Vault List View ---
type vaultListView struct {
	parent *model
	list   list.Model
}

func newVaultListView(parent *model) *vaultListView {
	v := &vaultListView{parent: parent}
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = parent.styles.Selected
	delegate.Styles.NormalTitle = parent.styles.Normal
	v.list = list.New(nil, delegate, 0, 0)
	v.list.Title = "Select a Vault"
	v.list.SetShowHelp(false)
	// v.list.SetFocus(true) // удалено, такого метода нет
	v.refreshVaultList()
	return v
}
func (v *vaultListView) Init() tea.Cmd {
	v.list.SetSize(v.parent.width-4, v.parent.height-lipgloss.Height(v.parent.headerView())-lipgloss.Height(v.parent.footerView())-2)
	v.refreshVaultList()
	return nil
}
func (v *vaultListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "/":
			var cmd tea.Cmd
			v.list, cmd = v.list.Update(msg)
			return v, cmd
		}
	}
	var cmd tea.Cmd
	v.list, cmd = v.list.Update(msg)
	return v, cmd
}
func (v *vaultListView) View() string {
	if len(config.Cfg.Vaults) == 0 {
		v.list.SetItems([]list.Item{vaultItem{name: "Нет ни одного хранилища (vault). Нажмите 'A', чтобы создать новый.", desc: ""}})
	}
	return v.list.View()
}
func (v *vaultListView) Title() string { return v.list.Title }
func (v *vaultListView) Help() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "add")),
		key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "clone vault")),
		key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "token menu")),
		key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "settings/config")),
		key.NewBinding(key.WithKeys("O"), key.WithHelp("O", "autocomplete")),
		key.NewBinding(key.WithKeys("I"), key.WithHelp("I", "init vault")),
	}
}
func (v *vaultListView) refreshVaultList() {
	items := []list.Item{}
	names := make([]string, 0, len(config.Cfg.Vaults))
	for name := range config.Cfg.Vaults {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		details := config.Cfg.Vaults[name]
		desc := fmt.Sprintf("Type: %s, Encryption: %s", details.Type, details.Encryption)
		if name == config.Cfg.ActiveVault {
			desc = "ACTIVE | " + desc
		}
		items = append(items, vaultItem{name: name, desc: desc})
	}
	v.list.SetItems(items)
}
func (v *vaultListView) showAddVaultForm() tea.Cmd {
	fields := []FormField{
		NewFormField("Vault Name (Prefix)", "My_EVM_Vault", 128, false),
		NewSelectField("Type", []string{"EVM", "COSMOS"}, 0),
		NewSelectField("Encryption", []string{"passphrase", "yubikey"}, 0),
		NewFormField("Key File Path", "my_vault.age", 128, false),
		NewFormField("Recipients File (optional)", "recipients.txt", 128, false),
	}
	form := NewForm(v.parent, "Add New Vault", fields, func(values []string) tea.Cmd {
		name, vtype, encryption, keyfile, recipientsfile := values[0], values[1], values[2], values[3], values[4]
		details := config.VaultDetails{Type: vtype, Encryption: encryption, KeyFile: keyfile, RecipientsFile: recipientsfile}
		if config.Cfg.Vaults == nil {
			config.Cfg.Vaults = make(map[string]config.VaultDetails)
		}
		config.Cfg.Vaults[name] = details
		if err := config.SaveConfig(); err != nil {
			return func() tea.Msg { return statusMsg{text: fmt.Sprintf("Save error: %v", err), isError: true} }
		}
		return tea.Sequence(
			func() tea.Msg { return popViewMsg{} },
			func() tea.Msg {
				return statusMsg{text: fmt.Sprintf("Vault '%s' added.", name), isError: false, duration: 3 * time.Second}
			},
		)
	})
	v.parent.pushView(form)
	return form.Init()
}

func (v *vaultListView) showCloneVaultForm() tea.Cmd {
	if len(config.Cfg.Vaults) == 0 {
		return func() tea.Msg { return statusMsg{text: "Нет vault для клонирования.", isError: true} }
	}
	// Выбор vault для клонирования
	vaultNames := make([]string, 0, len(config.Cfg.Vaults))
	for name := range config.Cfg.Vaults {
		vaultNames = append(vaultNames, name)
	}
	sort.Strings(vaultNames)
	// Если активный vault не выбран, не продолжаем
	if config.Cfg.ActiveVault == "" {
		return func() tea.Msg {
			return statusMsg{text: "Сначала выберите активный vault.", isError: true}
		}
	}
	// Получаем список кошельков из активного vault
	details := config.Cfg.Vaults[config.Cfg.ActiveVault]
	loadedVault, err := vault.LoadVault(details)
	if err != nil {
		return func() tea.Msg {
			return statusMsg{text: fmt.Sprintf("Ошибка загрузки vault: %v", err), isError: true}
		}
	}
	walletPrefixes := make([]string, 0, len(loadedVault))
	for prefix := range loadedVault {
		walletPrefixes = append(walletPrefixes, prefix)
	}
	sort.Strings(walletPrefixes)
	// Multi-select через строку (через запятую)
	fields := []FormField{
		NewFormField("Путь для нового vault", "cloned_vault.age", 256, false),
		NewFormField("Префиксы кошельков (через запятую)", strings.Join(walletPrefixes, ","), 512, false),
	}
	form := NewForm(v.parent, "Клонировать vault", fields, func(values []string) tea.Cmd {
		filePath := values[0]
		selected := strings.Split(values[1], ",")
		for i := range selected {
			selected[i] = strings.TrimSpace(selected[i])
		}
		clonedVault, err := actions.CloneVault(loadedVault, selected)
		if err != nil {
			return func() tea.Msg {
				return statusMsg{text: fmt.Sprintf("Ошибка клонирования: %v", err), isError: true}
			}
		}
		detailsNew := config.VaultDetails{
			KeyFile:        filePath,
			RecipientsFile: details.RecipientsFile,
			Encryption:     details.Encryption,
			Type:           details.Type,
		}
		if err := vault.SaveVault(detailsNew, clonedVault); err != nil {
			return func() tea.Msg {
				return statusMsg{text: fmt.Sprintf("Ошибка сохранения: %v", err), isError: true}
			}
		}
		return tea.Sequence(
			func() tea.Msg { return popViewMsg{} },
			func() tea.Msg {
				return statusMsg{text: "Клонирование завершено успешно", isError: false, duration: 5 * time.Second}
			},
		)
	})
	v.parent.pushView(form)
	return form.Init()
}

func (v *vaultListView) showTokenMenu() tea.Cmd {
	// Простое подменю с двумя кнопками: g (generate), s (show)
	choices := []choiceItem{
		{id: "generate", title: "Сгенерировать токен", desc: "Создать новый токен для API"},
		{id: "show", title: "Показать токен", desc: "Показать текущий токен"},
	}
	choiceView := newChoiceView(v.parent, "Управление токеном", choices, func(choiceID string) tea.Cmd {
		switch choiceID {
		case "generate":
			return v.tokenGenerateCmd()
		case "show":
			return v.tokenShowCmd()
		}
		return nil
	})
	v.parent.pushView(choiceView)
	return choiceView.Init()
}

func (v *vaultListView) tokenGenerateCmd() tea.Cmd {
	// Генерация токена (аналогично CLI)
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return func() tea.Msg { return statusMsg{text: "Ошибка генерации токена", isError: true} }
	}
	token := hex.EncodeToString(bytes)
	config.Cfg.AuthToken = token
	if err := config.SaveConfig(); err != nil {
		return func() tea.Msg {
			return statusMsg{text: "Ошибка сохранения токена", isError: true}
		}
	}
	return func() tea.Msg {
		return statusMsg{text: "Токен успешно сгенерирован и сохранён", isError: false, duration: 5 * time.Second}
	}
}
func (v *vaultListView) tokenShowCmd() tea.Cmd {
	if config.Cfg.AuthToken == "" {
		return func() tea.Msg {
			return statusMsg{text: "Токен ещё не сгенерирован.", isError: false, duration: 5 * time.Second}
		}
	}
	return func() tea.Msg {
		return statusMsg{text: "Текущий токен: " + config.Cfg.AuthToken, isError: false, duration: 10 * time.Second}
	}
}

func (v *vaultListView) showConfigForm() tea.Cmd {
	fields := []FormField{
		NewFormField("Ключ (key)", "authtoken", 64, false),
		NewFormField("Значение (value)", "", 256, false),
	}
	form := NewForm(v.parent, "Изменить конфиг", fields, func(values []string) tea.Cmd {
		key, value := values[0], values[1]
		// Сохраняем в viper и config.Cfg
		switch key {
		case "authtoken":
			config.Cfg.AuthToken = value
		case "yubikeyslot":
			config.Cfg.YubikeySlot = value
		case "active_vault":
			config.Cfg.ActiveVault = value
		// vaults вручную не редактируем через этот интерфейс
		default:
			return func() tea.Msg {
				return statusMsg{text: "Неизвестный или запрещённый ключ.", isError: true}
			}
		}
		if err := config.SaveConfig(); err != nil {
			return func() tea.Msg {
				return statusMsg{text: "Ошибка сохранения конфига", isError: true}
			}
		}
		return tea.Sequence(
			func() tea.Msg { return popViewMsg{} },
			func() tea.Msg {
				return statusMsg{text: "Конфиг успешно обновлён", isError: false, duration: 5 * time.Second}
			},
		)
	})
	v.parent.pushView(form)
	return form.Init()
}

func (v *vaultListView) showAutocompleteForm() tea.Cmd {
	fields := []FormField{
		NewFormField("Shell (bash/zsh/fish/powershell)", "bash", 32, false),
	}
	form := NewForm(v.parent, "Генерация автодополнения", fields, func(values []string) tea.Cmd {
		shell := values[0]
		var instruction string
		switch shell {
		case "bash":
			instruction = "Выполните: vault.module completion bash > ~/.bash_completion && source ~/.bash_completion"
		case "zsh":
			instruction = "Выполните: vault.module completion zsh > ~/.zshrc && source ~/.zshrc"
		case "fish":
			instruction = "Выполните: vault.module completion fish > ~/.config/fish/completions/vault.module.fish && source ~/.config/fish/completions/vault.module.fish"
		case "powershell":
			instruction = "Выполните: vault.module completion powershell > vault.module.ps1; .\vault.module.ps1"
		default:
			instruction = "Неизвестный shell. Поддерживаются: bash, zsh, fish, powershell."
		}
		return tea.Sequence(
			func() tea.Msg { return popViewMsg{} },
			func() tea.Msg { return statusMsg{text: instruction, isError: false, duration: 10 * time.Second} },
		)
	})
	v.parent.pushView(form)
	return form.Init()
}

func (v *vaultListView) showInitVaultConfirm() tea.Cmd {
	if config.Cfg.ActiveVault == "" {
		return func() tea.Msg {
			return statusMsg{text: "Нет активного vault для инициализации.", isError: true}
		}
	}
	prompt := fmt.Sprintf("Инициализировать (перезаписать) файл vault '%s'? Все данные будут удалены! (y/n)", config.Cfg.ActiveVault)
	onConfirm := func() tea.Msg {
		details := config.Cfg.Vaults[config.Cfg.ActiveVault]
		emptyVault := make(map[string]vault.Wallet)
		if err := vault.SaveVault(details, emptyVault); err != nil {
			return statusMsg{text: fmt.Sprintf("Ошибка инициализации: %v", err), isError: true}
		}
		return statusMsg{text: "Vault успешно инициализирован (файл перезаписан)", isError: false, duration: 5 * time.Second}
	}
	v.parent.pushView(newConfirmView(v.parent, prompt, onConfirm))
	return nil
}

// --- Wallet List View ---
type walletListView struct {
	parent *model
	list   list.Model
}

func newWalletListView(parent *model) *walletListView {
	v := &walletListView{parent: parent}
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = parent.styles.Selected
	delegate.Styles.NormalTitle = parent.styles.Normal
	v.list = list.New(nil, delegate, 0, 0)
	v.list.SetShowHelp(false)
	v.list.SetShowFilter(true)
	return v
}
func (v *walletListView) Init() tea.Cmd {
	v.list.Title = fmt.Sprintf("Wallets in '%s'", v.parent.activeVault.KeyFile)
	v.list.SetSize(v.parent.width-4, v.parent.height-lipgloss.Height(v.parent.headerView())-lipgloss.Height(v.parent.footerView())-2)
	v.refreshWalletList()
	return nil
}
func (v *walletListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if v.parent.loadedVault == nil || len(*v.parent.loadedVault) == 0 {
		return v, func() tea.Msg {
			return statusMsg{text: "Нет активного vault. Сначала создайте и выберите vault.", isError: true}
		}
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "enter":
			if selected, ok := v.list.SelectedItem().(walletItem); ok {
				detailView := newWalletDetailView(v.parent, selected.prefix)
				v.parent.pushView(detailView)
				return v, detailView.Init()
			}
		case "a":
			return v, v.showAddWalletChoice()
		case "d", "x":
			if selected, ok := v.list.SelectedItem().(walletItem); ok {
				prompt := fmt.Sprintf("Permanently delete wallet '%s'?", selected.prefix)
				onConfirm := func() tea.Msg {
					delete(*v.parent.loadedVault, selected.prefix)
					if cmd := v.parent.saveActiveVault(); cmd != nil {
						return cmd()
					}
					v.refreshWalletList()
					return statusMsg{text: fmt.Sprintf("Wallet '%s' deleted.", selected.prefix), isError: false, duration: 3 * time.Second}
				}
				v.parent.pushView(newConfirmView(v.parent, prompt, onConfirm))
			}
		case "r":
			if selected, ok := v.list.SelectedItem().(walletItem); ok {
				return v, v.showRenameWalletForm(selected.prefix)
			}
		case "m":
			if selected, ok := v.list.SelectedItem().(walletItem); ok {
				return v, v.showEditMetaForm(selected.prefix)
			}
		case "v": // Switch vault
			v.parent.loadedVault = nil
			// Pop wallet list and any other views on top of it, then push vault list
			v.parent.viewStack = nil
			vaultListView := newVaultListView(v.parent)
			v.parent.pushView(vaultListView)
			return v, vaultListView.Init()
		case "i":
			return v, v.showImportWalletsForm()
		case "e":
			return v, v.showExportWalletsForm()
		case "M":
			return v, v.showBatchEditNotesForm()
		}
	}
	var cmd tea.Cmd
	v.list, cmd = v.list.Update(msg)
	return v, cmd
}
func (v *walletListView) View() string {
	if v.parent.loadedVault == nil || len(*v.parent.loadedVault) == 0 {
		return v.parent.styles.Status.Render("Нет активного vault. Сначала создайте и выберите vault.")
	}
	return v.list.View()
}
func (v *walletListView) Title() string { return v.list.Title }
func (v *walletListView) Help() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
		key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
		key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "edit meta")),
		key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "switch vault")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "import wallets")),
		key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "export wallets")),
		key.NewBinding(key.WithKeys("M"), key.WithHelp("M", "batch edit notes")),
	}
}
func (v *walletListView) refreshWalletList() {
	if v.parent.loadedVault == nil {
		return
	}
	items := []list.Item{}
	prefixes := make([]string, 0, len(*v.parent.loadedVault))
	for prefix := range *v.parent.loadedVault {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	for _, prefix := range prefixes {
		wallet := (*v.parent.loadedVault)[prefix]
		desc := fmt.Sprintf("Addresses: %d", len(wallet.Addresses))
		items = append(items, walletItem{prefix: prefix, desc: desc})
	}
	v.list.SetItems(items)
}
func (v *walletListView) showAddWalletChoice() tea.Cmd {
	choices := []choiceItem{
		{id: "mnemonic", title: "From Mnemonic", desc: "Create a new wallet from a recovery phrase"},
		{id: "privatekey", title: "From Private Key", desc: "Import a single address from a private key"},
	}
	choiceView := newChoiceView(v.parent, "Select Wallet Type", choices, func(choiceID string) tea.Cmd {
		isMnemonic := choiceID == "mnemonic"
		return v.showAddWalletForm(isMnemonic)
	})
	v.parent.pushView(choiceView)
	return nil
}
func (v *walletListView) showAddWalletForm(isMnemonic bool) tea.Cmd {
	fields := []FormField{
		NewFormField("Wallet Prefix", "My_New_Wallet", 128, false).WithValidator(actions.ValidatePrefix),
		NewFormField("Secret", "...", 256, true),
	}
	form := NewForm(v.parent, "Add New Wallet", fields, func(values []string) tea.Cmd {
		prefix, secret := values[0], values[1]
		var newWallet vault.Wallet
		var err error
		if isMnemonic {
			newWallet, _, err = actions.CreateWalletFromMnemonic(secret, v.parent.activeVault.Type)
		} else {
			newWallet, _, err = actions.CreateWalletFromPrivateKey(secret, v.parent.activeVault.Type)
		}
		if err != nil {
			return func() tea.Msg { return statusMsg{text: fmt.Sprintf("Creation error: %v", err), isError: true} }
		}
		(*v.parent.loadedVault)[prefix] = newWallet
		if cmd := v.parent.saveActiveVault(); cmd != nil {
			return cmd
		}
		return tea.Sequence(
			func() tea.Msg { return popToRootMsg{} },
			func() tea.Msg {
				return statusMsg{text: fmt.Sprintf("Wallet '%s' added.", prefix), isError: false, duration: 3 * time.Second}
			},
		)
	})
	v.parent.pushView(form)
	return form.Init()
}
func (v *walletListView) showRenameWalletForm(oldPrefix string) tea.Cmd {
	fields := []FormField{NewFormField("New Prefix", oldPrefix, 128, false).WithValidator(actions.ValidatePrefix)}
	form := NewForm(v.parent, "Rename Wallet", fields, func(values []string) tea.Cmd {
		newPrefix := values[0]
		walletToRename := (*v.parent.loadedVault)[oldPrefix]
		delete(*v.parent.loadedVault, oldPrefix)
		(*v.parent.loadedVault)[newPrefix] = walletToRename
		if cmd := v.parent.saveActiveVault(); cmd != nil {
			return cmd
		}
		v.refreshWalletList()
		return tea.Sequence(
			func() tea.Msg { return popViewMsg{} },
			func() tea.Msg {
				return statusMsg{text: fmt.Sprintf("Wallet renamed to '%s'", newPrefix), isError: false, duration: 3 * time.Second}
			},
		)
	})
	v.parent.pushView(form)
	return form.Init()
}
func (v *walletListView) showEditMetaForm(prefix string) tea.Cmd {
	wallet := (*v.parent.loadedVault)[prefix]
	fields := []FormField{
		NewFormField("Notes", "Wallet notes", 256, false).WithValue(wallet.Notes),
	}
	form := NewForm(v.parent, "Edit Metadata", fields, func(values []string) tea.Cmd {
		wallet.Notes = values[0]
		(*v.parent.loadedVault)[prefix] = wallet
		if cmd := v.parent.saveActiveVault(); cmd != nil {
			return cmd
		}
		v.refreshWalletList()
		return tea.Sequence(
			func() tea.Msg { return popViewMsg{} },
			func() tea.Msg {
				return statusMsg{text: fmt.Sprintf("Metadata for '%s' updated.", prefix), isError: false, duration: 3 * time.Second}
			},
		)
	})
	v.parent.pushView(form)
	return form.Init()
}
func (v *walletListView) showImportWalletsForm() tea.Cmd {
	fields := []FormField{
		NewFormField("Путь к файлу импорта", "import.json", 256, false),
		NewFormField("Формат (json/key-value)", "json", 16, false),
		NewFormField("on-conflict (skip/overwrite/fail)", "skip", 16, false),
	}
	form := NewForm(v.parent, "Импортировать кошельки", fields, func(values []string) tea.Cmd {
		filePath, format, conflict := values[0], values[1], values[2]
		content, err := os.ReadFile(filePath)
		if err != nil {
			return func() tea.Msg {
				return statusMsg{text: fmt.Sprintf("Ошибка чтения файла: %v", err), isError: true}
			}
		}
		updatedVault, report, err := actions.ImportWallets(*v.parent.loadedVault, content, format, conflict, v.parent.activeVault.Type)
		if err != nil {
			return func() tea.Msg {
				return statusMsg{text: fmt.Sprintf("Ошибка импорта: %v", err), isError: true}
			}
		}
		*v.parent.loadedVault = updatedVault
		if cmd := v.parent.saveActiveVault(); cmd != nil {
			return cmd
		}
		v.refreshWalletList()
		return tea.Sequence(
			func() tea.Msg { return popViewMsg{} },
			func() tea.Msg {
				return statusMsg{text: fmt.Sprintf("Импорт завершён: %s", report), isError: false, duration: 5 * time.Second}
			},
		)
	})
	v.parent.pushView(form)
	return form.Init()
}
func (v *walletListView) showExportWalletsForm() tea.Cmd {
	fields := []FormField{
		NewFormField("Путь для экспорта", "export.json", 256, false),
	}
	form := NewForm(v.parent, "Экспортировать кошельки", fields, func(values []string) tea.Cmd {
		filePath := values[0]
		jsonData, err := actions.ExportVault(*v.parent.loadedVault)
		if err != nil {
			return func() tea.Msg {
				return statusMsg{text: fmt.Sprintf("Ошибка экспорта: %v", err), isError: true}
			}
		}
		if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
			return func() tea.Msg {
				return statusMsg{text: fmt.Sprintf("Ошибка записи файла: %v", err), isError: true}
			}
		}
		return tea.Sequence(
			func() tea.Msg { return popViewMsg{} },
			func() tea.Msg {
				return statusMsg{text: "Экспорт завершён успешно", isError: false, duration: 5 * time.Second}
			},
		)
	})
	v.parent.pushView(form)
	return form.Init()
}

func (v *walletListView) showBatchEditNotesForm() tea.Cmd {
	if v.parent.loadedVault == nil || len(*v.parent.loadedVault) == 0 {
		return func() tea.Msg {
			return statusMsg{text: "Нет кошельков для массового редактирования.", isError: true}
		}
	}
	// Формируем шаблон для редактирования
	var b strings.Builder
	for prefix, wallet := range *v.parent.loadedVault {
		b.WriteString(prefix + "\n" + wallet.Notes + "\n---\n")
	}
	fields := []FormField{
		NewFormField("Массовое редактирование notes (формат: prefix\nnotes\n---)", b.String(), 4096, false),
	}
	form := NewForm(v.parent, "Массовое редактирование notes", fields, func(values []string) tea.Cmd {
		input := values[0]
		blocks := strings.Split(input, "---")
		updated := false
		for _, block := range blocks {
			lines := strings.SplitN(strings.TrimSpace(block), "\n", 2)
			if len(lines) < 2 {
				continue
			}
			prefix := strings.TrimSpace(lines[0])
			notes := strings.TrimSpace(lines[1])
			wallet, ok := (*v.parent.loadedVault)[prefix]
			if ok {
				wallet.Notes = notes
				(*v.parent.loadedVault)[prefix] = wallet
				updated = true
			}
		}
		if updated {
			if cmd := v.parent.saveActiveVault(); cmd != nil {
				return cmd
			}
			v.refreshWalletList()
			return tea.Sequence(
				func() tea.Msg { return popViewMsg{} },
				func() tea.Msg {
					return statusMsg{text: "Notes обновлены для выбранных кошельков.", isError: false, duration: 5 * time.Second}
				},
			)
		}
		return tea.Sequence(
			func() tea.Msg { return popViewMsg{} },
			func() tea.Msg {
				return statusMsg{text: "Нет изменений.", isError: false, duration: 3 * time.Second}
			},
		)
	})
	v.parent.pushView(form)
	return form.Init()
}

// --- Wallet Detail View ---
type walletDetailView struct {
	parent       *model
	table        table.Model
	walletPrefix string
}

func newWalletDetailView(parent *model, prefix string) *walletDetailView {
	cols := []table.Column{{Title: "Index", Width: 5}, {Title: "Address", Width: 42}}
	tbl := table.New(table.WithColumns(cols), table.WithFocused(true))
	ts := table.DefaultStyles()
	ts.Header = parent.styles.TableHeader
	ts.Selected = parent.styles.SelectedTableRow
	tbl.SetStyles(ts)
	v := &walletDetailView{parent: parent, table: tbl, walletPrefix: prefix}
	return v
}
func (v *walletDetailView) Init() tea.Cmd {
	v.table.SetHeight(v.parent.height - lipgloss.Height(v.parent.headerView()) - lipgloss.Height(v.parent.footerView()) - 8)
	v.updateAddressTable()
	return nil
}
func (v *walletDetailView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if v.parent.loadedVault == nil || len(*v.parent.loadedVault) == 0 {
		return v, func() tea.Msg {
			return statusMsg{text: "Нет активного vault. Сначала создайте и выберите vault.", isError: true}
		}
	}
	switch msg := msg.(type) {
	case deriveCompletedMsg:
		v.parent.popView() // Pop loading view
		if msg.err != nil {
			return v, func() tea.Msg { return statusMsg{text: fmt.Sprintf("Derive error: %v", msg.err), isError: true} }
		}
		(*v.parent.loadedVault)[v.walletPrefix] = msg.w
		if cmd := v.parent.saveActiveVault(); cmd != nil {
			return v, cmd
		}
		v.updateAddressTable()
		return v, func() tea.Msg {
			return statusMsg{text: fmt.Sprintf("New address: %s", msg.newAdr.Address), isError: false, duration: 5 * time.Second}
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "n":
			wallet := (*v.parent.loadedVault)[v.walletPrefix]
			deriveCmd := func() tea.Msg {
				w, adr, err := actions.DeriveNextAddress(wallet, v.parent.activeVault.Type)
				return deriveCompletedMsg{w: w, newAdr: adr, err: err}
			}
			loading := newLoadingView(v.parent, fmt.Sprintf("Deriving new address for '%s'...", v.walletPrefix))
			v.parent.pushView(loading)
			return v, tea.Batch(loading.Init(), deriveCmd)
		case "e":
			if len(v.table.SelectedRow()) > 0 {
				return v, v.showCopyChoice()
			}
		case "c":
			if len(v.table.SelectedRow()) > 0 {
				return v, v.showCopyChoice()
			}
		}
	}
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return v, cmd
}
func (v *walletDetailView) View() string {
	if v.parent.loadedVault == nil || len(*v.parent.loadedVault) == 0 {
		return v.parent.styles.Status.Render("Нет активного vault. Сначала создайте и выберите vault.")
	}
	wallet := (*v.parent.loadedVault)[v.walletPrefix]
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Notes: %s\n", wallet.Notes))
	b.WriteString(v.table.View())
	return v.parent.styles.Bordered.Render(b.String())
}
func (v *walletDetailView) Title() string { return fmt.Sprintf("Wallet Details: %s", v.walletPrefix) }
func (v *walletDetailView) Help() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new address")),
		key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy")),
		key.NewBinding(key.WithKeys("↑/↓"), key.WithHelp("↑/↓", "navigate")),
	}
}
func (v *walletDetailView) updateAddressTable() {
	if v.parent.loadedVault == nil || len(*v.parent.loadedVault) == 0 {
		return
	}
	wallet := (*v.parent.loadedVault)[v.walletPrefix]
	rows := make([]table.Row, len(wallet.Addresses))
	for i, addr := range wallet.Addresses {
		rows[i] = table.Row{fmt.Sprintf("%d", addr.Index), addr.Address}
	}
	v.table.SetRows(rows)
}
func (v *walletDetailView) showCopyChoice() tea.Cmd {
	if v.parent.loadedVault == nil || len(*v.parent.loadedVault) == 0 {
		return func() tea.Msg {
			return statusMsg{text: "Нет активного vault. Сначала создайте и выберите vault.", isError: true}
		}
	}
	wallet := (*v.parent.loadedVault)[v.walletPrefix]
	choices := []choiceItem{{id: "address", title: "Address", desc: "Copy public address"}}
	if wallet.Mnemonic != "" {
		choices = append(choices, choiceItem{id: "mnemonic", title: "Mnemonic", desc: "Copy secret recovery phrase"})
	}
	choices = append(choices, choiceItem{id: "privatekey", title: "Private Key", desc: "Copy secret private key for this address"})

	choiceView := newChoiceView(v.parent, "Select Field to Copy", choices, func(choiceID string) tea.Cmd {
		selectedRow := v.table.SelectedRow()
		var result string
		var isSecret bool
		switch choiceID {
		case "address":
			result = selectedRow[1]
		case "privatekey":
			index, _ := strconv.Atoi(selectedRow[0])
			addressData, found := findAddressByIndex(wallet, index)
			if !found {
				return func() tea.Msg { return statusMsg{text: "Address not found", isError: true} }
			}
			result = addressData.PrivateKey
			isSecret = true
		case "mnemonic":
			result = wallet.Mnemonic
			isSecret = true
		}

		if err := clipboard.WriteAll(result); err != nil {
			return func() tea.Msg { return statusMsg{text: "Failed to copy to clipboard", isError: true} }
		}

		statusText := fmt.Sprintf("Copied '%s' to clipboard.", choiceID)
		if isSecret {
			audit.Logger.Warn("Secret data accessed via TUI", "vault", config.Cfg.ActiveVault, "prefix", v.walletPrefix, "field", choiceID)
			statusText = fmt.Sprintf("Copied secret '%s'. Clipboard will clear in %s.", choiceID, clipboardClearTimeout)
			go func(secret string) {
				time.Sleep(clipboardClearTimeout)
				currentClipboard, _ := clipboard.ReadAll()
				if currentClipboard == secret {
					clipboard.WriteAll("")
				}
			}(result)
		}
		return tea.Sequence(
			func() tea.Msg { return popViewMsg{} },
			func() tea.Msg { return statusMsg{text: statusText, isError: false, duration: 5 * time.Second} },
		)
	})
	v.parent.pushView(choiceView)
	return choiceView.Init()
}

// --- SelectListView ---
type selectListView struct {
	parent   *FormModel
	title    string
	options  []string
	selected int
	onSelect func(idx int)
	list     list.Model
}

func newSelectListView(parent *FormModel, title string, options []string, selected int, onSelect func(idx int)) *selectListView {
	items := make([]list.Item, len(options))
	for i, opt := range options {
		items[i] = selectListItem{label: opt}
	}
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 30, 10)
	l.Title = title
	l.SetShowHelp(false)
	l.Select(selected)
	return &selectListView{
		parent:   parent,
		title:    title,
		options:  options,
		selected: selected,
		onSelect: onSelect,
		list:     l,
	}
}

type selectListItem struct{ label string }

func (i selectListItem) Title() string       { return i.label }
func (i selectListItem) Description() string { return "" }
func (i selectListItem) FilterValue() string { return i.label }

func (v *selectListView) Init() tea.Cmd { return nil }
func (v *selectListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			idx := v.list.Index()
			v.onSelect(idx)
			return v.parent, nil
		case "esc":
			return v.parent, nil
		}
	}
	var cmd tea.Cmd
	v.list, cmd = v.list.Update(msg)
	return v, cmd
}
func (v *selectListView) View() string        { return v.list.View() }
func (v *selectListView) Title() string       { return v.title }
func (v *selectListView) Help() []key.Binding { return nil }

// --- Helper Functions ---
func findAddressByIndex(wallet vault.Wallet, index int) (*vault.Address, bool) {
	for i := range wallet.Addresses {
		if wallet.Addresses[i].Index == index {
			return &wallet.Addresses[i], true
		}
	}
	return nil, false
}
