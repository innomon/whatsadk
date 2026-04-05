package adksim

import (
	"encoding/base64"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/innomon/whatsadk/internal/agent"
)

type model struct {
	viewport    viewport.Model
	textarea    textarea.Model
	requests    chan *IncomingRequest
	pending     []*IncomingRequest
	messages    []string
	attachments []agent.Part
	err         error
}

func NewModel(requests chan *IncomingRequest) model {
	ta := textarea.New()
	ta.Placeholder = "Type agent response or /command..."
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 1000
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(80, 20)
	vp.SetContent("Welcome to ADK Reverse Simulator!\nWaiting for incoming requests from the gateway...\n\n")

	return model{
		textarea: ta,
		viewport: vp,
		requests: requests,
		pending:  []*IncomingRequest{},
		messages: []string{},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.waitForRequest())
}

func (m model) waitForRequest() tea.Cmd {
	return func() tea.Msg {
		return <-m.requests
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case *IncomingRequest:
		m.pending = append(m.pending, msg)
		m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render(fmt.Sprintf("User (%s): ", msg.UserID))+formatNewMessage(msg.NewMessage))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
		return m, m.waitForRequest()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			input := m.textarea.Value()
			if input == "" && len(m.attachments) == 0 {
				return m, nil
			}

			if strings.HasPrefix(input, "/") {
				m.handleCommand(input)
				m.textarea.Reset()
				return m, nil
			}

			if len(m.pending) == 0 {
				m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("System: No pending requests to respond to."))
				m.textarea.Reset()
				m.viewport.SetContent(strings.Join(m.messages, "\n"))
				m.viewport.GotoBottom()
				return m, nil
			}

			// Fulfill oldest request
			req := m.pending[0]
			m.pending = m.pending[1:]

			parts := []agent.Part{}
			if input != "" {
				parts = append(parts, agent.Part{Text: input})
			}
			parts = append(parts, m.attachments...)

			req.ResponseChan <- OutgoingResponse{Parts: parts}
			
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render("Agent: ")+input)
			for _, p := range m.attachments {
				if p.InlineData != nil {
					m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(fmt.Sprintf("  [Attached: %s]", p.InlineData.MimeType)))
				}
			}
			
			m.attachments = nil
			m.textarea.Reset()
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
		}
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *model) handleCommand(input string) {
	parts := strings.Fields(input[1:])
	if len(parts) == 0 {
		return
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "help":
		m.messages = append(m.messages, "Available commands:\n  /attach <path> - Attach a file\n  /clear - Clear screen\n  /help - Show this help")
	case "clear":
		m.messages = []string{}
		m.viewport.SetContent("")
	case "attach":
		if len(args) == 0 {
			m.messages = append(m.messages, "Usage: /attach <path>")
			return
		}
		path := args[0]
		data, err := os.ReadFile(path)
		if err != nil {
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("Error: ")+err.Error())
			return
		}
		mimeType := mime.TypeByExtension(filepath.Ext(path))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		m.attachments = append(m.attachments, agent.Part{
			InlineData: &agent.InlineData{
				MimeType: mimeType,
				Data:     base64.StdEncoding.EncodeToString(data),
			},
		})
		m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(fmt.Sprintf("System: Attached %s (%s)", path, mimeType)))
	default:
		m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("Unknown command: ")+cmd)
	}
	m.viewport.SetContent(strings.Join(m.messages, "\n"))
	m.viewport.GotoBottom()
}

func (m model) View() string {
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render(fmt.Sprintf("Pending requests: %d", len(m.pending)))
	return fmt.Sprintf(
		"%s\n\n%s\n\n%s",
		status,
		m.viewport.View(),
		m.textarea.View(),
	) + "\n\n"
}

func formatNewMessage(msg *agent.Message) string {
	if msg == nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range msg.Parts {
		if p.Text != "" {
			if sb.Len() > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(p.Text)
		}
		if p.InlineData != nil {
			sb.WriteString(fmt.Sprintf(" [Media: %s]", p.InlineData.MimeType))
		}
	}
	return sb.String()
}
