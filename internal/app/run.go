package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"sort"
	"time"

	"github.com/browser-forensics/browser-forensics/internal/decoder"
	"github.com/browser-forensics/browser-forensics/internal/finder"
	"github.com/browser-forensics/browser-forensics/internal/flagging"
	"github.com/browser-forensics/browser-forensics/internal/model"
	"github.com/browser-forensics/browser-forensics/internal/sqliteio"
)

// Browser defines what Run needs from each browser adapter.
// This avoids importing the browsers package from app.
type Browser interface {
	Name() string
	DBPaths(userHome string) []string
	Query(ctx context.Context, db *sql.DB, cutoff time.Time) ([]model.Visit, error)
}

// Run executes the full forensics pipeline and returns a RunReport.
func Run(ctx context.Context, cfg Config, allBrowsers []Browser) model.RunReport {
	now := time.Now()
	cutoff := now.Add(-cfg.Lookback)

	rr := model.RunReport{
		StartedAt: now,
		Cutoff:    cutoff,
	}

	decoders := decoder.DefaultRegistry()
	flagger := flagging.DefaultFlagger()

	type dbEntry struct {
		browser Browser
		path    string
	}
	var dbs []dbEntry

	if len(cfg.DBOverrides) > 0 {
		defaultBrowser := allBrowsers[0]
		for _, p := range cfg.DBOverrides {
			for _, expanded := range finder.ExpandPath(p) {
				dbs = append(dbs, dbEntry{browser: defaultBrowser, path: expanded})
			}
		}
	} else {
		userDirs := finder.UserDirs()
		for _, b := range allBrowsers {
			for _, home := range userDirs {
				for _, pattern := range b.DBPaths(home) {
					for _, p := range finder.ExpandPath(pattern) {
						dbs = append(dbs, dbEntry{browser: b, path: p})
					}
				}
			}
		}
	}

	log.Printf("Found %d history database(s)", len(dbs))

	for _, entry := range dbs {
		dbr := processDB(ctx, entry.browser, entry.path, cutoff, decoders, flagger)
		rr.DBReports = append(rr.DBReports, dbr)
	}

	return rr
}

func processDB(ctx context.Context, b Browser, path string, cutoff time.Time, dec *decoder.Registry, flagger *flagging.Flagger) model.DBReport {
	log.Printf("Processing %s: %s", b.Name(), path)

	dbr := model.DBReport{
		Browser: b.Name(),
		DBPath:  path,
		Cutoff:  cutoff,
	}

	db, err := sqliteio.OpenReadonly(ctx, path)
	if err != nil {
		dbr.Error = fmt.Sprintf("failed to open: %v", err)
		return dbr
	}
	defer db.Close()

	visits, err := b.Query(ctx, db, cutoff)
	if err != nil {
		dbr.Error = fmt.Sprintf("query failed: %v", err)
		return dbr
	}

	for i := range visits {
		visits[i].DBPath = path
		visits[i].Decoded = dec.DecodeAll(visits[i].URL)
		visits[i].Flags = flagger.FlagVisit(&visits[i])
	}

	dbr.Visits = visits
	dbr.Summary = buildSummary(visits)
	return dbr
}

func buildSummary(visits []model.Visit) model.DBSummary {
	s := model.DBSummary{
		TotalVisits:    len(visits),
		CategoryCounts: make(map[string]int),
	}

	domainCounts := make(map[string]int)
	for _, v := range visits {
		if u, err := url.Parse(v.URL); err == nil {
			domainCounts[u.Hostname()]++
		}
		if len(v.Flags) > 0 {
			s.FlaggedVisits++
			for _, f := range v.Flags {
				s.CategoryCounts[f.Category]++
			}
		}
	}

	type dc struct {
		domain string
		count  int
	}
	var sorted []dc
	for d, c := range domainCounts {
		sorted = append(sorted, dc{d, c})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	limit := 10
	if len(sorted) < limit {
		limit = len(sorted)
	}
	for _, d := range sorted[:limit] {
		s.TopDomains = append(s.TopDomains, model.DomainCount{Domain: d.domain, Count: d.count})
	}

	return s
}
