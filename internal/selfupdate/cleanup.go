package selfupdate

import (
	"log"
	"os"
	"path/filepath"
)

// Cleanup removes stale .old files left over from a previous update.
// Call this at startup.
func Cleanup() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return
	}
	old := exe + ".old"
	if _, err := os.Stat(old); err == nil {
		if err := os.Remove(old); err != nil {
			log.Printf("selfupdate: failed to clean up %s: %v", old, err)
		}
	}
}
