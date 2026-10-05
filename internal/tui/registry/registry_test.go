package registry

import (
	"context"
	"testing"

	"github.com/innomon/whatsadk/internal/store"
)

func TestRegistry_ParseLine(t *testing.T) {
	tests := []struct {
		input       string
		expectedCmd string
		expectedLen int
	}{
		{"sql query=\"SELECT * FROM filesys\"", "sql", 1},
		{"/send jid=123@s.whatsapp.net text=hello", "send", 2},
		{"GET_GROUPS", "get_groups", 0},
		{"  jid_to_phone   15551234567@s.whatsapp.net  ", "jid_to_phone", 1},
	}

	for _, tt := range tests {
		cmd, args := ParseLine(tt.input)
		if cmd != tt.expectedCmd {
			t.Errorf("ParseLine(%q) cmd = %q, want %q", tt.input, cmd, tt.expectedCmd)
		}
		if len(args) != tt.expectedLen {
			t.Errorf("ParseLine(%q) args len = %d, want %d", tt.input, len(args), tt.expectedLen)
		}
	}
}

func TestRegistry_ParseParams(t *testing.T) {
	defs := []ParamDef{
		{Name: "limit", Type: "int", Value: "10", Help: "Max items"},
		{Name: "verbose", Type: "bool", Value: "true", Help: "Verbose output"},
		{Name: "query", Type: "string", Help: "SQL query"},
	}

	raw := map[string]string{
		"limit": "25",
		"query": "SELECT 1",
	}

	parsed, missing, err := ParseParams(defs, raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("expected 0 missing params, got %d", len(missing))
	}
	if parsed["limit"] != 25 {
		t.Errorf("expected limit=25, got %v", parsed["limit"])
	}
	if parsed["verbose"] != true {
		t.Errorf("expected verbose=true from prefilled default, got %v", parsed["verbose"])
	}
	if parsed["query"] != "SELECT 1" {
		t.Errorf("expected query='SELECT 1', got %v", parsed["query"])
	}
}

func TestRegistry_Execution(t *testing.T) {
	reg := NewRegistry()
	reg.Register("echo", func(ctx context.Context, s *store.Store, params map[string]any) (Result, error) {
		msg, _ := params["msg"].(string)
		return Result{Type: ResultTypeText, Content: "Echo: " + msg}, nil
	})

	handler, ok := reg.GetHandler("ECHO")
	if !ok {
		t.Fatalf("expected handler for ECHO")
	}

	res, err := handler(context.Background(), nil, map[string]any{"msg": "Hello World"})
	if err != nil {
		t.Fatalf("handler execution failed: %v", err)
	}
	if res.Content != "Echo: Hello World" {
		t.Errorf("unexpected content: %s", res.Content)
	}
}
