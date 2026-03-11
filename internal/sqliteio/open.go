package sqliteio

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	_ "modernc.org/sqlite"
)

// DB wraps sql.DB and tracks a temp directory for cleanup.
type DB struct {
	*sql.DB
	tmpDir string
}

// Close closes the database and removes any temp files.
func (d *DB) Close() error {
	err := d.DB.Close()
	if d.tmpDir != "" {
		os.RemoveAll(d.tmpDir)
	}
	return err
}

// OpenReadonly opens a SQLite database for reading. It copies the database
// (and any WAL/SHM files) to a temp directory first so that SQLite can
// replay the WAL even without write access to the original files.
func OpenReadonly(ctx context.Context, path string) (*DB, error) {
	tmpDir, tmpPath, err := copyDBFiles(path)
	if err != nil {
		return nil, fmt.Errorf("copy %s: %w", path, err)
	}

	// Open the temp copy in read-write mode so SQLite can replay the WAL,
	// then set query_only to prevent accidental writes.
	dsn := "file:" + tmpPath
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.ExecContext(ctx, "PRAGMA query_only = ON"); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		return nil, wrapAccessError(path, err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		return nil, wrapAccessError(path, err)
	}

	return &DB{DB: db, tmpDir: tmpDir}, nil
}

// copyDBFiles copies the main DB file and any -wal/-shm companions to a
// temp directory so SQLite can replay the WAL.
func copyDBFiles(path string) (tmpDir, tmpPath string, err error) {
	tmpDir, err = os.MkdirTemp("", "osprey-db-*")
	if err != nil {
		return "", "", err
	}

	base := filepath.Base(path)
	tmpPath = filepath.Join(tmpDir, base)

	if err := copyFile(path, tmpPath); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", err
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		src := path + suffix
		if _, statErr := os.Stat(src); statErr == nil {
			if cpErr := copyFile(src, tmpPath+suffix); cpErr != nil {
				os.RemoveAll(tmpDir)
				return "", "", cpErr
			}
		}
	}

	return tmpDir, tmpPath, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// wrapAccessError adds macOS-specific guidance when SQLite returns misleading
// "out of memory" errors that actually indicate TCC/Full Disk Access denial.
func wrapAccessError(path string, err error) error {
	if runtime.GOOS == "darwin" && strings.Contains(err.Error(), "out of memory") {
		return fmt.Errorf("%s: access denied by macOS (grant Full Disk Access to the agent in System Settings > Privacy & Security)", path)
	}
	return fmt.Errorf("set query_only on %s: %w", path, err)
}
