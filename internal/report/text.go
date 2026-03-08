package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/watsoncj/osprey/internal/model"
)

type TextReporter struct{}

func (t *TextReporter) Write(w io.Writer, rr model.RunReport) error {
	fmt.Fprintf(w, "Osprey Report\n")
	fmt.Fprintf(w, "========================\n")
	fmt.Fprintf(w, "Scan started: %s\n", rr.StartedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "History since: %s\n\n", rr.Cutoff.Format("2006-01-02 15:04:05"))

	for _, dbr := range rr.DBReports {
		fmt.Fprintf(w, "─── %s (%s) ───\n", dbr.Browser, dbr.DBPath)

		if dbr.Error != "" {
			fmt.Fprintf(w, "  ERROR: %s\n\n", dbr.Error)
			continue
		}

		fmt.Fprintf(w, "  Total visits: %d\n", dbr.Summary.TotalVisits)
		fmt.Fprintf(w, "  Flagged visits: %d\n", dbr.Summary.FlaggedVisits)

		if len(dbr.Summary.TopDomains) > 0 {
			fmt.Fprintf(w, "  Top domains:\n")
			for _, dc := range dbr.Summary.TopDomains {
				fmt.Fprintf(w, "    %-40s %d\n", dc.Domain, dc.Count)
			}
		}

		if len(dbr.Summary.CategoryCounts) > 0 {
			fmt.Fprintf(w, "\n  ⚠ FLAGS BY CATEGORY:\n")
			for cat, count := range dbr.Summary.CategoryCounts {
				fmt.Fprintf(w, "    %-20s %d\n", cat, count)
			}
		}

		// Print YouTube video summary
		type ytEntry struct {
			time  string
			id    string
			title string
		}
		var ytVideos []ytEntry
		for _, v := range dbr.Visits {
			for _, d := range v.Decoded {
				if d.Decoder == "youtube" && d.Kind == "video" {
					ytVideos = append(ytVideos, ytEntry{
						time:  v.Time.Format("2006-01-02 15:04:05"),
						id:    d.Data["video_id"],
						title: d.Data["title"],
					})
				}
			}
		}
		if len(ytVideos) > 0 {
			fmt.Fprintf(w, "\n  YOUTUBE VIDEOS (%d):\n", len(ytVideos))
			for _, yt := range ytVideos {
				if yt.title != "" {
					fmt.Fprintf(w, "    [%s] %s (%s)\n", yt.time, yt.title, yt.id)
				} else {
					fmt.Fprintf(w, "    [%s] %s\n", yt.time, yt.id)
				}
			}
		}

		// Print flagged visits
		var flagged []model.Visit
		for _, v := range dbr.Visits {
			if len(v.Flags) > 0 {
				flagged = append(flagged, v)
			}
		}

		if len(flagged) > 0 {
			fmt.Fprintf(w, "\n  FLAGGED ITEMS:\n")
			for _, v := range flagged {
				fmt.Fprintf(w, "    [%s] %s\n", v.Time.Format("2006-01-02 15:04:05"), truncate(v.URL, 100))
				if v.Title != "" {
					fmt.Fprintf(w, "      Title: %s\n", truncate(v.Title, 80))
				}
				for _, d := range v.Decoded {
					for k, val := range d.Data {
						fmt.Fprintf(w, "      %s.%s: %s\n", d.Decoder, k, val)
					}
				}
				var cats []string
				for _, f := range v.Flags {
					cats = append(cats, fmt.Sprintf("%s(%s via %s)", f.Category, f.Keyword, f.Source))
				}
				fmt.Fprintf(w, "      Flags: %s\n", strings.Join(cats, ", "))
			}
		}

		// Print incognito indicators
		if len(dbr.IncognitoIndicators) > 0 {
			fmt.Fprintf(w, "\n  POSSIBLE INCOGNITO VISITS (%d):\n", len(dbr.IncognitoIndicators))
			fmt.Fprintf(w, "    (URLs found in Favicons DB but absent from History)\n")
			for _, ind := range dbr.IncognitoIndicators {
				fmt.Fprintf(w, "    %s\n", truncate(ind.URL, 100))
				for _, d := range ind.Decoded {
					for k, val := range d.Data {
						fmt.Fprintf(w, "      %s.%s: %s\n", d.Decoder, k, val)
					}
				}
			}
		}

		// Print decoded search queries
		var searches []model.Visit
		for _, v := range dbr.Visits {
			if len(v.Decoded) > 0 && len(v.Flags) == 0 {
				searches = append(searches, v)
			}
		}
		if len(searches) > 0 {
			fmt.Fprintf(w, "\n  DECODED URLS (unflagged):\n")
			for _, v := range searches {
				for _, d := range v.Decoded {
					var parts []string
					for k, val := range d.Data {
						parts = append(parts, fmt.Sprintf("%s=%s", k, val))
					}
					fmt.Fprintf(w, "    [%s] %s: %s\n", v.Time.Format("2006-01-02 15:04:05"), d.Decoder, strings.Join(parts, ", "))
				}
			}
		}

		fmt.Fprintln(w)
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
