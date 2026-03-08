package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/watsoncj/osprey/internal/app"
	"github.com/watsoncj/osprey/internal/browsers"
	"github.com/watsoncj/osprey/internal/report"
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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
