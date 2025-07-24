// File: internal/tui/form.go
package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// --- Form Model ---

// FormFieldType определяет тип поля: input или select
type FormFieldType int

const (
	FieldInput FormFieldType = iota
	FieldSelect
)

// FormField represents a single input field in a form.
type FormField struct {
	input     textinput.Model
	validator func(string) error // A function to validate the field's value.
	err       error              // Validation error for this field.
	// Для select
	fieldType FormFieldType
	options   []string
	selected  int
}

// NewFormField для обычного текстового поля
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

// NewSelectField для поля выбора из списка
func NewSelectField(prompt string, options []string, defaultIdx int) FormField {
	if defaultIdx < 0 || defaultIdx >= len(options) {
		defaultIdx = 0
	}
	return FormField{
		fieldType: FieldSelect,
		options:   options,
		selected:  defaultIdx,
		input:     textinput.New(), // не используется, но нужен для совместимости
	}
}

// WithValidator adds a validation function to the field.
func (f FormField) WithValidator(v func(string) error) FormField {
	f.validator = v
	return f
}

// WithValue sets an initial value for the field.
func (f FormField) WithValue(val string) FormField {
	f.input.SetValue(val)
	return f
}

// FormModel represents a complete form with multiple fields.
type FormModel struct {
	parent      *model // Reference to the main model for styles and dimensions
	title       string
	fields      []FormField
	focusIndex  int
	submitted   bool
	cancelled   bool
	OnSubmit    func([]string) tea.Cmd // Callback on successful submission. Returns a command.
	popupSelect *PopupSelectModel
}

// NewForm creates a new form model.
func NewForm(parent *model, title string, fields []FormField, onSubmit func([]string) tea.Cmd) *FormModel {
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

// Init initializes the form.
func (m *FormModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages for the form model.
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
				// Открываем popupSelect (внутри формы)
				m.popupSelect = NewPopupSelectModel(m, field.input.Prompt, field.options, field.selected, func(idx int) {
					field.selected = idx
					m.popupSelect = nil
					m.nextField() // автоматически перейти к следующему полю
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

// View renders the form.
func (m *FormModel) View() string {
	var b strings.Builder
	b.WriteString(m.parent.styles.Title.Render(m.title) + "\n\n")

	for _, field := range m.fields {
		if field.fieldType == FieldSelect {
			// Только выбранное значение и подсказка
			b.WriteString(field.input.Prompt + "[" + field.options[field.selected] + "] (Enter для выбора)\n")
		} else {
			b.WriteString(field.input.View() + "\n")
		}
		if field.err != nil {
			errorStyle := m.parent.styles.Error.Copy().Width(m.parent.width).PaddingLeft(len(field.input.Prompt))
			b.WriteString(errorStyle.Render(field.err.Error()) + "\n")
		}
	}

	if m.popupSelect != nil {
		b.WriteString(m.popupSelect.View())
	}

	b.WriteString("\n" + m.parent.styles.Help.Render("(enter to submit, esc to cancel, tab/стрелки для навигации)"))
	return m.parent.styles.Bordered.Render(b.String())
}

// Values returns the values of all fields in the form.
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

// Help returns the key bindings for the form view.
func (m *FormModel) Help() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "next/submit")),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field")),
	}
}

// Title returns the title of the form.
func (m *FormModel) Title() string {
	return m.title
}

// --- Internal methods ---

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

// PopupSelectModel — отдельный view для выбора значения из списка

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
