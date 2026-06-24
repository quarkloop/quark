package store

import (
	"database/sql"

	"github.com/quarkloop/quark/quark-catalog/internal/api"
)

// --- Source operations ---
//
// The source text of a .quark.ts file is stored as the "source" column
// on the systems row, so there is no separate sources table. These
// methods read/write that column and list the (namespace, name) pairs
// of systems that have non-empty source.

// SaveSource updates the source column for an existing system row.
// If the (namespace, name) does not exist, this is a no-op (the UPDATE
// matches zero rows).
func (s *Store) SaveSource(ns, name, source string) error {
	_, err := s.db.Exec(
		`UPDATE systems SET source=?, updated_at=? WHERE namespace=? AND name=?`,
		source, now(), ns, name,
	)
	return err
}

// GetSource returns the source text for a system. Returns "" if the
// system does not exist (callers distinguish "no row" from "empty
// source" by also calling GetSystem).
func (s *Store) GetSource(ns, name string) (string, error) {
	var source string
	err := s.db.QueryRow(
		`SELECT source FROM systems WHERE namespace=? AND name=?`, ns, name,
	).Scan(&source)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return source, err
}

// ListSources returns the (namespace, name) pair for every system row,
// regardless of whether source is non-empty.
func (s *Store) ListSources() ([]api.SourceEntry, error) {
	rows, err := s.db.Query(`SELECT namespace, name FROM systems ORDER BY namespace, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []api.SourceEntry
	for rows.Next() {
		var e api.SourceEntry
		if err := rows.Scan(&e.Namespace, &e.Name); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}
