package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/innomon/whatsadk/internal/agent"
	"github.com/innomon/whatsadk/internal/auth"
	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/simulator"
	"github.com/innomon/whatsadk/internal/store"
	"github.com/innomon/whatsadk/internal/whatsapp"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "export" {
		runExport()
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	var jwtGen *auth.JWTGenerator
	if cfg.Auth.JWT.PrivateKeyPath != "" {
		ttl := 24 * time.Hour
		if cfg.Auth.JWT.TTL != "" {
			if d, err := time.ParseDuration(cfg.Auth.JWT.TTL); err == nil {
				ttl = d
			}
		}
		jwtGen, err = auth.NewJWTGenerator(cfg.Auth.JWT.PrivateKeyPath, cfg.Auth.JWT.Issuer, cfg.Auth.JWT.Audience, ttl)
		if err != nil {
			fmt.Printf("Error initializing JWT: %v\n", err)
			os.Exit(1)
		}
	}

	adkClient := agent.NewClient(&cfg.ADK, jwtGen)
	mediaProc := whatsapp.NewProcessor()
	sim := simulator.New(adkClient, mediaProc)
	registry := simulator.NewRegistry()

	p := tea.NewProgram(initialModel(sim, registry))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running simulator: %v\n", err)
		os.Exit(1)
	}
}

func runExport() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: simulator export <phone> <output.json>")
		os.Exit(1)
	}

	phone := os.Args[2]
	outPath := os.Args[3]

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	s, err := store.Open(cfg.WhatsApp.StoreDSN)
	if err != nil {
		fmt.Printf("Error opening store: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	if err := simulator.ExportRealSession(context.Background(), s, phone, outPath); err != nil {
		fmt.Printf("Export failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Exported session for %s to %s\n", phone, outPath)
}

// TUI Model
type model struct {
	sim        *simulator.Simulator
	registry   *simulator.Registry
	viewport   viewport.Model
	textarea   textarea.Model
	messages   []string
	err        error
	sender     string
}

func initialModel(sim *simulator.Simulator, registry *simulator.Registry) model {
	ta := textarea.New()
	ta.Placeholder = "Type a message or /command..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 280

	ta.SetWidth(80)
	ta.SetHeight(3)

	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent("Welcome to WhatsADK Simulator!\nType /help for commands.\n\n")

	ta.KeyMap.InsertNewline.SetEnabled(false)

	return model{
		sim:      sim,
		registry: registry,
		textarea: ta,
		viewport: vp,
		messages: []string{},
		sender:   sim.UserID(),
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			input := m.textarea.Value()
			if input == "" {
				return m, nil
			}

			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render("You: ")+input)
			m.textarea.Reset()

			// Check for commands
			res, handled, err := m.registry.Execute(context.Background(), m.sim, input)
			if handled {
				if err != nil {
					m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("Error: ")+err.Error())
				} else if res != "" {
					m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("System: ")+res)
				}
				m.viewport.SetContent(strings.Join(m.messages, "\n"))
				m.viewport.GotoBottom()
				return m, nil
			}

			// Regular message to ADK
			return m, func() tea.Msg {
				resp, err := m.sim.SendText(context.Background(), input)
				if err != nil {
					return err
				}
				return resp
			}
		}

	case string: // Response from ADK
		m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render("Agent: ")+msg)
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case error:
		m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("Error: ")+msg.Error())
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m model) View() string {
	return fmt.Sprintf(
		"Sender: %s\n\n%s\n\n%s",
		m.sim.UserID(),
		m.viewport.View(),
		m.textarea.View(),
	) + "\n\n"
}
