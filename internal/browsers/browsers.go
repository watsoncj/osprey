package browsers

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"time"

	"github.com/browser-forensics/browser-forensics/internal/model"
)

// Browser defines the interface each browser adapter must implement.
type Browser interface {
	Name() string
	DBPaths(userHome string) []string
	Query(ctx context.Context, db *sql.DB, cutoff time.Time) ([]model.Visit, error)
}

// All returns every registered browser adapter.
func All() []Browser {
	return []Browser{
		&Chrome{},
		&Edge{},
		&Brave{},
		&Firefox{},
		&Safari{},
	}
}

// --- Chromium shared query helper ---

var chromiumEpoch = time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)

func chromiumQuery(ctx context.Context, db *sql.DB, cutoff time.Time, browserName string) ([]model.Visit, error) {
	chromiumCutoff := cutoff.Sub(chromiumEpoch).Microseconds()

	const q = `
		SELECT u.url, COALESCE(u.title, ''), v.visit_time
		FROM visits v
		JOIN urls u ON v.url = u.id
		WHERE v.visit_time >= ?
		ORDER BY v.visit_time DESC
	`

	rows, err := db.QueryContext(ctx, q, chromiumCutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var visits []model.Visit
	for rows.Next() {
		var urlStr, title string
		var visitTime int64
		if err := rows.Scan(&urlStr, &title, &visitTime); err != nil {
			return nil, err
		}
		t := chromiumEpoch.Add(time.Duration(visitTime) * time.Microsecond)
		visits = append(visits, model.Visit{
			Time:    t,
			URL:     urlStr,
			Title:   title,
			Browser: browserName,
		})
	}
	return visits, rows.Err()
}

// --- Chrome ---

type Chrome struct{}

func (c *Chrome) Name() string { return "Chrome" }

func (c *Chrome) DBPaths(userHome string) []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			filepath.Join(userHome, `AppData\Local\Google\Chrome\User Data\Default\History`),
			filepath.Join(userHome, `AppData\Local\Google\Chrome\User Data\Profile *\History`),
		}
	case "darwin":
		return []string{
			filepath.Join(userHome, "Library/Application Support/Google/Chrome/Default/History"),
			filepath.Join(userHome, "Library/Application Support/Google/Chrome/Profile */History"),
		}
	default:
		return []string{
			filepath.Join(userHome, ".config/google-chrome/Default/History"),
			filepath.Join(userHome, ".config/google-chrome/Profile */History"),
		}
	}
}

func (c *Chrome) Query(ctx context.Context, db *sql.DB, cutoff time.Time) ([]model.Visit, error) {
	return chromiumQuery(ctx, db, cutoff, c.Name())
}

// --- Edge ---

type Edge struct{}

func (e *Edge) Name() string { return "Edge" }

func (e *Edge) DBPaths(userHome string) []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			filepath.Join(userHome, `AppData\Local\Microsoft\Edge\User Data\Default\History`),
			filepath.Join(userHome, `AppData\Local\Microsoft\Edge\User Data\Profile *\History`),
		}
	case "darwin":
		return []string{
			filepath.Join(userHome, "Library/Application Support/Microsoft Edge/Default/History"),
			filepath.Join(userHome, "Library/Application Support/Microsoft Edge/Profile */History"),
		}
	default:
		return nil
	}
}

func (e *Edge) Query(ctx context.Context, db *sql.DB, cutoff time.Time) ([]model.Visit, error) {
	return chromiumQuery(ctx, db, cutoff, e.Name())
}

// --- Brave ---

type Brave struct{}

func (b *Brave) Name() string { return "Brave" }

func (b *Brave) DBPaths(userHome string) []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			filepath.Join(userHome, `AppData\Local\BraveSoftware\Brave-Browser\User Data\Default\History`),
			filepath.Join(userHome, `AppData\Local\BraveSoftware\Brave-Browser\User Data\Profile *\History`),
		}
	case "darwin":
		return []string{
			filepath.Join(userHome, "Library/Application Support/BraveSoftware/Brave-Browser/Default/History"),
			filepath.Join(userHome, "Library/Application Support/BraveSoftware/Brave-Browser/Profile */History"),
		}
	default:
		return nil
	}
}

func (b *Brave) Query(ctx context.Context, db *sql.DB, cutoff time.Time) ([]model.Visit, error) {
	return chromiumQuery(ctx, db, cutoff, b.Name())
}

// --- Firefox ---

type Firefox struct{}

func (f *Firefox) Name() string { return "Firefox" }

func (f *Firefox) DBPaths(userHome string) []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			filepath.Join(userHome, `AppData\Roaming\Mozilla\Firefox\Profiles\*\places.sqlite`),
		}
	case "darwin":
		return []string{
			filepath.Join(userHome, "Library/Application Support/Firefox/Profiles/*/places.sqlite"),
		}
	default:
		return []string{
			filepath.Join(userHome, ".mozilla/firefox/*/places.sqlite"),
		}
	}
}

func (f *Firefox) Query(ctx context.Context, db *sql.DB, cutoff time.Time) ([]model.Visit, error) {
	firefoxCutoff := cutoff.UnixMicro()

	const query = `
		SELECT p.url, COALESCE(p.title, ''), h.visit_date
		FROM moz_historyvisits h
		JOIN moz_places p ON h.place_id = p.id
		WHERE h.visit_date >= ?
		ORDER BY h.visit_date DESC
	`

	rows, err := db.QueryContext(ctx, query, firefoxCutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var visits []model.Visit
	for rows.Next() {
		var urlStr, title string
		var visitDate int64
		if err := rows.Scan(&urlStr, &title, &visitDate); err != nil {
			return nil, err
		}
		t := time.UnixMicro(visitDate)
		visits = append(visits, model.Visit{
			Time:    t,
			URL:     urlStr,
			Title:   title,
			Browser: f.Name(),
		})
	}
	return visits, rows.Err()
}

// --- Safari ---

type Safari struct{}

func (s *Safari) Name() string { return "Safari" }

func (s *Safari) DBPaths(userHome string) []string {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return []string{
		filepath.Join(userHome, "Library/Safari/History.db"),
	}
}

func (s *Safari) Query(ctx context.Context, db *sql.DB, cutoff time.Time) ([]model.Visit, error) {
	safariEpoch := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	safariCutoff := cutoff.Sub(safariEpoch).Seconds()

	const query = `
		SELECT i.url, COALESCE(v.title, ''), v.visit_time
		FROM history_visits v
		JOIN history_items i ON v.history_item = i.id
		WHERE v.visit_time >= ?
		ORDER BY v.visit_time DESC
	`

	rows, err := db.QueryContext(ctx, query, safariCutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var visits []model.Visit
	for rows.Next() {
		var urlStr, title string
		var visitTime float64
		if err := rows.Scan(&urlStr, &title, &visitTime); err != nil {
			return nil, err
		}
		t := safariEpoch.Add(time.Duration(visitTime * float64(time.Second)))
		visits = append(visits, model.Visit{
			Time:    t,
			URL:     urlStr,
			Title:   title,
			Browser: s.Name(),
		})
	}
	return visits, rows.Err()
}
