package store

import (
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

// DDL for SQLite views projecting unified crm_store into whatsadk compatible schemas.
const viewsDDL = `
CREATE VIEW IF NOT EXISTS filesys AS
SELECT
    substr(key, 18) AS path,
    CASE 
        WHEN json_extract(metadata, '$._is_null') = 1 THEN NULL
        ELSE json_remove(metadata, '$._tmstamp')
    END AS metadata,
    data AS content,
    json_extract(metadata, '$._tmstamp') AS tmstamp
FROM crm_store
WHERE key LIKE 'whatsadk:filesys:%';

CREATE VIEW IF NOT EXISTS whatsmeow_contacts AS
SELECT
    json_extract(metadata, '$.our_jid') AS our_jid,
    json_extract(metadata, '$.their_jid') AS their_jid,
    json_extract(metadata, '$.full_name') AS full_name,
    json_extract(metadata, '$.short_name') AS short_name,
    json_extract(metadata, '$.push_name') AS push_name,
    json_extract(metadata, '$.business_name') AS business_name
FROM crm_store
WHERE key LIKE 'whatsadk:contact:%';

CREATE VIEW IF NOT EXISTS whatsmeow_commands AS
SELECT
    CAST(json_extract(metadata, '$.id') AS INTEGER) AS id,
    json_extract(metadata, '$.command') AS command,
    json_extract(metadata, '$.payload') AS payload,
    json_extract(metadata, '$.status') AS status,
    json_extract(metadata, '$.result') AS result,
    json_extract(metadata, '$.created_at') AS created_at,
    json_extract(metadata, '$.updated_at') AS updated_at
FROM crm_store
WHERE key LIKE 'whatsadk:command:%';

CREATE VIEW IF NOT EXISTS blacklisted_numbers AS
SELECT
    substr(key, 20) AS phone,
    json_extract(metadata, '$.reason') AS reason,
    json_extract(metadata, '$.created_at') AS created_at
FROM crm_store
WHERE key LIKE 'whatsadk:blacklist:%';
`

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

	// Initialize views for whatsadk SQL compatibility
	if _, err := db.Exec(viewsDDL); err != nil {
		if ownsDB {
			_ = db.Close()
		}
		return nil, fmt.Errorf("failed to create whatsadk views: %w", err)
	}

	repo := opts.Repo
	if repo == nil {
		repo = p2p.NewRepository(db)
	}

	tracker := opts.Tracker
	if tracker == nil {
		tracker = p2p.NewChangesetTracker(repo)
	}

	backend := &Backend{
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

	pol, err := cfg.BuildPolicy()
	if err != nil {
		return nil, fmt.Errorf("failed to build replication policy: %w", err)
	}

	var topic [32]byte
	if cfg.SwarmTopic != "" {
		copy(topic[:], []byte(cfg.SwarmTopic))
	}

	p2pEngine, err := p2p.OpenEngine(p2p.EngineOptions{
		DBPath:       cfg.DBPath,
		EnableWAL:    cfg.EnableWAL,
		EnableCrypto: cfg.EnableCrypto,
		SwarmTopic:   topic,
		SwarmPort:    cfg.SwarmPort,
		Bootstrap:    cfg.Bootstrap,
		Policy:       pol,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open p2p engine: %w", err)
	}

	backend, err := NewBackend(Options{
		DB:        p2pEngine.DB(),
		Repo:      p2pEngine.Repository(),
		Tracker:   p2pEngine.ChangesetTracker(),
		P2PEngine: p2pEngine,
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
