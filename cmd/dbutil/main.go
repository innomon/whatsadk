// Package main provides the dbutil CLI utility for database export and import operations.
//
// It exports and imports WhatsApp database records (contacts, message logs, blacklist,
// and command queue entries) to and from JSON Lines (JSONL) files, enabling backups and
// seamless migrations between PostgreSQL, SurrealDB, and SQLite-P2P.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/store"
)

// Command defines the interface for a dbutil CLI subcommand.
type Command interface {
	Name() string
	Description() string
	Run(ctx context.Context, s *store.Store, args []string) error
}

// CommandRegistry manages available dbutil commands.
type CommandRegistry struct {
	commands map[string]Command
	order    []string
}

// NewCommandRegistry creates a registry initialized with default dbutil subcommands.
func NewCommandRegistry() *CommandRegistry {
	r := &CommandRegistry{
		commands: make(map[string]Command),
	}
	r.Register(&exportCmd{})
	r.Register(&importCmd{})
	return r
}

// Register registers a subcommand with the registry.
func (r *CommandRegistry) Register(cmd Command) {
	r.commands[cmd.Name()] = cmd
	r.order = append(r.order, cmd.Name())
}

// Get retrieves a command by its name.
func (r *CommandRegistry) Get(name string) (Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

// Commands returns all registered commands in registration order.
func (r *CommandRegistry) Commands() []Command {
	var list []Command
	for _, name := range r.order {
		if cmd, ok := r.commands[name]; ok {
			list = append(list, cmd)
		}
	}
	return list
}

// exportCmd implements the export subcommand.
type exportCmd struct{}

// Name returns the subcommand name "export".
func (c *exportCmd) Name() string { return "export" }

// Description returns a brief summary of the export command.
func (c *exportCmd) Description() string { return "Export database contents to a JSONL file" }

// importCmd implements the import subcommand.
type importCmd struct{}

// Name returns the subcommand name "import".
func (c *importCmd) Name() string { return "import" }

// Description returns a brief summary of the import command.
func (c *importCmd) Description() string { return "Import database contents from a JSONL file" }

// ExportRecord represents a single typed record envelope in a JSONL file.
type ExportRecord struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// BlacklistData represents exported phone blacklist entries.
type BlacklistData struct {
	Phone     string    `json:"phone"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// ContactData represents exported WhatsApp contact roster entries.
type ContactData struct {
	OurJID       string `json:"our_jid"`
	TheirJID     string `json:"their_jid"`
	FullName     string `json:"full_name"`
	ShortName    string `json:"short_name"`
	PushName     string `json:"push_name"`
	BusinessName string `json:"business_name"`
}

// CommandData represents exported asynchronous WhatsApp command entries.
type CommandData struct {
	ID        int64           `json:"id"`
	Command   string          `json:"command"`
	Payload   json.RawMessage `json:"payload"`
	Status    string          `json:"status"`
	Result    json.RawMessage `json:"result"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// FileData represents exported virtual file system log and media entries.
type FileData struct {
	Path      string    `json:"path"`
	Metadata  *string   `json:"metadata,omitempty"`
	Content   string    `json:"content,omitempty"` // Base64 encoded
	Timestamp time.Time `json:"timestamp"`
}

// Run executes the database export command.
func (c *exportCmd) Run(ctx context.Context, s *store.Store, args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	outPath := fs.String("out", "export.jsonl", "Output path for JSONL export (use '-' for stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var writer io.Writer
	if *outPath == "-" {
		writer = os.Stdout
	} else {
		f, err := os.Create(*outPath)
		if err != nil {
			return fmt.Errorf("create export file: %w", err)
		}
		defer f.Close()
		writer = f
	}

	bufferedWriter := bufio.NewWriter(writer)
	defer bufferedWriter.Flush()

	// 1. Export blacklist
	blacklist, err := s.ListBlacklist(ctx)
	if err != nil {
		return fmt.Errorf("export blacklist: %w", err)
	}
	for _, b := range blacklist {
		data, err := json.Marshal(BlacklistData{
			Phone:     b.Phone,
			Reason:    b.Reason,
			CreatedAt: b.CreatedAt,
		})
		if err != nil {
			return fmt.Errorf("marshal blacklist item: %w", err)
		}
		rec := ExportRecord{Type: "blacklist", Data: data}
		line, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshal record: %w", err)
		}
		if _, err := bufferedWriter.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("write record: %w", err)
		}
	}

	// 2. Export contacts
	contacts, err := s.GetAllContacts(ctx)
	if err != nil {
		return fmt.Errorf("export contacts: %w", err)
	}
	for _, ct := range contacts {
		data, err := json.Marshal(ContactData{
			OurJID:       ct.OurJID,
			TheirJID:     ct.TheirJID,
			FullName:     ct.FullName,
			ShortName:    ct.ShortName,
			PushName:     ct.PushName,
			BusinessName: ct.BusinessName,
		})
		if err != nil {
			return fmt.Errorf("marshal contact item: %w", err)
		}
		rec := ExportRecord{Type: "contact", Data: data}
		line, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshal record: %w", err)
		}
		if _, err := bufferedWriter.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("write record: %w", err)
		}
	}

	// 3. Export commands
	commands, err := s.GetAllCommands(ctx)
	if err != nil {
		return fmt.Errorf("export commands: %w", err)
	}
	for _, cmd := range commands {
		data, err := json.Marshal(CommandData{
			ID:        cmd.ID,
			Command:   cmd.Command,
			Payload:   cmd.Payload,
			Status:    cmd.Status,
			Result:    cmd.Result,
			CreatedAt: cmd.CreatedAt,
			UpdatedAt: cmd.UpdatedAt,
		})
		if err != nil {
			return fmt.Errorf("marshal command item: %w", err)
		}
		rec := ExportRecord{Type: "command", Data: data}
		line, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshal record: %w", err)
		}
		if _, err := bufferedWriter.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("write record: %w", err)
		}
	}

	// 4. Export filesys
	files, err := s.GetAllFiles(ctx)
	if err != nil {
		return fmt.Errorf("export files: %w", err)
	}
	for _, file := range files {
		var metaStr *string
		if file.Metadata.Valid {
			metaStr = &file.Metadata.String
		}
		data, err := json.Marshal(FileData{
			Path:      file.Path,
			Metadata:  metaStr,
			Content:   base64.StdEncoding.EncodeToString(file.Content),
			Timestamp: file.Timestamp,
		})
		if err != nil {
			return fmt.Errorf("marshal file item: %w", err)
		}
		rec := ExportRecord{Type: "file", Data: data}
		line, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshal record: %w", err)
		}
		if _, err := bufferedWriter.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("write record: %w", err)
		}
	}

	return nil
}

// Run executes the database import command from a JSONL file or standard input.
func (c *importCmd) Run(ctx context.Context, s *store.Store, args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	inPath := fs.String("in", "export.jsonl", "Input path for JSONL import (use '-' for stdin)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var reader io.Reader
	if *inPath == "-" {
		reader = os.Stdin
	} else {
		f, err := os.Open(*inPath)
		if err != nil {
			return fmt.Errorf("open import file: %w", err)
		}
		defer f.Close()
		reader = f
	}

	scanner := bufio.NewScanner(reader)
	const maxCapacity = 10 * 1024 * 1024 // 10MB limit for long lines (e.g. base64 files)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxCapacity)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec ExportRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return fmt.Errorf("line %d: parse record: %w", lineNum, err)
		}

		switch rec.Type {
		case "blacklist":
			var b BlacklistData
			if err := json.Unmarshal(rec.Data, &b); err != nil {
				return fmt.Errorf("line %d: parse blacklist data: %w", lineNum, err)
			}
			if err := s.AddBlacklist(ctx, b.Phone, b.Reason); err != nil {
				return fmt.Errorf("line %d: import blacklist: %w", lineNum, err)
			}
		case "contact":
			var ct ContactData
			if err := json.Unmarshal(rec.Data, &ct); err != nil {
				return fmt.Errorf("line %d: parse contact data: %w", lineNum, err)
			}
			err := s.PutContact(ctx, store.Contact{
				OurJID:       ct.OurJID,
				TheirJID:     ct.TheirJID,
				FullName:     ct.FullName,
				ShortName:    ct.ShortName,
				PushName:     ct.PushName,
				BusinessName: ct.BusinessName,
			})
			if err != nil {
				return fmt.Errorf("line %d: import contact: %w", lineNum, err)
			}
		case "command":
			var cmd CommandData
			if err := json.Unmarshal(rec.Data, &cmd); err != nil {
				return fmt.Errorf("line %d: parse command data: %w", lineNum, err)
			}
			err := s.PutCommand(ctx, store.Command{
				ID:        cmd.ID,
				Command:   cmd.Command,
				Payload:   cmd.Payload,
				Status:    cmd.Status,
				Result:    cmd.Result,
				CreatedAt: cmd.CreatedAt,
				UpdatedAt: cmd.UpdatedAt,
			})
			if err != nil {
				return fmt.Errorf("line %d: import command: %w", lineNum, err)
			}
		case "file":
			var fd FileData
			if err := json.Unmarshal(rec.Data, &fd); err != nil {
				return fmt.Errorf("line %d: parse file data: %w", lineNum, err)
			}
			content, err := base64.StdEncoding.DecodeString(fd.Content)
			if err != nil {
				return fmt.Errorf("line %d: decode base64 content: %w", lineNum, err)
			}
			var meta interface{}
			if fd.Metadata != nil {
				meta = *fd.Metadata
			}
			if err := s.PutFile(ctx, fd.Path, meta, content, fd.Timestamp); err != nil {
				return fmt.Errorf("line %d: import file: %w", lineNum, err)
			}
		default:
			return fmt.Errorf("line %d: unknown record type %q", lineNum, rec.Type)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan input: %w", err)
	}

	// Reset sequence counters (if applicable) after importing commands
	if err := s.ResetSequence(ctx); err != nil {
		return fmt.Errorf("reset database sequence: %w", err)
	}

	return nil
}

// parsedCLI stores the parsed command-line arguments.
type parsedCLI struct {
	commandName string
	configFile  string
	dbPath      string
	dsn         string
	subArgs     []string
	showHelp    bool
}

// parseCLIArgs extracts global options and separates the subcommand and its arguments.
func parseCLIArgs(args []string) *parsedCLI {
	p := &parsedCLI{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-h" || arg == "--help" || arg == "-help" || arg == "help" {
			if p.commandName == "" {
				p.showHelp = true
				return p
			}
			p.subArgs = append(p.subArgs, "-help")
			continue
		}
		if arg == "-config" || arg == "--config" {
			if i+1 < len(args) {
				p.configFile = args[i+1]
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "-config=") || strings.HasPrefix(arg, "--config=") {
			parts := strings.SplitN(arg, "=", 2)
			p.configFile = parts[1]
			continue
		}
		if arg == "-db" || arg == "--db" {
			if i+1 < len(args) {
				p.dbPath = args[i+1]
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "-db=") || strings.HasPrefix(arg, "--db=") {
			parts := strings.SplitN(arg, "=", 2)
			p.dbPath = parts[1]
			continue
		}
		if arg == "-dsn" || arg == "--dsn" {
			if i+1 < len(args) {
				p.dsn = args[i+1]
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "-dsn=") || strings.HasPrefix(arg, "--dsn=") {
			parts := strings.SplitN(arg, "=", 2)
			p.dsn = parts[1]
			continue
		}
		if !strings.HasPrefix(arg, "-") && p.commandName == "" {
			p.commandName = arg
			continue
		}
		p.subArgs = append(p.subArgs, arg)
	}
	return p
}

// openDatabase opens the database store based on CLI arguments or loaded configuration.
func openDatabase(cli *parsedCLI) (*store.Store, error) {
	if cli.dbPath != "" {
		return store.Open(cli.dbPath)
	}
	if cli.dsn != "" {
		return store.Open(cli.dsn)
	}

	if cli.configFile != "" {
		os.Setenv("CONFIG_FILE", cli.configFile)
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	if cfg.P2P.Enabled {
		nodeCfg := cfg.P2P.ToNodeConfig()
		s, err := store.OpenP2PFromNodeConfig(nodeCfg)
		if err == nil {
			fmt.Printf("🌐 SQLite P2P Mesh enabled (Node: %s, Topic: %s)\n", cfg.P2P.NodeID, cfg.P2P.SwarmTopic)
			return s, nil
		}
		// If full P2P mesh cannot start (e.g. swarm port in use by active gateway or hypercore feed locked),
		// fall back to direct local SQLite P2P storage in WAL mode.
		dbPath := cfg.P2P.DBPath
		if dbPath == "" {
			dbPath = cfg.Verification.DatabaseURL
		}
		if dbPath == "" {
			dbPath = cfg.WhatsApp.StoreDSN
		}
		if dbPath != "" {
			s, openErr := store.Open(dbPath)
			if openErr == nil {
				fmt.Println("ℹ️  Opened SQLite P2P database in local direct mode (concurrent with active gateway)")
				return s, nil
			}
		}
		return nil, fmt.Errorf("open p2p store: %w", err)
	}

	dbURL := cfg.Verification.DatabaseURL
	if dbURL == "" {
		dbURL = cfg.WhatsApp.StoreDSN
	}
	return store.Open(dbURL)
}

// main is the entry point for the dbutil CLI tool.
func main() {
	cli := parseCLIArgs(os.Args[1:])
	if cli.showHelp || cli.commandName == "" {
		printUsage()
		if cli.commandName == "" && !cli.showHelp {
			os.Exit(1)
		}
		return
	}

	registry := NewCommandRegistry()
	cmd, ok := registry.Get(cli.commandName)
	if !ok {
		fmt.Printf("Error: unknown command %q\n", cli.commandName)
		printUsage()
		os.Exit(1)
	}

	// If subcommand help is requested, run subcommand directly without opening the database.
	for _, a := range cli.subArgs {
		if a == "-h" || a == "-help" || a == "--help" {
			_ = cmd.Run(context.Background(), nil, cli.subArgs)
			return
		}
	}

	s, err := openDatabase(cli)
	if err != nil {
		log.Fatalf("Failed to open database store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	if err := cmd.Run(ctx, s, cli.subArgs); err != nil {
		log.Fatalf("Command failed: %v", err)
	}

	fmt.Println("Success!")
}

// printUsage outputs usage instructions for the dbutil CLI tool.
func printUsage() {
	fmt.Println("Usage: dbutil [options] <command> [command options]")
	fmt.Println("")
	fmt.Println("Commands:")
	registry := NewCommandRegistry()
	for _, cmd := range registry.Commands() {
		fmt.Printf("  %-8s %s\n", cmd.Name(), cmd.Description())
	}
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -config <path>   Path to config.yaml (default: auto-detected)")
	fmt.Println("  -db <path>       Direct path to SQLite database file")
	fmt.Println("  -dsn <url>       Database connection DSN (sqlite://, postgres://, surrealdb://)")
	fmt.Println("  -help, -h        Show help")
	fmt.Println("")
	fmt.Println("Use 'dbutil <command> -help' for command-specific options.")
}
