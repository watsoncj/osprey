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

// UserDirs returns home directories to scan. On Windows and macOS it
// enumerates user profile directories so that a service running as root
// can discover all users' browser history. On other platforms it falls
// back to the current user's home.
func UserDirs() []string {
	switch runtime.GOOS {
	case "windows":
		return windowsUserDirs()
	case "darwin":
		return darwinUserDirs()
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		return []string{home}
	}
}

func darwinUserDirs() []string {
	entries, err := os.ReadDir("/Users")
	if err != nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		return []string{home}
	}

	skip := map[string]bool{
		"shared": true,
		"guest":  true,
	}

	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if skip[name] || strings.HasPrefix(name, ".") {
			continue
		}
		dirs = append(dirs, filepath.Join("/Users", e.Name()))
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
