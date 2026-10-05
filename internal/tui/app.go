package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/store"
	"github.com/innomon/whatsadk/internal/tui/handlers"
	"github.com/innomon/whatsadk/internal/tui/registry"
)

// UIState represents the current view mode of the TUI.
type UIState int

const (
	StateNormal UIState = iota
	StateSlashModal
	StateDynamicPrompt
)

// OutputEntry represents an entry in the output history.
type OutputEntry struct {
	Command string
	Result  registry.Result
	Err     error
}

// AppModel implements the Charm Bubble Tea model for the WhatsADK Command TUI.
type AppModel struct {
	cfg        *config.Config
	store      *store.Store
	registry   *registry.Registry
	commands   []registry.CommandDef
	cmdMap     map[string]registry.CommandDef

	// TUI Components
	state         UIState
	viewport      viewport.Model
	input         textinput.Model
	modalInput    textinput.Model
	selectedCmd   registry.CommandDef
	missingParams []registry.ParamDef
	promptValues  map[string]string
	promptIdx     int

	// Output History & Layout
	history  []OutputEntry
	width    int
	height   int
	p2pInfo  string

	// Styling
	styles Styles
}

// Styles holds Lip Gloss style definitions.
type Styles struct {
	Header         lipgloss.Style
	StatusBar      lipgloss.Style
	Viewport       lipgloss.Style
	InputPrompt    lipgloss.Style
	ModalBorder    lipgloss.Style
	ModalHeader    lipgloss.Style
	ModalText      lipgloss.Style
	ResultText     lipgloss.Style
	ResultMarkdown lipgloss.Style
	ResultA2UI     lipgloss.Style
	ErrorText      lipgloss.Style
}

func DefaultStyles() Styles {
	return Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1),
		StatusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0A0A0")).
			Background(lipgloss.Color("#222222")).
			Padding(0, 1),
		Viewport: lipgloss.NewStyle().
			Padding(0, 1),
		InputPrompt: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true),
		ModalBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(1, 2).
			Width(64),
		ModalHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF7676")).
			MarginBottom(1),
		ModalText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDDDDD")),
		ResultText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E2E2")),
		ResultMarkdown: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D7FF")),
		ResultA2UI: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#04B575")).
			Padding(0, 1).
			Foreground(lipgloss.Color("#5FFB17")),
		ErrorText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0055")).
			Bold(true),
	}
}

// NewAppModel constructs a new WhatsADK Command TUI application model.
func NewAppModel(cfg *config.Config, s *store.Store) *AppModel {
	reg := registry.NewRegistry()
	handlers.RegisterAllHandlers(reg)

	// Combine default commands with config commands
	cmdList := handlers.DefaultCommands()
	if cfg != nil && len(cfg.Commands) > 0 {
		cmdList = append(cmdList, cfg.Commands...)
	}

	cmdMap := make(map[string]registry.CommandDef)
	for _, c := range cmdList {
		cmdMap[strings.ToLower(c.Name)] = c
	}

	ti := textinput.New()
	ti.Placeholder = "Type a command (e.g. sql query=\"SELECT * FROM filesys\", or '/' for slash modal)..."
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = 80

	mi := textinput.New()
	mi.Placeholder = "Enter parameter value..."
	mi.Width = 50

	vp := viewport.New(80, 20)
	vp.SetContent("Welcome to WhatsADK Interactive Command & MCP TUI!\nType 'help' or press '/' to open the slash command modal.\n")

	p2pStr := "Local Store"
	if cfg != nil && cfg.P2P.Enabled {
		p2pStr = fmt.Sprintf("P2P Swarm Node: %s | Topic: %s", cfg.P2P.NodeID, cfg.P2P.SwarmTopic)
	}

	return &AppModel{
		cfg:          cfg,
		store:        s,
		registry:     reg,
		commands:     cmdList,
		cmdMap:       cmdMap,
		state:        StateNormal,
		viewport:     vp,
		input:        ti,
		modalInput:   mi,
		promptValues: make(map[string]string),
		p2pInfo:      p2pStr,
		styles:       DefaultStyles(),
	}
}

func (m *AppModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.state != StateNormal {
				m.state = StateNormal
				m.input.Focus()
				return m, nil
			}
		case "/":
			if m.state == StateNormal && m.input.Value() == "" {
				m.openSlashModal("")
				return m, nil
			}
		}

		switch m.state {
		case StateNormal:
			if msg.Type == tea.KeyEnter {
				line := m.input.Value()
				m.input.SetValue("")
				if strings.HasPrefix(strings.TrimSpace(line), "/") {
					m.openSlashModal(strings.TrimPrefix(strings.TrimSpace(line), "/"))
					return m, nil
				}
				m.executeLine(line)
				return m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)

		case StateSlashModal:
			if msg.Type == tea.KeyEnter {
				val := m.modalInput.Value()
				m.modalInput.SetValue("")
				m.executeLine(val)
				m.state = StateNormal
				m.input.Focus()
				return m, nil
			}
			var cmd tea.Cmd
			m.modalInput, cmd = m.modalInput.Update(msg)
			cmds = append(cmds, cmd)

		case StateDynamicPrompt:
			if msg.Type == tea.KeyEnter {
				val := m.modalInput.Value()
				m.modalInput.SetValue("")
				if m.promptIdx < len(m.missingParams) {
					p := m.missingParams[m.promptIdx]
					m.promptValues[strings.ToLower(p.Name)] = val
					m.promptIdx++
				}

				if m.promptIdx >= len(m.missingParams) {
					m.state = StateNormal
					m.input.Focus()
					m.executeSelectedCommand()
				} else {
					m.modalInput.Placeholder = fmt.Sprintf("Enter %s (%s)...", m.missingParams[m.promptIdx].Name, m.missingParams[m.promptIdx].Type)
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.modalInput, cmd = m.modalInput.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 6
		m.input.Width = msg.Width - 10
		m.modalInput.Width = msg.Width - 20
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m *AppModel) openSlashModal(initialText string) {
	m.state = StateSlashModal
	m.modalInput.SetValue(initialText)
	m.modalInput.Placeholder = "Type command name and arguments..."
	m.modalInput.Focus()
}

func (m *AppModel) executeLine(line string) {
	cmdName, rawArgs := registry.ParseLine(line)
	if cmdName == "" {
		return
	}

	if cmdName == "help" {
		m.renderHelp()
		return
	}

	cmdDef, exists := m.cmdMap[cmdName]
	if !exists {
		m.addOutput(line, registry.Result{}, fmt.Errorf("unknown command %q. Type 'help' or press '/' for commands list", cmdName))
		return
	}

	// Map positional arguments
	rawMapped := registry.MapPositionalToDefs(cmdDef.Params, rawArgs)

	// Parse parameters
	parsedParams, missing, err := registry.ParseParams(cmdDef.Params, rawMapped)
	if err != nil {
		m.addOutput(line, registry.Result{}, err)
		return
	}

	if len(missing) > 0 {
		m.selectedCmd = cmdDef
		m.missingParams = missing
		m.promptValues = make(map[string]string)
		for k, v := range parsedParams {
			m.promptValues[strings.ToLower(k)] = fmt.Sprintf("%v", v)
		}
		m.promptIdx = 0
		m.state = StateDynamicPrompt
		m.modalInput.SetValue("")
		m.modalInput.Placeholder = fmt.Sprintf("Enter %s (%s)...", missing[0].Name, missing[0].Type)
		m.modalInput.Focus()
		return
	}

	m.executeCommandWithParsed(line, cmdDef, parsedParams)
}

func (m *AppModel) executeSelectedCommand() {
	rawMap := make(map[string]string)
	for k, v := range m.promptValues {
		rawMap[k] = v
	}

	parsedParams, _, err := registry.ParseParams(m.selectedCmd.Params, rawMap)
	if err != nil {
		m.addOutput(m.selectedCmd.Name, registry.Result{}, err)
		return
	}

	m.executeCommandWithParsed(m.selectedCmd.Name, m.selectedCmd, parsedParams)
}

func (m *AppModel) executeCommandWithParsed(line string, cmdDef registry.CommandDef, params map[string]any) {
	handler, ok := m.registry.GetHandler(cmdDef.Handler)
	if !ok {
		m.addOutput(line, registry.Result{}, fmt.Errorf("no handler registered for %q", cmdDef.Handler))
		return
	}

	ctx := context.Background()
	res, err := handler(ctx, m.store, params)
	m.addOutput(line, res, err)
}

func (m *AppModel) renderHelp() {
	var sb strings.Builder
	sb.WriteString("### Available Commands & MCP Tools\n\n")
	for _, c := range m.commands {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", c.Name, c.Help))
		if len(c.Params) > 0 {
			for _, p := range c.Params {
				valStr := ""
				if p.Value != "" {
					valStr = fmt.Sprintf(" (default: %s)", p.Value)
				}
				sb.WriteString(fmt.Sprintf("   - `%s` (%s)%s: %s\n", p.Name, p.Type, valStr, p.Help))
			}
		}
	}
	m.addOutput("help", registry.Result{Type: registry.ResultTypeMarkdown, Content: sb.String()}, nil)
}

func (m *AppModel) addOutput(cmd string, res registry.Result, err error) {
	m.history = append(m.history, OutputEntry{
		Command: cmd,
		Result:  res,
		Err:     err,
	})

	var sb strings.Builder
	for _, entry := range m.history {
		sb.WriteString(fmt.Sprintf("> %s\n", entry.Command))
		if entry.Err != nil {
			sb.WriteString(m.styles.ErrorText.Render(fmt.Sprintf("Error: %v\n\n", entry.Err)))
			continue
		}

		switch entry.Result.Type {
		case registry.ResultTypeMarkdown:
			sb.WriteString(m.styles.ResultMarkdown.Render(entry.Result.Content) + "\n\n")
		case registry.ResultTypeA2UI:
			sb.WriteString(m.styles.ResultA2UI.Render("A2UI Component:\n"+entry.Result.Content) + "\n\n")
		default:
			sb.WriteString(m.styles.ResultText.Render(entry.Result.Content) + "\n\n")
		}
	}

	m.viewport.SetContent(sb.String())
	m.viewport.GotoBottom()
}

func (m *AppModel) View() string {
	header := m.styles.Header.Render("WhatsADK Command TUI & MCP Shell")
	statusBar := m.styles.StatusBar.Render(fmt.Sprintf("[%s] Mode: %s", m.p2pInfo, m.stateString()))

	var mainView string
	if m.state == StateNormal {
		inputView := fmt.Sprintf("%s %s", m.styles.InputPrompt.Render(">"), m.input.View())
		mainView = lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			m.styles.Viewport.Render(m.viewport.View()),
			inputView,
			statusBar,
		)
	} else {
		modalTitle := "Slash Command Modal (/)"
		if m.state == StateDynamicPrompt {
			pName := ""
			if m.promptIdx < len(m.missingParams) {
				pName = m.missingParams[m.promptIdx].Name
			}
			modalTitle = fmt.Sprintf("Missing Parameter Input: %s", pName)
		}

		modalContent := lipgloss.JoinVertical(
			lipgloss.Left,
			m.styles.ModalHeader.Render(modalTitle),
			m.styles.ModalText.Render("Type parameter values and press Enter:"),
			m.modalInput.View(),
			"",
			m.styles.ModalText.Render("Press Esc to return to main prompt"),
		)

		modalView := m.styles.ModalBorder.Render(modalContent)

		mainView = lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			m.styles.Viewport.Render(m.viewport.View()),
			modalView,
			statusBar,
		)
	}

	return mainView
}

func (m *AppModel) stateString() string {
	switch m.state {
	case StateSlashModal:
		return "SLASH_MODAL"
	case StateDynamicPrompt:
		return "DYNAMIC_PROMPT"
	default:
		return "NORMAL"
	}
}
