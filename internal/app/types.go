package app

import "time"

// Config holds runtime configuration parsed from CLI flags.
type Config struct {
	Lookback    time.Duration
	Format      string   // "json" or "text"
	DBOverrides []string // explicit DB paths to scan instead of auto-discovery
}
