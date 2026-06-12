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
	"time"

	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/store"
)

type Command interface {
	Name() string
	Description() string
	Run(ctx context.Context, s *store.Store, args []string) error
}

type exportCmd struct{}

func (c *exportCmd) Name() string        { return "export" }
func (c *exportCmd) Description() string { return "Export database contents to a JSONL file" }

type importCmd struct{}

func (c *importCmd) Name() string        { return "import" }
func (c *importCmd) Description() string { return "Import database contents from a JSONL file" }

// JSONL format structures
type ExportRecord struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type BlacklistData struct {
	Phone     string    `json:"phone"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

type ContactData struct {
	OurJID       string `json:"our_jid"`
	TheirJID     string `json:"their_jid"`
	FullName     string `json:"full_name"`
	ShortName    string `json:"short_name"`
	PushName     string `json:"push_name"`
	BusinessName string `json:"business_name"`
}

type CommandData struct {
	ID        int64           `json:"id"`
	Command   string          `json:"command"`
	Payload   json.RawMessage `json:"payload"`
	Status    string          `json:"status"`
	Result    json.RawMessage `json:"result"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type FileData struct {
	Path      string    `json:"path"`
	Metadata  *string   `json:"metadata,omitempty"`
	Content   string    `json:"content,omitempty"` // Base64 encoded
	Timestamp time.Time `json:"timestamp"`
}

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

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmdName := os.Args[1]
	var cmd Command
	switch cmdName {
	case "export":
		cmd = &exportCmd{}
	case "import":
		cmd = &importCmd{}
	default:
		fmt.Printf("Error: unknown command %q\n", cmdName)
		printUsage()
		os.Exit(1)
	}

	// Filter os.Args temporarily so config.Load() (which calls flag.Parse())
	// only parses the -config flag if it exists, ignoring subcommand-specific flags.
	origArgs := os.Args
	var filteredArgs []string
	filteredArgs = append(filteredArgs, origArgs[0])
	for i := 1; i < len(origArgs); i++ {
		if origArgs[i] == "-config" {
			if i+1 < len(origArgs) {
				filteredArgs = append(filteredArgs, "-config", origArgs[i+1])
				i++
			}
		}
	}
	os.Args = filteredArgs

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Restore original os.Args
	os.Args = origArgs

	// Open the database store using the configured DatabaseURL
	s, err := store.Open(cfg.Verification.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to open database store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	// Run the subcommand with its arguments (excluding program name and subcommand name)
	if err := cmd.Run(ctx, s, os.Args[2:]); err != nil {
		log.Fatalf("Command failed: %v", err)
	}

	fmt.Println("Success!")
}

func printUsage() {
	fmt.Println("Usage: dbutil <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  export   Export database contents to a JSONL file")
	fmt.Println("  import   Import database contents from a JSONL file")
	fmt.Println("Use 'dbutil <command> -help' for command options.")
}
