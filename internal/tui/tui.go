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
	"vault.module/internal/constants"
	"vault.module/internal/vault"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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

func newStyles() Styles {
	s := Styles{}
	s.App = lipgloss.NewStyle().Margin(1, 2)
	s.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5")).Background(lipgloss.Color("#25A065")).Padding(0, 1)
	s.Status = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"})
	s.Error = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5733")).Bold(true)
	s.Help = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	s.Bordered = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(1)
	s.Selected = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	s.Normal = lipgloss.NewStyle()
	s.TableHeader = lipgloss.NewStyle().Bold(true)
	s.TableRow = lipgloss.NewStyle()
	s.SelectedTableRow = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	return s
}

// --- View State ---
type viewState int
type searchState int

const (
	vaultListView viewState = iota
	vaultAddFormView
	vaultConfirmDeleteView
	loadingVaultView
	walletListView
	walletDetailsView
	walletConfirmDeleteView
	walletAddFormView
	walletEditLabelView
	walletCopyView
	walletRenameView
	walletEditMetadataView
	walletCloneSelectView
	walletCloneFilenameView
	walletImportView
	walletExportView
	searchView
)

const (
	searchByPrefix searchState = iota
	searchByTag
	searchByNotes
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
type clipboardClearedMsg struct{}

// --- Keymaps ---
type keyMap struct {
	Quit, Back, Enter, AddVault, DeleteVault, SwitchVault, AddWallet, DeleteWallet, RenameWallet, EditMeta, Clone, Import, Export, Derive, EditLabel, Copy, Yes, No, Select, Search, FilterByTag key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.Back, k.Enter, k.SwitchVault}
}
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.AddWallet, k.DeleteWallet, k.RenameWallet, k.EditMeta},
		{k.Import, k.Export, k.Clone, k.Search, k.FilterByTag},
		{k.Derive, k.EditLabel, k.Copy},
		{k.Select, k.Yes, k.No},
		{k.SwitchVault, k.AddVault, k.DeleteVault},
	}
}

var keys = keyMap{
	Quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Back:         key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Enter:        key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	AddVault:     key.NewBinding(key.WithKeys("A"), key.WithHelp("shift+a", "add vault")),
	DeleteVault:  key.NewBinding(key.WithKeys("D", "X"), key.WithHelp("shift+d/X", "delete vault")),
	SwitchVault:  key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "switch vault")),
	AddWallet:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add wallet")),
	DeleteWallet: key.NewBinding(key.WithKeys("d", "x"), key.WithHelp("d/x", "delete wallet")),
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
	Search:       key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	FilterByTag:  key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter by tag")),
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

// --- List Delegates ---

func newItemDelegate(s Styles) list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles.NormalTitle = s.Normal
	d.Styles.NormalDesc = s.Help
	d.Styles.SelectedTitle = s.Selected
	d.Styles.SelectedDesc = s.Help
	return d
}

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
	if _, ok := d.model.cloneSelection[str]; ok {
		str = "[x] " + str
	} else {
		str = "[ ] " + str
	}
	fn := d.model.styles.Normal.Render
	if index == l.Index() {
		fn = func(s ...string) string { return d.model.styles.Selected.Render(s...) }
	}
	fmt.Fprint(w, fn(str))
}

// --- Main Model ---
type model struct {
	state          viewState
	previousState  viewState
	searchState    searchState
	vaultList      list.Model
	walletList     list.Model
	addressTable   table.Model
	viewport       viewport.Model
	inputs         []textinput.Model
	spinner        spinner.Model
	help           help.Model
	keys           keyMap
	styles         Styles
	focusIndex     int
	loadedVault    *vault.Vault
	activeVault    config.VaultDetails
	currentPrefix  string
	prefixToDelete string
	prefixToRename string
	prefixToEdit   string
	cloneSelection map[string]struct{}
	formError      error
	loadingMsg     string
	statusMsg      string
	width, height  int
}

func newModel() model {
	s := newStyles()

	// Setup spinner
	spinner := spinner.New()
	spinner.Spinner = spinner.Dot
	spinner.Style = s.Status

	// Setup main list for vaults/wallets
	vaultList := list.New([]list.Item{}, newItemDelegate(s), 0, 0)
	vaultList.SetShowHelp(false)
	vaultList.Styles.Title = s.Title

	m := model{
		keys:       keys,
		styles:     s,
		help:       help.New(),
		spinner:    spinner,
		vaultList:  vaultList,
		walletList: vaultList, // Use the same list initially
	}
	m.help.ShowAll = true
	return m
}

func (m *model) initLists(width, height int) {
	// This is called once on the first window size message.
	listHeight := height - lipgloss.Height(m.headerView()) - lipgloss.Height(m.footerView())
	m.vaultList.SetSize(width, listHeight)
	m.walletList.SetSize(width, listHeight)
	m.viewport.Width = width
	m.viewport.Height = listHeight
	m.help.Width = width

	// Setup address table
	cols := []table.Column{
		{Title: "Index", Width: 5},
		{Title: "Label", Width: 20},
		{Title: "Address", Width: 42},
	}
	m.addressTable = table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(listHeight),
	)
	ts := table.DefaultStyles()
	ts.Header = m.styles.TableHeader
	ts.Selected = m.styles.SelectedTableRow
	m.addressTable.SetStyles(ts)
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
	m.vaultList.Title = "Select a Vault"
}

func (m *model) refreshWalletList(filterTag, searchQuery string) {
	if m.loadedVault == nil {
		return
	}
	items := []list.Item{}
	prefixes := make([]string, 0, len(*m.loadedVault))
	for prefix := range *m.loadedVault {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)

	title := fmt.Sprintf("Wallets in '%s'", config.Cfg.ActiveVault)

	for _, prefix := range prefixes {
		wallet := (*m.loadedVault)[prefix]

		// Apply filters
		if filterTag != "" {
			tagFound := false
			for _, t := range wallet.Tags {
				if strings.EqualFold(t, filterTag) {
					tagFound = true
					break
				}
			}
			if !tagFound {
				continue
			}
			title = fmt.Sprintf("Wallets in '%s' tagged '%s'", config.Cfg.ActiveVault, filterTag)
		}
		if searchQuery != "" {
			if !strings.Contains(strings.ToLower(prefix), searchQuery) && !strings.Contains(strings.ToLower(wallet.Notes), searchQuery) {
				continue
			}
			title = fmt.Sprintf("Search results for '%s' in '%s'", searchQuery, config.Cfg.ActiveVault)
		}

		desc := fmt.Sprintf("Addresses: %d, Tags: [%s]", len(wallet.Addresses), strings.Join(wallet.Tags, ", "))
		items = append(items, walletItem{prefix: prefix, desc: desc})
	}
	m.walletList.SetItems(items)
	m.walletList.Title = title
}

func (m *model) setState(s viewState) {
	m.previousState = m.state
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
	m.keys.Search.SetEnabled(isWalletListView)
	m.keys.FilterByTag.SetEnabled(isWalletListView)

	isDetailsView := m.state == walletDetailsView
	m.keys.Derive.SetEnabled(isDetailsView)
	m.keys.EditLabel.SetEnabled(isDetailsView)
	m.keys.Copy.SetEnabled(isDetailsView)

	isConfirmView := m.state == walletConfirmDeleteView || m.state == vaultConfirmDeleteView
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
		m.refreshVaultList()
		return nil
	}
	m.loadingMsg = fmt.Sprintf("Loading vault '%s'...", config.Cfg.ActiveVault)
	m.setState(loadingVaultView)
	m.activeVault = activeVault
	return tea.Batch(m.spinner.Tick, loadVaultCmd(activeVault))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle messages that can arrive in any state
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// The first WindowSizeMsg initializes the lists
		if m.vaultList.Items() == nil {
			m.initLists(m.width, m.height)
		}
		return m, nil

	case vaultLoadedMsg:
		m.setState(walletListView)
		if msg.err != nil {
			m.statusMsg = m.styles.Error.Render(fmt.Sprintf("Error: %v", msg.err))
			return m, nil
		}
		v := msg.v
		m.loadedVault = &v
		m.refreshWalletList("", "")
		return m, nil

	case deriveCompletedMsg:
		m.setState(walletDetailsView)
		if msg.err != nil {
			m.statusMsg = m.styles.Error.Render(fmt.Sprintf("Error: %v", msg.err))
			return m, nil
		}
		(*m.loadedVault)[m.currentPrefix] = msg.w
		if err := m.saveActiveVault(); err != nil {
			m.statusMsg = m.styles.Error.Render(fmt.Sprintf("Save error: %v", err))
			return m, nil
		}
		m.updateAddressTable()
		m.statusMsg = m.styles.Status.Render(fmt.Sprintf("New address generated: %s", msg.newAdr.Address))
		return m, nil

	case clipboardClearedMsg:
		m.statusMsg = m.styles.Status.Render("Clipboard cleared.")
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		if m.state == loadingVaultView {
			m.spinner, cmd = m.spinner.Update(msg)
		}
		return m, cmd
	}

	// State-specific updates
	switch m.state {
	case vaultListView:
		return m.updateVaultListView(msg)
	case walletListView:
		return m.updateWalletListView(msg)
	case walletDetailsView:
		return m.updateWalletDetailsView(msg)
	case searchView:
		return m.updateSearchView(msg)
	// Forms and confirmations
	case vaultAddFormView:
		return m.updateVaultAddFormView(msg)
	case vaultConfirmDeleteView:
		return m.updateVaultConfirmDeleteView(msg)
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

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	var mainContent string
	switch m.state {
	case loadingVaultView:
		mainContent = fmt.Sprintf("\n   %s %s\n", m.spinner.View(), m.loadingMsg)
	case vaultListView:
		mainContent = m.vaultList.View()
	case walletListView:
		mainContent = m.walletList.View()
	case walletDetailsView:
		mainContent = m.walletDetailsView()
	case searchView:
		mainContent = m.viewSearchForm()
	// Forms and confirmations
	case vaultAddFormView:
		mainContent = m.viewVaultAddForm()
	case vaultConfirmDeleteView:
		mainContent = m.styles.Bordered.Render(fmt.Sprintf("Are you sure you want to remove vault '%s' from the configuration?\nThis will NOT delete the vault file itself.\n\n(y/n)", m.prefixToDelete))
	case walletConfirmDeleteView:
		mainContent = m.styles.Bordered.Render(fmt.Sprintf("Are you sure you want to delete wallet '%s'?\n\n(y/n)", m.prefixToDelete))
	case walletAddFormView:
		mainContent = m.viewWalletAddForm()
	case walletEditLabelView:
		mainContent = m.viewWalletEditLabelForm()
	case walletCopyView:
		mainContent = m.viewWalletCopyForm()
	case walletRenameView:
		mainContent = m.viewWalletRenameForm()
	case walletEditMetadataView:
		mainContent = m.viewWalletEditMetadataForm()
	case walletCloneSelectView:
		mainContent = m.walletList.View()
	case walletCloneFilenameView:
		mainContent = m.viewWalletCloneFilenameForm()
	case walletImportView:
		mainContent = m.viewWalletImportForm()
	case walletExportView:
		mainContent = m.viewWalletExportForm()
	default:
		mainContent = "Unknown state."
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		m.headerView(),
		mainContent,
		m.footerView(),
	)
}

// --- View Helpers ---

func (m model) headerView() string {
	var title string
	if m.state == vaultListView {
		title = m.vaultList.Title
	} else {
		title = m.walletList.Title
	}
	return m.styles.Title.Render(title)
}

func (m model) footerView() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		m.styles.Status.Render(m.statusMsg),
		m.styles.Help.Render(m.help.View(m.keys)),
	)
}

// --- Update Functions ---

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
				m.statusMsg = m.styles.Error.Render("Failed to save config")
				return m, nil
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
			return m, tea.Quit
		case key.Matches(msg, m.keys.SwitchVault):
			m.loadedVault = nil
			m.setState(vaultListView)
			m.refreshVaultList()
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			if selected, ok := m.walletList.SelectedItem().(walletItem); ok {
				m.currentPrefix = selected.prefix
				m.updateAddressTable()
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
				m.prefixToEdit = selected.prefix
				m.setState(walletEditMetadataView)
				m.setupWalletEditMetadataForm()
				return m, m.inputs[0].Focus()
			}
		case key.Matches(msg, m.keys.Clone):
			m.setState(walletCloneSelectView)
			m.setupWalletCloneSelect()
		case key.Matches(msg, m.keys.Import):
			m.setState(walletImportView)
			m.setupWalletImportForm()
			return m, m.inputs[0].Focus()
		case key.Matches(msg, m.keys.Export):
			m.setState(walletExportView)
			m.setupWalletExportForm()
			return m, m.inputs[0].Focus()
		case key.Matches(msg, m.keys.Search):
			m.searchState = searchByNotes
			m.setState(searchView)
			m.setupSearchForm("Search by notes or prefix")
			return m, m.inputs[0].Focus()
		case key.Matches(msg, m.keys.FilterByTag):
			m.searchState = searchByTag
			m.setState(searchView)
			m.setupSearchForm("Filter by tag")
			return m, m.inputs[0].Focus()
		}
	}
	var cmd tea.Cmd
	m.walletList, cmd = m.walletList.Update(msg)
	return m, cmd
}

func (m model) updateWalletDetailsView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Back):
			m.setState(walletListView)
			m.refreshWalletList("", "") // Reset any filters
		case key.Matches(msg, m.keys.SwitchVault):
			m.loadedVault = nil
			m.setState(vaultListView)
			m.refreshVaultList()
		case key.Matches(msg, m.keys.Derive):
			m.loadingMsg = fmt.Sprintf("Deriving new address for '%s'...", m.currentPrefix)
			m.setState(loadingVaultView)
			wallet := (*m.loadedVault)[m.currentPrefix]
			deriveCmd := func() tea.Msg {
				w, adr, err := actions.DeriveNextAddress(wallet, m.activeVault.Type)
				return deriveCompletedMsg{w: w, newAdr: adr, err: err}
			}
			return m, tea.Batch(m.spinner.Tick, deriveCmd)
		case key.Matches(msg, m.keys.EditLabel):
			if len(m.addressTable.SelectedRow()) > 0 {
				m.setState(walletEditLabelView)
				m.setupWalletEditLabelForm()
				return m, m.inputs[0].Focus()
			}
		case key.Matches(msg, m.keys.Copy):
			m.setState(walletCopyView)
			m.setupWalletCopyForm()
			return m, m.inputs[0].Focus()
		}
	}
	m.addressTable, cmd = m.addressTable.Update(msg)
	return m, cmd
}

func (m model) updateSearchView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.setState(walletListView)
			m.refreshWalletList("", "")
			return m, nil
		case "enter":
			query := m.inputs[0].Value()
			if m.searchState == searchByTag {
				m.refreshWalletList(query, "")
			} else {
				m.refreshWalletList("", strings.ToLower(query))
			}
			m.setState(walletListView)
			return m, nil
		}
	}
	m.inputs[0], cmd = m.inputs[0].Update(msg)
	return m, cmd
}

// --- Confirmation Handlers ---

func (m model) updateVaultConfirmDeleteView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Yes):
			if _, exists := config.Cfg.Vaults[m.prefixToDelete]; !exists {
				m.statusMsg = m.styles.Error.Render("Vault not found in config")
				m.setState(vaultListView)
				return m, nil
			}

			delete(config.Cfg.Vaults, m.prefixToDelete)
			if config.Cfg.ActiveVault == m.prefixToDelete {
				config.Cfg.ActiveVault = ""
			}

			if err := config.SaveConfig(); err != nil {
				m.statusMsg = m.styles.Error.Render(fmt.Sprintf("Delete error (config): %v", err))
				m.setState(vaultListView)
				return m, nil
			}

			m.refreshVaultList()
			m.setState(vaultListView)
			m.statusMsg = m.styles.Status.Render(fmt.Sprintf("Vault '%s' removed from configuration.", m.prefixToDelete))
			return m, nil
		case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Back):
			m.setState(vaultListView)
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
				m.statusMsg = m.styles.Error.Render(fmt.Sprintf("Save error: %v", err))
				m.setState(walletListView)
				return m, nil
			}
			m.refreshWalletList("", "")
			m.setState(walletListView)
			audit.Logger.Info("Wallet deleted successfully via TUI", "prefix", m.prefixToDelete, "vault", config.Cfg.ActiveVault)
			m.statusMsg = m.styles.Status.Render(fmt.Sprintf("Wallet '%s' deleted.", m.prefixToDelete))
			return m, nil
		case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Back):
			m.setState(walletListView)
		}
	}
	return m, nil
}

// --- Form Handlers ---

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
				m.statusMsg = m.styles.Status.Render(fmt.Sprintf("Vault '%s' added to configuration.", name))
				return m, nil
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
					newWallet, _, err = actions.CreateWalletFromMnemonic(secret, m.activeVault.Type)
				case "2":
					newWallet, _, err = actions.CreateWalletFromPrivateKey(secret, m.activeVault.Type)
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
				m.refreshWalletList("", "")
				m.setState(walletListView)
				m.statusMsg = m.styles.Status.Render(fmt.Sprintf("Wallet '%s' added.", prefix))
				return m, nil
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

func (m model) updateWalletEditLabelView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.setState(walletDetailsView)
			return m, nil
		case "enter":
			selectedRow := m.addressTable.SelectedRow()
			indexStr := selectedRow[0]
			newLabel := m.inputs[0].Value()
			index, _ := strconv.Atoi(indexStr)

			wallet := (*m.loadedVault)[m.currentPrefix]
			addressToUpdate, found := findAddressByIndex(wallet, index)
			if !found {
				m.formError = fmt.Errorf("address with index %d not found", index)
				return m, nil
			}
			addressToUpdate.Label = newLabel
			(*m.loadedVault)[m.currentPrefix] = wallet
			if err := m.saveActiveVault(); err != nil {
				m.formError = err
				return m, nil
			}
			m.updateAddressTable()
			m.setState(walletDetailsView)
			m.statusMsg = m.styles.Status.Render(fmt.Sprintf("Label for address %d updated.", index))
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.inputs[0], cmd = m.inputs[0].Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
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
				field := strings.ToLower(m.inputs[0].Value())
				wallet := (*m.loadedVault)[m.currentPrefix]
				var result string
				var isSecret bool
				switch field {
				case constants.FieldAddress:
					selectedRow := m.addressTable.SelectedRow()
					if len(selectedRow) == 0 {
						m.formError = fmt.Errorf("no address selected")
						return m, nil
					}
					result = selectedRow[2] // Address is the 3rd column
					isSecret = false
				case constants.FieldPrivateKey:
					selectedRow := m.addressTable.SelectedRow()
					if len(selectedRow) == 0 {
						m.formError = fmt.Errorf("no address selected")
						return m, nil
					}
					index, _ := strconv.Atoi(selectedRow[0])
					addressData, found := findAddressByIndex(wallet, index)
					if !found {
						m.formError = fmt.Errorf("address with index %d not found", index)
						return m, nil
					}
					result = addressData.PrivateKey
					isSecret = true
				case constants.FieldMnemonic:
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
					go func(secret string) {
						time.Sleep(clipboardClearTimeout)
						currentClipboard, _ := clipboard.ReadAll()
						if currentClipboard == secret {
							clipboard.WriteAll("")
						}
					}(result)
				} else {
					statusMessage = fmt.Sprintf("✅ Field '%s' copied.", field)
				}
				m.setState(walletDetailsView)
				m.statusMsg = m.styles.Status.Render(statusMessage)
				return m, nil
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
			m.refreshWalletList("", "")
			m.setState(walletListView)
			m.statusMsg = m.styles.Status.Render(fmt.Sprintf("Wallet '%s' renamed to '%s'", m.prefixToRename, newPrefix))
			return m, nil
		}
	}
	m.inputs[0], cmd = m.inputs[0].Update(msg)
	return m, cmd
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
				wallet := (*m.loadedVault)[m.prefixToEdit]
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
				(*m.loadedVault)[m.prefixToEdit] = wallet
				if err := m.saveActiveVault(); err != nil {
					m.formError = err
					return m, nil
				}
				m.refreshWalletList("", "")
				m.setState(walletListView)
				m.statusMsg = m.styles.Status.Render(fmt.Sprintf("Metadata for '%s' updated.", m.prefixToEdit))
				return m, nil
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

func (m model) updateWalletCloneSelectView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Back):
			m.resetToDefaultWalletList()
			m.setState(walletListView)
		case key.Matches(msg, m.keys.Enter):
			if len(m.cloneSelection) > 0 {
				m.setState(walletCloneFilenameView)
				m.setupWalletCloneFilenameForm()
				return m, m.inputs[0].Focus()
			}
		case key.Matches(msg, m.keys.Select):
			if i, ok := m.walletList.SelectedItem().(walletItem); ok {
				title := i.Title()
				if _, ok := m.cloneSelection[title]; ok {
					delete(m.cloneSelection, title)
				} else {
					m.cloneSelection[title] = struct{}{}
				}
			}
		}
	}
	var cmd tea.Cmd
	m.walletList, cmd = m.walletList.Update(msg)
	return m, cmd
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
			prefixes := make([]string, 0, len(m.cloneSelection))
			for p := range m.cloneSelection {
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
			m.statusMsg = m.styles.Status.Render(fmt.Sprintf("Cloned vault '%s' created.", outputFile))
			return m, nil
		}
	}
	m.inputs[0], cmd = m.inputs[0].Update(msg)
	return m, cmd
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
				updatedVault, report, err := actions.ImportWallets(*m.loadedVault, content, format, conflictPolicy, m.activeVault.Type)
				if err != nil {
					m.formError = err
					return m, nil
				}
				if err := vault.SaveVault(m.activeVault, updatedVault); err != nil {
					m.formError = err
					return m, nil
				}
				*m.loadedVault = updatedVault
				m.refreshWalletList("", "")
				m.setState(walletListView)
				m.statusMsg = m.styles.Status.Render(report)
				return m, nil
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
			m.statusMsg = m.styles.Status.Render(fmt.Sprintf("Vault successfully exported to '%s'", filePath))
			return m, nil
		}
	}
	m.inputs[0], cmd = m.inputs[0].Update(msg)
	return m, cmd
}

// --- Final Section: Helpers, Forms Views, and Entry Point ---

func (m *model) updateAddressTable() {
	wallet := (*m.loadedVault)[m.currentPrefix]
	rows := make([]table.Row, len(wallet.Addresses))
	for i, addr := range wallet.Addresses {
		rows[i] = table.Row{
			fmt.Sprintf("%d", addr.Index),
			addr.Label,
			addr.Address,
		}
	}
	m.addressTable.SetRows(rows)
}

func (m model) walletDetailsView() string {
	var b strings.Builder
	wallet := (*m.loadedVault)[m.currentPrefix]
	b.WriteString(fmt.Sprintf("Notes: %s\n", wallet.Notes))
	b.WriteString(fmt.Sprintf("Tags: [%s]\n", strings.Join(wallet.Tags, ", ")))
	b.WriteString("\n")
	b.WriteString(m.addressTable.View())
	return m.styles.Bordered.Render(b.String())
}

func (m *model) setupSearchForm(placeholder string) {
	m.inputs = make([]textinput.Model, 1)
	t := textinput.New()
	t.Placeholder = placeholder
	t.Focus()
	t.CharLimit = 64
	t.Width = 50
	m.inputs[0] = t
}

func (m model) viewSearchForm() string {
	return fmt.Sprintf("\n%s\n", m.inputs[0].View())
}

func (m *model) setupVaultAddForm() {
	m.inputs = make([]textinput.Model, 5)
	prompts := []string{"Vault Name (Prefix)", "Type (e.g., EVM, COSMOS)", "Encryption (passphrase or yubikey)", "Key File Path (e.g., my_vault.age)", "Recipients File (if yubikey)"}
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

func (m *model) setupWalletEditLabelForm() {
	m.inputs = make([]textinput.Model, 1)
	selectedRow := m.addressTable.SelectedRow()
	currentLabel := selectedRow[1]
	t := textinput.New()
	t.Prompt = "New Label: "
	t.Placeholder = currentLabel
	t.Focus()
	t.CharLimit = 128
	t.Width = 50
	m.inputs[0] = t
}

func (m model) viewWalletEditLabelForm() string {
	var b strings.Builder
	selectedRow := m.addressTable.SelectedRow()
	b.WriteString(m.styles.Title.Render(fmt.Sprintf("Editing Label for Address %s", selectedRow[0])) + "\n\n")
	b.WriteString(m.inputs[0].View())
	if m.formError != nil {
		b.WriteString("\n" + m.styles.Error.Render(m.formError.Error()))
	}
	b.WriteString("\n(enter to submit, esc to cancel)")
	return m.styles.Bordered.Render(b.String())
}

func (m *model) setupWalletCopyForm() {
	m.inputs = make([]textinput.Model, 1)
	t := textinput.New()
	t.Prompt = "Field (address, privatekey, mnemonic): "
	t.Placeholder = "privatekey"
	t.Focus()
	t.CharLimit = 32
	t.Width = 50
	m.inputs[0] = t
	m.focusIndex = 0 // Only one input
}

func (m model) viewWalletCopyForm() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render("Copy Data to Clipboard") + "\n\n")
	b.WriteString(m.inputs[0].View())
	if m.formError != nil {
		b.WriteString("\n" + m.styles.Error.Render(m.formError.Error()))
	}
	b.WriteString("\n(enter to submit, esc to cancel)")
	return m.styles.Bordered.Render(b.String())
}

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

func (m *model) setupWalletEditMetadataForm() {
	m.inputs = make([]textinput.Model, 2)
	wallet := (*m.loadedVault)[m.prefixToEdit]
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

func (m model) viewWalletEditMetadataForm() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render(fmt.Sprintf("Editing metadata for '%s'", m.prefixToEdit)) + "\n\n")
	for i := range m.inputs {
		b.WriteString(m.inputs[i].View() + "\n")
	}
	if m.formError != nil {
		b.WriteString("\n\n" + m.styles.Error.Render(m.formError.Error()))
	}
	b.WriteString("\n\n(enter to submit, esc to cancel)")
	return m.styles.Bordered.Render(b.String())
}

func (m *model) setupWalletCloneSelect() {
	m.cloneSelection = make(map[string]struct{})
	delegate := &cloneDelegate{model: m}
	m.walletList.SetDelegate(delegate)
	m.walletList.Title = "Select wallets to clone (space to select, enter to confirm)"
}

func (m *model) resetToDefaultWalletList() {
	m.walletList.SetDelegate(newItemDelegate(m.styles))
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
	// Initialize logger for TUI session
	if err := audit.InitLogger(); err != nil {
		fmt.Println("Failed to initialize audit logger:", err)
		os.Exit(1)
	}
	// Load config for TUI session
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
