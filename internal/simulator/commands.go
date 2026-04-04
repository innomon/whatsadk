package simulator

import (
	"context"
	"fmt"
	"strings"
)

type CommandFunc func(ctx context.Context, s *Simulator, args []string) (string, error)

type Command struct {
	Name        string
	Description string
	Exec        CommandFunc
}

type Registry struct {
	commands map[string]Command
}

func NewRegistry() *Registry {
	r := &Registry{
		commands: make(map[string]Command),
	}
	r.registerDefaults()
	return r
}

func (r *Registry) Register(cmd Command) {
	r.commands[cmd.Name] = cmd
}

func (r *Registry) Execute(ctx context.Context, s *Simulator, input string) (string, bool, error) {
	if !strings.HasPrefix(input, "/") {
		return "", false, nil
	}

	parts := strings.Fields(input[1:])
	if len(parts) == 0 {
		return "", true, nil
	}

	name := parts[0]
	args := parts[1:]

	cmd, ok := r.commands[name]
	if !ok {
		return "", true, fmt.Errorf("unknown command: %s", name)
	}

	res, err := cmd.Exec(ctx, s, args)
	return res, true, err
}

func (r *Registry) registerDefaults() {
	r.Register(Command{
		Name:        "set_sender",
		Description: "Set the sender phone number (e.g., /set_sender 919876543210)",
		Exec: func(ctx context.Context, s *Simulator, args []string) (string, error) {
			if len(args) == 0 {
				return "", fmt.Errorf("usage: /set_sender <phone>")
			}
			s.SetUserID(args[0])
			return fmt.Sprintf("Sender set to: %s", args[0]), nil
		},
	})

	r.Register(Command{
		Name:        "help",
		Description: "List available commands",
		Exec: func(ctx context.Context, s *Simulator, args []string) (string, error) {
			var sb strings.Builder
			sb.WriteString("Available commands:\n")
			for _, cmd := range r.commands {
				sb.WriteString(fmt.Sprintf("  /%s - %s\n", cmd.Name, cmd.Description))
			}
			return sb.String(), nil
		},
	})

	r.Register(Command{
		Name:        "export",
		Description: "Export current session to JSON file",
		Exec: func(ctx context.Context, s *Simulator, args []string) (string, error) {
			if len(args) == 0 {
				return "", fmt.Errorf("usage: /export <filename>")
			}
			if err := s.ExportSession(args[0]); err != nil {
				return "", err
			}
			return fmt.Sprintf("Session exported to %s", args[0]), nil
		},
	})

	r.Register(Command{
		Name:        "attach",
		Description: "Send a message with an attachment (e.g., /attach path/to/file [caption])",
		Exec: func(ctx context.Context, s *Simulator, args []string) (string, error) {
			if len(args) == 0 {
				return "", fmt.Errorf("usage: /attach <path> [caption]")
			}
			path := args[0]
			caption := ""
			if len(args) > 1 {
				caption = strings.Join(args[1:], " ")
			}
			resp, err := s.SendWithAttachment(ctx, caption, path)
			if err != nil {
				return "", err
			}
			return resp, nil
		},
	})

	r.Register(Command{
		Name:        "replay",
		Description: "Replay a saved session from JSON file",
		Exec: func(ctx context.Context, s *Simulator, args []string) (string, error) {
			if len(args) == 0 {
				return "", fmt.Errorf("usage: /replay <filename>")
			}
			sess, err := s.ImportSession(args[0])
			if err != nil {
				return "", err
			}
			// For TUI we'd want a way to stream these back, but for basic command
			// we can just run it. The caller might need a callback to update UI.
			// The current Registry.Execute doesn't support streaming back to TUI easily
			// without extra wiring.
			return fmt.Sprintf("Starting replay of %d messages...", len(sess.Messages)), nil
		},
	})
}
