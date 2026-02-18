package model

import "time"

// Visit represents a single browser history entry.
type Visit struct {
	Time    time.Time `json:"time"`
	URL     string    `json:"url"`
	Title   string    `json:"title"`
	Browser string    `json:"browser"`
	DBPath  string    `json:"db_path"`

	Decoded []DecodedURL `json:"decoded,omitempty"`
	Flags   []Flag       `json:"flags,omitempty"`
}

// DecodedURL holds structured data extracted from a URL by a decoder.
type DecodedURL struct {
	Decoder string            `json:"decoder"`
	Kind    string            `json:"kind"`
	Data    map[string]string `json:"data"`
}

// Flag represents a content flag raised during analysis.
type Flag struct {
	Category string `json:"category"`
	Keyword  string `json:"keyword"`
	Source   string `json:"source"` // "url", "title", or "decoded"
}

// DBReport is the analysis result for a single browser history database.
type DBReport struct {
	Browser string    `json:"browser"`
	DBPath  string    `json:"db_path"`
	Cutoff  time.Time `json:"cutoff"`
	Visits  []Visit   `json:"visits"`
	Summary DBSummary `json:"summary"`
	Error   string    `json:"error,omitempty"`
}

// DBSummary provides aggregate statistics for a database.
type DBSummary struct {
	TotalVisits    int            `json:"total_visits"`
	FlaggedVisits  int            `json:"flagged_visits"`
	TopDomains     []DomainCount  `json:"top_domains"`
	CategoryCounts map[string]int `json:"category_counts,omitempty"`
}

// DomainCount pairs a domain with its visit count.
type DomainCount struct {
	Domain string `json:"domain"`
	Count  int    `json:"count"`
}

// RunReport is the top-level output of a full scan.
type RunReport struct {
	StartedAt time.Time  `json:"started_at"`
	Cutoff    time.Time  `json:"cutoff"`
	DBReports []DBReport `json:"db_reports"`
}

// Config holds runtime configuration parsed from CLI flags.
type Config struct {
	Lookback    time.Duration
	Format      string   // "json" or "text"
	DBOverrides []string // explicit DB paths to scan instead of auto-discovery
}
