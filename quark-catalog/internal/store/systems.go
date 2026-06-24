package store

import (
        "database/sql"

        "github.com/quarkloop/quark/quark-catalog/internal/api"
)

// --- System operations ---

// SaveSystem upserts a system row.
func (s *Store) SaveSystem(req api.SaveSystemRequest) error {
        _, err := s.db.Exec(
                `INSERT INTO systems (namespace, name, source, state, health, version, created_at, updated_at)
                 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                 ON CONFLICT(namespace, name) DO UPDATE SET
                   source=excluded.source, state=excluded.state, health=excluded.health,
                   version=excluded.version, updated_at=excluded.updated_at`,
                req.Namespace, req.Name, req.Source, req.State, req.Health, req.Version, now(), now(),
        )
        return err
}

// GetSystem returns a single system by (namespace, name). Returns
// (nil, nil) when no row matches.
func (s *Store) GetSystem(ns, name string) (*api.SystemResponse, error) {
        row := s.db.QueryRow(
                `SELECT namespace, name, source, state, health, version, created_at, updated_at
                 FROM systems WHERE namespace=? AND name=?`, ns, name,
        )
        return scanSystemRow(row)
}

// ListSystems returns all systems in a namespace. If ns is empty,
// returns all systems across every namespace.
func (s *Store) ListSystems(ns string) ([]api.SystemResponse, error) {
        var rows *sql.Rows
        var err error
        if ns != "" {
                rows, err = s.db.Query(
                        `SELECT namespace, name, source, state, health, version, created_at, updated_at
                         FROM systems WHERE namespace=? ORDER BY name`, ns,
                )
        } else {
                rows, err = s.db.Query(
                        `SELECT namespace, name, source, state, health, version, created_at, updated_at
                         FROM systems ORDER BY namespace, name`,
                )
        }
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        var out []api.SystemResponse
        for rows.Next() {
                var sr api.SystemResponse
                if err := rows.Scan(&sr.Namespace, &sr.Name, &sr.Source, &sr.State, &sr.Health,
                        &sr.Version, &sr.CreatedAt, &sr.UpdatedAt); err != nil {
                        return nil, err
                }
                out = append(out, sr)
        }
        return out, nil
}

// DeleteSystem removes a system row.
func (s *Store) DeleteSystem(ns, name string) error {
        _, err := s.db.Exec(`DELETE FROM systems WHERE namespace=? AND name=?`, ns, name)
        return err
}

// UpdateSystemState updates the state/health/version columns.
func (s *Store) UpdateSystemState(ns, name, state, health string, version int64) error {
        _, err := s.db.Exec(
                `UPDATE systems SET state=?, health=?, version=?, updated_at=? WHERE namespace=? AND name=?`,
                state, health, version, now(), ns, name,
        )
        return err
}

// scanSystemRow scans a single system row from a *sql.Row.
func scanSystemRow(row *sql.Row) (*api.SystemResponse, error) {
        var sr api.SystemResponse
        err := row.Scan(&sr.Namespace, &sr.Name, &sr.Source, &sr.State, &sr.Health,
                &sr.Version, &sr.CreatedAt, &sr.UpdatedAt)
        if err == sql.ErrNoRows {
                return nil, nil
        }
        return &sr, err
}
