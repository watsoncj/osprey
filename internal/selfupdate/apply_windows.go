//go:build windows

package selfupdate

import (
	"fmt"
	"io"
	"os"
)

// applyBinary on Windows cannot atomically replace a running executable.
// Instead, it renames the current exe to .old, then writes the new binary
// to the original path. The .old file is cleaned up on next startup.
func applyBinary(exe string, r io.Reader) error {
	oldPath := exe + ".old"

	// Clean up any previous .old file.
	os.Remove(oldPath)

	// Move the running executable out of the way.
	if err := os.Rename(exe, oldPath); err != nil {
		return fmt.Errorf("rename current exe to .old: %w", err)
	}

	f, err := os.OpenFile(exe, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		// Try to restore the old binary.
		os.Rename(oldPath, exe)
		return fmt.Errorf("create new exe: %w", err)
	}

	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(exe)
		os.Rename(oldPath, exe)
		return fmt.Errorf("write new exe: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(exe)
		os.Rename(oldPath, exe)
		return fmt.Errorf("close new exe: %w", err)
	}

	return nil
}
