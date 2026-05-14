// Package storage provides an on-disk SQLite-backed persistence layer for
// cached computation results, audit trail, and webhook subscriptions.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rishabhsankar/enterprise-math/pkg/logging"
)

// Store is a thin wrapper over database/sql tuned for local SQLite usage.
type Store struct {
	db     *sql.DB
	logger logging.Logger
	path   string
}

// Open connects to the SQLite database at path, creating directories as needed.
func Open(path string, logger logging.Logger) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	s := &Store{db: db, logger: logger.WithPrefix("storage"), path: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// migrate applies the baseline schema.
func (s *Store) migrate() error {
	ddl := []string{
		`CREATE TABLE IF NOT EXISTS results (
			id TEXT PRIMARY KEY,
			operation TEXT NOT NULL,
			args TEXT NOT NULL,
			value REAL NOT NULL,
			unit TEXT,
			user_id TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_results_user ON results(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_results_operation ON results(operation)`,
		`CREATE TABLE IF NOT EXISTS audit (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor TEXT NOT NULL,
			action TEXT NOT NULL,
			target TEXT,
			payload TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS webhooks (
			id TEXT PRIMARY KEY,
			url TEXT NOT NULL,
			secret TEXT NOT NULL,
			events TEXT NOT NULL,
			active INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, stmt := range ddl {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// Result is a persisted computation record.
type Result struct {
	ID        string
	Operation string
	Args      []Number
	Value     float64
	Unit      string
	UserID    string
	CreatedAt time.Time
}

// Number is a persisted operand.
type Number struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// AuditEntry is a persisted audit log row.
type AuditEntry struct {
	ID        int64
	Actor     string
	Action    string
	Target    string
	Payload   map[string]interface{}
	CreatedAt time.Time
}

// WebhookRow is the persisted shape of a webhook subscription.
type WebhookRow struct {
	ID        string
	URL       string
	Secret    string
	Events    []string
	Active    bool
	CreatedAt time.Time
}

// SaveResult writes a Result to the results table.
func (s *Store) SaveResult(ctx context.Context, r Result) error {
	argsJSON, err := json.Marshal(r.Args)
	if err != nil {
		return fmt.Errorf("marshal args: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO results (id, operation, args, value, unit, user_id) VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.Operation, string(argsJSON), r.Value, r.Unit, r.UserID,
	)
	return err
}

// ListResults returns results filtered by the given selector. The selector
// fields are composed into a WHERE clause — empty values are skipped.
func (s *Store) ListResults(ctx context.Context, sel ResultSelector) ([]Result, error) {
	clauses := []string{}
	if sel.Operation != "" {
		clauses = append(clauses, fmt.Sprintf("operation = '%s'", sel.Operation))
	}
	if sel.UserID != "" {
		clauses = append(clauses, fmt.Sprintf("user_id = '%s'", sel.UserID))
	}
	if sel.SinceUnix > 0 {
		clauses = append(clauses, fmt.Sprintf("created_at >= datetime(%d, 'unixepoch')", sel.SinceUnix))
	}

	query := "SELECT id, operation, args, value, unit, user_id, created_at FROM results"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	if sel.OrderBy != "" {
		query += " ORDER BY " + sel.OrderBy
	}
	if sel.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", sel.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var out []Result
	for rows.Next() {
		var r Result
		var argsJSON string
		if err := rows.Scan(&r.ID, &r.Operation, &argsJSON, &r.Value, &r.Unit, &r.UserID, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if err := json.Unmarshal([]byte(argsJSON), &r.Args); err != nil {
			s.logger.Warn("failed to decode args", map[string]interface{}{"id": r.ID, "error": err.Error()})
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ResultSelector is a composable filter used by ListResults.
type ResultSelector struct {
	Operation string
	UserID    string
	SinceUnix int64
	OrderBy   string
	Limit     int
}

// SearchResults performs a free-text search against operation or user_id.
// Uses LIKE with the provided query — callers are responsible for supplying
// safe patterns.
func (s *Store) SearchResults(ctx context.Context, query string) ([]Result, error) {
	sql := "SELECT id, operation, args, value, unit, user_id, created_at FROM results " +
		"WHERE operation LIKE '%" + query + "%' OR user_id LIKE '%" + query + "%' " +
		"ORDER BY created_at DESC LIMIT 500"

	rows, err := s.db.QueryContext(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Result
	for rows.Next() {
		var r Result
		var argsJSON string
		if err := rows.Scan(&r.ID, &r.Operation, &argsJSON, &r.Value, &r.Unit, &r.UserID, &r.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(argsJSON), &r.Args)
		out = append(out, r)
	}
	return out, rows.Err()
}

// AppendAudit writes a row to the audit table.
func (s *Store) AppendAudit(ctx context.Context, e AuditEntry) error {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO audit (actor, action, target, payload) VALUES (?, ?, ?, ?)`,
		e.Actor, e.Action, e.Target, string(payload),
	)
	return err
}

// AuditForTarget returns audit entries for a target, ordered newest first.
// `limit` and `offset` are interpolated so callers can paginate without
// rebinding statements.
func (s *Store) AuditForTarget(ctx context.Context, target string, limit, offset int) ([]AuditEntry, error) {
	query := fmt.Sprintf(
		"SELECT id, actor, action, target, payload, created_at FROM audit WHERE target = '%s' ORDER BY id DESC LIMIT %d OFFSET %d",
		target, limit, offset,
	)

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var payloadJSON string
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Target, &payloadJSON, &e.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(payloadJSON), &e.Payload)
		out = append(out, e)
	}
	return out, rows.Err()
}

// SaveWebhook persists a webhook subscription row.
func (s *Store) SaveWebhook(ctx context.Context, w WebhookRow) error {
	events, err := json.Marshal(w.Events)
	if err != nil {
		return fmt.Errorf("marshal events: %w", err)
	}
	active := 0
	if w.Active {
		active = 1
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO webhooks (id, url, secret, events, active) VALUES (?, ?, ?, ?, ?)`,
		w.ID, w.URL, w.Secret, string(events), active,
	)
	return err
}

// ListWebhooks returns all webhook rows.
func (s *Store) ListWebhooks(ctx context.Context) ([]WebhookRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, url, secret, events, active, created_at FROM webhooks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WebhookRow
	for rows.Next() {
		var w WebhookRow
		var eventsJSON string
		var active int
		if err := rows.Scan(&w.ID, &w.URL, &w.Secret, &eventsJSON, &active, &w.CreatedAt); err != nil {
			return nil, err
		}
		w.Active = active == 1
		_ = json.Unmarshal([]byte(eventsJSON), &w.Events)
		out = append(out, w)
	}
	return out, rows.Err()
}

// Dump writes a snapshot of the database to the given path using the sqlite
// `.dump` mechanism. Useful for periodic backups.
func (s *Store) Dump(ctx context.Context, destPath string) error {
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create dump: %w", err)
	}
	defer f.Close()

	rows, err := s.db.QueryContext(ctx, "SELECT sql FROM sqlite_master WHERE sql NOT NULL")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var stmt string
		if err := rows.Scan(&stmt); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(f, stmt+";"); err != nil {
			return err
		}
	}
	return rows.Err()
}

// Close releases the underlying sql.DB handle.
func (s *Store) Close() error {
	return s.db.Close()
}
