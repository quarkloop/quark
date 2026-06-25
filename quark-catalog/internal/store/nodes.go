package store

import (
        "database/sql"
        "encoding/json"

        "github.com/quarkloop/quark/quark-catalog/internal/api"
)

// --- Node operations ---

// nodeInsertSQL is the parameterised UPSERT for a single node row.
//
// Columns (18 total, in declaration order from store.go's CREATE TABLE):
//   1.  namespace           ?
//   2.  system_name         ?
//   3.  name                ?
//   4.  uri                 ?
//   5.  state               ?
//   6.  health              ?
//   7.  version             ?
//   8.  error_message       NULL  (literal — SaveNode never persists an error message;
//                                   updateState carries one when a node fails)
//   9.  listens             ?
//   10. events              ?
//   11. config              ?
//   12. labels              ?
//   13. annotations         ?
//   14. on_failure_retry    ?
//   15. on_failure_route_to ?
//   16. timeout             ?
//   17. created_at          ?
//   18. updated_at          ?
//
// That is 17 `?` placeholders + 1 NULL literal = 18 values for 18 columns.
// A previous version of this statement had 19 value slots (one extra `?`)
// which SQLite rejects with "19 values for 18 columns" — that broke
// every node save silently because the catalog server swallows the
// error in a replyError and the control plane treats a non-OK reply as
// a soft failure (log.warn, not throw).
//
// The ON CONFLICT(...) DO UPDATE clause uses SQLite UPSERT semantics:
// each column on the left of `=excluded.<col>` pulls from the row that
// would have been inserted. A previous version used
// `uri=excluded.uri=excluded.category` which (a) referenced a
// non-existent `category` column (removed in the v8 URI refactor —
// see docs/NODE-URI.md) and (b) SQLite parses `a=b=c` as `a=(b=c)` —
// a boolean comparison that would store 0/1 in `uri`.
const nodeInsertSQL = `INSERT INTO nodes (namespace, system_name, name, uri, state, health, version,
  error_message, listens, events, config, labels, annotations,
  on_failure_retry, on_failure_route_to, timeout, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(namespace, system_name, name) DO UPDATE SET
  uri=excluded.uri, state=excluded.state, health=excluded.health,
  version=excluded.version, listens=excluded.listens, events=excluded.events,
  config=excluded.config, labels=excluded.labels, annotations=excluded.annotations,
  on_failure_retry=excluded.on_failure_retry, on_failure_route_to=excluded.on_failure_route_to,
  timeout=excluded.timeout, updated_at=excluded.updated_at`

// nodeInsertArgs binds the 17 non-NULL columns of SaveNodeRequest to the
// `?` slots in nodeInsertSQL. Shared by SaveNode and SaveNodes so the
// column↔arg mapping lives in exactly one place.
func nodeInsertArgs(req api.SaveNodeRequest) []any {
        return []any{
                req.Namespace, req.SystemName, req.Name, req.URI,
                req.State, req.Health, req.Version,
                toJSON(req.Listens), toJSON(req.Events), toJSON(req.Config),
                toJSON(req.Labels), toJSON(req.Annotations),
                req.OnFailureRetry, req.OnFailureRouteTo, req.Timeout,
                now(), now(),
        }
}

// SaveNode upserts a node row.
func (s *Store) SaveNode(req api.SaveNodeRequest) error {
        _, err := s.db.Exec(nodeInsertSQL, nodeInsertArgs(req)...)
        return err
}

// SaveNodes batches SaveNode across many rows in a single transaction.
// The prepared statement is shared across all rows in the batch — only
// the bound args differ per row.
func (s *Store) SaveNodes(reqs []api.SaveNodeRequest) error {
        tx, err := s.db.Begin()
        if err != nil {
                return err
        }
        defer tx.Rollback()
        stmt, err := tx.Prepare(nodeInsertSQL)
        if err != nil {
                return err
        }
        defer stmt.Close()
        for _, req := range reqs {
                if _, err = stmt.Exec(nodeInsertArgs(req)...); err != nil {
                        return err
                }
        }
        return tx.Commit()
}

// ListNodes returns all nodes for a (namespace, systemName).
func (s *Store) ListNodes(ns, sysName string) ([]api.NodeResponse, error) {
        rows, err := s.db.Query(
                `SELECT namespace, system_name, name, uri, state, health, version,
                   error_message, listens, events, config, labels, annotations,
                   on_failure_retry, on_failure_route_to, timeout, created_at, updated_at
                 FROM nodes WHERE namespace=? AND system_name=? ORDER BY name`, ns, sysName,
        )
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        return scanNodes(rows)
}

// ListNodesByNamespace returns all nodes in a namespace, across all systems.
func (s *Store) ListNodesByNamespace(ns string) ([]api.NodeResponse, error) {
        rows, err := s.db.Query(
                `SELECT namespace, system_name, name, uri, state, health, version,
                   error_message, listens, events, config, labels, annotations,
                   on_failure_retry, on_failure_route_to, timeout, created_at, updated_at
                 FROM nodes WHERE namespace=? ORDER BY system_name, name`, ns,
        )
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        return scanNodes(rows)
}

// DeleteNodesBySystem removes all nodes belonging to (namespace, systemName).
func (s *Store) DeleteNodesBySystem(ns, sysName string) error {
        _, err := s.db.Exec(`DELETE FROM nodes WHERE namespace=? AND system_name=?`, ns, sysName)
        return err
}

// scanNodes iterates a *sql.Rows of node columns into a slice of NodeResponse.
// JSON-encoded columns (listens, events, config, labels, annotations) are
// decoded inline; NULL columns are coerced to empty values.
func scanNodes(rows *sql.Rows) ([]api.NodeResponse, error) {
        var out []api.NodeResponse
        for rows.Next() {
                var n api.NodeResponse
                var listens, events, config, labels, annotations, errMsg sql.NullString
                if err := rows.Scan(&n.Namespace, &n.SystemName, &n.Name, &n.URI,
                        &n.State, &n.Health, &n.Version, &errMsg, &listens, &events,
                        &config, &labels, &annotations, &n.OnFailureRetry, &n.OnFailureRouteTo,
                        &n.Timeout, &n.CreatedAt, &n.UpdatedAt); err != nil {
                        return nil, err
                }
                n.ErrorMessage = errMsg.String
                n.Listens = fromJSONStringSlice(listens.String)
                n.Events = fromJSONStringSlice(events.String)
                n.Config = fromJSONAnyMap(config.String)
                n.Labels = fromJSONStringMap(labels.String)
                n.Annotations = fromJSONStringMap(annotations.String)
                out = append(out, n)
        }
        return out, nil
}

// --- JSON column helpers ---

// toJSON marshals v to a string, returning "" on error. Used for
// packing Go slices/maps into SQLite TEXT columns.
func toJSON(v any) string {
        b, err := json.Marshal(v)
        if err != nil {
                return ""
        }
        return string(b)
}

func fromJSONStringSlice(s string) []string {
        if s == "" {
                return []string{}
        }
        var r []string
        _ = json.Unmarshal([]byte(s), &r)
        return r
}

func fromJSONAnyMap(s string) map[string]any {
        if s == "" {
                return nil
        }
        var r map[string]any
        _ = json.Unmarshal([]byte(s), &r)
        return r
}

func fromJSONStringMap(s string) map[string]string {
        if s == "" {
                return nil
        }
        var r map[string]string
        _ = json.Unmarshal([]byte(s), &r)
        return r
}
