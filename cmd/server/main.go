package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/watsoncj/osprey/internal/buildinfo"
	"github.com/watsoncj/osprey/internal/ingest"
	"github.com/watsoncj/osprey/internal/model"
	"github.com/watsoncj/osprey/internal/selfupdate"
	"github.com/watsoncj/osprey/internal/store"
	"github.com/watsoncj/osprey/internal/web"
)

func main() {
	listen := flag.String("listen", ":9753", "Address to listen on")
	dataDir := flag.String("data", "./data", "Directory to store reports")
	certFile := flag.String("cert", "", "Path to TLS certificate file (enables HTTPS)")
	keyFile := flag.String("key", "", "Path to TLS private key file")
	apiKey := flag.String("api-key", "", "API key required for report uploads (optional)")
	noUpdate := flag.Bool("no-update", false, "Disable automatic self-update")
	version := flag.Bool("version", false, "Print version and exit")
	selfUpdate := flag.Bool("self-update", false, "Check for update and exit")
	install := flag.Bool("install", false, "Install server as a system service")
	uninstall := flag.Bool("uninstall", false, "Uninstall server system service")
	flag.Parse()

	if *version {
		fmt.Println(buildinfo.Version)
		return
	}

	if *selfUpdate {
		newVer, err := selfupdate.CheckAndApply(context.Background(), buildinfo.Version, selfupdate.Server, nil)
		if err != nil {
			log.Fatalf("self-update: %v", err)
		}
		if newVer == "" {
			fmt.Println("Already up to date.")
		} else {
			fmt.Printf("Updated to %s. Restart the server to use the new version.\n", newVer)
		}
		return
	}

	if *install {
		if err := installService(*listen, *dataDir, *apiKey); err != nil {
			log.Fatalf("install service: %v", err)
		}
		fmt.Println("Service installed successfully.")
		return
	}

	if *uninstall {
		if err := uninstallService(); err != nil {
			log.Fatalf("uninstall service: %v", err)
		}
		fmt.Println("Service uninstalled successfully.")
		return
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	selfupdate.Cleanup()

	if !*noUpdate {
		go func() {
			// Check for updates every 12 hours.
			ticker := time.NewTicker(12 * time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				newVer, err := selfupdate.CheckAndApply(ctx, buildinfo.Version, selfupdate.Server, nil)
				cancel()
				if err != nil {
					log.Printf("Self-update check failed: %v", err)
				} else if newVer != "" {
					log.Printf("Updated to %s — exiting for service restart", newVer)
					os.Exit(0)
				}
			}
		}()
	}

	s := &store.Store{Dir: *dataDir}
	pipeline := ingest.New()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/visits", handlePostVisits(s, pipeline, *apiKey))
	mux.HandleFunc("GET /api/visits", handleListVisits(s))
	mux.HandleFunc("GET /api/hosts", handleListHosts(s))
	mux.HandleFunc("POST /api/hosts/{hostname}/dismissals", handleDismiss(s))
	mux.HandleFunc("POST /api/reports", handlePost(s, pipeline, *apiKey))
	mux.HandleFunc("GET /api/reports", handleList(s))
	mux.HandleFunc("GET /api/reports/{rest...}", handleGet(s))
	mux.Handle("/", web.Handler(s))

	log.Printf("Listening on %s (data: %s) version=%s", *listen, *dataDir, buildinfo.Version)
	if *certFile != "" && *keyFile != "" {
		log.Printf("TLS enabled")
		if err := http.ListenAndServeTLS(*listen, *certFile, *keyFile, mux); err != nil {
			log.Fatalf("server: %v", err)
		}
	} else {
		if err := http.ListenAndServe(*listen, mux); err != nil {
			log.Fatalf("server: %v", err)
		}
	}
}

func handlePostVisits(s *store.Store, p *ingest.Pipeline, apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apiKey != "" {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer "+apiKey {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}

		var sub model.Submission
		if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		hostname := r.Header.Get("X-Hostname")
		if hostname == "" {
			hostname = sub.Hostname
		}
		if hostname == "" {
			http.Error(w, "missing hostname (set X-Hostname header or hostname in body)", http.StatusBadRequest)
			return
		}
		sub.Hostname = hostname

		enrichedVisits := p.ProcessVisits(sub.Visits)
		enrichedIncognito := p.ProcessIncognito(sub.IncognitoIndicators)

		newVisits, err := s.AppendVisits(hostname, enrichedVisits)
		if err != nil {
			log.Printf("append visits error: %v", err)
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}

		if len(enrichedIncognito) > 0 {
			if _, err := s.AppendIncognito(hostname, enrichedIncognito); err != nil {
				log.Printf("append incognito error: %v", err)
				http.Error(w, "storage error", http.StatusInternalServerError)
				return
			}
		}

		if sub.AgentVersion != "" {
			if err := s.SaveHostMeta(hostname, sub.AgentVersion); err != nil {
				log.Printf("save host meta error: %v", err)
			}
		}

		log.Printf("Stored %d new visits from %s (agent %s)", newVisits, hostname, sub.AgentVersion)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "new_visits": newVisits})
	}
}

func handleListVisits(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		hostname := q.Get("hostname")
		if hostname == "" {
			http.Error(w, "missing hostname query parameter", http.StatusBadRequest)
			return
		}

		vq := store.VisitQuery{}

		if v := q.Get("flagged"); v == "true" {
			vq.FlaggedOnly = true
		}
		vq.Browser = q.Get("browser")
		vq.User = q.Get("user")

		if v := q.Get("since"); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				http.Error(w, "invalid since: "+err.Error(), http.StatusBadRequest)
				return
			}
			vq.Since = t
		}
		if v := q.Get("until"); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				http.Error(w, "invalid until: "+err.Error(), http.StatusBadRequest)
				return
			}
			vq.Until = t
		}

		limit := 100
		if v := q.Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				http.Error(w, "invalid limit: "+err.Error(), http.StatusBadRequest)
				return
			}
			limit = n
		}
		vq.Limit = limit

		if v := q.Get("offset"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				http.Error(w, "invalid offset: "+err.Error(), http.StatusBadRequest)
				return
			}
			vq.Offset = n
		}

		visits, err := s.LoadVisits(hostname, vq)
		if err != nil {
			log.Printf("load visits error: %v", err)
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(visits)
	}
}

func handleListHosts(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hosts, err := s.ListHosts()
		if err != nil {
			log.Printf("list hosts error: %v", err)
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}

		var stats []store.HostStats
		for _, h := range hosts {
			hs, err := s.HostStats(h)
			if err != nil {
				log.Printf("host stats error for %s: %v", h, err)
				continue
			}
			stats = append(stats, hs)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

func handleDismiss(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hostname := r.PathValue("hostname")

		var req struct {
			URL       string    `json:"url"`
			Time      time.Time `json:"time"`
			Browser   string    `json:"browser"`
			Dismissed bool      `json:"dismissed"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		key := store.VisitKey(req.URL, req.Time, req.Browser)
		if err := s.SetVisitDismissed(hostname, key, req.Dismissed); err != nil {
			log.Printf("dismiss error: %v", err)
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handlePost(s *store.Store, p *ingest.Pipeline, apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apiKey != "" {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer "+apiKey {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}

		var rr model.RunReport
		if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		hostname := r.Header.Get("X-Hostname")
		if hostname == "" {
			hostname = rr.Hostname
		}
		if hostname == "" {
			http.Error(w, "missing hostname (set X-Hostname header or hostname in body)", http.StatusBadRequest)
			return
		}

		rr.Hostname = hostname

		var rawVisits []model.RawVisit
		var rawIncognito []model.RawIncognitoIndicator
		for _, db := range rr.DBReports {
			for _, v := range db.Visits {
				rawVisits = append(rawVisits, model.RawVisit{
					Time: v.Time, URL: v.URL, Title: v.Title, Browser: v.Browser, DBPath: v.DBPath,
				})
			}
			for _, ind := range db.IncognitoIndicators {
				rawIncognito = append(rawIncognito, model.RawIncognitoIndicator{
					URL: ind.URL, Browser: ind.Browser, DBPath: ind.DBPath,
				})
			}
		}

		enrichedVisits := p.ProcessVisits(rawVisits)
		enrichedIncognito := p.ProcessIncognito(rawIncognito)

		newVisits, err := s.AppendVisits(hostname, enrichedVisits)
		if err != nil {
			log.Printf("append visits error (legacy): %v", err)
		}
		if len(enrichedIncognito) > 0 {
			if _, err := s.AppendIncognito(hostname, enrichedIncognito); err != nil {
				log.Printf("append incognito error (legacy): %v", err)
			}
		}

		log.Printf("legacy report endpoint used: stored report from %s (%d new visits)", hostname, newVisits)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleList(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hostname := r.URL.Query().Get("hostname")

		metas, err := s.List(hostname)
		if err != nil {
			log.Printf("list error: %v", err)
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metas)
	}
}

func handleGet(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rest := r.PathValue("rest")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			http.Error(w, "use /api/reports/{hostname}/{id}", http.StatusBadRequest)
			return
		}

		hostname, id := parts[0], parts[1]
		rr, err := s.Load(hostname, id)
		if err != nil {
			http.Error(w, "report not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rr)
	}
}

const serviceName = "osprey-server"
