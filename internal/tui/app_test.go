package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/innomon/whatsadk/internal/config"
)

func TestAppModel_CommandHistory(t *testing.T) {
	model := NewAppModel(&config.Config{}, nil)

	// Execute two lines
	model.executeLine("help")
	model.executeLine("sql query=\"SELECT 1\"")

	if len(model.cmdHistory) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(model.cmdHistory))
	}

	// Press Up Arrow (should select "sql query=\"SELECT 1\"")
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyUp})
	m := newModel.(*AppModel)
	if m.input.Value() != "sql query=\"SELECT 1\"" {
		t.Errorf("expected 'sql query=\"SELECT 1\"', got %q", m.input.Value())
	}

	// Press Up Arrow again (should select "help")
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(*AppModel)
	if m.input.Value() != "help" {
		t.Errorf("expected 'help', got %q", m.input.Value())
	}

	// Press Down Arrow (should select "sql query=\"SELECT 1\"")
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(*AppModel)
	if m.input.Value() != "sql query=\"SELECT 1\"" {
		t.Errorf("expected 'sql query=\"SELECT 1\"', got %q", m.input.Value())
	}

	// Press Down Arrow again (should restore empty draft)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(*AppModel)
	if m.input.Value() != "" {
		t.Errorf("expected empty draft, got %q", m.input.Value())
	}
}

func TestAppModel_RenderMarkdown(t *testing.T) {
	md := "### Title\n- Item 1\n- Item 2"
	rendered := renderMarkdown(md, 80)

	if rendered == "" {
		t.Fatalf("expected non-empty rendered markdown")
	}
}
