package registry

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/innomon/whatsadk/internal/store"
)

// ResultType indicates how the TUI should render the command execution output.
type ResultType string

const (
	ResultTypeText     ResultType = "text"
	ResultTypeMarkdown ResultType = "markdown"
	ResultTypeA2UI     ResultType = "a2ui"
)

// Result represents the formatted output returned by a command handler.
type Result struct {
	Type    ResultType `json:"type"`
	Content string     `json:"content"`
}

// ParamDef defines a parameter for a command.
type ParamDef struct {
	Name  string `yaml:"name" json:"name"`
	Type  string `yaml:"type" json:"type"`   // "string", "int", "bool"
	Value string `yaml:"value" json:"value"` // optional prefilled value
	Help  string `yaml:"help" json:"help"`
}

// CommandDef defines a command schema loaded from config or default registry.
type CommandDef struct {
	Name    string     `yaml:"name" json:"name"`
	Handler string     `yaml:"handler" json:"handler"`
	Help    string     `yaml:"help" json:"help"`
	Params  []ParamDef `yaml:"params" json:"params"`
}

// HandlerFunc is the signature for pre-registered handcrafted command handlers.
type HandlerFunc func(ctx context.Context, s *store.Store, params map[string]any) (Result, error)

// Registry manages handcrafted command handlers and command lookup.
type Registry struct {
	handlers map[string]HandlerFunc
}

// NewRegistry initializes a new handcrafted command registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]HandlerFunc),
	}
}

// Register adds a handcrafted handler function to the registry.
func (r *Registry) Register(handlerName string, fn HandlerFunc) {
	r.handlers[strings.ToLower(handlerName)] = fn
}

// GetHandler retrieves a handler function by name (case-insensitive).
func (r *Registry) GetHandler(handlerName string) (HandlerFunc, bool) {
	fn, ok := r.handlers[strings.ToLower(handlerName)]
	return fn, ok
}

// ParseParams converts raw string arguments into typed parameter values (int, bool, string).
func ParseParams(defs []ParamDef, raw map[string]string) (map[string]any, []ParamDef, error) {
	parsed := make(map[string]any)
	var missing []ParamDef

	for _, p := range defs {
		valStr, exists := raw[strings.ToLower(p.Name)]
		if !exists || strings.TrimSpace(valStr) == "" {
			if strings.TrimSpace(p.Value) != "" {
				valStr = p.Value
			} else {
				missing = append(missing, p)
				continue
			}
		}

		switch strings.ToLower(p.Type) {
		case "int":
			iVal, err := strconv.Atoi(strings.TrimSpace(valStr))
			if err != nil {
				return nil, nil, fmt.Errorf("parameter %q must be an integer, got %q", p.Name, valStr)
			}
			parsed[p.Name] = iVal
		case "bool":
			bVal, err := strconv.ParseBool(strings.TrimSpace(valStr))
			if err != nil {
				return nil, nil, fmt.Errorf("parameter %q must be a boolean (true/false), got %q", p.Name, valStr)
			}
			parsed[p.Name] = bVal
		default: // "string" or empty
			parsed[p.Name] = valStr
		}
	}

	return parsed, missing, nil
}

// ParseLine parses a raw terminal input line into command name and argument key-value/positional map.
func ParseLine(line string) (string, map[string]string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", nil
	}

	// Remove leading slash if typed directly in prompt
	line = strings.TrimPrefix(line, "/")

	parts := tokenizeInput(line)
	if len(parts) == 0 {
		return "", nil
	}

	cmdName := strings.ToLower(parts[0])
	rawArgs := make(map[string]string)

	positionalIdx := 0
	for _, arg := range parts[1:] {
		if idx := strings.Index(arg, "="); idx > 0 {
			k := strings.ToLower(arg[:idx])
			v := arg[idx+1:]
			v = strings.Trim(v, "\"'")
			rawArgs[k] = v
		} else {
			rawArgs[fmt.Sprintf("pos_%d", positionalIdx)] = strings.Trim(arg, "\"'")
			positionalIdx++
		}
	}

	return cmdName, rawArgs
}

func tokenizeInput(input string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := rune(0)
	escaped := false

	for _, r := range input {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' {
			escaped = true
			continue
		}

		if r == '"' || r == '\'' {
			if inQuote == r {
				inQuote = 0
			} else if inQuote == 0 {
				inQuote = r
			} else {
				current.WriteRune(r)
			}
			continue
		}

		if (r == ' ' || r == '\t') && inQuote == 0 {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// MapPositionalToDefs maps positional arguments (pos_0, pos_1) to defined parameters in order.
func MapPositionalToDefs(defs []ParamDef, raw map[string]string) map[string]string {
	mapped := make(map[string]string)
	for k, v := range raw {
		mapped[k] = v
	}

	posIdx := 0
	for _, p := range defs {
		pKey := strings.ToLower(p.Name)
		if _, exists := mapped[pKey]; !exists {
			if posVal, posExists := mapped[fmt.Sprintf("pos_%d", posIdx)]; posExists {
				mapped[pKey] = posVal
				delete(mapped, fmt.Sprintf("pos_%d", posIdx))
				posIdx++
			}
		}
	}
	return mapped
}
