package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// FormFieldType определяет тип поля: input или select
// (select реализован через popup внутри формы)
type FormFieldType int

const (
	FieldInput FormFieldType = iota
	FieldSelect
)

type FormField struct {
	input     textinput.Model
	validator func(string) error
	err       error
	fieldType FormFieldType
	options   []string
	selected  int
}

func NewFormField(prompt, placeholder string, charLimit int, isPassword bool) FormField {
	ti := textinput.New()
	ti.Prompt = prompt + ": "
	ti.Placeholder = placeholder
	ti.CharLimit = charLimit
	if isPassword {
		ti.EchoMode = textinput.EchoPassword
	}
	return FormField{input: ti, fieldType: FieldInput}
}

func NewSelectField(prompt string, options []string, defaultIdx int) FormField {
	if defaultIdx < 0 || defaultIdx >= len(options) {
		defaultIdx = 0
	}
	return FormField{
		fieldType: FieldSelect,
		options:   options,
		selected:  defaultIdx,
		input:     textinput.New(),
	}
}

func (f FormField) WithValidator(v func(string) error) FormField {
	f.validator = v
	return f
}
func (f FormField) WithValue(val string) FormField {
	f.input.SetValue(val)
	return f
}

type FormModel struct {
	parent      *Model
	title       string
	fields      []FormField
	focusIndex  int
	submitted   bool
	cancelled   bool
	OnSubmit    func([]string) tea.Cmd
	popupSelect *PopupSelectModel
}

func NewForm(parent *Model, title string, fields []FormField, onSubmit func([]string) tea.Cmd) *FormModel {
	fm := &FormModel{
		parent:   parent,
		title:    title,
		fields:   fields,
		OnSubmit: onSubmit,
	}
	if len(fm.fields) > 0 {
		fm.fields[0].input.Focus()
	}
	return fm
}

func (m *FormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.submitted || m.cancelled {
		return m, nil
	}

	if m.popupSelect != nil {
		var cmd tea.Cmd
		var newPopup tea.Model
		newPopup, cmd = m.popupSelect.Update(msg)
		if ps, ok := newPopup.(*PopupSelectModel); ok {
			m.popupSelect = ps
		} else {
			m.popupSelect = nil
		}
		return m, cmd
	}

	var cmds []tea.Cmd
	field := &m.fields[m.focusIndex]

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.cancelled = true
			return m, func() tea.Msg { return popViewMsg{} }
		case tea.KeyEnter:
			if field.fieldType == FieldSelect {
				m.popupSelect = NewPopupSelectModel(m, field.input.Prompt, field.options, field.selected, func(idx int) {
					field.selected = idx
					m.popupSelect = nil
					m.nextField()
				})
				return m, nil
			}
			if m.focusIndex == len(m.fields)-1 {
				return m.submitForm()
			}
			m.nextField()
		case tea.KeyUp, tea.KeyShiftTab:
			if field.fieldType == FieldSelect {
				field.selected--
				if field.selected < 0 {
					field.selected = len(field.options) - 1
				}
			} else {
				m.prevField()
			}
		case tea.KeyDown, tea.KeyTab:
			if field.fieldType == FieldSelect {
				field.selected++
				if field.selected >= len(field.options) {
					field.selected = 0
				}
			} else {
				m.nextField()
			}
		}
	}

	if field.fieldType == FieldInput {
		var cmd tea.Cmd
		field.input, cmd = field.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *FormModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.title) + "\n\n")

	for _, field := range m.fields {
		if field.fieldType == FieldSelect {
			b.WriteString(field.input.Prompt + "[" + field.options[field.selected] + "] (Enter для выбора)\n")
		} else {
			b.WriteString(field.input.View() + "\n")
		}
		if field.err != nil {
			errorStyle := errorStyle.Copy().Width(m.parent.width).PaddingLeft(len(field.input.Prompt))
			b.WriteString(errorStyle.Render(field.err.Error()) + "\n")
		}
	}
	if m.popupSelect != nil {
		b.WriteString(m.popupSelect.View())
	}
	b.WriteString("\n" + statusStyle.Render("(enter to submit, esc to cancel, tab/стрелки для навигации)"))
	return b.String()
}

func (m *FormModel) Values() []string {
	values := make([]string, len(m.fields))
	for i, field := range m.fields {
		if field.fieldType == FieldSelect {
			values[i] = field.options[field.selected]
		} else {
			values[i] = field.input.Value()
		}
	}
	return values
}

func (m *FormModel) Help() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "next/submit")),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field")),
	}
}

func (m *FormModel) Title() string {
	return m.title
}

func (m *FormModel) nextField() {
	m.fields[m.focusIndex].input.Blur()
	m.focusIndex = (m.focusIndex + 1) % len(m.fields)
	m.fields[m.focusIndex].input.Focus()
}

func (m *FormModel) prevField() {
	m.fields[m.focusIndex].input.Blur()
	m.focusIndex--
	if m.focusIndex < 0 {
		m.focusIndex = len(m.fields) - 1
	}
	m.fields[m.focusIndex].input.Focus()
}

func (m *FormModel) submitForm() (*FormModel, tea.Cmd) {
	isValid := true
	for i := range m.fields {
		m.fields[i].err = nil
		if m.fields[i].validator != nil {
			if err := m.fields[i].validator(m.fields[i].input.Value()); err != nil {
				m.fields[i].err = err
				isValid = false
			}
		}
	}

	if !isValid {
		return m, nil
	}

	m.submitted = true
	if m.OnSubmit != nil {
		return m, m.OnSubmit(m.Values())
	}
	return m, nil
}

// PopupSelectModel — отдельный popup для выбора значения

type PopupSelectModel struct {
	parent   *FormModel
	title    string
	options  []string
	selected int
	onSelect func(idx int)
}

func NewPopupSelectModel(parent *FormModel, title string, options []string, selected int, onSelect func(idx int)) *PopupSelectModel {
	return &PopupSelectModel{
		parent:   parent,
		title:    title,
		options:  options,
		selected: selected,
		onSelect: onSelect,
	}
}

func (m *PopupSelectModel) Init() tea.Cmd { return nil }
func (m *PopupSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.selected--
			if m.selected < 0 {
				m.selected = len(m.options) - 1
			}
		case "down", "j":
			m.selected++
			if m.selected >= len(m.options) {
				m.selected = 0
			}
		case "enter":
			m.onSelect(m.selected)
			return m.parent, nil
		case "esc":
			return m.parent, nil
		}
	}
	return m, nil
}
func (m *PopupSelectModel) View() string {
	var b strings.Builder
	b.WriteString(m.title + "\n\n")
	for i, opt := range m.options {
		prefix := "  "
		if i == m.selected {
			prefix = "> "
		}
		b.WriteString(prefix + opt + "\n")
	}
	b.WriteString("\n(стрелки — навигация, Enter — выбрать, Esc — отмена)")
	return b.String()
}
func (m *PopupSelectModel) Help() []key.Binding { return nil }
func (m *PopupSelectModel) Title() string       { return m.title }

type popViewMsg struct{}
