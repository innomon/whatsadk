package store

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
)

type surrealStore struct {
	db *surrealdb.DB
}

func openSurrealDB(dsn string) (storeBackend, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid surrealdb dsn: %w", err)
	}

	protocol := "ws"
	if u.Scheme == "ws" || u.Scheme == "wss" || u.Scheme == "http" || u.Scheme == "https" {
		protocol = u.Scheme
	} else if p := u.Query().Get("protocol"); p != "" {
		protocol = p
	}

	host := u.Host
	username := ""
	password := ""
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}

	ns := "whatsadk"
	dbName := "whatsadk"
	pathParts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(pathParts) > 0 && pathParts[0] != "" {
		ns = pathParts[0]
	}
	if len(pathParts) > 1 && pathParts[1] != "" {
		dbName = pathParts[1]
	}

	surrealURL := fmt.Sprintf("%s://%s", protocol, host)

	db, err := surrealdb.New(surrealURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to surrealdb: %w", err)
	}

	ctx := context.Background()
	if err := db.Use(ctx, ns, dbName); err != nil {
		db.Close(ctx)
		return nil, fmt.Errorf("failed to select namespace/database: %w", err)
	}

	if username != "" && password != "" {
		auth := surrealdb.Auth{
			Username: username,
			Password: password,
		}
		if _, err := db.SignIn(ctx, auth); err != nil {
			db.Close(ctx)
			return nil, fmt.Errorf("failed to authenticate: %w", err)
		}
	}

	return &surrealStore{db: db}, nil
}

func (s *surrealStore) Close() error {
	s.db.Close(context.Background())
	return nil
}

type surrealCommand struct {
	ID        int64     `json:"id"`
	Command   string    `json:"command"`
	Payload   string    `json:"payload"`
	Status    string    `json:"status"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toCommand(sc surrealCommand) Command {
	return Command{
		ID:        sc.ID,
		Command:   sc.Command,
		Payload:   json.RawMessage(sc.Payload),
		Status:    sc.Status,
		Result:    json.RawMessage(sc.Result),
		CreatedAt: sc.CreatedAt,
		UpdatedAt: sc.UpdatedAt,
	}
}

func (s *surrealStore) EnqueueCommand(ctx context.Context, cmd string, payload interface{}) (int64, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal payload: %w", err)
	}

	// Atomically increment counter
	res, err := surrealdb.Query[[]map[string]interface{}](ctx, s.db, "UPDATE counter:whatsmeow_commands SET val = (val OR 0) + 1", nil)
	if err != nil {
		return 0, fmt.Errorf("failed to increment counter: %w", err)
	}

	var newID int64
	if res != nil && len(*res) > 0 && len((*res)[0].Result) > 0 {
		if val, ok := (*res)[0].Result[0]["val"]; ok {
			switch v := val.(type) {
			case float64:
				newID = int64(v)
			case int64:
				newID = v
			case int:
				newID = int64(v)
			}
		}
	}
	if newID == 0 {
		newID = time.Now().UnixNano()
	}

	now := time.Now().UTC()
	recordID := fmt.Sprintf("whatsmeow_commands:%d", newID)
	_, err = surrealdb.Query[interface{}](ctx, s.db, 
		"CREATE $record_id SET id = $id, command = $command, payload = $payload, status = 'pending', created_at = $created_at, updated_at = $updated_at",
		map[string]interface{}{
			"record_id":  recordID,
			"id":         newID,
			"command":    cmd,
			"payload":    string(payloadJSON),
			"created_at": now,
			"updated_at": now,
		},
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create command: %w", err)
	}

	return newID, nil
}

func (s *surrealStore) UpdateCommandStatus(ctx context.Context, id int64, status string, result interface{}) error {
	var resultJSON []byte
	var err error
	if result != nil {
		resultJSON, err = json.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal result: %w", err)
		}
	}

	recordID := fmt.Sprintf("whatsmeow_commands:%d", id)
	_, err = surrealdb.Query[interface{}](ctx, s.db,
		"UPDATE $record_id SET status = $status, result = $result, updated_at = $updated_at",
		map[string]interface{}{
			"record_id":  recordID,
			"status":     status,
			"result":     string(resultJSON),
			"updated_at": time.Now().UTC(),
		},
	)
	if err != nil {
		return fmt.Errorf("update command status: %w", err)
	}
	return nil
}

func (s *surrealStore) PollPendingCommands(ctx context.Context) ([]Command, error) {
	res, err := surrealdb.Query[[]surrealCommand](ctx, s.db,
		"SELECT * FROM whatsmeow_commands WHERE status = 'pending' ORDER BY created_at ASC", nil)
	if err != nil {
		return nil, fmt.Errorf("poll pending commands: %w", err)
	}

	var commands []Command
	if res != nil && len(*res) > 0 {
		for _, sc := range (*res)[0].Result {
			commands = append(commands, toCommand(sc))
		}
	}
	return commands, nil
}

func (s *surrealStore) WaitForCommand(ctx context.Context, id int64, timeout time.Duration) (*Command, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	recordID := fmt.Sprintf("whatsmeow_commands:%d", id)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			res, err := surrealdb.Query[[]surrealCommand](ctx, s.db,
				"SELECT * FROM $record_id", map[string]interface{}{"record_id": recordID})
			if err != nil {
				return nil, fmt.Errorf("get command: %w", err)
			}

			if res != nil && len(*res) > 0 && len((*res)[0].Result) > 0 {
				c := toCommand((*res)[0].Result[0])
				if c.Status == "completed" || c.Status == "failed" {
					return &c, nil
				}
			}
		}
	}
}

type surrealFileEntry struct {
	Path      string    `json:"path"`
	Metadata  string    `json:"metadata"`
	Content   []byte    `json:"content"`
	Timestamp time.Time `json:"tmstamp"`
}

func toFileEntry(sfe surrealFileEntry) FileEntry {
	return FileEntry{
		Path: sfe.Path,
		Metadata: sql.NullString{
			String: sfe.Metadata,
			Valid:  sfe.Metadata != "",
		},
		Content:   sfe.Content,
		Timestamp: sfe.Timestamp,
	}
}

func (s *surrealStore) PutFile(ctx context.Context, path string, metadata interface{}, content []byte, timestamp time.Time) error {
	var metadataJSON string
	var err error
	if metadata != nil {
		switch m := metadata.(type) {
		case []byte:
			metadataJSON = string(m)
		case string:
			metadataJSON = m
		default:
			b, err := json.Marshal(metadata)
			if err != nil {
				return fmt.Errorf("marshal metadata: %w", err)
			}
			metadataJSON = string(b)
		}
	}

	hasher := md5.New()
	hasher.Write([]byte(path))
	idPart := hex.EncodeToString(hasher.Sum(nil))
	recordID := fmt.Sprintf("filesys:%s", idPart)

	_, err = surrealdb.Query[interface{}](ctx, s.db,
		"UPSERT $record_id SET path = $path, metadata = $metadata, content = $content, tmstamp = $tmstamp",
		map[string]interface{}{
			"record_id": recordID,
			"path":      path,
			"metadata":  metadataJSON,
			"content":   content,
			"tmstamp":   timestamp,
		},
	)
	if err != nil {
		return fmt.Errorf("put file: %w", err)
	}
	return nil
}

func (s *surrealStore) GetFile(ctx context.Context, path string) (*FileEntry, error) {
	hasher := md5.New()
	hasher.Write([]byte(path))
	idPart := hex.EncodeToString(hasher.Sum(nil))
	recordID := fmt.Sprintf("filesys:%s", idPart)

	res, err := surrealdb.Query[[]surrealFileEntry](ctx, s.db,
		"SELECT * FROM $record_id", map[string]interface{}{"record_id": recordID})
	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}

	if res != nil && len(*res) > 0 && len((*res)[0].Result) > 0 {
		fe := toFileEntry((*res)[0].Result[0])
		return &fe, nil
	}
	return nil, nil
}

func (s *surrealStore) DeleteFile(ctx context.Context, path string) error {
	hasher := md5.New()
	hasher.Write([]byte(path))
	idPart := hex.EncodeToString(hasher.Sum(nil))
	recordID := fmt.Sprintf("filesys:%s", idPart)

	_, err := surrealdb.Query[interface{}](ctx, s.db,
		"DELETE FROM $record_id", map[string]interface{}{"record_id": recordID})
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

func (s *surrealStore) ListFiles(ctx context.Context, prefix string, limit int) ([]FileEntry, error) {
	var res *[]surrealdb.QueryResult[[]surrealFileEntry]
	var err error

	if prefix != "" {
		res, err = surrealdb.Query[[]surrealFileEntry](ctx, s.db,
			"SELECT * FROM filesys WHERE path CONTAINS $prefix ORDER BY tmstamp DESC LIMIT $limit",
			map[string]interface{}{"prefix": prefix, "limit": limit})
	} else {
		res, err = surrealdb.Query[[]surrealFileEntry](ctx, s.db,
			"SELECT * FROM filesys ORDER BY tmstamp DESC LIMIT $limit",
			map[string]interface{}{"limit": limit})
	}

	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}

	var entries []FileEntry
	if res != nil && len(*res) > 0 {
		for _, sfe := range (*res)[0].Result {
			entries = append(entries, toFileEntry(sfe))
		}
	}
	return entries, nil
}

func (s *surrealStore) GetFilesysLogs(ctx context.Context, phone string, limit int) ([]FileEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	prefix := "whatsmeow/" + phone + "/"

	res, err := surrealdb.Query[[]surrealFileEntry](ctx, s.db,
		"SELECT * FROM filesys WHERE path CONTAINS $prefix ORDER BY tmstamp DESC LIMIT $limit",
		map[string]interface{}{"prefix": prefix, "limit": limit})
	if err != nil {
		return nil, fmt.Errorf("get filesys logs: %w", err)
	}

	var entries []FileEntry
	if res != nil && len(*res) > 0 {
		for _, sfe := range (*res)[0].Result {
			fe := toFileEntry(sfe)
			var metaMap map[string]interface{}
			if err := json.Unmarshal([]byte(sfe.Metadata), &metaMap); err == nil {
				if mime, ok := metaMap["mime_type"].(string); !ok || mime != "text/plain" {
					fe.Content = nil
				}
			} else {
				fe.Content = nil
			}
			entries = append(entries, fe)
		}
	}
	return entries, nil
}

func (s *surrealStore) GetLatestGlobalMessages(ctx context.Context, limit int) ([]FileEntry, error) {
	if limit <= 0 {
		limit = 20
	}

	res, err := surrealdb.Query[[]surrealFileEntry](ctx, s.db,
		"SELECT * FROM filesys WHERE path CONTAINS '/request' OR path CONTAINS '/response' ORDER BY tmstamp DESC LIMIT $limit",
		map[string]interface{}{"limit": limit})
	if err != nil {
		return nil, fmt.Errorf("get latest global messages: %w", err)
	}

	var entries []FileEntry
	if res != nil && len(*res) > 0 {
		for _, sfe := range (*res)[0].Result {
			fe := toFileEntry(sfe)
			var metaMap map[string]interface{}
			if err := json.Unmarshal([]byte(sfe.Metadata), &metaMap); err == nil {
				if mime, ok := metaMap["mime_type"].(string); !ok || mime != "text/plain" {
					fe.Content = nil
				}
			} else {
				fe.Content = nil
			}
			entries = append(entries, fe)
		}
	}
	return entries, nil
}

func (s *surrealStore) QueryFilesys(ctx context.Context, query string, args ...interface{}/* unused in surrealQL standard dynamic filesys */) ([]map[string]interface{}, error) {
	vars := make(map[string]interface{})
	for i, arg := range args {
		vars[fmt.Sprintf("v%d", i+1)] = arg
		query = strings.ReplaceAll(query, fmt.Sprintf("$%d", i+1), fmt.Sprintf("$v%d", i+1))
	}

	res, err := surrealdb.Query[[]map[string]interface{}](ctx, s.db, query, vars)
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	if res != nil && len(*res) > 0 {
		results = (*res)[0].Result
	}
	return results, nil
}

type surrealBlacklist struct {
	Phone     string    `json:"phone"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *surrealStore) IsBlacklisted(ctx context.Context, phone string) (bool, error) {
	recordID := fmt.Sprintf("blacklisted_numbers:%s", phone)
	res, err := surrealdb.Query[[]surrealBlacklist](ctx, s.db,
		"SELECT * FROM $record_id", map[string]interface{}{"record_id": recordID})
	if err != nil {
		return false, fmt.Errorf("check blacklist: %w", err)
	}

	if res != nil && len(*res) > 0 && len((*res)[0].Result) > 0 {
		return true, nil
	}
	return false, nil
}

func (s *surrealStore) AddBlacklist(ctx context.Context, phone, reason string) error {
	recordID := fmt.Sprintf("blacklisted_numbers:%s", phone)
	_, err := surrealdb.Query[interface{}](ctx, s.db,
		"UPSERT $record_id SET phone = $phone, reason = $reason, created_at = $created_at",
		map[string]interface{}{
			"record_id":  recordID,
			"phone":      phone,
			"reason":     reason,
			"created_at": time.Now().UTC(),
		},
	)
	return err
}

func (s *surrealStore) RemoveBlacklist(ctx context.Context, phone string) error {
	recordID := fmt.Sprintf("blacklisted_numbers:%s", phone)
	_, err := surrealdb.Query[interface{}](ctx, s.db,
		"DELETE FROM $record_id", map[string]interface{}{"record_id": recordID})
	return err
}

func (s *surrealStore) ListBlacklist(ctx context.Context) ([]BlacklistedNumber, error) {
	res, err := surrealdb.Query[[]surrealBlacklist](ctx, s.db,
		"SELECT * FROM blacklisted_numbers ORDER BY created_at DESC", nil)
	if err != nil {
		return nil, fmt.Errorf("list blacklist: %w", err)
	}

	var numbers []BlacklistedNumber
	if res != nil && len(*res) > 0 {
		for _, sb := range (*res)[0].Result {
			numbers = append(numbers, BlacklistedNumber{
				Phone:     sb.Phone,
				Reason:    sb.Reason,
				CreatedAt: sb.CreatedAt,
			})
		}
	}
	return numbers, nil
}

type surrealContact struct {
	OurJID       string `json:"our_jid"`
	TheirJID     string `json:"their_jid"`
	FullName     string `json:"full_name"`
	ShortName    string `json:"short_name"`
	PushName     string `json:"push_name"`
	BusinessName string `json:"business_name"`
}

func (s *surrealStore) ListContacts(ctx context.Context, query string) ([]Contact, error) {
	var res *[]surrealdb.QueryResult[[]surrealContact]
	var err error

	if query == "" {
		res, err = surrealdb.Query[[]surrealContact](ctx, s.db,
			"SELECT * FROM whatsmeow_contacts ORDER BY full_name ASC LIMIT 100", nil)
	} else {
		q := strings.ToLower(query)
		res, err = surrealdb.Query[[]surrealContact](ctx, s.db,
			`SELECT * FROM whatsmeow_contacts WHERE 
				string::lowercase(full_name) CONTAINS $q OR 
				string::lowercase(push_name) CONTAINS $q OR 
				string::lowercase(their_jid) CONTAINS $q 
			 ORDER BY full_name ASC LIMIT 100`,
			map[string]interface{}{"q": q})
	}

	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}

	var contacts []Contact
	if res != nil && len(*res) > 0 {
		for _, sc := range (*res)[0].Result {
			contacts = append(contacts, Contact{
				OurJID:       sc.OurJID,
				TheirJID:     sc.TheirJID,
				FullName:     sc.FullName,
				ShortName:    sc.ShortName,
				PushName:     sc.PushName,
				BusinessName: sc.BusinessName,
			})
		}
	}
	return contacts, nil
}
