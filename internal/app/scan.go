package app

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/watsoncj/osprey/internal/finder"
	"github.com/watsoncj/osprey/internal/model"
	"github.com/watsoncj/osprey/internal/sqliteio"
)

// ScanRaw performs the scanning pipeline but returns raw visits without
// decoding or flagging. The server handles enrichment.
func ScanRaw(ctx context.Context, cfg Config, allBrowsers []Browser) model.Submission {
	now := time.Now()
	cutoff := now.Add(-cfg.Lookback)

	sub := model.Submission{
		ScannedAt: now,
	}

	type dbEntry struct {
		browser Browser
		path    string
		user    string
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
				user := filepath.Base(home)
				for _, pattern := range b.DBPaths(home) {
					for _, p := range finder.ExpandPath(pattern) {
						dbs = append(dbs, dbEntry{browser: b, path: p, user: user})
					}
				}
			}
		}
	}

	log.Printf("Found %d history database(s)", len(dbs))

	for _, entry := range dbs {
		dbCutoff := cutoff
		if t, ok := cfg.DBCutoffs[entry.path]; ok && t.After(dbCutoff) {
			dbCutoff = t
		}
		visits, indicators := scanDB(ctx, entry.browser, entry.path, entry.user, dbCutoff)
		sub.Visits = append(sub.Visits, visits...)
		sub.IncognitoIndicators = append(sub.IncognitoIndicators, indicators...)
	}

	return sub
}

func scanDB(ctx context.Context, b Browser, path, user string, cutoff time.Time) ([]model.RawVisit, []model.RawIncognitoIndicator) {
	log.Printf("Scanning %s: %s", b.Name(), path)

	db, err := sqliteio.OpenReadonly(ctx, path)
	if err != nil {
		log.Printf("Failed to open %s: %v", path, err)
		return nil, nil
	}
	defer db.Close()

	visits, err := b.Query(ctx, db.DB, cutoff)
	if err != nil {
		log.Printf("Query failed for %s: %v", path, err)
		return nil, nil
	}

	log.Printf("Found %d visit(s) in %s", len(visits), path)

	raw := make([]model.RawVisit, len(visits))
	for i, v := range visits {
		raw[i] = model.RawVisit{
			Time:    v.Time,
			URL:     v.URL,
			Title:   v.Title,
			Browser: b.Name(),
			DBPath:  path,
			User:    user,
		}
	}

	indicators := scanIncognito(ctx, path, b.Name(), user)
	return raw, indicators
}

// scanIncognito queries the Favicons DB for page_urls without decoding.
func scanIncognito(ctx context.Context, historyPath string, browserName string, user string) []model.RawIncognitoIndicator {
	if !chromiumBrowsers[browserName] {
		return nil
	}

	faviconPath := filepath.Join(filepath.Dir(historyPath), "Favicons")
	if _, err := os.Stat(faviconPath); err != nil {
		return nil
	}

	fdb, err := sqliteio.OpenReadonly(ctx, faviconPath)
	if err != nil {
		log.Printf("Incognito detection: failed to open %s: %v", faviconPath, err)
		return nil
	}
	defer fdb.Close()

	const faviconQuery = `SELECT DISTINCT page_url FROM icon_mapping`
	rows, err := fdb.QueryContext(ctx, faviconQuery)
	if err != nil {
		log.Printf("Incognito detection: favicon query failed: %v", err)
		return nil
	}
	defer rows.Close()

	var indicators []model.RawIncognitoIndicator
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			continue
		}
		indicators = append(indicators, model.RawIncognitoIndicator{
			URL:     u,
			Browser: browserName,
			DBPath:  faviconPath,
			User:    user,
		})
	}
	if err := rows.Err(); err != nil {
		return nil
	}

	if len(indicators) > 0 {
		log.Printf("Favicon URLs: found %d URL(s) in %s", len(indicators), faviconPath)
	}
	return indicators
}
