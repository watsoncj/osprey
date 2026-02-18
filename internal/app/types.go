package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration parsed from CLI flags.
type Config struct {
	Lookback    time.Duration
	Format      string   // "json" or "text"
	DBOverrides []string // explicit DB paths to scan instead of auto-discovery
}

// ParseLookback parses a duration string that supports days and weeks
// in addition to Go's standard time.ParseDuration units.
// Examples: "24h", "5d", "2w", "1w3d", "2d12h".
func ParseLookback(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	var total time.Duration
	remaining := s

	for remaining != "" {
		// Find the next numeric part.
		i := 0
		for i < len(remaining) && (remaining[i] == '.' || (remaining[i] >= '0' && remaining[i] <= '9')) {
			i++
		}
		if i == 0 {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		numStr := remaining[:i]
		remaining = remaining[i:]

		// Find the unit suffix.
		j := 0
		for j < len(remaining) && remaining[j] >= 'a' && remaining[j] <= 'z' {
			j++
		}
		if j == 0 {
			return 0, fmt.Errorf("missing unit in duration %q", s)
		}
		unit := remaining[:j]
		remaining = remaining[j:]

		val, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}

		switch unit {
		case "w":
			total += time.Duration(val * float64(7*24*time.Hour))
		case "d":
			total += time.Duration(val * float64(24*time.Hour))
		default:
			// Delegate to Go's parser for h, m, s, ms, us, ns.
			d, err := time.ParseDuration(numStr + unit)
			if err != nil {
				return 0, fmt.Errorf("invalid duration %q", s)
			}
			total += d
		}
	}
	return total, nil
}
