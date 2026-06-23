// Package main — SQLite storage layer for the Catalog service.
//
// Uses modernc.org/sqlite (pure Go, no CGO) for maximum portability.
// The database file is stored at $STATE_ROOT/catalog.db.
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite connection for catalog operations.
type Store struct {
	db *sql.DB
}

// MigrationResult tracks how many items were migrated from legacy JSONL.
type MigrationResult struct {
	Systems int
	Events  int
}

// NewStore opens (or creates) the SQLite database at stateRoot/catalog.db.
func NewStore(stateRoot string) (*Store, error) {
	if err := os.MkdirAll(stateRoot, 0755); err != nil {
		return nil, fmt.Errorf("cannot create state root: %w", err)
	}
	dbPath := filepath.Join(stateRoot, "catalog.db")
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on&_txlock=immediate", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("cannot open SQLite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite serializes writes
	s := &Store{db: db}
	if err := s.createSchema(); err != nil {
		return nil, fmt.Errorf("cannot create schema: %w", err)
	}
	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) createSchema() error {
	_, err := s.db.Exec(`
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
			namespace        TEXT NOT NULL,
			system_name      TEXT NOT NULL,
			name             TEXT NOT NULL,
			uri              TEXT NOT NULL,
			category         TEXT NOT NULL,
			state            TEXT NOT NULL DEFAULT 'CREATING',
			health           TEXT NOT NULL DEFAULT 'UNKNOWN',
			version          INTEGER NOT NULL DEFAULT 1,
			error_message    TEXT,
			listens          TEXT,
			events           TEXT,
			config           TEXT,
			labels           TEXT,
			annotations      TEXT,
			on_failure_retry TEXT,
			on_failure_route_to TEXT,
			timeout          TEXT,
			created_at       TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at       TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (namespace, system_name, name)
		);
		CREATE INDEX IF NOT EXISTS idx_nodes_ns ON nodes(namespace);
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
		CREATE INDEX IF NOT EXISTS idx_events_ns ON events(namespace);
		CREATE INDEX IF NOT EXISTS idx_events_sys ON events(namespace, system_name);
		CREATE INDEX IF NOT EXISTS idx_events_node ON events(namespace, system_name, node_name);
		CREATE INDEX IF NOT EXISTS idx_events_ts ON events(timestamp DESC);
		CREATE INDEX IF NOT EXISTS idx_events_ns_ts ON events(namespace, timestamp DESC);

		CREATE TABLE IF NOT EXISTS node_packages (
			uri          TEXT PRIMARY KEY,
			category     TEXT NOT NULL,
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
			category    TEXT NOT NULL,
			active      INTEGER NOT NULL DEFAULT 1,
			description TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS catalog_meta (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
		INSERT OR IGNORE INTO catalog_meta (key, value) VALUES ('schema_version', '1');
	`)
	return err
}

// now returns the current UTC time as an ISO-8601 string.
func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// toJSON marshals a value to a string, returning "" on error.
func toJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// fromJSONSlice parses a JSON array string into a []string.
func fromJSONSlice(s string) []string {
	if s == "" {
		return []string{}
	}
	var r []string
	json.Unmarshal([]byte(s), &r)
	return r
}

// fromJSONMap parses a JSON object string into a map.
func fromJSONMap(s string) map[string]interface{} {
	if s == "" {
		return nil
	}
	var r map[string]interface{}
	json.Unmarshal([]byte(s), &r)
	return r
}

func fromJSONStringMap(s string) map[string]string {
	if s == "" {
		return nil
	}
	var r map[string]string
	json.Unmarshal([]byte(s), &r)
	return r
}

// --- System operations ---

func (s *Store) SaveSystem(req SaveSystemRequest) error {
	_, err := s.db.Exec(
		`INSERT INTO systems (namespace, name, source, state, health, version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(namespace, name) DO UPDATE SET
		   source=excluded.source, state=excluded.state, health=excluded.health,
		   version=excluded.version, updated_at=excluded.updated_at`,
		req.Namespace, req.Name, req.Source, req.State, req.Health, req.Version, now(), now())
	return err
}

func (s *Store) GetSystem(ns, name string) (*SystemResponse, error) {
	row := s.db.QueryRow(
		`SELECT namespace, name, source, state, health, version, created_at, updated_at
		 FROM systems WHERE namespace=? AND name=?`, ns, name)
	return scanSystem(row)
}

func (s *Store) ListSystems(ns string) ([]SystemResponse, error) {
	var rows *sql.Rows
	var err error
	if ns != "" {
		rows, err = s.db.Query(
			`SELECT namespace, name, source, state, health, version, created_at, updated_at
			 FROM systems WHERE namespace=? ORDER BY name`, ns)
	} else {
		rows, err = s.db.Query(
			`SELECT namespace, name, source, state, health, version, created_at, updated_at
			 FROM systems ORDER BY namespace, name`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SystemResponse
	for rows.Next() {
		var sr SystemResponse
		if err := rows.Scan(&sr.Namespace, &sr.Name, &sr.Source, &sr.State, &sr.Health,
			&sr.Version, &sr.CreatedAt, &sr.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sr)
	}
	return out, nil
}

func (s *Store) ListAllSystems() ([]SystemResponse, error) {
	return s.ListSystems("")
}

func (s *Store) DeleteSystem(ns, name string) error {
	_, err := s.db.Exec(`DELETE FROM systems WHERE namespace=? AND name=?`, ns, name)
	return err
}

func (s *Store) UpdateSystemState(ns, name, state, health string, version int64) error {
	_, err := s.db.Exec(
		`UPDATE systems SET state=?, health=?, version=?, updated_at=? WHERE namespace=? AND name=?`,
		state, health, version, now(), ns, name)
	return err
}

func scanSystem(row *sql.Row) (*SystemResponse, error) {
	var sr SystemResponse
	err := row.Scan(&sr.Namespace, &sr.Name, &sr.Source, &sr.State, &sr.Health,
		&sr.Version, &sr.CreatedAt, &sr.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &sr, err
}

// --- Node operations ---

func (s *Store) SaveNode(req SaveNodeRequest) error {
	_, err := s.db.Exec(
		`INSERT INTO nodes (namespace, system_name, name, uri, category, state, health, version,
		   error_message, listens, events, config, labels, annotations,
		   on_failure_retry, on_failure_route_to, timeout, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(namespace, system_name, name) DO UPDATE SET
		   uri=excluded.uri, category=excluded.category, state=excluded.state, health=excluded.health,
		   version=excluded.version, listens=excluded.listens, events=excluded.events,
		   config=excluded.config, labels=excluded.labels, annotations=excluded.annotations,
		   on_failure_retry=excluded.on_failure_retry, on_failure_route_to=excluded.on_failure_route_to,
		   timeout=excluded.timeout, updated_at=excluded.updated_at`,
		req.Namespace, req.SystemName, req.Name, req.URI, req.Category, req.State, req.Health, req.Version,
		toJSON(req.Listens), toJSON(req.Events), toJSON(req.Config), toJSON(req.Labels), toJSON(req.Annotations),
		req.OnFailureRetry, req.OnFailureRouteTo, req.Timeout, now(), now())
	return err
}

func (s *Store) SaveNodes(reqs []SaveNodeRequest) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(
		`INSERT INTO nodes (namespace, system_name, name, uri, category, state, health, version,
		   error_message, listens, events, config, labels, annotations,
		   on_failure_retry, on_failure_route_to, timeout, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(namespace, system_name, name) DO UPDATE SET
		   uri=excluded.uri, category=excluded.category, state=excluded.state, health=excluded.health,
		   version=excluded.version, listens=excluded.listens, events=excluded.events,
		   config=excluded.config, labels=excluded.labels, annotations=excluded.annotations,
		   on_failure_retry=excluded.on_failure_retry, on_failure_route_to=excluded.on_failure_route_to,
		   timeout=excluded.timeout, updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, req := range reqs {
		_, err = stmt.Exec(req.Namespace, req.SystemName, req.Name, req.URI, req.Category,
			req.State, req.Health, req.Version, toJSON(req.Listens), toJSON(req.Events),
			toJSON(req.Config), toJSON(req.Labels), toJSON(req.Annotations),
			req.OnFailureRetry, req.OnFailureRouteTo, req.Timeout, now(), now())
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListNodes(ns, sysName string) ([]NodeResponse, error) {
	rows, err := s.db.Query(
		`SELECT namespace, system_name, name, uri, category, state, health, version,
		   error_message, listens, events, config, labels, annotations,
		   on_failure_retry, on_failure_route_to, timeout, created_at, updated_at
		 FROM nodes WHERE namespace=? AND system_name=? ORDER BY name`, ns, sysName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

func (s *Store) ListNodesByNamespace(ns string) ([]NodeResponse, error) {
	rows, err := s.db.Query(
		`SELECT namespace, system_name, name, uri, category, state, health, version,
		   error_message, listens, events, config, labels, annotations,
		   on_failure_retry, on_failure_route_to, timeout, created_at, updated_at
		 FROM nodes WHERE namespace=? ORDER BY system_name, name`, ns)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

func (s *Store) DeleteNodesBySystem(ns, sysName string) error {
	_, err := s.db.Exec(`DELETE FROM nodes WHERE namespace=? AND system_name=?`, ns, sysName)
	return err
}

func scanNodes(rows *sql.Rows) ([]NodeResponse, error) {
	var out []NodeResponse
	for rows.Next() {
		var n NodeResponse
		var listens, events, config, labels, annotations, errMsg sql.NullString
		if err := rows.Scan(&n.Namespace, &n.SystemName, &n.Name, &n.URI, &n.Category,
			&n.State, &n.Health, &n.Version, &errMsg, &listens, &events,
			&config, &labels, &annotations, &n.OnFailureRetry, &n.OnFailureRouteTo,
			&n.Timeout, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		n.ErrorMessage = errMsg.String
		n.Listens = fromJSONSlice(listens.String)
		n.Events = fromJSONSlice(events.String)
		n.Config = fromJSONMap(config.String)
		n.Labels = fromJSONStringMap(labels.String)
		n.Annotations = fromJSONStringMap(annotations.String)
		out = append(out, n)
	}
	return out, nil
}

// --- Event operations ---

func (s *Store) AppendEvent(req AppendEventRequest) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO events (id, kind, node_name, system_name, namespace, timestamp, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		req.ID, req.Kind, req.NodeName, req.SystemName, req.Namespace,
		req.Timestamp, toJSON(req.Payload))
	return err
}

func (s *Store) AppendEvents(reqs []AppendEventRequest) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(
		`INSERT OR REPLACE INTO events (id, kind, node_name, system_name, namespace, timestamp, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, req := range reqs {
		_, err = stmt.Exec(req.ID, req.Kind, req.NodeName, req.SystemName, req.Namespace,
			req.Timestamp, toJSON(req.Payload))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) QueryEvents(req QueryEventsRequest) ([]EventResponse, error) {
	var sb strings.Builder
	sb.WriteString(`SELECT id, kind, node_name, system_name, namespace, timestamp, payload FROM events WHERE 1=1`)
	args := []interface{}{}
	if req.Namespace != "" {
		sb.WriteString(" AND namespace=?")
		args = append(args, req.Namespace)
	}
	if req.SystemName != "" {
		sb.WriteString(" AND system_name=?")
		args = append(args, req.SystemName)
	}
	if req.NodeName != "" {
		sb.WriteString(" AND node_name=?")
		args = append(args, req.NodeName)
	}
	if len(req.Kinds) > 0 {
		placeholders := make([]string, len(req.Kinds))
		for i, k := range req.Kinds {
			placeholders[i] = "?"
			args = append(args, k)
		}
		sb.WriteString(" AND kind IN (" + strings.Join(placeholders, ",") + ")")
	}
	sb.WriteString(" ORDER BY timestamp DESC")
	limit := req.Limit
	if limit <= 0 || limit > 10000 {
		limit = 100
	}
	sb.WriteString(fmt.Sprintf(" LIMIT %d", limit))

	rows, err := s.db.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EventResponse
	for rows.Next() {
		var e EventResponse
		var payload sql.NullString
		if err := rows.Scan(&e.ID, &e.Kind, &e.NodeName, &e.SystemName, &e.Namespace,
			&e.Timestamp, &payload); err != nil {
			return nil, err
		}
		e.Payload = fromJSONMap(payload.String)
		out = append(out, e)
	}
	return out, nil
}

func (s *Store) CountEvents(req CountEventsRequest) (int64, error) {
	var sb strings.Builder
	sb.WriteString("SELECT COUNT(*) FROM events WHERE 1=1")
	args := []interface{}{}
	if req.Namespace != "" {
		sb.WriteString(" AND namespace=?")
		args = append(args, req.Namespace)
	}
	if req.SystemName != "" {
		sb.WriteString(" AND system_name=?")
		args = append(args, req.SystemName)
	}
	if req.NodeName != "" {
		sb.WriteString(" AND node_name=?")
		args = append(args, req.NodeName)
	}
	if len(req.Kinds) > 0 {
		placeholders := make([]string, len(req.Kinds))
		for i, k := range req.Kinds {
			placeholders[i] = "?"
			args = append(args, k)
		}
		sb.WriteString(" AND kind IN (" + strings.Join(placeholders, ",") + ")")
	}
	var count int64
	err := s.db.QueryRow(sb.String(), args...).Scan(&count)
	return count, err
}

// --- Source operations ---

func (s *Store) SaveSource(ns, name, source string) error {
	// Source is stored in the systems table (the source column)
	_, err := s.db.Exec(`UPDATE systems SET source=?, updated_at=? WHERE namespace=? AND name=?`,
		source, now(), ns, name)
	return err
}

func (s *Store) GetSource(ns, name string) (string, error) {
	var source string
	err := s.db.QueryRow(`SELECT source FROM systems WHERE namespace=? AND name=?`, ns, name).Scan(&source)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return source, err
}

func (s *Store) ListSources() ([]SourceEntry, error) {
	rows, err := s.db.Query(`SELECT namespace, name FROM systems ORDER BY namespace, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SourceEntry
	for rows.Next() {
		var e SourceEntry
		if err := rows.Scan(&e.Namespace, &e.Name); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// --- Registry (node packages) ---

func (s *Store) PushNodePackage(req PushNodeRequest) error {
	checksum := sha256hex(req.Content)
	_, err := s.db.Exec(
		`INSERT INTO node_packages (uri, category, version, manifest, content, content_type, checksum, created_at, downloads)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)
		 ON CONFLICT(uri) DO UPDATE SET
		   category=excluded.category, version=excluded.version, manifest=excluded.manifest,
		   content=excluded.content, content_type=excluded.content_type, checksum=excluded.checksum,
		   created_at=excluded.created_at`,
		req.URI, req.Category, req.Version, req.Manifest, req.Content, req.ContentType, checksum, now())
	return err
}

func (s *Store) PullNodePackage(uri string) (*NodePackageResponse, error) {
	// Increment download count
	s.db.Exec(`UPDATE node_packages SET downloads=downloads+1 WHERE uri=?`, uri)
	row := s.db.QueryRow(
		`SELECT uri, category, version, manifest, content, content_type, checksum, created_at, downloads
		 FROM node_packages WHERE uri=?`, uri)
	var p NodePackageResponse
	err := row.Scan(&p.URI, &p.Category, &p.Version, &p.Manifest, &p.Content, &p.ContentType,
		&p.Checksum, &p.CreatedAt, &p.Downloads)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

func (s *Store) GetNodeInfo(uri string) (*NodeInfoResponse, error) {
	row := s.db.QueryRow(
		`SELECT uri, category, version, manifest, content_type, checksum, created_at, downloads
		 FROM node_packages WHERE uri=?`, uri)
	var p NodeInfoResponse
	err := row.Scan(&p.URI, &p.Category, &p.Version, &p.Manifest, &p.ContentType,
		&p.Checksum, &p.CreatedAt, &p.Downloads)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

func (s *Store) ListNodePackages(category string) ([]NodeInfoResponse, error) {
	var rows *sql.Rows
	var err error
	if category != "" {
		rows, err = s.db.Query(
			`SELECT uri, category, version, manifest, content_type, checksum, created_at, downloads
			 FROM node_packages WHERE category=? ORDER BY uri`, category)
	} else {
		rows, err = s.db.Query(
			`SELECT uri, category, version, manifest, content_type, checksum, created_at, downloads
			 FROM node_packages ORDER BY uri`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeInfoResponse
	for rows.Next() {
		var p NodeInfoResponse
		if err := rows.Scan(&p.URI, &p.Category, &p.Version, &p.Manifest, &p.ContentType,
			&p.Checksum, &p.CreatedAt, &p.Downloads); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Store) SearchNodePackages(keyword string) ([]NodeInfoResponse, error) {
	rows, err := s.db.Query(
		`SELECT uri, category, version, manifest, content_type, checksum, created_at, downloads
		 FROM node_packages WHERE uri LIKE ? OR manifest LIKE ? ORDER BY uri`,
		"%"+keyword+"%", "%"+keyword+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeInfoResponse
	for rows.Next() {
		var p NodeInfoResponse
		if err := rows.Scan(&p.URI, &p.Category, &p.Version, &p.Manifest, &p.ContentType,
			&p.Checksum, &p.CreatedAt, &p.Downloads); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Store) DeleteNodePackage(uri string) error {
	_, err := s.db.Exec(`DELETE FROM node_packages WHERE uri=?`, uri)
	return err
}

func (s *Store) NodePackageExists(uri string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM node_packages WHERE uri=?`, uri).Scan(&count)
	return count > 0, err
}

// --- Registry records (built-in node registration) ---

func (s *Store) SaveRegistryRecord(req SaveRegistryRequest) error {
	active := 0
	if req.Active {
		active = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO registry (uri, pattern, category, active, description)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(uri) DO UPDATE SET
		   pattern=excluded.pattern, category=excluded.category, active=excluded.active,
		   description=excluded.description`,
		req.URI, req.Pattern, req.Category, active, req.Description)
	return err
}

func (s *Store) FindRegistryRecord(uri string) (*RegistryResponse, error) {
	row := s.db.QueryRow(`SELECT uri, pattern, category, active, description FROM registry WHERE uri=?`, uri)
	var r RegistryResponse
	var activeInt int
	err := row.Scan(&r.URI, &r.Pattern, &r.Category, &activeInt, &r.Description)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	r.Active = activeInt != 0
	return &r, err
}

func (s *Store) ListRegistryRecords() ([]RegistryResponse, error) {
	rows, err := s.db.Query(`SELECT uri, pattern, category, active, description FROM registry ORDER BY uri`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RegistryResponse
	for rows.Next() {
		var r RegistryResponse
		var activeInt int
		if err := rows.Scan(&r.URI, &r.Pattern, &r.Category, &activeInt, &r.Description); err != nil {
			return nil, err
		}
		r.Active = activeInt != 0
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) RegistryExists(uri string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM registry WHERE uri=?`, uri).Scan(&count)
	return count > 0, err
}

// --- Migration ---

func (s *Store) MigrateLegacy(stateRoot string) (*MigrationResult, error) {
	legacyDir := filepath.Join(stateRoot, "systems")
	if _, err := os.Stat(legacyDir); os.IsNotExist(err) {
		return &MigrationResult{}, nil
	}
	result := &MigrationResult{}
	err := filepath.Walk(legacyDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if info.Name() == "source.ts" {
			// Extract namespace/system from path: systems/<ns>/<sys>/source.ts
			rel, _ := filepath.Rel(legacyDir, path)
			parts := strings.Split(filepath.Dir(rel), string(filepath.Separator))
			if len(parts) >= 2 {
				ns, sysName := parts[0], parts[1]
				data, err := os.ReadFile(path)
				if err == nil {
					s.SaveSystem(SaveSystemRequest{
						Namespace: ns, Name: sysName, Source: string(data),
						State: "ACTIVE", Health: "HEALTHY", Version: 1,
					})
					result.Systems++
				}
			}
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	// Rename systems/ to systems.backup/
	backupDir := filepath.Join(stateRoot, "systems.backup")
	os.Rename(legacyDir, backupDir)
	if result.Systems > 0 {
		log.Printf("[INFO] Migrated %d systems, renamed %s to %s", result.Systems, legacyDir, backupDir)
	}
	return result, nil
}
