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
	Format      string            // "json" or "text"
	DBOverrides []string          // explicit DB paths to scan instead of auto-discovery
	DBCutoffs   map[string]time.Time // per-DB-path cutoff overrides (from last successful scan)
}

// Duration is a flag.Value that extends time.ParseDuration with "d" and "w" units.
// Usage: flag.Var(&d, "lookback", "duration (e.g. 24h, 5d, 2w)")
type Duration struct {
	D time.Duration
}

func (d *Duration) String() string {
	if d == nil {
		return "24h"
	}
	return d.D.String()
}

func (d *Duration) Set(s string) error {
	parsed, err := parseDuration(s)
	if err != nil {
		return err
	}
	d.D = parsed
	return nil
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	var total time.Duration
	remaining := s

	for remaining != "" {
		i := 0
		for i < len(remaining) && (remaining[i] == '.' || (remaining[i] >= '0' && remaining[i] <= '9')) {
			i++
		}
		if i == 0 {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		numStr := remaining[:i]
		remaining = remaining[i:]

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
			d, err := time.ParseDuration(numStr + unit)
			if err != nil {
				return 0, fmt.Errorf("invalid duration %q", s)
			}
			total += d
		}
	}
	return total, nil
}
