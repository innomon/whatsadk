package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/innomon/whatsadk/internal/config"
	"github.com/innomon/whatsadk/internal/store"
)

type BlacklistAddArgs struct {
	Phone  string `json:"phone"`
	Reason string `json:"reason"`
}

func BlacklistAdd(ctx context.Context, s *store.Store, args BlacklistAddArgs) (*mcp.CallToolResult, any, error) {
	if err := s.AddBlacklist(ctx, args.Phone, args.Reason); err != nil {
		return nil, nil, fmt.Errorf("failed to add to blacklist: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Successfully blacklisted %s", args.Phone),
			},
		},
	}, nil, nil
}

type BlacklistRemoveArgs struct {
	Phone string `json:"phone"`
}

func BlacklistRemove(ctx context.Context, s *store.Store, args BlacklistRemoveArgs) (*mcp.CallToolResult, any, error) {
	if err := s.RemoveBlacklist(ctx, args.Phone); err != nil {
		return nil, nil, fmt.Errorf("failed to remove from blacklist: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Successfully removed %s from blacklist", args.Phone),
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
		Description: "Add a phone number or JID to the global blacklist",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args BlacklistAddArgs) (*mcp.CallToolResult, any, error) {
		return BlacklistAdd(ctx, s, args)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "blacklist_remove",
		Description: "Remove a phone number or JID from the global blacklist",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args BlacklistRemoveArgs) (*mcp.CallToolResult, any, error) {
		return BlacklistRemove(ctx, s, args)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_contacts",
		Description: "Search WhatsApp contacts by name or JID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args QueryContactsArgs) (*mcp.CallToolResult, any, error) {
		return QueryContacts(ctx, s, args)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_message_logs",
		Description: "Retrieve recent message logs for a specific user",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetLogsArgs) (*mcp.CallToolResult, any, error) {
		return GetLogs(ctx, s, args)
	})

	transport := &mcp.StdioTransport{}
	session, err := server.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("Failed to connect server: %v", err)
	}

	// Wait for the session to finish (client disconnects or error)
	session.Wait()
}
