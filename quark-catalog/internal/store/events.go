package store

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/quarkloop/quark/quark-catalog/internal/api"
)

// --- Event operations ---

// AppendEvent inserts a single event row, replacing any existing row
// with the same ID.
func (s *Store) AppendEvent(req api.AppendEventRequest) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO events (id, kind, node_name, system_name, namespace, timestamp, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		req.ID, req.Kind, req.NodeName, req.SystemName, req.Namespace,
		req.Timestamp, toJSON(req.Payload),
	)
	return err
}

// AppendEvents batches AppendEvent across many rows in a single tx.
func (s *Store) AppendEvents(reqs []api.AppendEventRequest) error {
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

// QueryEvents selects events by optional filters, ordered newest-first.
// Limit defaults to 100 and is clamped to 10000 to avoid runaway queries.
func (s *Store) QueryEvents(req api.QueryEventsRequest) ([]api.EventResponse, error) {
	q := newQueryBuilder("SELECT id, kind, node_name, system_name, namespace, timestamp, payload FROM events WHERE 1=1")
	if req.Namespace != "" {
		q.add(" AND namespace=?", req.Namespace)
	}
	if req.SystemName != "" {
		q.add(" AND system_name=?", req.SystemName)
	}
	if req.NodeName != "" {
		q.add(" AND node_name=?", req.NodeName)
	}
	if len(req.Kinds) > 0 {
	 placeholders := make([]string, len(req.Kinds))
	 args := make([]any, len(req.Kinds))
	 for i, k := range req.Kinds {
	 	placeholders[i] = "?"
	 	args[i] = k
	 }
	 q.add(" AND kind IN ("+strings.Join(placeholders, ",")+")", args...)
	}
	q.sql.WriteString(" ORDER BY timestamp DESC")
	limit := req.Limit
	if limit <= 0 || limit > 10000 {
		limit = 100
	}
	q.sql.WriteString(fmt.Sprintf(" LIMIT %d", limit))

	rows, err := s.db.Query(q.sql.String(), q.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []api.EventResponse
	for rows.Next() {
		var e api.EventResponse
		var payload sql.NullString
		if err := rows.Scan(&e.ID, &e.Kind, &e.NodeName, &e.SystemName, &e.Namespace,
			&e.Timestamp, &payload); err != nil {
			return nil, err
		}
		e.Payload = fromJSONAnyMap(payload.String)
		out = append(out, e)
	}
	return out, nil
}

// CountEvents returns the number of events matching the filters.
func (s *Store) CountEvents(req api.CountEventsRequest) (int64, error) {
	q := newQueryBuilder("SELECT COUNT(*) FROM events WHERE 1=1")
	if req.Namespace != "" {
		q.add(" AND namespace=?", req.Namespace)
	}
	if req.SystemName != "" {
		q.add(" AND system_name=?", req.SystemName)
	}
	if req.NodeName != "" {
		q.add(" AND node_name=?", req.NodeName)
	}
	if len(req.Kinds) > 0 {
	 placeholders := make([]string, len(req.Kinds))
	 args := make([]any, len(req.Kinds))
	 for i, k := range req.Kinds {
	 	placeholders[i] = "?"
	 	args[i] = k
	 }
	 q.add(" AND kind IN ("+strings.Join(placeholders, ",")+")", args...)
	}
	var count int64
	err := s.db.QueryRow(q.sql.String(), q.args...).Scan(&count)
	return count, err
}

// --- query builder helper (events-only for now; promote to store.go if reused) ---

type queryBuilder struct {
	sql  strings.Builder
	args []any
}

func newQueryBuilder(prefix string) *queryBuilder {
	var sb strings.Builder
	sb.WriteString(prefix)
	return &queryBuilder{sql: sb}
}

func (q *queryBuilder) add(clause string, args ...any) {
	q.sql.WriteString(clause)
	q.args = append(q.args, args...)
}
