package sqliteio

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

// OpenReadonly opens a SQLite database in read-only mode with defensive pragmas.
func OpenReadonly(ctx context.Context, path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro", url.PathEscape(path))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.ExecContext(ctx, "PRAGMA query_only = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set query_only on %s: %w", path, err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy_timeout on %s: %w", path, err)
	}

	return db, nil
}
