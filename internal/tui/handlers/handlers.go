package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/innomon/whatsadk/internal/store"
	"github.com/innomon/whatsadk/internal/tui/registry"
)

// RegisterAllHandlers registers all default handcrafted handlers (MCP tools + SQL execution).
func RegisterAllHandlers(reg *registry.Registry) {
	// 1. SQL Handler
	reg.Register("sql_exec", HandleSQLExec)
	reg.Register("sql", HandleSQLExec)

	// 2. MCP Tools
	reg.Register("send_message", HandleSendMessage)
	reg.Register("query_contacts", HandleQueryContacts)
	reg.Register("jid_to_phone", HandleJIDToPhone)
	reg.Register("get_phone_from_jid", HandleJIDToPhone)
	reg.Register("list_groups", HandleGetGroups)
	reg.Register("get_groups", HandleGetGroups)
	reg.Register("get_joined_groups", HandleGetGroups)
	reg.Register("list_group_members", HandleGetGroupInfo)
	reg.Register("get_group_info", HandleGetGroupInfo)
	reg.Register("get_recent_messages", HandleGetRecentMessages)
	reg.Register("get_message_logs", HandleGetRecentMessages)
	reg.Register("blacklist_add", HandleBlacklistAdd)
	reg.Register("blacklist_remove", HandleBlacklistRemove)
	reg.Register("blacklist_get_remote", HandleBlacklistGetRemote)
	reg.Register("get_database_type", HandleGetDatabaseType)
	reg.Register("filesys_sql_select", HandleFileSysSQLSelect)
	reg.Register("filesys_put", HandleFileSysPut)
	reg.Register("filesys_get", HandleFileSysGet)
	reg.Register("filesys_delete", HandleFileSysDelete)
	reg.Register("filesys_list", HandleFileSysList)
}

// DefaultCommands returns default CommandDef definitions for config initialization or fallback.
func DefaultCommands() []registry.CommandDef {
	return []registry.CommandDef{
		{
			Name:    "sql",
			Handler: "sql_exec",
			Help:    "Execute custom SQL queries against the local/P2P filesys table",
			Params: []registry.ParamDef{
				{Name: "query", Type: "string", Help: "SQL query string (e.g. SELECT * FROM filesys LIMIT 5)"},
			},
		},
		{
			Name:    "send_message",
			Handler: "send_message",
			Help:    "Send a multi-modal message to a WhatsApp user or group JID",
			Params: []registry.ParamDef{
				{Name: "jid", Type: "string", Help: "Recipient WhatsApp JID (e.g. 15551234567@s.whatsapp.net)"},
				{Name: "text", Type: "string", Help: "Message text body"},
			},
		},
		{
			Name:    "query_contacts",
			Handler: "query_contacts",
			Help:    "Search WhatsApp contacts by name or JID",
			Params: []registry.ParamDef{
				{Name: "query", Type: "string", Help: "Search string for contact name or JID"},
			},
		},
		{
			Name:    "jid_to_phone",
			Handler: "jid_to_phone",
			Help:    "Extract international phone number (E.164) and details from a WhatsApp JID",
			Params: []registry.ParamDef{
				{Name: "jid", Type: "string", Help: "WhatsApp JID (e.g. 15551234567:2@s.whatsapp.net or 1234567890@lid)"},
			},
		},
		{
			Name:    "list_groups",
			Handler: "list_groups",
			Help:    "List all joined WhatsApp groups, topics, owners, and member lists",
			Params:  []registry.ParamDef{},
		},
		{
			Name:    "list_group_members",
			Handler: "list_group_members",
			Help:    "List members/participants for a specific WhatsApp group JID",
			Params: []registry.ParamDef{
				{Name: "jid", Type: "string", Help: "WhatsApp Group JID (e.g. 12036301234567890@g.us)"},
			},
		},
		{
			Name:    "get_recent_messages",
			Handler: "get_recent_messages",
			Help:    "Retrieve recent message logs globally or for a specific JID",
			Params: []registry.ParamDef{
				{Name: "jid", Type: "string", Help: "Optional user JID filter"},
				{Name: "limit", Type: "int", Value: "20", Help: "Number of recent log entries to retrieve"},
			},
		},
		{
			Name:    "blacklist_add",
			Handler: "blacklist_add",
			Help:    "Add a phone number/JID to local blacklist and block on WhatsApp",
			Params: []registry.ParamDef{
				{Name: "phone", Type: "string", Help: "Phone number or JID"},
				{Name: "reason", Type: "string", Value: "Blocked via TUI", Help: "Blacklist reason"},
			},
		},
		{
			Name:    "blacklist_remove",
			Handler: "blacklist_remove",
			Help:    "Remove a phone number/JID from local blacklist and unblock on WhatsApp",
			Params: []registry.ParamDef{
				{Name: "phone", Type: "string", Help: "Phone number or JID"},
			},
		},
		{
			Name:    "get_database_type",
			Handler: "get_database_type",
			Help:    "Discover active database backend type (postgres, surrealdb, sqlite-p2p)",
			Params:  []registry.ParamDef{},
		},
	}
}

// HandleSQLExec executes SQL queries on filesys or store.
func HandleSQLExec(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	if s == nil {
		return registry.Result{}, fmt.Errorf("store is not initialized")
	}

	query, _ := params["query"].(string)
	if strings.TrimSpace(query) == "" {
		return registry.Result{}, fmt.Errorf("query parameter is required")
	}

	results, err := s.QueryFilesys(ctx, query)
	if err != nil {
		return registry.Result{}, fmt.Errorf("sql execution failed: %w", err)
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return registry.Result{}, fmt.Errorf("failed to format sql results: %w", err)
	}

	return registry.Result{
		Type:    registry.ResultTypeMarkdown,
		Content: fmt.Sprintf("### SQL Query Result\n```json\n%s\n```", string(data)),
	}, nil
}

// HandleSendMessage sends a WhatsApp message.
func HandleSendMessage(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	if s == nil {
		return registry.Result{}, fmt.Errorf("store is not initialized")
	}

	jid, _ := params["jid"].(string)
	text, _ := params["text"].(string)

	if jid == "" {
		return registry.Result{}, fmt.Errorf("jid is required")
	}
	if text == "" {
		return registry.Result{}, fmt.Errorf("text is required")
	}

	cmdPayload := map[string]any{
		"jid":  jid,
		"text": text,
	}

	cmdID, err := s.EnqueueCommand(ctx, "send_message", cmdPayload)
	if err != nil {
		return registry.Result{}, fmt.Errorf("failed to enqueue message: %w", err)
	}

	cmd, err := s.WaitForCommand(ctx, cmdID, 15*time.Second)
	if err != nil {
		return registry.Result{
			Type:    registry.ResultTypeText,
			Content: fmt.Sprintf("Message enqueued (ID: %d) but Gateway response timed out. It will deliver when Gateway is online.", cmdID),
		}, nil
	}

	if cmd.Status == "failed" {
		return registry.Result{}, fmt.Errorf("failed to send message: %s", string(cmd.Result))
	}

	return registry.Result{
		Type:    registry.ResultTypeMarkdown,
		Content: fmt.Sprintf("✅ **Message Delivered** to `%s`\n- Command ID: `%d`\n- Status: `%s`", jid, cmdID, cmd.Status),
	}, nil
}

// HandleQueryContacts searches contacts.
func HandleQueryContacts(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	if s == nil {
		return registry.Result{}, fmt.Errorf("store is not initialized")
	}

	query, _ := params["query"].(string)
	contacts, err := s.ListContacts(ctx, query)
	if err != nil {
		return registry.Result{}, fmt.Errorf("failed to query contacts: %w", err)
	}

	data, _ := json.MarshalIndent(contacts, "", "  ")
	return registry.Result{
		Type:    registry.ResultTypeMarkdown,
		Content: fmt.Sprintf("### Contacts Found (%d)\n```json\n%s\n```", len(contacts), string(data)),
	}, nil
}

// HandleJIDToPhone converts JID to phone number details.
func HandleJIDToPhone(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	jidStr, _ := params["jid"].(string)
	jidStr = strings.TrimSpace(jidStr)
	if jidStr == "" {
		return registry.Result{}, fmt.Errorf("jid parameter is required")
	}

	parts := strings.Split(jidStr, "@")
	if len(parts) < 2 {
		raw := strings.TrimPrefix(jidStr, "+")
		isDigits := true
		for _, c := range raw {
			if c < '0' || c > '9' {
				isDigits = false
				break
			}
		}
		if isDigits && len(raw) > 0 {
			res := map[string]any{
				"jid":         raw + "@s.whatsapp.net",
				"phone":       raw,
				"e164":        "+" + raw,
				"type":        "user",
				"valid":       true,
				"description": "Parsed raw phone number string into standard user JID",
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return registry.Result{
				Type:    registry.ResultTypeMarkdown,
				Content: fmt.Sprintf("```json\n%s\n```", string(data)),
			}, nil
		}
		return registry.Result{}, fmt.Errorf("invalid JID format: missing '@'")
	}

	userPart := parts[0]
	serverPart := parts[1]
	userClean := strings.Split(userPart, ":")[0]

	res := map[string]any{
		"jid": jidStr,
	}

	switch serverPart {
	case "s.whatsapp.net", "c.us":
		res["phone"] = userClean
		res["e164"] = "+" + userClean
		res["type"] = "user"
		res["valid"] = true
		res["description"] = "Standard phone number user JID"
	case "g.us":
		res["type"] = "group"
		res["valid"] = false
		res["description"] = "JID represents a WhatsApp group chat, not an individual user."
	case "lid":
		if s != nil {
			contacts, _ := s.ListContacts(ctx, jidStr)
			if len(contacts) > 0 {
				for _, c := range contacts {
					if c.TheirJID != "" && strings.HasSuffix(c.TheirJID, "@s.whatsapp.net") {
						pnUser := strings.Split(strings.Split(c.TheirJID, "@")[0], ":")[0]
						res["phone"] = pnUser
						res["e164"] = "+" + pnUser
						res["type"] = "user"
						res["valid"] = true
						res["description"] = fmt.Sprintf("Resolved LID via cached contact (%s)", c.FullName)
						data, _ := json.MarshalIndent(res, "", "  ")
						return registry.Result{
							Type:    registry.ResultTypeMarkdown,
							Content: fmt.Sprintf("```json\n%s\n```", string(data)),
						}, nil
					}
				}
			}
		}
		res["type"] = "lid"
		res["valid"] = false
		res["description"] = "LID privacy-protected user identity."
	default:
		res["phone"] = userClean
		res["type"] = "unknown"
		res["valid"] = false
		res["description"] = fmt.Sprintf("Unknown server domain: %s", serverPart)
	}

	data, _ := json.MarshalIndent(res, "", "  ")
	return registry.Result{
		Type:    registry.ResultTypeMarkdown,
		Content: fmt.Sprintf("```json\n%s\n```", string(data)),
	}, nil
}

// HandleGetGroups fetches joined groups.
func HandleGetGroups(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	if s == nil {
		return registry.Result{}, fmt.Errorf("store is not initialized")
	}

	cmdID, err := s.EnqueueCommand(ctx, "get_groups", nil)
	if err != nil {
		return registry.Result{}, fmt.Errorf("failed to enqueue get_groups: %w", err)
	}

	cmd, err := s.WaitForCommand(ctx, cmdID, 15*time.Second)
	if err != nil {
		return registry.Result{}, fmt.Errorf("request timed out: %w", err)
	}

	if cmd.Status == "failed" {
		return registry.Result{}, fmt.Errorf("remote request failed: %s", string(cmd.Result))
	}

	return registry.Result{
		Type:    registry.ResultTypeMarkdown,
		Content: fmt.Sprintf("### WhatsApp Joined Groups\n```json\n%s\n```", string(cmd.Result)),
	}, nil
}

// HandleGetGroupInfo fetches metadata for a group JID.
func HandleGetGroupInfo(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	if s == nil {
		return registry.Result{}, fmt.Errorf("store is not initialized")
	}

	jid, _ := params["jid"].(string)
	if jid == "" {
		return registry.Result{}, fmt.Errorf("jid is required")
	}

	cmdID, err := s.EnqueueCommand(ctx, "get_group_info", map[string]string{"jid": jid})
	if err != nil {
		return registry.Result{}, fmt.Errorf("failed to enqueue get_group_info: %w", err)
	}

	cmd, err := s.WaitForCommand(ctx, cmdID, 15*time.Second)
	if err != nil {
		return registry.Result{}, fmt.Errorf("request timed out: %w", err)
	}

	if cmd.Status == "failed" {
		return registry.Result{}, fmt.Errorf("remote request failed: %s", string(cmd.Result))
	}

	return registry.Result{
		Type:    registry.ResultTypeMarkdown,
		Content: fmt.Sprintf("### Group Info (`%s`)\n```json\n%s\n```", jid, string(cmd.Result)),
	}, nil
}

// HandleGetRecentMessages retrieves recent logs.
func HandleGetRecentMessages(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	if s == nil {
		return registry.Result{}, fmt.Errorf("store is not initialized")
	}

	jid, _ := params["jid"].(string)
	limit := 20
	if l, ok := params["limit"].(int); ok && l > 0 {
		limit = l
	}

	var logs []store.FileEntry
	var err error

	if jid != "" {
		logs, err = s.GetFilesysLogs(ctx, jid, limit)
	} else {
		logs, err = s.GetLatestGlobalMessages(ctx, limit)
	}

	if err != nil {
		return registry.Result{}, fmt.Errorf("failed to get messages: %w", err)
	}

	data, _ := json.MarshalIndent(logs, "", "  ")
	return registry.Result{
		Type:    registry.ResultTypeMarkdown,
		Content: fmt.Sprintf("### Recent Messages (%d)\n```json\n%s\n```", len(logs), string(data)),
	}, nil
}

// HandleBlacklistAdd adds a user to local blacklist & blocks remotely.
func HandleBlacklistAdd(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	if s == nil {
		return registry.Result{}, fmt.Errorf("store is not initialized")
	}

	phone, _ := params["phone"].(string)
	reason, _ := params["reason"].(string)
	if phone == "" {
		return registry.Result{}, fmt.Errorf("phone parameter is required")
	}

	if err := s.AddBlacklist(ctx, phone, reason); err != nil {
		return registry.Result{}, fmt.Errorf("failed to add to local blacklist: %w", err)
	}

	cmdID, _ := s.EnqueueCommand(ctx, "block", map[string]string{"jid": phone})
	return registry.Result{
		Type:    registry.ResultTypeMarkdown,
		Content: fmt.Sprintf("🚫 **Blacklisted %s** (Local & Remote Block Enqueued ID: `%d`)", phone, cmdID),
	}, nil
}

// HandleBlacklistRemove removes user from local blacklist & unblocks remotely.
func HandleBlacklistRemove(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	if s == nil {
		return registry.Result{}, fmt.Errorf("store is not initialized")
	}

	phone, _ := params["phone"].(string)
	if phone == "" {
		return registry.Result{}, fmt.Errorf("phone parameter is required")
	}

	if err := s.RemoveBlacklist(ctx, phone); err != nil {
		return registry.Result{}, fmt.Errorf("failed to remove from local blacklist: %w", err)
	}

	cmdID, _ := s.EnqueueCommand(ctx, "unblock", map[string]string{"jid": phone})
	return registry.Result{
		Type:    registry.ResultTypeMarkdown,
		Content: fmt.Sprintf("✅ **Removed %s from Blacklist** (Remote Unblock Enqueued ID: `%d`)", phone, cmdID),
	}, nil
}

// HandleBlacklistGetRemote fetches remote blocklist.
func HandleBlacklistGetRemote(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	if s == nil {
		return registry.Result{}, fmt.Errorf("store is not initialized")
	}

	cmdID, err := s.EnqueueCommand(ctx, "get_blocklist", nil)
	if err != nil {
		return registry.Result{}, fmt.Errorf("failed to enqueue get_blocklist: %w", err)
	}

	cmd, err := s.WaitForCommand(ctx, cmdID, 10*time.Second)
	if err != nil {
		return registry.Result{}, fmt.Errorf("request timed out: %w", err)
	}

	return registry.Result{
		Type:    registry.ResultTypeMarkdown,
		Content: fmt.Sprintf("### WhatsApp Server Blocklist\n```json\n%s\n```", string(cmd.Result)),
	}, nil
}

// HandleGetDatabaseType returns database backend.
func HandleGetDatabaseType(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	if s == nil {
		return registry.Result{Type: registry.ResultTypeText, Content: "database_type: memory"}, nil
	}
	return registry.Result{
		Type:    registry.ResultTypeText,
		Content: fmt.Sprintf("database_type: %s", s.DatabaseType()),
	}, nil
}

// HandleFileSysSQLSelect executes custom SELECT on filesys.
func HandleFileSysSQLSelect(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	return HandleSQLExec(ctx, s, params)
}

// HandleFileSysPut stores entry in filesys.
func HandleFileSysPut(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	if s == nil {
		return registry.Result{}, fmt.Errorf("store is not initialized")
	}
	path, _ := params["path"].(string)
	content, _ := params["content"].(string)
	if path == "" {
		return registry.Result{}, fmt.Errorf("path is required")
	}

	err := s.PutFile(ctx, path, nil, []byte(content), time.Now())
	if err != nil {
		return registry.Result{}, fmt.Errorf("failed to put file: %w", err)
	}
	return registry.Result{
		Type:    registry.ResultTypeText,
		Content: fmt.Sprintf("Successfully stored entry at %s", path),
	}, nil
}

// HandleFileSysGet retrieves entry from filesys.
func HandleFileSysGet(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	if s == nil {
		return registry.Result{}, fmt.Errorf("store is not initialized")
	}
	path, _ := params["path"].(string)
	if path == "" {
		return registry.Result{}, fmt.Errorf("path is required")
	}

	entry, err := s.GetFile(ctx, path)
	if err != nil || entry == nil {
		return registry.Result{}, fmt.Errorf("file not found: %s", path)
	}

	data, _ := json.MarshalIndent(entry, "", "  ")
	return registry.Result{
		Type:    registry.ResultTypeMarkdown,
		Content: fmt.Sprintf("```json\n%s\n```", string(data)),
	}, nil
}

// HandleFileSysDelete deletes entry from filesys.
func HandleFileSysDelete(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	if s == nil {
		return registry.Result{}, fmt.Errorf("store is not initialized")
	}
	path, _ := params["path"].(string)
	if path == "" {
		return registry.Result{}, fmt.Errorf("path is required")
	}

	err := s.DeleteFile(ctx, path)
	if err != nil {
		return registry.Result{}, fmt.Errorf("failed to delete file: %w", err)
	}
	return registry.Result{
		Type:    registry.ResultTypeText,
		Content: fmt.Sprintf("Successfully deleted entry at %s", path),
	}, nil
}

// HandleFileSysList lists filesys entries.
func HandleFileSysList(ctx context.Context, s *store.Store, params map[string]any) (registry.Result, error) {
	if s == nil {
		return registry.Result{}, fmt.Errorf("store is not initialized")
	}
	prefix, _ := params["prefix"].(string)
	limit := 50
	if l, ok := params["limit"].(int); ok && l > 0 {
		limit = l
	}

	entries, err := s.ListFiles(ctx, prefix, limit)
	if err != nil {
		return registry.Result{}, fmt.Errorf("failed to list files: %w", err)
	}

	data, _ := json.MarshalIndent(entries, "", "  ")
	return registry.Result{
		Type:    registry.ResultTypeMarkdown,
		Content: fmt.Sprintf("### Filesys Entries (%d)\n```json\n%s\n```", len(entries), string(data)),
	}, nil
}
