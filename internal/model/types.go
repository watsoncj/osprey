package model

import "time"

// Visit represents a single browser history entry.
type Visit struct {
	Time    time.Time `json:"time"`
	URL     string    `json:"url"`
	Title   string    `json:"title"`
	Browser string    `json:"browser"`
	DBPath  string    `json:"db_path"`
	User      string `json:"user,omitempty"`
	Dismissed bool   `json:"dismissed,omitempty"`

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

// IncognitoIndicator represents a URL found in the Favicons database
// but absent from History, suggesting it was visited in incognito mode.
type IncognitoIndicator struct {
	URL     string       `json:"url"`
	Browser string       `json:"browser"`
	DBPath  string       `json:"db_path"` // path to the Favicons DB
	Decoded []DecodedURL `json:"decoded,omitempty"`
}

// DBReport is the analysis result for a single browser history database.
type DBReport struct {
	Browser             string               `json:"browser"`
	DBPath              string               `json:"db_path"`
	Cutoff              time.Time            `json:"cutoff"`
	Visits              []Visit              `json:"visits"`
	Summary             DBSummary            `json:"summary"`
	IncognitoIndicators []IncognitoIndicator `json:"incognito_indicators,omitempty"`
	Error               string               `json:"error,omitempty"`
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
	Hostname  string     `json:"hostname"`
	StartedAt time.Time  `json:"started_at"`
	Cutoff    time.Time  `json:"cutoff"`
	DBReports []DBReport `json:"db_reports"`
}

// RawVisit is the minimal visit record sent by the agent.
// No decoded or flag data — the server handles enrichment.
type RawVisit struct {
	Time    time.Time `json:"time"`
	URL     string    `json:"url"`
	Title   string    `json:"title"`
	Browser string    `json:"browser"`
	DBPath  string    `json:"db_path"`
	User    string    `json:"user,omitempty"`
}

// RawIncognitoIndicator is an incognito indicator without decoded data.
type RawIncognitoIndicator struct {
	URL     string `json:"url"`
	Browser string `json:"browser"`
	DBPath  string `json:"db_path"`
}

// Submission is the payload the agent sends to the server.
// Contains raw visit data — no decoding or flagging.
type Submission struct {
	Hostname            string                  `json:"hostname"`
	AgentVersion        string                  `json:"agent_version,omitempty"`
	ScannedAt           time.Time               `json:"scanned_at"`
	Visits              []RawVisit              `json:"visits"`
	IncognitoIndicators []RawIncognitoIndicator `json:"incognito_indicators,omitempty"`
}

// DismissalEvent records a dismiss or restore action on a flagged visit.
type DismissalEvent struct {
	VisitKey  string    `json:"visit_key"`
	Dismissed bool      `json:"dismissed"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Config holds runtime configuration parsed from CLI flags.
type Config struct {
	Lookback    time.Duration
	Format      string   // "json" or "text"
	DBOverrides []string // explicit DB paths to scan instead of auto-discovery
}
