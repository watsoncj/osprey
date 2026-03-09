package web

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/watsoncj/osprey/internal/model"
	"github.com/watsoncj/osprey/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

var funcMap = template.FuncMap{
	"fmtTime": func(t time.Time) string {
		return t.Format("2006-01-02 15:04:05")
	},
	"fmtDate": func(t time.Time) string {
		return t.Format("2006-01-02")
	},
	"truncate": func(s string, n int) string {
		if len(s) <= n {
			return s
		}
		return s[:n] + "..."
	},
	"hasFlags": func(v model.Visit) bool {
		return len(v.Flags) > 0
	},
	"isVideo": func(d model.DecodedURL) bool {
		return d.Decoder == "youtube" && d.Kind == "video"
	},
	"topCategory": func(cats map[string]int) string {
		if len(cats) == 0 {
			return ""
		}
		best := ""
		bestCount := 0
		for cat, count := range cats {
			if count > bestCount {
				best = cat
				bestCount = count
			}
		}
		return best
	},
	"add":            func(a, b int) int { return a + b },
	"uniqueCategories": func(flags []model.Flag) []string {
		seen := make(map[string]bool)
		var cats []string
		for _, f := range flags {
			if !seen[f.Category] {
				seen[f.Category] = true
				cats = append(cats, f.Category)
			}
		}
		return cats
	},
}

var (
	dashboardTmpl *template.Template
	hostTmpl      *template.Template
	flaggedTmpl   *template.Template
	incognitoTmpl *template.Template
)

func init() {
	dashboardTmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/dashboard.html"))
	hostTmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/host.html"))
	flaggedTmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/flagged.html"))
	incognitoTmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/incognito.html"))
}

type dashboardData struct {
	Hosts []store.HostStats
}

type hostDetailData struct {
	Hostname  string
	Visits    []model.Visit
	Incognito []model.IncognitoIndicator
	Stats     store.HostStats
	Query     queryParams
}

type queryParams struct {
	FlaggedOnly bool
	Browser     string
	User        string
	Since       string
	Until       string
	Page        int
	PageSize    int
	HasMore     bool
}

type flaggedData struct {
	Visits []visitWithHost
}

type visitWithHost struct {
	model.Visit
	Hostname string
}

type incognitoData struct {
	Hostname   string
	Indicators []model.IncognitoIndicator
}

// Handler returns an http.Handler with all web UI routes.
func Handler(s *store.Store) http.Handler {
	mux := http.NewServeMux()

	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticSub)))

	mux.HandleFunc("GET /", handleDashboard(s))
	mux.HandleFunc("GET /hosts/{hostname}", handleHost(s))
	mux.HandleFunc("GET /flagged", handleFlagged(s))
	mux.HandleFunc("GET /hosts/{hostname}/incognito", handleIncognito(s))

	return mux
}

func handleDashboard(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hosts, err := s.ListHosts()
		if err != nil {
			log.Printf("error listing hosts: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		var stats []store.HostStats
		for _, hostname := range hosts {
			hs, err := s.HostStats(hostname)
			if err != nil {
				log.Printf("error loading stats for %s: %v", hostname, err)
				continue
			}
			stats = append(stats, hs)
		}

		sort.Slice(stats, func(i, j int) bool {
			return stats[i].LatestVisit.After(stats[j].LatestVisit)
		})

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := dashboardTmpl.ExecuteTemplate(w, "layout", dashboardData{Hosts: stats}); err != nil {
			log.Printf("error executing dashboard template: %v", err)
		}
	}
}

func handleHost(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hostname := r.PathValue("hostname")

		q := r.URL.Query()
		flaggedOnly := q.Get("flagged") == "true"
		browser := q.Get("browser")
		userFilter := q.Get("user")
		since := q.Get("since")
		until := q.Get("until")
		page, _ := strconv.Atoi(q.Get("page"))
		if page < 1 {
			page = 1
		}
		pageSize, _ := strconv.Atoi(q.Get("page_size"))
		if pageSize < 1 {
			pageSize = 50
		}

		vq := store.VisitQuery{
			FlaggedOnly: flaggedOnly,
			Browser:     browser,
			User:        userFilter,
			Limit:       pageSize + 1,
			Offset:      (page - 1) * pageSize,
		}

		if since != "" {
			if t, err := time.Parse(time.RFC3339, since); err == nil {
				vq.Since = t
			} else if t, err := time.Parse("2006-01-02", since); err == nil {
				vq.Since = t
			}
		}
		if until != "" {
			if t, err := time.Parse(time.RFC3339, until); err == nil {
				vq.Until = t
			} else if t, err := time.Parse("2006-01-02", until); err == nil {
				vq.Until = t.Add(24*time.Hour - time.Nanosecond)
			}
		}

		visits, err := s.LoadVisits(hostname, vq)
		if err != nil {
			log.Printf("error loading visits for %s: %v", hostname, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		hasMore := len(visits) > pageSize
		if hasMore {
			visits = visits[:pageSize]
		}

		stats, err := s.HostStats(hostname)
		if err != nil {
			log.Printf("error loading stats for %s: %v", hostname, err)
		}

		incognito, err := s.LoadIncognito(hostname)
		if err != nil {
			log.Printf("error loading incognito for %s: %v", hostname, err)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := hostTmpl.ExecuteTemplate(w, "layout", hostDetailData{
			Hostname:  hostname,
			Visits:    visits,
			Incognito: incognito,
			Stats:     stats,
			Query: queryParams{
				FlaggedOnly: flaggedOnly,
				Browser:     browser,
				User:        userFilter,
				Since:       since,
				Until:       until,
				Page:        page,
				PageSize:    pageSize,
				HasMore:     hasMore,
			},
		}); err != nil {
			log.Printf("error executing host template: %v", err)
		}
	}
}

func handleFlagged(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hosts, err := s.ListHosts()
		if err != nil {
			log.Printf("error listing hosts: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		var all []visitWithHost
		for _, hostname := range hosts {
			visits, err := s.LoadVisits(hostname, store.VisitQuery{FlaggedOnly: true, Limit: 100})
			if err != nil {
				log.Printf("error loading flagged visits for %s: %v", hostname, err)
				continue
			}
			for _, v := range visits {
				all = append(all, visitWithHost{Visit: v, Hostname: hostname})
			}
		}

		sort.Slice(all, func(i, j int) bool {
			return all[i].Time.After(all[j].Time)
		})

		if len(all) > 200 {
			all = all[:200]
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := flaggedTmpl.ExecuteTemplate(w, "layout", flaggedData{Visits: all}); err != nil {
			log.Printf("error executing flagged template: %v", err)
		}
	}
}

func handleIncognito(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hostname := r.PathValue("hostname")

		indicators, err := s.LoadIncognito(hostname)
		if err != nil {
			log.Printf("error loading incognito for %s: %v", hostname, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := incognitoTmpl.ExecuteTemplate(w, "layout", incognitoData{
			Hostname:   hostname,
			Indicators: indicators,
		}); err != nil {
			log.Printf("error executing incognito template: %v", err)
		}
	}
}
