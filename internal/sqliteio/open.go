package sqliteio

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"runtime"
	"strings"

	_ "modernc.org/sqlite"
)

// OpenReadonly opens a SQLite database in read-only mode with defensive pragmas.
func OpenReadonly(ctx context.Context, path string) (*sql.DB, error) {
	u := url.URL{Path: path, RawQuery: "mode=ro"}
	dsn := "file:" + u.String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.ExecContext(ctx, "PRAGMA query_only = ON"); err != nil {
		db.Close()
		return nil, wrapAccessError(path, err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, wrapAccessError(path, err)
	}

	return db, nil
}

// wrapAccessError adds macOS-specific guidance when SQLite returns misleading
// "out of memory" errors that actually indicate TCC/Full Disk Access denial.
func wrapAccessError(path string, err error) error {
	if runtime.GOOS == "darwin" && strings.Contains(err.Error(), "out of memory") {
		return fmt.Errorf("%s: access denied by macOS (grant Full Disk Access to the agent in System Settings > Privacy & Security)", path)
	}
	return fmt.Errorf("%s: %w", path, err)
}
