//go:build !windows

package selfupdate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// applyBinary downloads the new binary from r and atomically replaces exe.
func applyBinary(exe string, r io.Reader) error {
	dir := filepath.Dir(exe)

	tmp, err := os.CreateTemp(dir, "osprey-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, 0o755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if err := os.Rename(tmpPath, exe); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename to target: %w", err)
	}
	return nil
}
