// File: internal/tui/tui.go
package tui

import (
	"fmt"
	"io"
	"log/slog"
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
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Styles ---
type Styles struct {
	App, Title, Status, Error, Help, Bordered lipgloss.Style
	ListItem, ListItemSelected                lipgloss.Style
	ListFilterPrompt, ListFilterCursor        lipgloss.Style
}

func NewStyles() Styles {
	return Styles{
		App:              lipgloss.NewStyle().Margin(1, 2),
		Title:            lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5")).Background(lipgloss.Color("#25A065")).Padding(0, 1),
		Status:           lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}),
		Error:            lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5733")).Bold(true),
		Help:             lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		Bordered:         lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(1),
		ListItem:         lipgloss.NewStyle().PaddingLeft(4),
		ListItemSelected: lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170")),
		ListFilterPrompt: lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
		ListFilterCursor: lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
	}
}

// --- View State ---
type viewState int

const (
	vaultListView viewState = iota
	vaultAddFormView
	vaultConfirmDeleteView
	loadingVaultView
	walletListView
	walletDetailsView
	walletConfirmDeleteView
	walletConfirmQuitView
	walletAddFormView
	walletEditLabelView
	walletCopyView
	walletRenameView
	walletEditMetadataView
	walletCloneSelectView
	walletCloneFilenameView
	walletImportView
	walletExportView
)

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

// --- Keymaps ---
type KeyMap struct {
	Quit, Back, Enter, AddVault, DeleteVault, SwitchVault, AddWallet, DeleteWallet, RenameWallet, EditMeta, Clone, Import, Export, Derive, EditLabel, Copy, Yes, No, Select key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.Back, k.Enter, k.SwitchVault}
}
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.AddWallet, k.DeleteWallet, k.RenameWallet, k.EditMeta},
		{k.Import, k.Export, k.Clone},
		{k.Derive, k.EditLabel, k.Copy},
		{k.Select},
		{k.Yes, k.No},
		{k.SwitchVault},
		{k.AddVault, k.DeleteVault},
	}
}

var Keys = KeyMap{
	Quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Back:         key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc", "back")),
	Enter:        key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	AddVault:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add vault")),
	DeleteVault:  key.NewBinding(key.WithKeys("d", "x"), key.WithHelp("d/x", "delete vault")),
	SwitchVault:  key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "switch vault")),
	AddWallet:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add wallet")),
	DeleteWallet: key.NewBinding(key.WithKeys("d", "x"), key.WithHelp("d/x", "delete")),
	RenameWallet: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
	EditMeta:     key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "edit meta")),
	Clone:        key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "clone")),
	Import:       key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "import")),
	Export:       key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "export")),
	Derive:       key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new address")),
	EditLabel:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit label")),
	Copy:         key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy")),
	Yes:          key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yes")),
	No:           key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "no")),
	Select:       key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "select")),
}

// --- List Items ---
type vaultItem struct{ name, desc string }

func (i vaultItem) Title() string       { return i.name }
func (i vaultItem) Description() string { return i.desc }
func (i vaultItem) FilterValue() string { return i.name }

type walletItem struct{ prefix, desc string }

func (i walletItem) Title() string       { return i.prefix }
func (i walletItem) Description() string { return i.desc }
func (i walletItem) FilterValue() string { return i.prefix }

// --- Main Model ---
type model struct {
	state                viewState
	vaultList            list.Model
	walletList           list.Model
	viewport             viewport.Model
	inputs               []textinput.Model
	spinner              spinner.Model
	help                 help.Model
	keys                 KeyMap
	styles               Styles
	focusIndex           int
	loadedVault          *vault.Vault
	activeVault          config.VaultDetails
	currentPrefix        string
	prefixToDelete       string
	prefixToRename       string
	prefixToEditMetadata string
	selectedToClone      map[string]struct{}
	formError            error
	loadingMsg           string
	width, height        int
}

func NewModel() model {
	styles := NewStyles()
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.Status
	m := model{
		keys:    Keys,
		styles:  styles,
		help:    help.New(),
		spinner: s,
	}
	m.help.ShowAll = true
	return m
}

func (m *model) initLists(width, height int) {
	defaultDelegate := list.NewDefaultDelegate()
	m.vaultList = list.New([]list.Item{}, defaultDelegate, width, height)
	m.vaultList.Title = "Select a Vault to Open"
	m.vaultList.SetShowHelp(false)
	m.vaultList.Styles.Title = m.styles.Title
	m.refreshVaultList()
	m.walletList = list.New([]list.Item{}, defaultDelegate, width, height)
	m.walletList.SetShowHelp(false)
	m.walletList.Styles.Title = m.styles.Title
}

func (m *model) refreshVaultList() {
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
	m.vaultList.SetItems(items)
}

func (m *model) refreshWalletList() {
	if m.loadedVault == nil {
		return
	}
	items := []list.Item{}
	prefixes := make([]string, 0, len(*m.loadedVault))
	for prefix := range *m.loadedVault {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	for _, prefix := range prefixes {
		wallet := (*m.loadedVault)[prefix]
		items = append(items, walletItem{prefix: prefix, desc: fmt.Sprintf("Addresses: %d", len(wallet.Addresses))})
	}
	m.walletList.SetItems(items)
	m.walletList.Title = fmt.Sprintf("Wallets in '%s'", config.Cfg.ActiveVault)
}

func (m *model) setState(s viewState) {
	m.state = s
	m.formError = nil
	m.syncKeyMap()
}

func (m *model) syncKeyMap() {
	isWalletLayer := m.state >= walletListView
	m.keys.SwitchVault.SetEnabled(isWalletLayer)
	isWalletListView := m.state == walletListView
	m.keys.AddWallet.SetEnabled(isWalletListView)
	m.keys.DeleteWallet.SetEnabled(isWalletListView)
	m.keys.RenameWallet.SetEnabled(isWalletListView)
	m.keys.EditMeta.SetEnabled(isWalletListView)
	m.keys.Import.SetEnabled(isWalletListView)
	m.keys.Export.SetEnabled(isWalletListView)
	m.keys.Clone.SetEnabled(isWalletListView)
	isDetailsView := m.state == walletDetailsView
	m.keys.Derive.SetEnabled(isDetailsView && m.activeVault.Type == "EVM")
	m.keys.EditLabel.SetEnabled(isDetailsView)
	m.keys.Copy.SetEnabled(isDetailsView)
	isConfirmView := m.state == walletConfirmDeleteView || m.state == vaultConfirmDeleteView || m.state == walletConfirmQuitView
	m.keys.Yes.SetEnabled(isConfirmView)
	m.keys.No.SetEnabled(isConfirmView)
	isCloneSelectView := m.state == walletCloneSelectView
	m.keys.Select.SetEnabled(isCloneSelectView)
	isVaultListView := m.state == vaultListView
	m.keys.AddVault.SetEnabled(isVaultListView)
	m.keys.DeleteVault.SetEnabled(isVaultListView)
}

func (m *model) saveActiveVault() error {
	if m.loadedVault == nil {
		return fmt.Errorf("no vault is loaded to save")
	}
	return vault.SaveVault(m.activeVault, *m.loadedVault)
}

func loadVaultCmd(details config.VaultDetails) tea.Cmd {
	return func() tea.Msg {
		v, err := vault.LoadVault(details)
		return vaultLoadedMsg{v: v, err: err}
	}
}

func (m model) Init() tea.Cmd {
	activeVault, err := config.GetActiveVault()
	if err != nil || len(config.Cfg.Vaults) == 0 {
		m.setState(vaultListView)
		return nil
	}
	m.loadingMsg = fmt.Sprintf("Loading vault '%s'...", config.Cfg.ActiveVault)
	m.setState(loadingVaultView)
	m.activeVault = activeVault
	return tea.Batch(m.spinner.Tick, loadVaultCmd(activeVault))
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = msg.Width, msg.Height
		h, v := m.styles.App.GetFrameSize()
		listHeight := m.height - v - 5
		if m.vaultList.Items() == nil {
			m.initLists(m.width-h, listHeight)
		}
		m.vaultList.SetSize(m.width-h, listHeight)
		m.walletList.SetSize(m.width-h, listHeight)
		m.viewport.Width = m.width - h
		m.viewport.Height = listHeight
		m.help.Width = m.width - h
		return m, nil
	}
	switch msg := msg.(type) {
	case vaultLoadedMsg:
		if msg.err != nil {
			m.setState(vaultListView)
			return m, m.vaultList.NewStatusMessage(m.styles.Error.Render(fmt.Sprintf("Error: %v", msg.err)))
		}
		v := msg.v
		m.loadedVault = &v
		m.refreshWalletList()
		m.setState(walletListView)
		return m, nil
	case deriveCompletedMsg:
		m.setState(walletDetailsView)
		if msg.err != nil {
			return m, m.walletList.NewStatusMessage(m.styles.Error.Render(fmt.Sprintf("Error: %v", msg.err)))
		}
		(*m.loadedVault)[m.currentPrefix] = msg.w
		if err := m.saveActiveVault(); err != nil {
			return m, m.walletList.NewStatusMessage(m.styles.Error.Render(fmt.Sprintf("Save error: %v", err)))
		}
		m.viewport.SetContent(formatWalletDetails(msg.w.Sanitize(), m.styles, m.activeVault.Type))
		return m, m.walletList.NewStatusMessage(m.styles.Status.Render(fmt.Sprintf("New address generated: %s", msg.newAdr.Address)))
	case spinner.TickMsg:
		var cmd tea.Cmd
		if m.state == loadingVaultView {
			m.spinner, cmd = m.spinner.Update(msg)
		}
		return m, cmd
	}
	switch m.state {
	case vaultListView:
		return m.updateVaultListView(msg)
	case vaultAddFormView:
		return m.updateVaultAddFormView(msg)
	case vaultConfirmDeleteView:
		return m.updateVaultConfirmDeleteView(msg)
	case walletListView:
		return m.updateWalletListView(msg)
	case walletDetailsView:
		return m.updateWalletDetailsView(msg)
	case walletConfirmQuitView:
		return m.updateWalletConfirmQuitView(msg)
	case walletConfirmDeleteView:
		return m.updateWalletConfirmDeleteView(msg)
	case walletAddFormView:
		return m.updateWalletAddFormView(msg)
	case walletEditLabelView:
		return m.updateWalletEditLabelView(msg)
	case walletCopyView:
		return m.updateWalletCopyView(msg)
	case walletRenameView:
		return m.updateWalletRenameView(msg)
	case walletEditMetadataView:
		return m.updateWalletEditMetadataView(msg)
	case walletCloneSelectView:
		return m.updateWalletCloneSelectView(msg)
	case walletCloneFilenameView:
		return m.updateWalletCloneFilenameView(msg)
	case walletImportView:
		return m.updateWalletImportView(msg)
	case walletExportView:
		return m.updateWalletExportView(msg)
	}
	return m, nil
}

func (m model) View() string {
	if m.vaultList.Items() == nil {
		return "Initializing..."
	}
	var finalView string
	switch m.state {
	case loadingVaultView:
		finalView = fmt.Sprintf("\n   %s %s\n", m.spinner.View(), m.loadingMsg)
	case vaultListView:
		finalView = m.vaultList.View()
	case vaultAddFormView:
		finalView = m.viewVaultAddForm()
	case vaultConfirmDeleteView:
		finalView = m.styles.Bordered.Render(fmt.Sprintf("Are you sure you want to remove vault '%s' from the configuration?\nThis will NOT delete the vault file itself.\n\n(y/n)", m.prefixToDelete))
	case walletListView:
		finalView = m.walletList.View()
	case walletDetailsView:
		finalView = m.viewport.View()
	case walletConfirmQuitView:
		finalView = m.styles.Bordered.Render("Are you sure you want to quit?\n\n(y/n)")
	case walletConfirmDeleteView:
		finalView = m.styles.Bordered.Render(fmt.Sprintf("Are you sure you want to delete wallet '%s'?\n\n(y/n)", m.prefixToDelete))
	case walletAddFormView:
		finalView = m.viewWalletAddForm()
	case walletEditLabelView:
		finalView = m.viewWalletEditLabelForm()
	case walletCopyView:
		finalView = m.viewWalletCopyForm()
	case walletRenameView:
		finalView = m.viewWalletRenameForm()
	case walletEditMetadataView:
		finalView = m.viewWalletEditMetadataForm()
	case walletCloneSelectView:
		finalView = m.walletList.View()
	case walletCloneFilenameView:
		finalView = m.viewWalletCloneFilenameForm()
	case walletImportView:
		finalView = m.viewWalletImportForm()
	case walletExportView:
		finalView = m.viewWalletExportForm()
	default:
		finalView = m.vaultList.View()
	}
	helpView := m.help.View(m.keys)
	return m.styles.App.Render(finalView + "\n\n" + m.styles.Help.Render(helpView))
}
func (m model) updateVaultListView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.vaultList.FilterState() == list.Filtering {
			break
		}
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Enter):
			selected, ok := m.vaultList.SelectedItem().(vaultItem)
			if !ok {
				return m, nil
			}
			config.Cfg.ActiveVault = selected.name
			if err := config.SaveConfig(); err != nil {
				return m, m.vaultList.NewStatusMessage(m.styles.Error.Render("Failed to save config"))
			}
			details := config.Cfg.Vaults[selected.name]
			m.activeVault = details
			m.loadingMsg = fmt.Sprintf("Loading vault '%s'...", selected.name)
			m.setState(loadingVaultView)
			return m, tea.Batch(m.spinner.Tick, loadVaultCmd(details))
		case key.Matches(msg, m.keys.AddVault):
			m.setState(vaultAddFormView)
			m.setupVaultAddForm()
			return m, m.inputs[0].Focus()
		case key.Matches(msg, m.keys.DeleteVault):
			if selected, ok := m.vaultList.SelectedItem().(vaultItem); ok {
				m.prefixToDelete = selected.name
				m.setState(vaultConfirmDeleteView)
			}
		}
	}
	var cmd tea.Cmd
	m.vaultList, cmd = m.vaultList.Update(msg)
	return m, cmd
}

func (m model) updateWalletListView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.walletList.FilterState() == list.Filtering {
			break
		}
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.setState(walletConfirmQuitView)
			return m, nil
		case key.Matches(msg, m.keys.SwitchVault):
			m.loadedVault = nil
			m.setState(vaultListView)
			m.refreshVaultList()
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			selected, ok := m.walletList.SelectedItem().(walletItem)
			if ok {
				m.currentPrefix = selected.prefix
				wallet := (*m.loadedVault)[m.currentPrefix]
				m.viewport.SetContent(formatWalletDetails(wallet.Sanitize(), m.styles, m.activeVault.Type))
				m.setState(walletDetailsView)
			}
		case key.Matches(msg, m.keys.DeleteWallet):
			if selected, ok := m.walletList.SelectedItem().(walletItem); ok {
				m.prefixToDelete = selected.prefix
				m.setState(walletConfirmDeleteView)
			}
		case key.Matches(msg, m.keys.AddWallet):
			m.setState(walletAddFormView)
			m.setupWalletAddForm()
			return m, m.inputs[0].Focus()
		case key.Matches(msg, m.keys.RenameWallet):
			if selected, ok := m.walletList.SelectedItem().(walletItem); ok {
				m.prefixToRename = selected.prefix
				m.setState(walletRenameView)
				m.setupWalletRenameForm()
				return m, m.inputs[0].Focus()
			}
		case key.Matches(msg, m.keys.EditMeta):
			if selected, ok := m.walletList.SelectedItem().(walletItem); ok {
				m.prefixToEditMetadata = selected.prefix
				m.setState(walletEditMetadataView)
				m.setupWalletEditMetadataForm()
				return m, m.inputs[0].Focus()
			}
		case key.Matches(msg, m.keys.Clone):
			m.setState(walletCloneSelectView)
			m.setupWalletCloneSelect()
			return m, nil
		case key.Matches(msg, m.keys.Import):
			m.setState(walletImportView)
			m.setupWalletImportForm()
			return m, m.inputs[0].Focus()
		case key.Matches(msg, m.keys.Export):
			m.setState(walletExportView)
			m.setupWalletExportForm()
			return m, m.inputs[0].Focus()
		}
	}
	var cmd tea.Cmd
	m.walletList, cmd = m.walletList.Update(msg)
	return m, cmd
}

func (m model) updateWalletDetailsView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Back):
			m.setState(walletListView)
		case key.Matches(msg, m.keys.SwitchVault):
			m.loadedVault = nil
			m.setState(vaultListView)
			m.refreshVaultList()
			return m, nil
		case key.Matches(msg, m.keys.Derive):
			if !m.keys.Derive.Enabled() {
				return m, nil
			}
			m.loadingMsg = fmt.Sprintf("Deriving new address for '%s'...", m.currentPrefix)
			m.setState(loadingVaultView)
			wallet := (*m.loadedVault)[m.currentPrefix]
			deriveCmd := func() tea.Msg {
				w, adr, err := actions.DeriveNextAddress(wallet)
				return deriveCompletedMsg{w: w, newAdr: adr, err: err}
			}
			return m, tea.Batch(m.spinner.Tick, deriveCmd)
		case key.Matches(msg, m.keys.EditLabel):
			m.setState(walletEditLabelView)
			m.setupWalletEditLabelForm()
			return m, m.inputs[0].Focus()
		case key.Matches(msg, m.keys.Copy):
			m.setState(walletCopyView)
			m.setupWalletCopyForm()
			return m, m.inputs[0].Focus()
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m model) updateWalletConfirmQuitView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Yes):
			return m, tea.Quit
		case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Back):
			m.setState(walletListView)
		}
	}
	return m, nil
}

func (m model) updateWalletConfirmDeleteView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Yes):
			audit.Logger.Warn("Attempting wallet deletion via TUI",
				slog.String("vault", config.Cfg.ActiveVault),
				slog.String("prefix", m.prefixToDelete),
			)
			delete(*m.loadedVault, m.prefixToDelete)
			if err := m.saveActiveVault(); err != nil {
				audit.Logger.Error("Failed to save vault after TUI deletion", "error", err.Error(), "prefix", m.prefixToDelete)
				return m, m.walletList.NewStatusMessage(m.styles.Error.Render(fmt.Sprintf("Save error: %v", err)))
			}
			m.refreshWalletList()
			m.setState(walletListView)
			audit.Logger.Info("Wallet deleted successfully via TUI", "prefix", m.prefixToDelete, "vault", config.Cfg.ActiveVault)
			return m, m.walletList.NewStatusMessage(m.styles.Status.Render(fmt.Sprintf("✅ Wallet '%s' deleted.", m.prefixToDelete)))
		case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Back):
			m.setState(walletListView)
		}
	}
	return m, nil
}

// --- Form Logic: Add Vault ---

func (m *model) setupVaultAddForm() {
	m.inputs = make([]textinput.Model, 5)
	prompts := []string{"Vault Name (Prefix)", "Type (e.g., EVM)", "Encryption (passphrase or yubikey)", "Key File Path (e.g., my_vault.age)", "Recipients File (if yubikey)"}
	placeholders := []string{"My_EVM_Vault", "EVM", "passphrase", "my_vault.age", "recipients.txt"}
	for i := range m.inputs {
		t := textinput.New()
		t.Prompt = prompts[i] + ": "
		t.Placeholder = placeholders[i]
		t.CharLimit = 128
		t.Width = 60
		m.inputs[i] = t
	}
	m.inputs[0].Focus()
	m.focusIndex = 0
}

func (m model) updateVaultAddFormView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.setState(vaultListView)
			return m, nil
		case "tab", "shift+tab", "up", "down", "enter":
			s := msg.String()
			if s == "enter" && m.focusIndex == len(m.inputs)-1 {
				name, vtype, encryption, keyfile, recipientsfile := m.inputs[0].Value(), m.inputs[1].Value(), m.inputs[2].Value(), m.inputs[3].Value(), m.inputs[4].Value()

				if name == "" || vtype == "" || encryption == "" || keyfile == "" {
					m.formError = fmt.Errorf("name, type, encryption, and keyfile cannot be empty")
					return m, nil
				}
				if _, exists := config.Cfg.Vaults[name]; exists {
					m.formError = fmt.Errorf("a vault with name '%s' already exists", name)
					return m, nil
				}

				details := config.VaultDetails{
					Type:           vtype,
					Encryption:     encryption,
					KeyFile:        keyfile,
					RecipientsFile: recipientsfile,
				}

				if config.Cfg.Vaults == nil {
					config.Cfg.Vaults = make(map[string]config.VaultDetails)
				}
				config.Cfg.Vaults[name] = details
				if err := config.SaveConfig(); err != nil {
					m.formError = fmt.Errorf("could not save config: %w", err)
					return m, nil
				}

				m.refreshVaultList()
				m.setState(vaultListView)
				return m, m.vaultList.NewStatusMessage(m.styles.Status.Render(fmt.Sprintf("✅ Vault '%s' added to configuration.", name)))
			}
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}
			if m.focusIndex > len(m.inputs)-1 {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs) - 1
			}
			for i := 0; i < len(m.inputs); i++ {
				if i == m.focusIndex {
					cmds = append(cmds, m.inputs[i].Focus())
				} else {
					m.inputs[i].Blur()
				}
			}
			return m, tea.Batch(cmds...)
		}
	}
	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m model) viewVaultAddForm() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render("Add New Vault to Configuration") + "\n\n")
	for i := range m.inputs {
		b.WriteString(m.inputs[i].View() + "\n")
	}
	if m.formError != nil {
		b.WriteString("\n" + m.styles.Error.Render(m.formError.Error()))
	}
	b.WriteString("\n(enter to submit, esc to cancel)")
	return m.styles.Bordered.Render(b.String())
}

// --- Confirmation Logic: Delete Vault ---

func (m model) updateVaultConfirmDeleteView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Yes):
			if _, exists := config.Cfg.Vaults[m.prefixToDelete]; !exists {
				return m, m.vaultList.NewStatusMessage(m.styles.Error.Render("Vault not found in config"))
			}

			delete(config.Cfg.Vaults, m.prefixToDelete)
			if config.Cfg.ActiveVault == m.prefixToDelete {
				config.Cfg.ActiveVault = ""
			}

			if err := config.SaveConfig(); err != nil {
				return m, m.vaultList.NewStatusMessage(m.styles.Error.Render(fmt.Sprintf("Delete error (config): %v", err)))
			}

			m.refreshVaultList()
			m.setState(vaultListView)
			return m, m.vaultList.NewStatusMessage(m.styles.Status.Render(fmt.Sprintf("✅ Vault '%s' removed from configuration.", m.prefixToDelete)))
		case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Back):
			m.setState(vaultListView)
		}
	}
	return m, nil
}

// --- Form Logic: Add Wallet ---
func (m *model) setupWalletAddForm() {
	m.inputs = make([]textinput.Model, 3)
	prompts := []string{"Prefix", "Type (1=Mnemonic, 2=Private Key)", "Secret"}
	placeholders := []string{"My_New_Wallet", "1", "..."}
	for i := range m.inputs {
		t := textinput.New()
		t.Prompt = prompts[i] + ": "
		t.Placeholder = placeholders[i]
		t.CharLimit = 128
		t.Width = 50
		m.inputs[i] = t
	}
	m.inputs[2].EchoMode = textinput.EchoPassword
	m.inputs[0].Focus()
	m.focusIndex = 0
}

func (m model) updateWalletAddFormView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.setState(walletListView)
			return m, nil
		case "tab", "shift+tab", "up", "down", "enter":
			s := msg.String()
			if s == "enter" && m.focusIndex == len(m.inputs)-1 {
				prefix := m.inputs[0].Value()
				choice := m.inputs[1].Value()
				secret := m.inputs[2].Value()
				if err := actions.ValidatePrefix(prefix); err != nil {
					m.formError = err
					return m, nil
				}
				if _, exists := (*m.loadedVault)[prefix]; exists {
					m.formError = fmt.Errorf("wallet with prefix '%s' already exists", prefix)
					return m, nil
				}
				var newWallet vault.Wallet
				var err error
				switch choice {
				case "1":
					newWallet, _, err = actions.CreateWalletFromMnemonic(secret)
				case "2":
					newWallet, _, err = actions.CreateWalletFromPrivateKey(secret)
				default:
					err = fmt.Errorf("invalid type: must be 1 or 2")
				}
				if err != nil {
					m.formError = err
					return m, nil
				}
				(*m.loadedVault)[prefix] = newWallet
				if err := m.saveActiveVault(); err != nil {
					m.formError = err
					return m, nil
				}
				m.refreshWalletList()
				m.setState(walletListView)
				return m, m.walletList.NewStatusMessage(m.styles.Status.Render(fmt.Sprintf("✅ Wallet '%s' added.", prefix)))
			}
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}
			if m.focusIndex > len(m.inputs)-1 {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs) - 1
			}
			for i := 0; i < len(m.inputs); i++ {
				if i == m.focusIndex {
					cmds = append(cmds, m.inputs[i].Focus())
				} else {
					m.inputs[i].Blur()
				}
			}
			return m, tea.Batch(cmds...)
		}
	}
	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m model) viewWalletAddForm() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render("Add New Wallet") + "\n\n")
	for i := range m.inputs {
		b.WriteString(m.inputs[i].View() + "\n")
	}
	if m.formError != nil {
		b.WriteString("\n" + m.styles.Error.Render(m.formError.Error()))
	}
	b.WriteString("\n(enter to submit, esc to cancel)")
	return m.styles.Bordered.Render(b.String())
}

// --- Form Logic: Edit Label ---
func (m *model) setupWalletEditLabelForm() {
	m.inputs = make([]textinput.Model, 2)
	prompts := []string{"Address Index", "New Label"}
	placeholders := []string{"0", "My new label"}
	for i := range m.inputs {
		t := textinput.New()
		t.Prompt = prompts[i] + ": "
		t.Placeholder = placeholders[i]
		t.CharLimit = 128
		t.Width = 50
		m.inputs[i] = t
	}
	m.inputs[0].Focus()
	m.focusIndex = 0
}

func (m model) updateWalletEditLabelView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.setState(walletDetailsView)
			return m, nil
		case "tab", "shift+tab", "up", "down", "enter":
			s := msg.String()
			if s == "enter" && m.focusIndex == len(m.inputs)-1 {
				indexStr := m.inputs[0].Value()
				newLabel := m.inputs[1].Value()
				index, err := strconv.Atoi(indexStr)
				if err != nil {
					m.formError = fmt.Errorf("index must be a number")
					return m, nil
				}
				wallet := (*m.loadedVault)[m.currentPrefix]
				var addressToUpdate *vault.Address
				for i := range wallet.Addresses {
					if wallet.Addresses[i].Index == index {
						addressToUpdate = &wallet.Addresses[i]
						break
					}
				}
				if addressToUpdate == nil {
					m.formError = fmt.Errorf("address with index %d not found", index)
					return m, nil
				}
				addressToUpdate.Label = newLabel
				(*m.loadedVault)[m.currentPrefix] = wallet
				if err := m.saveActiveVault(); err != nil {
					m.formError = err
					return m, nil
				}
				m.viewport.SetContent(formatWalletDetails(wallet.Sanitize(), m.styles, m.activeVault.Type))
				m.setState(walletDetailsView)
				return m, m.walletList.NewStatusMessage(m.styles.Status.Render(fmt.Sprintf("✅ Label for address %d updated.", index)))
			}
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}
			if m.focusIndex > len(m.inputs)-1 {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs) - 1
			}
			for i := 0; i < len(m.inputs); i++ {
				if i == m.focusIndex {
					cmds = append(cmds, m.inputs[i].Focus())
				} else {
					m.inputs[i].Blur()
				}
			}
			return m, tea.Batch(cmds...)
		}
	}
	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m model) viewWalletEditLabelForm() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render("Edit Address Label") + "\n\n")
	for i := range m.inputs {
		b.WriteString(m.inputs[i].View() + "\n")
	}
	if m.formError != nil {
		b.WriteString("\n" + m.styles.Error.Render(m.formError.Error()))
	}
	b.WriteString("\n(enter to submit, esc to cancel)")
	return m.styles.Bordered.Render(b.String())
}

// --- Form Logic: Copy ---
func (m *model) setupWalletCopyForm() {
	m.inputs = make([]textinput.Model, 2)
	prompts := []string{"Address Index", "Field (address, privatekey, mnemonic)"}
	placeholders := []string{"0", "privatekey"}
	for i := range m.inputs {
		t := textinput.New()
		t.Prompt = prompts[i] + ": "
		t.Placeholder = placeholders[i]
		t.CharLimit = 128
		t.Width = 50
		m.inputs[i] = t
	}
	m.inputs[0].Focus()
	m.focusIndex = 0
}

func (m model) updateWalletCopyView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.setState(walletDetailsView)
			return m, nil
		case "tab", "shift+tab", "up", "down", "enter":
			s := msg.String()
			if s == "enter" && m.focusIndex == len(m.inputs)-1 {
				indexStr := m.inputs[0].Value()
				field := strings.ToLower(m.inputs[1].Value())
				index, err := strconv.Atoi(indexStr)
				if err != nil && field != "mnemonic" {
					m.formError = fmt.Errorf("index must be a number")
					return m, nil
				}
				wallet := (*m.loadedVault)[m.currentPrefix]
				var result string
				var isSecret bool
				switch field {
				case "address":
					addressData, found := findAddressByIndex(wallet, index)
					if !found {
						m.formError = fmt.Errorf("address with index %d not found", index)
						return m, nil
					}
					result = addressData.Address
					isSecret = false
				case "privatekey":
					addressData, found := findAddressByIndex(wallet, index)
					if !found {
						m.formError = fmt.Errorf("address with index %d not found", index)
						return m, nil
					}
					result = addressData.PrivateKey
					isSecret = true
				case "mnemonic":
					if wallet.Mnemonic == "" {
						m.formError = fmt.Errorf("wallet '%s' does not have a mnemonic", m.currentPrefix)
						return m, nil
					}
					result = wallet.Mnemonic
					isSecret = true
				default:
					m.formError = fmt.Errorf("invalid field: use 'address', 'privatekey', or 'mnemonic'")
					return m, nil
				}
				if err := clipboard.WriteAll(result); err != nil {
					m.formError = fmt.Errorf("failed to copy to clipboard: %w", err)
					return m, nil
				}
				var statusMessage string
				if isSecret {
					audit.Logger.Warn("Secret data accessed via TUI",
						slog.String("vault", config.Cfg.ActiveVault),
						slog.String("prefix", m.currentPrefix),
						slog.String("field", field),
					)
					statusMessage = fmt.Sprintf("✅ Secret '%s' copied. Clipboard will be cleared in 30s.", field)
					go func() {
						time.Sleep(30 * time.Second)
						currentClipboard, _ := clipboard.ReadAll()
						if currentClipboard == result {
							clipboard.WriteAll("")
						}
					}()
				} else {
					statusMessage = fmt.Sprintf("✅ Field '%s' for address %d copied.", field, index)
				}
				m.setState(walletDetailsView)
				return m, m.walletList.NewStatusMessage(m.styles.Status.Render(statusMessage))
			}
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}
			if m.focusIndex > len(m.inputs)-1 {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs) - 1
			}
			for i := 0; i < len(m.inputs); i++ {
				if i == m.focusIndex {
					cmds = append(cmds, m.inputs[i].Focus())
				} else {
					m.inputs[i].Blur()
				}
			}
			return m, tea.Batch(cmds...)
		}
	}
	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m model) viewWalletCopyForm() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render("Copy Data to Clipboard") + "\n\n")
	for i := range m.inputs {
		b.WriteString(m.inputs[i].View() + "\n")
	}
	if m.formError != nil {
		b.WriteString("\n" + m.styles.Error.Render(m.formError.Error()))
	}
	b.WriteString("\n(enter to submit, esc to cancel)")
	return m.styles.Bordered.Render(b.String())
}

// --- Form Logic: Rename ---
func (m *model) setupWalletRenameForm() {
	m.inputs = make([]textinput.Model, 1)
	t := textinput.New()
	t.Prompt = "New prefix: "
	t.Placeholder = "My_Renamed_Wallet"
	t.Focus()
	t.CharLimit = 32
	t.Width = 50
	m.inputs[0] = t
}

func (m model) updateWalletRenameView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.setState(walletListView)
			return m, nil
		case "enter":
			newPrefix := m.inputs[0].Value()
			if err := actions.ValidatePrefix(newPrefix); err != nil {
				m.formError = err
				return m, nil
			}
			if _, exists := (*m.loadedVault)[newPrefix]; exists {
				m.formError = fmt.Errorf("wallet with prefix '%s' already exists", newPrefix)
				return m, nil
			}
			walletToRename := (*m.loadedVault)[m.prefixToRename]
			delete(*m.loadedVault, m.prefixToRename)
			(*m.loadedVault)[newPrefix] = walletToRename
			if err := m.saveActiveVault(); err != nil {
				m.formError = err
				return m, nil
			}
			m.refreshWalletList()
			m.setState(walletListView)
			statusCmd := m.walletList.NewStatusMessage(m.styles.Status.Render(fmt.Sprintf("✅ Wallet '%s' renamed to '%s'", m.prefixToRename, newPrefix)))
			return m, statusCmd
		}
	}
	m.inputs[0], cmd = m.inputs[0].Update(msg)
	return m, cmd
}

func (m model) viewWalletRenameForm() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render(fmt.Sprintf("Renaming '%s'", m.prefixToRename)) + "\n\n")
	b.WriteString(m.inputs[0].View())
	if m.formError != nil {
		b.WriteString("\n\n" + m.styles.Error.Render(m.formError.Error()))
	}
	b.WriteString("\n\n(enter to confirm, esc to cancel)")
	return m.styles.Bordered.Render(b.String())
}

// --- Form Logic: Edit Metadata ---
func (m *model) setupWalletEditMetadataForm() {
	m.inputs = make([]textinput.Model, 2)
	wallet := (*m.loadedVault)[m.prefixToEditMetadata]
	prompts := []string{"Notes", "Tags (comma separated)"}
	initialValues := []string{wallet.Notes, strings.Join(wallet.Tags, ", ")}
	for i := range m.inputs {
		t := textinput.New()
		t.Prompt = prompts[i] + ": "
		t.Placeholder = prompts[i]
		t.SetValue(initialValues[i])
		t.CharLimit = 256
		t.Width = 80
		m.inputs[i] = t
	}
	m.inputs[0].Focus()
	m.focusIndex = 0
}

func (m model) updateWalletEditMetadataView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.setState(walletListView)
			return m, nil
		case "tab", "shift+tab", "up", "down", "enter":
			s := msg.String()
			if s == "enter" && m.focusIndex == len(m.inputs)-1 {
				wallet := (*m.loadedVault)[m.prefixToEditMetadata]
				wallet.Notes = m.inputs[0].Value()
				tagsStr := m.inputs[1].Value()
				if tagsStr == "" {
					wallet.Tags = []string{}
				} else {
					tags := strings.Split(tagsStr, ",")
					for i, tag := range tags {
						tags[i] = strings.TrimSpace(tag)
					}
					wallet.Tags = tags
				}
				(*m.loadedVault)[m.prefixToEditMetadata] = wallet
				if err := m.saveActiveVault(); err != nil {
					m.formError = err
					return m, nil
				}
				m.refreshWalletList()
				m.setState(walletListView)
				statusCmd := m.walletList.NewStatusMessage(m.styles.Status.Render(fmt.Sprintf("✅ Metadata for '%s' updated.", m.prefixToEditMetadata)))
				return m, statusCmd
			}
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}
			if m.focusIndex > len(m.inputs)-1 {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs) - 1
			}
			for i := 0; i < len(m.inputs); i++ {
				if i == m.focusIndex {
					cmds = append(cmds, m.inputs[i].Focus())
				} else {
					m.inputs[i].Blur()
				}
			}
			return m, tea.Batch(cmds...)
		}
	}
	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m model) viewWalletEditMetadataForm() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render(fmt.Sprintf("Editing metadata for '%s'", m.prefixToEditMetadata)) + "\n\n")
	for i := range m.inputs {
		b.WriteString(m.inputs[i].View() + "\n")
	}
	if m.formError != nil {
		b.WriteString("\n\n" + m.styles.Error.Render(m.formError.Error()))
	}
	b.WriteString("\n\n(enter to submit, esc to cancel)")
	return m.styles.Bordered.Render(b.String())
}

// --- Clone Logic ---
type cloneDelegate struct{ model *model }

func (d *cloneDelegate) Height() int                               { return 1 }
func (d *cloneDelegate) Spacing() int                              { return 0 }
func (d *cloneDelegate) Update(msg tea.Msg, l *list.Model) tea.Cmd { return nil }
func (d *cloneDelegate) Render(w io.Writer, l list.Model, index int, listItem list.Item) {
	i, ok := listItem.(walletItem)
	if !ok {
		return
	}
	str := i.Title()
	if _, ok := d.model.selectedToClone[str]; ok {
		str = "[x] " + str
	} else {
		str = "[ ] " + str
	}
	fn := d.model.styles.ListItem.Render
	if index == l.Index() {
		fn = func(s ...string) string { return d.model.styles.ListItemSelected.Render(s...) }
	}
	fmt.Fprint(w, fn(str))
}

func (m *model) setupWalletCloneSelect() {
	m.selectedToClone = make(map[string]struct{})
	delegate := &cloneDelegate{model: m}
	m.walletList.SetDelegate(delegate)
	m.walletList.Title = "Select wallets to clone (space to select, enter to confirm)"
}

func (m model) updateWalletCloneSelectView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Back):
			m.resetToDefaultWalletList()
			m.setState(walletListView)
		case key.Matches(msg, m.keys.Enter):
			if len(m.selectedToClone) > 0 {
				m.setState(walletCloneFilenameView)
				m.setupWalletCloneFilenameForm()
				return m, m.inputs[0].Focus()
			}
		case key.Matches(msg, m.keys.Select):
			if i, ok := m.walletList.SelectedItem().(walletItem); ok {
				title := i.Title()
				if _, ok := m.selectedToClone[title]; ok {
					delete(m.selectedToClone, title)
				} else {
					m.selectedToClone[title] = struct{}{}
				}
			}
		}
	}
	var cmd tea.Cmd
	m.walletList, cmd = m.walletList.Update(msg)
	return m, cmd
}

func (m *model) resetToDefaultWalletList() {
	defaultDelegate := list.NewDefaultDelegate()
	m.walletList.SetDelegate(defaultDelegate)
	m.walletList.Title = fmt.Sprintf("Wallets in '%s'", config.Cfg.ActiveVault)
}

func (m *model) setupWalletCloneFilenameForm() {
	m.inputs = make([]textinput.Model, 1)
	t := textinput.New()
	t.Prompt = "Filename for the new vault: "
	t.Placeholder = "bot_vault.age"
	t.Focus()
	t.CharLimit = 128
	t.Width = 50
	m.inputs[0] = t
}

func (m model) updateWalletCloneFilenameView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.resetToDefaultWalletList()
			m.setState(walletListView)
			return m, nil
		case "enter":
			outputFile := m.inputs[0].Value()
			if outputFile == "" {
				m.formError = fmt.Errorf("filename cannot be empty")
				return m, nil
			}
			prefixes := make([]string, 0, len(m.selectedToClone))
			for p := range m.selectedToClone {
				prefixes = append(prefixes, p)
			}
			clonedVault, err := actions.CloneVault(*m.loadedVault, prefixes)
			if err != nil {
				m.formError = err
				return m, nil
			}
			cloneDetails := m.activeVault
			cloneDetails.KeyFile = outputFile
			if err := vault.SaveVault(cloneDetails, clonedVault); err != nil {
				m.formError = err
				return m, nil
			}
			m.resetToDefaultWalletList()
			m.setState(walletListView)
			statusCmd := m.walletList.NewStatusMessage(m.styles.Status.Render(fmt.Sprintf("✅ Cloned vault '%s' created.", outputFile)))
			return m, statusCmd
		}
	}
	m.inputs[0], cmd = m.inputs[0].Update(msg)
	return m, cmd
}

func (m model) viewWalletCloneFilenameForm() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render("Create Cloned Vault") + "\n\n")
	b.WriteString(m.inputs[0].View())
	if m.formError != nil {
		b.WriteString("\n\n" + m.styles.Error.Render(m.formError.Error()))
	}
	b.WriteString("\n\n(enter to confirm, esc to cancel)")
	return m.styles.Bordered.Render(b.String())
}

// --- Import/Export Logic ---
func (m *model) setupWalletImportForm() {
	m.inputs = make([]textinput.Model, 3)
	prompts := []string{"Path to file", "Format (json or key-value)", "On Conflict (skip, overwrite, fail)"}
	placeholders := []string{"import.json", "json", "skip"}
	for i := range m.inputs {
		t := textinput.New()
		t.Prompt = prompts[i] + ": "
		t.Placeholder = placeholders[i]
		t.CharLimit = 128
		t.Width = 50
		m.inputs[i] = t
	}
	m.inputs[0].Focus()
	m.focusIndex = 0
}

func (m model) updateWalletImportView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.setState(walletListView)
			return m, nil
		case "tab", "shift+tab", "up", "down", "enter":
			s := msg.String()
			if s == "enter" && m.focusIndex == len(m.inputs)-1 {
				filePath, format, conflictPolicy := m.inputs[0].Value(), m.inputs[1].Value(), m.inputs[2].Value()
				if filePath == "" {
					m.formError = fmt.Errorf("file path cannot be empty")
					return m, nil
				}
				content, err := os.ReadFile(filePath)
				if err != nil {
					m.formError = fmt.Errorf("could not read file: %w", err)
					return m, nil
				}
				updatedVault, report, err := actions.ImportWallets(*m.loadedVault, content, format, conflictPolicy)
				if err != nil {
					m.formError = err
					return m, nil
				}
				if err := vault.SaveVault(m.activeVault, updatedVault); err != nil {
					m.formError = err
					return m, nil
				}
				*m.loadedVault = updatedVault
				m.refreshWalletList()
				m.setState(walletListView)
				return m, m.walletList.NewStatusMessage(m.styles.Status.Render(report))
			}
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}
			if m.focusIndex > len(m.inputs)-1 {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs) - 1
			}
			for i := 0; i < len(m.inputs); i++ {
				if i == m.focusIndex {
					cmds = append(cmds, m.inputs[i].Focus())
				} else {
					m.inputs[i].Blur()
				}
			}
			return m, tea.Batch(cmds...)
		}
	}
	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m model) viewWalletImportForm() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render("Import Wallets From File") + "\n\n")
	for i := range m.inputs {
		b.WriteString(m.inputs[i].View() + "\n")
	}
	if m.formError != nil {
		b.WriteString("\n" + m.styles.Error.Render(m.formError.Error()))
	}
	b.WriteString("\n(enter to submit, esc to cancel)")
	return m.styles.Bordered.Render(b.String())
}

func (m *model) setupWalletExportForm() {
	m.inputs = make([]textinput.Model, 1)
	t := textinput.New()
	t.Prompt = "Path for export file: "
	t.Placeholder = "export.json"
	t.Focus()
	t.CharLimit = 128
	t.Width = 50
	m.inputs[0] = t
}

func (m model) updateWalletExportView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.setState(walletListView)
			return m, nil
		case "enter":
			filePath := m.inputs[0].Value()
			if filePath == "" {
				m.formError = fmt.Errorf("filename cannot be empty")
				return m, nil
			}
			audit.Logger.Error("Executing plaintext export of an entire vault via TUI",
				slog.String("vault", config.Cfg.ActiveVault),
				slog.String("destination_file", filePath),
			)
			jsonData, err := actions.ExportVault(*m.loadedVault)
			if err != nil {
				m.formError = fmt.Errorf("failed to export data: %w", err)
				return m, nil
			}
			if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
				m.formError = fmt.Errorf("failed to write to file: %w", err)
				return m, nil
			}
			m.setState(walletListView)
			statusCmd := m.walletList.NewStatusMessage(m.styles.Status.Render(fmt.Sprintf("✅ Vault successfully exported to '%s'", filePath)))
			return m, statusCmd
		}
	}
	m.inputs[0], cmd = m.inputs[0].Update(msg)
	return m, cmd
}

func (m model) viewWalletExportForm() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render("Export Vault to Unencrypted JSON") + "\n\n")
	b.WriteString(m.inputs[0].View())
	if m.formError != nil {
		b.WriteString("\n\n" + m.styles.Error.Render(m.formError.Error()))
	}
	b.WriteString("\n\n(WARNING: The file will not be encrypted!)\n(enter to confirm, esc to cancel)")
	return m.styles.Bordered.Render(b.String())
}

// --- Helper Functions ---
func formatWalletDetails(wallet vault.Wallet, s Styles, vaultType string) string {
	var b strings.Builder
	b.WriteString(s.Title.Render(fmt.Sprintf("Wallet Details (Type: %s)", vaultType)) + "\n\n")
	if wallet.Mnemonic != "" {
		b.WriteString(fmt.Sprintf("Mnemonic: %s\n", wallet.Mnemonic))
	}
	if wallet.DerivationPath != "" {
		b.WriteString(fmt.Sprintf("Derivation Path: %s\n", wallet.DerivationPath))
	}
	b.WriteString("\n--- Addresses ---\n")
	for _, addr := range wallet.Addresses {
		b.WriteString(fmt.Sprintf("  Index: %d\n", addr.Index))
		b.WriteString(fmt.Sprintf("  Label: %s\n", addr.Label))
		b.WriteString(fmt.Sprintf("  Address: %s\n", addr.Address))
		b.WriteString(fmt.Sprintf("  Private Key: %s\n", addr.PrivateKey))
		b.WriteString("  ----------\n")
	}
	return b.String()
}

func findAddressByIndex(wallet vault.Wallet, index int) (*vault.Address, bool) {
	for i := range wallet.Addresses {
		if wallet.Addresses[i].Index == index {
			return &wallet.Addresses[i], true
		}
	}
	return nil, false
}

// StartTUI is the entry point for the TUI application.
func StartTUI() {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error starting TUI:", err)
		os.Exit(1)
	}
}
