// Agent is the scanner binary that runs on the target host.
// It discovers browser history databases, analyzes them, and outputs
// a report to stdout. When deployed remotely, the controller builds
// this for Windows and embeds it.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/watsoncj/osprey/internal/app"
	"github.com/watsoncj/osprey/internal/browsers"
	"github.com/watsoncj/osprey/internal/buildinfo"
	"github.com/watsoncj/osprey/internal/model"
	"github.com/watsoncj/osprey/internal/selfupdate"
	"github.com/watsoncj/osprey/internal/spool"
	"github.com/watsoncj/osprey/internal/upload"
)

func main() {
	lookback := app.Duration{D: 24 * time.Hour}
	interval := app.Duration{D: 1 * time.Hour}

	flag.Var(&lookback, "lookback", "How far back to analyze (e.g. 24h, 5d, 2w)")
	server := flag.String("server", "", "Server URL to upload reports to")
	flag.Var(&interval, "interval", "How often to scan in daemon mode (e.g. 1h, 30m)")
	hostname := flag.String("hostname", "", "Override machine hostname for reports")
	apiKey := flag.String("api-key", "", "API key for server authentication")
	spoolDir := flag.String("spool", "./spool", "Directory to spool failed uploads for retry")
	logFile := flag.String("logfile", "", "Path to log file (default: stderr)")
	skipVerify := flag.Bool("skip-verify", false, "Skip TLS certificate verification (for self-signed certs)")
	noUpdate := flag.Bool("no-update", false, "Disable automatic self-update")
	selfUpdate := flag.Bool("self-update", false, "Check for update and exit")
	version := flag.Bool("version", false, "Print version and exit")
	install := flag.Bool("install", false, "Install agent as a system service")
	uninstall := flag.Bool("uninstall", false, "Uninstall agent system service")
	flag.Parse()

	if *version {
		fmt.Println(buildinfo.Version)
		return
	}

	if *selfUpdate {
		newVer, err := selfupdate.CheckAndApply(context.Background(), buildinfo.Version, selfupdate.Agent, nil)
		if err != nil {
			log.Fatalf("self-update: %v", err)
		}
		if newVer == "" {
			fmt.Println("Already up to date.")
		} else {
			fmt.Printf("Updated to %s. Restart the agent to use the new version.\n", newVer)
		}
		return
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		log.SetOutput(f)
	} else {
		log.SetOutput(os.Stderr)
	}

	if *hostname == "" {
		h, err := os.Hostname()
		if err != nil {
			log.Fatalf("failed to get hostname: %v", err)
		}
		*hostname = h
	}

	if *install {
		if *server == "" {
			log.Fatal("-server is required with -install")
		}
		if err := installService(*server, *hostname, interval.D, lookback.D, *apiKey, *spoolDir, *logFile, *skipVerify); err != nil {
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

	selfupdate.Cleanup()

	var httpClient *http.Client
	if *skipVerify {
		httpClient = upload.InsecureClient()
	}

	if *server == "" {
		log.Fatal("-server is required")
	}

	daemonFn := func(ctx context.Context) {
		runDaemon(ctx, *server, *hostname, lookback.D, interval.D, *apiKey, *spoolDir, httpClient, !*noUpdate)
	}
	if isWindowsService() {
		if err := runWindowsService(daemonFn); err != nil {
			log.Fatalf("service: %v", err)
		}
	} else {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		daemonFn(ctx)
	}
}

// lastScanState tracks per-DB-path latest visit times so each browser DB
// gets its own lookback window. If a DB was locked on the previous scan,
// the next scan reaches back to its last successful time.
type lastScanState struct {
	// DBs maps DB file paths to the latest visit time seen from that DB.
	DBs map[string]time.Time `json:"dbs"`
}

func loadLastScan(spoolDir string) lastScanState {
	data, err := os.ReadFile(filepath.Join(spoolDir, "last_scan.json"))
	if err != nil {
		return lastScanState{}
	}
	var state lastScanState
	if err := json.Unmarshal(data, &state); err != nil {
		return lastScanState{}
	}
	return state
}

func saveLastScan(spoolDir string, state lastScanState) {
	data, err := json.Marshal(state)
	if err != nil {
		log.Printf("Failed to marshal last_scan: %v", err)
		return
	}
	if err := os.MkdirAll(spoolDir, 0o755); err != nil {
		log.Printf("Failed to create spool dir for last_scan: %v", err)
		return
	}
	if err := os.WriteFile(filepath.Join(spoolDir, "last_scan.json"), data, 0o644); err != nil {
		log.Printf("Failed to write last_scan.json: %v", err)
	}
}

// latestVisitPerDB returns the latest visit time for each DB path.
func latestVisitPerDB(sub *model.Submission) map[string]time.Time {
	m := make(map[string]time.Time)
	for _, v := range sub.Visits {
		if v.Time.After(m[v.DBPath]) {
			m[v.DBPath] = v.Time
		}
	}
	return m
}

func runDaemon(ctx context.Context, serverURL, hostname string, lookback time.Duration, interval time.Duration, apiKey string, spoolDir string, client *http.Client, autoUpdate bool) {
	log.Printf("Starting daemon: server=%s hostname=%s interval=%s version=%s", serverURL, hostname, interval, buildinfo.Version)

	sp := &spool.Spool{Dir: spoolDir}

	for {
		// Flush spooled submissions first
		flushSpool(ctx, sp, serverURL, apiKey, client)

		scanCtx, scanCancel := context.WithTimeout(ctx, 5*time.Minute)

		lastState := loadLastScan(spoolDir)

		cfg := app.Config{
			Lookback:  lookback,
			DBCutoffs: lastState.DBs,
		}

		all := browsers.All()
		adapters := make([]app.Browser, len(all))
		for i, b := range all {
			adapters[i] = b
		}

		sub := app.ScanRaw(scanCtx, cfg, adapters)
		sub.Hostname = hostname
		sub.AgentVersion = buildinfo.Version

		if err := upload.Upload(scanCtx, serverURL, hostname, sub, apiKey, client); err != nil {
			log.Printf("Upload failed: %v", err)
			if spoolErr := sp.Save(hostname, sub); spoolErr != nil {
				log.Printf("Spool save failed: %v", spoolErr)
			} else {
				log.Printf("Submission spooled for retry")
			}
		} else {
			log.Printf("Submission uploaded successfully")
			perDB := latestVisitPerDB(&sub)
			if len(perDB) > 0 {
				if lastState.DBs == nil {
					lastState.DBs = make(map[string]time.Time)
				}
				for db, t := range perDB {
					lastState.DBs[db] = t
				}
				saveLastScan(spoolDir, lastState)
			}
		}

		scanCancel()

		if autoUpdate {
			updateCtx, updateCancel := context.WithTimeout(ctx, 2*time.Minute)
			if newVer, err := selfupdate.CheckAndApply(updateCtx, buildinfo.Version, selfupdate.Agent, client); err != nil {
				log.Printf("Self-update check failed: %v", err)
			} else if newVer != "" {
				log.Printf("Updated to %s — exiting for service restart", newVer)
				updateCancel()
				os.Exit(0)
			}
			updateCancel()
		}

		log.Printf("Next scan in %s", interval)
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			log.Printf("Shutting down")
			return
		}
	}
}

func flushSpool(ctx context.Context, sp *spool.Spool, serverURL string, apiKey string, client *http.Client) {
	entries, err := sp.List()
	if err != nil {
		log.Printf("Spool list error: %v", err)
		return
	}
	if len(entries) == 0 {
		return
	}

	log.Printf("Flushing %d spooled submission(s)", len(entries))
	for _, e := range entries {
		if err := upload.Upload(ctx, serverURL, e.Hostname, e.Submission, apiKey, client); err != nil {
			log.Printf("Spool retry failed for %s: %v", filepath.Base(e.Path), err)
			return // Stop on first failure; server is probably down
		}
		log.Printf("Spooled submission uploaded: %s", filepath.Base(e.Path))
		if err := sp.Remove(e.Path); err != nil {
			log.Printf("Spool remove failed: %v", err)
		}
	}
}

const serviceName = "osprey-agent"
