package selfupdate

import (
	"fmt"
	"strconv"
	"strings"
)

type semver struct {
	Major, Minor, Patch int
}

// parseSemver parses a version string like "v1.2.3" or "1.2.3".
func parseSemver(s string) (semver, error) {
	s = strings.TrimPrefix(s, "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return semver{}, fmt.Errorf("invalid version %q", s)
	}
	// Strip anything after a hyphen (prerelease) or plus (build metadata).
	patch := parts[2]
	if i := strings.IndexAny(patch, "-+"); i >= 0 {
		patch = patch[:i]
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return semver{}, fmt.Errorf("invalid major %q: %w", parts[0], err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return semver{}, fmt.Errorf("invalid minor %q: %w", parts[1], err)
	}
	p, err := strconv.Atoi(patch)
	if err != nil {
		return semver{}, fmt.Errorf("invalid patch %q: %w", patch, err)
	}
	return semver{Major: major, Minor: minor, Patch: p}, nil
}

// newerThan reports whether v is newer than other.
func (v semver) newerThan(other semver) bool {
	if v.Major != other.Major {
		return v.Major > other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor > other.Minor
	}
	return v.Patch > other.Patch
}
