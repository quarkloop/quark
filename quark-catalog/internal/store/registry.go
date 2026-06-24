package store

import (
	"database/sql"

	"github.com/quarkloop/quark/quark-catalog/internal/api"
)

// --- Registry record operations (built-in node descriptors) ---

// SaveRegistryRecord upserts a registry row by URI.
func (s *Store) SaveRegistryRecord(req api.SaveRegistryRequest) error {
	_, err := s.db.Exec(
		`INSERT INTO registry (uri, pattern, description)
		 VALUES (?, ?, ?)
		 ON CONFLICT(uri) DO UPDATE SET
		   pattern=excluded.pattern,
		   description=excluded.description`,
		req.URI, req.Pattern, req.Description,
	)
	return err
}

// FindRegistryRecord returns a single registry row by URI. Returns
// (nil, nil) when no row matches.
func (s *Store) FindRegistryRecord(uri string) (*api.RegistryResponse, error) {
	row := s.db.QueryRow(
		`SELECT uri, pattern, description FROM registry WHERE uri=?`, uri,
	)
	var r api.RegistryResponse
	err := row.Scan(&r.URI, &r.Pattern, &r.Description)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &r, err
}

// ListRegistryRecords returns all registry rows, ordered by URI.
func (s *Store) ListRegistryRecords() ([]api.RegistryResponse, error) {
	rows, err := s.db.Query(`SELECT uri, pattern, description FROM registry ORDER BY uri`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []api.RegistryResponse
	for rows.Next() {
		var r api.RegistryResponse
		if err := rows.Scan(&r.URI, &r.Pattern, &r.Description); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// RegistryExists reports whether a registry row exists for the given URI.
func (s *Store) RegistryExists(uri string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM registry WHERE uri=?`, uri).Scan(&count)
	return count > 0, err
}
