package finder

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ExpandPath resolves ~, environment variables, and globs in a path pattern.
// Returns all matching paths that exist on disk.
func ExpandPath(pattern string) []string {
	pattern = expandHome(pattern)
	pattern = os.ExpandEnv(pattern)

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil
	}

	var results []string
	for _, m := range matches {
		if info, err := os.Stat(m); err == nil && !info.IsDir() {
			results = append(results, m)
		}
	}
	return results
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

// UserDirs returns home directories to scan. On Windows with admin privileges
// it attempts to enumerate user profile directories. On macOS it returns the
// current user's home.
func UserDirs() []string {
	if runtime.GOOS == "windows" {
		return windowsUserDirs()
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{home}
}

func windowsUserDirs() []string {
	usersRoot := os.Getenv("SystemDrive") + `\Users`
	entries, err := os.ReadDir(usersRoot)
	if err != nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		return []string{home}
	}

	skip := map[string]bool{
		"public":  true,
		"default": true,
		"default user": true,
		"all users":    true,
	}

	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if skip[strings.ToLower(e.Name())] {
			continue
		}
		dirs = append(dirs, filepath.Join(usersRoot, e.Name()))
	}
	if len(dirs) == 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		return []string{home}
	}
	return dirs
}
