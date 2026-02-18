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
	"os"
	"time"

	"github.com/browser-forensics/browser-forensics/internal/app"
	"github.com/browser-forensics/browser-forensics/internal/browsers"
	"github.com/browser-forensics/browser-forensics/internal/report"
)

func main() {
	lookback := app.Duration{D: 24 * time.Hour}
	flag.Var(&lookback, "lookback", "How far back to analyze (e.g. 24h, 5d, 2w)")
	format := flag.String("format", "text", "Output format: text or json")
	flag.Parse()

	cfg := app.Config{
		Lookback:    lookback.D,
		Format:      *format,
		DBOverrides: flag.Args(),
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stderr)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	all := browsers.All()
	adapters := make([]app.Browser, len(all))
	for i, b := range all {
		adapters[i] = b
	}
	rr := app.Run(ctx, cfg, adapters)

	reporter := report.Get(cfg.Format)
	if err := reporter.Write(os.Stdout, rr); err != nil {
		fmt.Fprintf(os.Stderr, "error writing report: %v\n", err)
		os.Exit(1)
	}
}
