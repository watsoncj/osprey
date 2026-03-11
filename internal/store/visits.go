package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/watsoncj/osprey/internal/model"
)

// VisitQuery holds optional filters for loading visits.
type VisitQuery struct {
	Since         time.Time
	Until         time.Time
	FlaggedOnly   bool
	ShowDismissed bool
	Browser       string
	User          string
	Limit         int
	Offset        int
}

// HostStats holds aggregate statistics for a hostname.
type HostStats struct {
	Hostname              string              `json:"hostname"`
	AgentVersion          string              `json:"agent_version,omitempty"`
	IPAddress             string              `json:"ip_address,omitempty"`
	TotalVisits           int                 `json:"total_visits"`
	FlaggedVisits         int                 `json:"flagged_visits"`
	DismissedFlaggedVisits int               `json:"dismissed_flagged_visits,omitempty"`
	LatestVisit           time.Time           `json:"latest_visit"`
	CategoryCounts        map[string]int      `json:"category_counts,omitempty"`
	TopDomains            []model.DomainCount `json:"top_domains,omitempty"`
	Users                 []string            `json:"users,omitempty"`
}

// HostMeta holds per-host metadata persisted to meta.json.
type HostMeta struct {
	AgentVersion string    `json:"agent_version"`
	IPAddress    string    `json:"ip_address,omitempty"`
	LastSeen     time.Time `json:"last_seen"`
}

// VisitKey computes the deduplication key for a visit.
func VisitKey(url string, t time.Time, browser string) string {
	return fmt.Sprintf("%s|%d|%s", url, t.UnixNano(), browser)
}

// SaveHostMeta writes agent metadata for a hostname.
func (s *Store) SaveHostMeta(hostname, agentVersion, ipAddress string) error {
	dir := filepath.Join(s.Dir, hostname)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	meta := HostMeta{
		AgentVersion: agentVersion,
		IPAddress:    ipAddress,
		LastSeen:     time.Now().UTC(),
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "meta.json"), data, 0o644)
}

// LoadHostMeta reads agent metadata for a hostname.
func (s *Store) LoadHostMeta(hostname string) (HostMeta, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, hostname, "meta.json"))
	if err != nil {
		return HostMeta{}, err
	}
	var meta HostMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return HostMeta{}, err
	}
	return meta, nil
}

// AppendVisits appends non-duplicate visits to the hostname's visits.jsonl file.
// It returns the count of new visits inserted and any error.
func (s *Store) AppendVisits(hostname string, visits []model.Visit) (int, error) {
	dir := filepath.Join(s.Dir, hostname)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, fmt.Errorf("create dir: %w", err)
	}

	path := filepath.Join(dir, "visits.jsonl")

	existing, err := loadVisitKeys(path)
	if err != nil {
		return 0, err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open visits file: %w", err)
	}
	defer f.Close()

	count := 0
	for _, v := range visits {
		key := VisitKey(v.URL, v.Time, v.Browser)
		if existing[key] {
			continue
		}
		data, err := json.Marshal(v)
		if err != nil {
			return count, fmt.Errorf("marshal visit: %w", err)
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return count, fmt.Errorf("write visit: %w", err)
		}
		existing[key] = true
		count++
	}

	return count, nil
}

// AppendIncognito appends non-duplicate incognito indicators to the hostname's incognito.jsonl file.
// It returns the count of new indicators inserted and any error.
func (s *Store) AppendIncognito(hostname string, indicators []model.IncognitoIndicator) (int, error) {
	dir := filepath.Join(s.Dir, hostname)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, fmt.Errorf("create dir: %w", err)
	}

	path := filepath.Join(dir, "incognito.jsonl")

	existing, err := loadIncognitoKeys(path)
	if err != nil {
		return 0, err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open incognito file: %w", err)
	}
	defer f.Close()

	count := 0
	for _, ind := range indicators {
		key := fmt.Sprintf("%s|%s", ind.URL, ind.Browser)
		if existing[key] {
			continue
		}
		data, err := json.Marshal(ind)
		if err != nil {
			return count, fmt.Errorf("marshal incognito indicator: %w", err)
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return count, fmt.Errorf("write incognito indicator: %w", err)
		}
		existing[key] = true
		count++
	}

	return count, nil
}

// ListHosts returns a sorted list of hostnames that have visit data.
func (s *Store) ListHosts() ([]string, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var hosts []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(s.Dir, e.Name(), "visits.jsonl")); err == nil {
			hosts = append(hosts, e.Name())
		}
	}
	sort.Strings(hosts)
	return hosts, nil
}

// LoadDismissals reads the dismissal state for a hostname.
// Returns a map of visit key → dismissed (last event wins).
func (s *Store) LoadDismissals(hostname string) (map[string]bool, error) {
	path := filepath.Join(s.Dir, hostname, "dismissals.jsonl")
	events, err := readJSONL[model.DismissalEvent](path)
	if err != nil {
		return nil, err
	}
	state := make(map[string]bool, len(events))
	for _, e := range events {
		if e.Dismissed {
			state[e.VisitKey] = true
		} else {
			delete(state, e.VisitKey)
		}
	}
	return state, nil
}

// SetVisitDismissed appends a dismissal event for a visit.
// No-op if the current state already matches.
func (s *Store) SetVisitDismissed(hostname, visitKey string, dismissed bool) error {
	dir := filepath.Join(s.Dir, hostname)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	current, err := s.LoadDismissals(hostname)
	if err != nil {
		return err
	}
	if current[visitKey] == dismissed {
		return nil
	}

	event := model.DismissalEvent{
		VisitKey:  visitKey,
		Dismissed: dismissed,
		UpdatedAt: time.Now().UTC(),
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal dismissal: %w", err)
	}

	path := filepath.Join(dir, "dismissals.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open dismissals file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write dismissal: %w", err)
	}
	return nil
}

// LoadVisits reads visits for a hostname, applies filters from VisitQuery,
// sorts newest first, and applies offset/limit.
func (s *Store) LoadVisits(hostname string, q VisitQuery) ([]model.Visit, error) {
	path := filepath.Join(s.Dir, hostname, "visits.jsonl")

	visits, err := readJSONL[model.Visit](path)
	if err != nil {
		return nil, err
	}

	dismissed, err := s.LoadDismissals(hostname)
	if err != nil {
		return nil, err
	}

	var filtered []model.Visit
	for _, v := range visits {
		key := VisitKey(v.URL, v.Time, v.Browser)
		if dismissed[key] {
			v.Dismissed = true
		}
		if !q.Since.IsZero() && v.Time.Before(q.Since) {
			continue
		}
		if !q.Until.IsZero() && v.Time.After(q.Until) {
			continue
		}
		if q.FlaggedOnly && len(v.Flags) == 0 {
			continue
		}
		if q.FlaggedOnly && v.Dismissed && !q.ShowDismissed {
			continue
		}
		if q.Browser != "" && v.Browser != q.Browser {
			continue
		}
		if q.User != "" && v.User != q.User {
			continue
		}
		filtered = append(filtered, v)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Time.After(filtered[j].Time)
	})

	if q.Offset > 0 {
		if q.Offset >= len(filtered) {
			return nil, nil
		}
		filtered = filtered[q.Offset:]
	}
	if q.Limit > 0 && q.Limit < len(filtered) {
		filtered = filtered[:q.Limit]
	}

	return filtered, nil
}

// LoadIncognito reads all incognito indicators for a hostname.
func (s *Store) LoadIncognito(hostname string) ([]model.IncognitoIndicator, error) {
	path := filepath.Join(s.Dir, hostname, "incognito.jsonl")
	return readJSONL[model.IncognitoIndicator](path)
}

// HostStats computes aggregate statistics for a hostname's visits.
func (s *Store) HostStats(hostname string) (HostStats, error) {
	stats := HostStats{Hostname: hostname}

	if meta, err := s.LoadHostMeta(hostname); err == nil {
		stats.AgentVersion = meta.AgentVersion
		stats.IPAddress = meta.IPAddress
	}

	visits, err := s.LoadVisits(hostname, VisitQuery{ShowDismissed: true})
	if err != nil {
		return stats, err
	}
	if len(visits) == 0 {
		return stats, nil
	}

	stats.TotalVisits = len(visits)
	stats.CategoryCounts = make(map[string]int)
	domainCounts := make(map[string]int)
	userSet := make(map[string]bool)

	for _, v := range visits {
		if len(v.Flags) > 0 {
			if v.Dismissed {
				stats.DismissedFlaggedVisits++
			} else {
				stats.FlaggedVisits++
			}
			for _, f := range v.Flags {
				stats.CategoryCounts[f.Category]++
			}
		}
		if stats.LatestVisit.IsZero() || v.Time.After(stats.LatestVisit) {
			stats.LatestVisit = v.Time
		}
		if u, err := url.Parse(v.URL); err == nil && u.Hostname() != "" {
			domainCounts[u.Hostname()]++
		}
		if v.User != "" {
			userSet[v.User] = true
		}
	}

	for u := range userSet {
		stats.Users = append(stats.Users, u)
	}
	sort.Strings(stats.Users)

	type dc struct {
		domain string
		count  int
	}
	var dcs []dc
	for d, c := range domainCounts {
		dcs = append(dcs, dc{d, c})
	}
	sort.Slice(dcs, func(i, j int) bool {
		return dcs[i].count > dcs[j].count
	})
	limit := 10
	if len(dcs) < limit {
		limit = len(dcs)
	}
	for _, d := range dcs[:limit] {
		stats.TopDomains = append(stats.TopDomains, model.DomainCount{
			Domain: d.domain,
			Count:  d.count,
		})
	}

	if len(stats.CategoryCounts) == 0 {
		stats.CategoryCounts = nil
	}

	return stats, nil
}

// readJSONL reads a JSONL file and returns a slice of T.
// If the file doesn't exist, it returns an empty slice.
func readJSONL[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var results []T
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var item T
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return results, fmt.Errorf("unmarshal line: %w", err)
		}
		results = append(results, item)
	}
	if err := scanner.Err(); err != nil {
		return results, fmt.Errorf("scan %s: %w", path, err)
	}

	return results, nil
}

// loadVisitKeys reads existing visit dedup keys from a JSONL file.
func loadVisitKeys(path string) (map[string]bool, error) {
	visits, err := readJSONL[model.Visit](path)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]bool, len(visits))
	for _, v := range visits {
		key := VisitKey(v.URL, v.Time, v.Browser)
		keys[key] = true
	}
	return keys, nil
}

// loadIncognitoKeys reads existing incognito dedup keys from a JSONL file.
func loadIncognitoKeys(path string) (map[string]bool, error) {
	indicators, err := readJSONL[model.IncognitoIndicator](path)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]bool, len(indicators))
	for _, ind := range indicators {
		key := fmt.Sprintf("%s|%s", ind.URL, ind.Browser)
		keys[key] = true
	}
	return keys, nil
}
