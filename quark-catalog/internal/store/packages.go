package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"

	"github.com/quarkloop/quark/quark-catalog/internal/api"
)

// --- Node package operations (pushed .ts/.so payloads) ---
//
// Node packages are stored in the "node_packages" table, addressed by
// URI (e.g. "source/timer:v1"). Push stores the content + a SHA-256
// checksum; pull increments the download counter and returns the full
// row including content. List/info variants omit the (potentially
// large) content blob.

// PushNodePackage upserts a node package row. The checksum is computed
// from Content inside this method so callers can't forget to set it.
func (s *Store) PushNodePackage(req api.PushNodeRequest) error {
	checksum := sha256hex(req.Content)
	_, err := s.db.Exec(
		`INSERT INTO node_packages (uri, category, version, manifest, content, content_type, checksum, created_at, downloads)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)
		 ON CONFLICT(uri) DO UPDATE SET
		   category=excluded.category, version=excluded.version, manifest=excluded.manifest,
		   content=excluded.content, content_type=excluded.content_type, checksum=excluded.checksum,
		   created_at=excluded.created_at`,
		req.URI, req.Category, req.Version, req.Manifest, req.Content, req.ContentType, checksum, now(),
	)
	return err
}

// PullNodePackage returns the full package (content included) by URI
// and increments the download counter as a side effect.
func (s *Store) PullNodePackage(uri string) (*api.NodePackageResponse, error) {
	// Increment download count first (best-effort, error is logged but not returned).
	_, _ = s.db.Exec(`UPDATE node_packages SET downloads=downloads+1 WHERE uri=?`, uri)

	row := s.db.QueryRow(
		`SELECT uri, category, version, manifest, content, content_type, checksum, created_at, downloads
		 FROM node_packages WHERE uri=?`, uri,
	)
	var p api.NodePackageResponse
	err := row.Scan(&p.URI, &p.Category, &p.Version, &p.Manifest, &p.Content, &p.ContentType,
		&p.Checksum, &p.CreatedAt, &p.Downloads)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

// GetNodeInfo returns package metadata (no content) by URI.
func (s *Store) GetNodeInfo(uri string) (*api.NodeInfoResponse, error) {
	row := s.db.QueryRow(
		`SELECT uri, category, version, manifest, content_type, checksum, created_at, downloads
		 FROM node_packages WHERE uri=?`, uri,
	)
	var p api.NodeInfoResponse
	err := row.Scan(&p.URI, &p.Category, &p.Version, &p.Manifest, &p.ContentType,
		&p.Checksum, &p.CreatedAt, &p.Downloads)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

// ListNodePackages returns metadata for all packages, optionally
// filtered by category. Ordered by URI.
func (s *Store) ListNodePackages(category string) ([]api.NodeInfoResponse, error) {
	var rows *sql.Rows
	var err error
	if category != "" {
		rows, err = s.db.Query(
			`SELECT uri, category, version, manifest, content_type, checksum, created_at, downloads
			 FROM node_packages WHERE category=? ORDER BY uri`, category,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT uri, category, version, manifest, content_type, checksum, created_at, downloads
			 FROM node_packages ORDER BY uri`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodeInfoRows(rows)
}

// SearchNodePackages returns metadata for packages whose URI or
// manifest contains keyword (LIKE %keyword%).
func (s *Store) SearchNodePackages(keyword string) ([]api.NodeInfoResponse, error) {
	rows, err := s.db.Query(
		`SELECT uri, category, version, manifest, content_type, checksum, created_at, downloads
		 FROM node_packages WHERE uri LIKE ? OR manifest LIKE ? ORDER BY uri`,
		"%"+keyword+"%", "%"+keyword+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodeInfoRows(rows)
}

// DeleteNodePackage removes a package row by URI.
func (s *Store) DeleteNodePackage(uri string) error {
	_, err := s.db.Exec(`DELETE FROM node_packages WHERE uri=?`, uri)
	return err
}

// NodePackageExists reports whether a package row exists for the URI.
func (s *Store) NodePackageExists(uri string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM node_packages WHERE uri=?`, uri).Scan(&count)
	return count > 0, err
}

// scanNodeInfoRows iterates a *sql.Rows of node_package metadata columns
// (without content) into a slice.
func scanNodeInfoRows(rows *sql.Rows) ([]api.NodeInfoResponse, error) {
	var out []api.NodeInfoResponse
	for rows.Next() {
		var p api.NodeInfoResponse
		if err := rows.Scan(&p.URI, &p.Category, &p.Version, &p.Manifest, &p.ContentType,
			&p.Checksum, &p.CreatedAt, &p.Downloads); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// sha256hex returns the hex-encoded SHA-256 hash of data. Used for
// computing package checksums at push time.
func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
