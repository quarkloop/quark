// Package store implements the SQLite-backed persistence layer for the
// Catalog service.
//
// The package exposes a single concrete type, Store, that owns the
// *sql.DB connection and groups persistence operations by domain.
// Operations for each domain live in a separate file (systems.go,
// nodes.go, ...) for navigability, but they are all methods on Store
// so callers don't have to compose multiple structs.
//
// SQLite driver: modernc.org/sqlite (pure Go, no CGO). This makes the
// Catalog binary trivially portable — no system libsqlite3 required.
//
// Concurrency: SQLite serializes writes, so we set MaxOpenConns(1).
// Reads are also serialized through the same connection. This is fine
// for the Catalog's workload (a few hundred TPS at most).
package store

import (
        "database/sql"
        "fmt"
        "os"
        "path/filepath"
        "time"

        _ "modernc.org/sqlite" // pure-Go SQLite driver
)

// Store wraps a SQLite connection for catalog operations.
type Store struct {
        db *sql.DB
}

// MigrationResult tracks how many items were migrated from legacy JSONL
// on first startup. Returned by MigrateLegacy so the caller can log it.
type MigrationResult struct {
        Systems int
        Events  int
}

// Open opens (or creates) the SQLite database at stateRoot/catalog.db
// and ensures the schema exists. The caller must call Close when done.
func Open(stateRoot string) (*Store, error) {
        if err := os.MkdirAll(stateRoot, 0o755); err != nil {
                return nil, fmt.Errorf("cannot create state root: %w", err)
        }
        dbPath := filepath.Join(stateRoot, "catalog.db")
        dsn := fmt.Sprintf(
                "file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on&_txlock=immediate",
                dbPath,
        )
        db, err := sql.Open("sqlite", dsn)
        if err != nil {
                return nil, fmt.Errorf("cannot open SQLite: %w", err)
        }
        // SQLite serializes writes — a single connection avoids lock contention.
        db.SetMaxOpenConns(1)
        s := &Store{db: db}
        if err := s.createSchema(); err != nil {
                return nil, fmt.Errorf("cannot create schema: %w", err)
        }
        return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the underlying *sql.DB for advanced callers (e.g. tests
// that need to inspect tables directly). Production code should prefer
// the domain-specific methods on Store.
func (s *Store) DB() *sql.DB { return s.db }

// createSchema runs the full DDL. Uses CREATE TABLE IF NOT EXISTS so
// it is safe to call on every startup.
func (s *Store) createSchema() error {
        _, err := s.db.Exec(schemaSQL)
        return err
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS systems (
    namespace   TEXT NOT NULL,
    name        TEXT NOT NULL,
    source      TEXT NOT NULL,
    state       TEXT NOT NULL DEFAULT 'ACTIVE',
    health      TEXT NOT NULL DEFAULT 'HEALTHY',
    version     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (namespace, name)
);

CREATE TABLE IF NOT EXISTS nodes (
    namespace           TEXT NOT NULL,
    system_name         TEXT NOT NULL,
    name                TEXT NOT NULL,
    uri                 TEXT NOT NULL,
    state               TEXT NOT NULL DEFAULT 'CREATING',
    health              TEXT NOT NULL DEFAULT 'UNKNOWN',
    version             INTEGER NOT NULL DEFAULT 1,
    error_message       TEXT,
    listens             TEXT,
    events              TEXT,
    config              TEXT,
    labels              TEXT,
    annotations         TEXT,
    on_failure_retry    TEXT,
    on_failure_route_to TEXT,
    timeout             TEXT,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (namespace, system_name, name)
);
CREATE INDEX IF NOT EXISTS idx_nodes_ns  ON nodes(namespace);
CREATE INDEX IF NOT EXISTS idx_nodes_sys ON nodes(namespace, system_name);

CREATE TABLE IF NOT EXISTS events (
    id          TEXT NOT NULL,
    kind        TEXT NOT NULL,
    node_name   TEXT,
    system_name TEXT,
    namespace   TEXT,
    timestamp   TEXT NOT NULL,
    payload     TEXT,
    PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_events_ns_ts   ON events(namespace, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_sys     ON events(namespace, system_name);
CREATE INDEX IF NOT EXISTS idx_events_node    ON events(namespace, system_name, node_name);
CREATE INDEX IF NOT EXISTS idx_events_ts      ON events(timestamp DESC);

CREATE TABLE IF NOT EXISTS node_packages (
    uri          TEXT PRIMARY KEY,
    category     TEXT,
    version      TEXT NOT NULL,
    manifest     TEXT NOT NULL,
    content      BLOB NOT NULL,
    content_type TEXT NOT NULL,
    checksum     TEXT NOT NULL,
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    downloads    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_packages_category ON node_packages(category);

CREATE TABLE IF NOT EXISTS registry (
    uri         TEXT PRIMARY KEY,
    pattern     TEXT NOT NULL,
    description TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS catalog_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT OR IGNORE INTO catalog_meta (key, value) VALUES ('schema_version', '1');
`

// --- shared helpers ---

// now returns the current UTC time as an RFC3339Nano string. SQLite
// stores timestamps as TEXT; using a single format everywhere avoids
// parsing ambiguity.
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
