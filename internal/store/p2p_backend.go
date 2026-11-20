package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"go-pear/pkg/policy"
	p2pconfig "sqlite-p2p/pkg/config"
	"sqlite-p2p/pkg/p2p"
)

// DefaultWhatsmeowExcludeTables lists all local whatsmeow E2EE Signal protocol and session
// tables that must never be auto-discovered or replicated across the P2P swarm.
var DefaultWhatsmeowExcludeTables = []string{
	"whatsmeow_device",
	"whatsmeow_identity_keys",
	"whatsmeow_pre_keys",
	"whatsmeow_sessions",
	"whatsmeow_sender_keys",
	"whatsmeow_app_state_sync_keys",
	"whatsmeow_app_state_version",
	"whatsmeow_app_state_mutation_macs",
	"whatsmeow_contacts",
	"whatsmeow_chat_settings",
	"whatsmeow_message_secrets",
	"whatsmeow_privacy_tokens",
	"whatsmeow_lid_map",
	"whatsmeow_event_buffer",
	"whatsmeow_version",
}

type viewDef struct {
	name string
	ddl  string
}

// whatsadkViews defines SQLite views projecting unified crm_store into whatsadk compatible schemas.
var whatsadkViews = []viewDef{
	{
		name: "filesys",
		ddl: `CREATE VIEW IF NOT EXISTS filesys AS
SELECT
    substr(key, 18) AS path,
    CASE 
        WHEN json_extract(metadata, '$._is_null') = 1 THEN NULL
        ELSE json_remove(metadata, '$._tmstamp')
    END AS metadata,
    data AS content,
    json_extract(metadata, '$._tmstamp') AS tmstamp
FROM crm_store
WHERE key LIKE 'whatsadk:filesys:%';`,
	},
	{
		name: "whatsmeow_contacts",
		ddl: `CREATE VIEW IF NOT EXISTS whatsmeow_contacts AS
SELECT
    json_extract(metadata, '$.our_jid') AS our_jid,
    json_extract(metadata, '$.their_jid') AS their_jid,
    json_extract(metadata, '$.full_name') AS full_name,
    json_extract(metadata, '$.short_name') AS short_name,
    json_extract(metadata, '$.push_name') AS push_name,
    json_extract(metadata, '$.business_name') AS business_name
FROM crm_store
WHERE key LIKE 'whatsadk:contact:%';`,
	},
	{
		name: "whatsmeow_commands",
		ddl: `CREATE VIEW IF NOT EXISTS whatsmeow_commands AS
SELECT
    CAST(json_extract(metadata, '$.id') AS INTEGER) AS id,
    json_extract(metadata, '$.command') AS command,
    json_extract(metadata, '$.payload') AS payload,
    json_extract(metadata, '$.status') AS status,
    json_extract(metadata, '$.result') AS result,
    json_extract(metadata, '$.created_at') AS created_at,
    json_extract(metadata, '$.updated_at') AS updated_at
FROM crm_store
WHERE key LIKE 'whatsadk:command:%';`,
	},
	{
		name: "blacklisted_numbers",
		ddl: `CREATE VIEW IF NOT EXISTS blacklisted_numbers AS
SELECT
    substr(key, 20) AS phone,
    json_extract(metadata, '$.reason') AS reason,
    json_extract(metadata, '$.created_at') AS created_at
FROM crm_store
WHERE key LIKE 'whatsadk:blacklist:%';`,
	},
	{
		name: "whitelisted_numbers",
		ddl: `CREATE VIEW IF NOT EXISTS whitelisted_numbers AS
SELECT
    substr(key, 20) AS phone,
    json_extract(metadata, '$.reason') AS reason,
    json_extract(metadata, '$.created_at') AS created_at
FROM crm_store
WHERE key LIKE 'whatsadk:whitelist:%';`,
	},
}

func initWhatsadkViews(db *sql.DB) error {
	for _, v := range whatsadkViews {
		var existingTable int
		err := db.QueryRow(
			`SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND lower(name) = lower(?)`,
			v.name,
		).Scan(&existingTable)
		if err == nil && existingTable > 0 {
			// A physical table with this name already exists (e.g. co-located whatsmeow_contacts); skip view creation.
			continue
		}
		if _, err := db.Exec(v.ddl); err != nil {
			return err
		}
	}
	return nil
}

func mergeExcludeTables(existing []string) []string {
	seen := make(map[string]bool, len(existing)+len(DefaultWhatsmeowExcludeTables))
	merged := make([]string, 0, len(existing)+len(DefaultWhatsmeowExcludeTables))
	for _, t := range existing {
		trimmed := strings.TrimSpace(t)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if !seen[lower] {
			seen[lower] = true
			merged = append(merged, trimmed)
		}
	}
	for _, t := range DefaultWhatsmeowExcludeTables {
		lower := strings.ToLower(t)
		if !seen[lower] {
			seen[lower] = true
			merged = append(merged, t)
		}
	}
	return merged
}

// Options configures the whatsadk storage backend.
type Options struct {
	DBPath    string
	DB        *sql.DB
	EnableWAL bool
	Repo      *p2p.Repository
	Tracker   *p2p.ChangesetTracker
	Engine    *p2p.ReplicationEngine
	P2PEngine *p2p.Engine
	Policy    *policy.ReplicationPolicy
}

// Backend implements StoreBackend using SQLite unified crm_store and Autobase replication.
type Backend struct {
	dbPath    string
	db        *sql.DB
	ownsDB    bool
	repo      *p2p.Repository
	tracker   *p2p.ChangesetTracker
	engine    *p2p.ReplicationEngine
	p2pEngine *p2p.Engine
	policy    *policy.ReplicationPolicy
	mu        sync.RWMutex
	cmdSeq    atomic.Int64
}

// Compile-time check that Backend implements storeBackend.
var _ storeBackend = (*Backend)(nil)

// NewBackend creates and initializes a Backend instance.
func NewBackend(opts Options) (*Backend, error) {
	var db *sql.DB
	var ownsDB bool
	var err error

	if opts.DB != nil {
		db = opts.DB
		ownsDB = false
	} else {
		if opts.DBPath == "" {
			return nil, errors.New("either DB or DBPath must be provided")
		}
		db, err = p2p.OpenDB(opts.DBPath, opts.EnableWAL)
		if err != nil {
			return nil, fmt.Errorf("failed to open sqlite database: %w", err)
		}
		ownsDB = true
	}

	repo := opts.Repo
	if repo == nil {
		repo = p2p.NewRepository(db)
	}

	// Initialize views for whatsadk SQL compatibility after crm_store exists
	if err := initWhatsadkViews(db); err != nil {
		if ownsDB {
			_ = db.Close()
		}
		return nil, fmt.Errorf("failed to create whatsadk views: %w", err)
	}

	tracker := opts.Tracker
	if tracker == nil {
		tracker = p2p.NewChangesetTracker(repo)
	}

	backend := &Backend{
		dbPath:    opts.DBPath,
		db:        db,
		ownsDB:    ownsDB,
		repo:      repo,
		tracker:   tracker,
		engine:    opts.Engine,
		p2pEngine: opts.P2PEngine,
		policy:    opts.Policy,
	}

	// Initialize command sequence from existing commands in DB
	var maxID sql.NullInt64
	err = db.QueryRow("SELECT MAX(CAST(json_extract(metadata, '$.id') AS INTEGER)) FROM crm_store WHERE key LIKE 'whatsadk:command:%'").Scan(&maxID)
	if err == nil && maxID.Valid {
		backend.cmdSeq.Store(maxID.Int64)
	}

	return backend, nil
}

// DBPath returns the database file path used by this backend.
func (b *Backend) DBPath() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.dbPath
}

// DB returns the underlying sql.DB instance.
func (b *Backend) DB() *sql.DB {
	return b.db
}

// Repo returns the underlying p2p.Repository instance.
func (b *Backend) Repo() *p2p.Repository {
	return b.repo
}

// Tracker returns the underlying p2p.ChangesetTracker instance.
func (b *Backend) Tracker() *p2p.ChangesetTracker {
	return b.tracker
}

// Policy returns the active replication gating policy if configured.
func (b *Backend) Policy() *policy.ReplicationPolicy {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.policy != nil {
		return b.policy
	}
	if b.p2pEngine != nil {
		return b.p2pEngine.Policy()
	}
	return nil
}

// SetPolicy updates the active replication policy dynamically.
func (b *Backend) SetPolicy(p *policy.ReplicationPolicy) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.policy = p
	if b.p2pEngine != nil {
		b.p2pEngine.SetPolicy(p)
	}
}

// Engine returns the high-level P2P Engine if initialized.
func (b *Backend) Engine() *p2p.Engine {
	return b.p2pEngine
}

// DrainCDCOutbox triggers an immediate scan for newly created SQLite tables and drains
// pending CDC outbox mutations into the Hypercore feed and P2P swarm.
func (b *Backend) DrainCDCOutbox(ctx context.Context) (int, error) {
	b.mu.RLock()
	p2pEng := b.p2pEngine
	replEng := b.engine
	b.mu.RUnlock()

	if p2pEng != nil {
		return p2pEng.DrainCDCOutbox(ctx)
	}
	if replEng != nil {
		return replEng.DrainCDCOutbox(ctx)
	}
	return 0, nil
}

// Close closes the underlying SQLite database or P2P engine if owned by this Backend.
func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.p2pEngine != nil {
		return b.p2pEngine.Close()
	}
	if b.ownsDB && b.db != nil {
		return b.db.Close()
	}
	return nil
}

// OpenP2PFromNodeConfig opens a Store instance using a NodeConfig with replication policy.
func OpenP2PFromNodeConfig(cfg *p2pconfig.NodeConfig) (*Store, error) {
	if cfg == nil {
		return nil, errors.New("node config cannot be nil")
	}

	cfg.ExcludeTables = mergeExcludeTables(cfg.ExcludeTables)

	pol, err := cfg.BuildPolicy()
	if err != nil {
		return nil, fmt.Errorf("failed to build replication policy: %w", err)
	}

	p2pEngine, err := p2p.OpenEngineFromNodeConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open p2p engine: %w", err)
	}

	backend, err := NewBackend(Options{
		DBPath:    cfg.DBPath,
		DB:        p2pEngine.DB(),
		Repo:      p2pEngine.Repository(),
		Tracker:   p2pEngine.ChangesetTracker(),
		P2PEngine: p2pEngine,
		Engine:    p2pEngine.ReplicationEngine(),
		Policy:    pol,
	})
	if err != nil {
		_ = p2pEngine.Close()
		return nil, err
	}

	return &Store{backend: backend}, nil
}

// openSQLiteP2PBackend opens the SQLite P2P storage backend from a DSN.
func openSQLiteP2PBackend(dsn string) (storeBackend, error) {
	dbPath := dsn
	for _, prefix := range []string{"sqlite-p2p://", "sqlite://", "p2p://", "pear://"} {
		if strings.HasPrefix(dsn, prefix) {
			dbPath = strings.TrimPrefix(dsn, prefix)
			break
		}
	}
	enableWAL := true
	if strings.Contains(dbPath, "?") {
		parts := strings.SplitN(dbPath, "?", 2)
		dbPath = parts[0]
		if strings.Contains(parts[1], "wal=false") || strings.Contains(parts[1], "wal=0") {
			enableWAL = false
		}
	}
	if dbPath == "" {
		dbPath = ":memory:"
	}
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if dir != "." && dir != "/" && dir != "" {
			_ = os.MkdirAll(dir, 0755)
		}
	}
	return NewBackend(Options{
		DBPath:    dbPath,
		EnableWAL: enableWAL,
	})
}
