package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/innomon/whatsadk/internal/agent"
	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/store"
)

type SendMessageArgs struct {
	JID   string             `json:"jid"`
	Text  string             `json:"text,omitempty"`
	Media []agent.InlineData `json:"media,omitempty"`
}

func SendMessage(ctx context.Context, s *store.Store, args SendMessageArgs) (*mcp.CallToolResult, any, error) {
	if args.JID == "" {
		return nil, nil, fmt.Errorf("jid is required")
	}
	if args.Text == "" && len(args.Media) == 0 {
		return nil, nil, fmt.Errorf("either text or media is required")
	}

	cmdID, err := s.EnqueueCommand(ctx, "send_message", args)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to enqueue send_message: %w", err)
	}

	cmd, err := s.WaitForCommand(ctx, cmdID, 15*time.Second)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "Message enqueued but Gateway response timed out. It will be sent when Gateway is online.",
				},
			},
		}, nil, nil
	}

	if cmd.Status == "failed" {
		return nil, nil, fmt.Errorf("failed to send message: %s", string(cmd.Result))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Message sent to %s. Status: %s. Result: %s", args.JID, cmd.Status, string(cmd.Result)),
			},
		},
	}, nil, nil
}

type GetRecentMessagesArgs struct {
	JID   string `json:"jid,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

func GetRecentMessages(ctx context.Context, s *store.Store, args GetRecentMessagesArgs) (*mcp.CallToolResult, any, error) {
	limit := args.Limit
	if limit <= 0 {
		limit = 20
	}

	var logs []store.FileEntry
	var err error

	if args.JID != "" {
		logs, err = s.GetFilesysLogs(ctx, args.JID, limit)
	} else {
		logs, err = s.GetLatestGlobalMessages(ctx, limit)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("failed to get messages: %w", err)
	}

	data, _ := json.MarshalIndent(logs, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(data),
			},
		},
	}, nil, nil
}

type BlacklistAddArgs struct {
	Phone  string `json:"phone"`
	Reason string `json:"reason"`
}

func BlacklistAdd(ctx context.Context, s *store.Store, args BlacklistAddArgs) (*mcp.CallToolResult, any, error) {
	// 1. Local Shadow Ban
	if err := s.AddBlacklist(ctx, args.Phone, args.Reason); err != nil {
		return nil, nil, fmt.Errorf("failed to add to local blacklist: %w", err)
	}

	// 2. Remote WhatsApp Block
	cmdID, err := s.EnqueueCommand(ctx, "block", map[string]string{"jid": args.Phone})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to enqueue remote block: %w", err)
	}

	// Wait for result
	cmd, err := s.WaitForCommand(ctx, cmdID, 10*time.Second)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Added %s to local blacklist, but remote block timed out. It will be processed when Gateway is online.", args.Phone),
				},
			},
		}, nil, nil
	}

	if cmd.Status == "failed" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Added %s to local blacklist, but remote block failed: %s", args.Phone, string(cmd.Result)),
				},
			},
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Successfully blacklisted %s (Local & Remote)", args.Phone),
			},
		},
	}, nil, nil
}

type BlacklistRemoveArgs struct {
	Phone string `json:"phone"`
}

func BlacklistRemove(ctx context.Context, s *store.Store, args BlacklistRemoveArgs) (*mcp.CallToolResult, any, error) {
	// 1. Local Removal
	if err := s.RemoveBlacklist(ctx, args.Phone); err != nil {
		return nil, nil, fmt.Errorf("failed to remove from local blacklist: %w", err)
	}

	// 2. Remote WhatsApp Unblock
	cmdID, err := s.EnqueueCommand(ctx, "unblock", map[string]string{"jid": args.Phone})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to enqueue remote unblock: %w", err)
	}

	// Wait for result
	cmd, err := s.WaitForCommand(ctx, cmdID, 10*time.Second)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Removed %s from local blacklist, but remote unblock timed out.", args.Phone),
				},
			},
		}, nil, nil
	}

	if cmd.Status == "failed" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Removed %s from local blacklist, but remote unblock failed: %s", args.Phone, string(cmd.Result)),
				},
			},
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Successfully removed %s from blacklist (Local & Remote)", args.Phone),
			},
		},
	}, nil, nil
}

type BlacklistGetRemoteArgs struct{}

func BlacklistGetRemote(ctx context.Context, s *store.Store, _ BlacklistGetRemoteArgs) (*mcp.CallToolResult, any, error) {
	cmdID, err := s.EnqueueCommand(ctx, "get_blocklist", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to enqueue get_blocklist: %w", err)
	}

	cmd, err := s.WaitForCommand(ctx, cmdID, 10*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("request timed out: %w", err)
	}

	if cmd.Status == "failed" {
		return nil, nil, fmt.Errorf("remote request failed: %s", string(cmd.Result))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(cmd.Result),
			},
		},
	}, nil, nil
}

type QueryContactsArgs struct {
	Query string `json:"query"`
}

func QueryContacts(ctx context.Context, s *store.Store, args QueryContactsArgs) (*mcp.CallToolResult, any, error) {
	contacts, err := s.ListContacts(ctx, args.Query)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query contacts: %w", err)
	}
	data, _ := json.MarshalIndent(contacts, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(data),
			},
		},
	}, nil, nil
}

type GetLogsArgs struct {
	Phone string `json:"phone"`
	Limit int    `json:"limit"`
}

func GetLogs(ctx context.Context, s *store.Store, args GetLogsArgs) (*mcp.CallToolResult, any, error) {
	limit := args.Limit
	if limit <= 0 {
		limit = 10
	}
	logs, err := s.GetFilesysLogs(ctx, args.Phone, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get logs: %w", err)
	}
	data, _ := json.MarshalIndent(logs, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(data),
			},
		},
	}, nil, nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	s, err := store.Open(cfg.WhatsApp.StoreDSN)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "whatsadk",
		Version: "1.0.0",
	}, nil)

	// Tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "blacklist_add",
		Description: "Add a phone number or JID to the global blacklist (Local Shadow Ban + Remote WhatsApp Block)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args BlacklistAddArgs) (*mcp.CallToolResult, any, error) {
		return BlacklistAdd(ctx, s, args)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "blacklist_remove",
		Description: "Remove a phone number or JID from the global blacklist (Local Shadow Ban + Remote WhatsApp Unblock)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args BlacklistRemoveArgs) (*mcp.CallToolResult, any, error) {
		return BlacklistRemove(ctx, s, args)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "blacklist_get_remote",
		Description: "Fetch the official blocklist from WhatsApp servers",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args BlacklistGetRemoteArgs) (*mcp.CallToolResult, any, error) {
		return BlacklistGetRemote(ctx, s, args)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_contacts",
		Description: "Search WhatsApp contacts by name or JID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args QueryContactsArgs) (*mcp.CallToolResult, any, error) {
		return QueryContacts(ctx, s, args)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_message_logs",
		Description: "Retrieve recent message logs for a specific user (Alias for get_recent_messages)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetRecentMessagesArgs) (*mcp.CallToolResult, any, error) {
		return GetRecentMessages(ctx, s, args)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_recent_messages",
		Description: "Retrieve recent message logs globally or for a specific user. Chronological order.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetRecentMessagesArgs) (*mcp.CallToolResult, any, error) {
		return GetRecentMessages(ctx, s, args)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "send_message",
		Description: "Send a multi-modal message (text and/or media) to a WhatsApp user",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SendMessageArgs) (*mcp.CallToolResult, any, error) {
		return SendMessage(ctx, s, args)
	})

	transport := &mcp.StdioTransport{}
	session, err := server.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("Failed to connect server: %v", err)
	}

	// Wait for the session to finish (client disconnects or error)
	session.Wait()
}
