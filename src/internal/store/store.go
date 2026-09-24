// Package store keeps quests, cards, needs and the quest log in SQLite, and
// computes card status. It is the only package that touches the database;
// the SQL is plain so another engine (Postgres) stays possible.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver, registered as "sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// CacheTTL is how long GitHub data is served without refetching.
const CacheTTL = 60 * time.Second

// Store is the quest database plus the GitHub client used to fill issue cards.
type Store struct {
	db  *sql.DB
	gh  GitHub
	now func() time.Time
}

// Open opens (creating if needed) the database at path and migrates it.
func Open(path string, gh GitHub) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	// One connection: SQLite has one writer anyway, and this rules out
	// SQLITE_BUSY between our own connections.
	db.SetMaxOpenConns(1)
	s := &Store{db: db, gh: gh, now: time.Now}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate %s: %w", path, err)
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// migrate applies every embedded migration newer than schema_version, each in
// its own transaction. Files are named NNN_name.sql.
func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return err
	}
	var current int
	err := s.db.QueryRowContext(ctx, `SELECT version FROM schema_version`).Scan(&current)
	if err == sql.ErrNoRows {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO schema_version (version) VALUES (0)`); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	latest := 0
	for _, name := range names {
		if n, err := strconv.Atoi(strings.SplitN(filepath.Base(name), "_", 2)[0]); err == nil && n > latest {
			latest = n
		}
	}
	if current > latest {
		return fmt.Errorf("the database is at schema version %d but this mikado knows only up to %d (was it made by a newer or pre-release build?)", current, latest)
	}
	for _, name := range names {
		base := filepath.Base(name)
		n, err := strconv.Atoi(strings.SplitN(base, "_", 2)[0])
		if err != nil {
			return fmt.Errorf("migration %s: name must start with a number", base)
		}
		if n <= current {
			continue
		}
		body, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", base, err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE schema_version SET version = ?`, n); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		current = n
	}
	return nil
}

// querier is what *sql.DB and *sql.Tx have in common.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// inTx runs fn in a transaction, committing only if it returns nil.
func (s *Store) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) stamp() string { return s.now().UTC().Format(time.RFC3339) }

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
