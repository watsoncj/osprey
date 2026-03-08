// Agent is the scanner binary that runs on the target host.
// It discovers browser history databases, analyzes them, and outputs
// a report to stdout. When deployed remotely, the controller builds
// this for Windows and embeds it.
package main

import (
	"context"
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
	"github.com/watsoncj/osprey/internal/report"
	"github.com/watsoncj/osprey/internal/spool"
	"github.com/watsoncj/osprey/internal/upload"
)

func main() {
	lookback := app.Duration{D: 24 * time.Hour}
	interval := app.Duration{D: 1 * time.Hour}

	flag.Var(&lookback, "lookback", "How far back to analyze (e.g. 24h, 5d, 2w)")
	format := flag.String("format", "text", "Output format: text or json")
	server := flag.String("server", "", "Server URL to upload reports to (enables daemon mode)")
	flag.Var(&interval, "interval", "How often to scan in daemon mode (e.g. 1h, 30m)")
	hostname := flag.String("hostname", "", "Override machine hostname for reports")
	apiKey := flag.String("api-key", "", "API key for server authentication")
	spoolDir := flag.String("spool", "./spool", "Directory to spool failed uploads for retry")
	skipVerify := flag.Bool("skip-verify", false, "Skip TLS certificate verification (for self-signed certs)")
	install := flag.Bool("install", false, "Install agent as a system service")
	uninstall := flag.Bool("uninstall", false, "Uninstall agent system service")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stderr)

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
		if err := installService(*server, *hostname, interval.D, lookback.D, *apiKey); err != nil {
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

	var httpClient *http.Client
	if *skipVerify {
		httpClient = upload.InsecureClient()
	}

	if *server != "" {
		runDaemon(*server, *hostname, lookback.D, *format, interval.D, *apiKey, *spoolDir, httpClient)
	} else {
		runOnce(lookback.D, *format, *hostname, flag.Args())
	}
}

func runOnce(lookback time.Duration, format, hostname string, dbOverrides []string) {
	cfg := app.Config{
		Lookback:    lookback,
		Format:      format,
		DBOverrides: dbOverrides,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	all := browsers.All()
	adapters := make([]app.Browser, len(all))
	for i, b := range all {
		adapters[i] = b
	}
	rr := app.Run(ctx, cfg, adapters)
	rr.Hostname = hostname

	reporter := report.Get(cfg.Format)
	if err := reporter.Write(os.Stdout, rr); err != nil {
		fmt.Fprintf(os.Stderr, "error writing report: %v\n", err)
		os.Exit(1)
	}
}

func runDaemon(serverURL, hostname string, lookback time.Duration, format string, interval time.Duration, apiKey string, spoolDir string, client *http.Client) {
	log.Printf("Starting daemon: server=%s hostname=%s interval=%s", serverURL, hostname, interval)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	sp := &spool.Spool{Dir: spoolDir}

	for {
		// Flush spooled reports first
		flushSpool(ctx, sp, serverURL, apiKey, client)

		scanCtx, scanCancel := context.WithTimeout(ctx, 5*time.Minute)

		cfg := app.Config{
			Lookback: lookback,
			Format:   "json",
		}

		all := browsers.All()
		adapters := make([]app.Browser, len(all))
		for i, b := range all {
			adapters[i] = b
		}

		rr := app.Run(scanCtx, cfg, adapters)
		rr.Hostname = hostname

		if err := upload.Upload(scanCtx, serverURL, hostname, rr, apiKey, client); err != nil {
			log.Printf("Upload failed: %v", err)
			if spoolErr := sp.Save(hostname, rr); spoolErr != nil {
				log.Printf("Spool save failed: %v", spoolErr)
			} else {
				log.Printf("Report spooled for retry")
			}
		} else {
			log.Printf("Report uploaded successfully")
		}

		scanCancel()

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

	log.Printf("Flushing %d spooled report(s)", len(entries))
	for _, e := range entries {
		if err := upload.Upload(ctx, serverURL, e.Hostname, e.Report, apiKey, client); err != nil {
			log.Printf("Spool retry failed for %s: %v", filepath.Base(e.Path), err)
			return // Stop on first failure; server is probably down
		}
		log.Printf("Spooled report uploaded: %s", filepath.Base(e.Path))
		if err := sp.Remove(e.Path); err != nil {
			log.Printf("Spool remove failed: %v", err)
		}
	}
}

const serviceName = "osprey-agent"
