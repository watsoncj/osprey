package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/watsoncj/osprey/internal/model"
)

// Store manages file-based report storage.
type Store struct {
	Dir string
}

// ReportMeta is a summary entry for listing stored reports.
type ReportMeta struct {
	ID        string    `json:"id"`
	Hostname  string    `json:"hostname"`
	Timestamp time.Time `json:"timestamp"`
}

// Save writes a report to disk under {dir}/{hostname}/{timestamp}.json.
func (s *Store) Save(hostname string, report model.RunReport) error {
	dir := filepath.Join(s.Dir, hostname)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	id := report.StartedAt.UTC().Format("20060102T150405Z")
	path := filepath.Join(dir, id+".json")

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	return nil
}

// List returns metadata for all reports stored for a given hostname.
// If hostname is empty, reports for all hosts are returned.
func (s *Store) List(hostname string) ([]ReportMeta, error) {
	var hosts []string
	if hostname != "" {
		hosts = []string{hostname}
	} else {
		entries, err := os.ReadDir(s.Dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				hosts = append(hosts, e.Name())
			}
		}
	}

	var metas []ReportMeta
	for _, h := range hosts {
		dir := filepath.Join(s.Dir, h)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			id := strings.TrimSuffix(e.Name(), ".json")
			ts, _ := time.Parse("20060102T150405Z", id)
			metas = append(metas, ReportMeta{
				ID:        id,
				Hostname:  h,
				Timestamp: ts,
			})
		}
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Timestamp.After(metas[j].Timestamp)
	})

	return metas, nil
}

// Load reads a single report from disk.
func (s *Store) Load(hostname, id string) (model.RunReport, error) {
	path := filepath.Join(s.Dir, hostname, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return model.RunReport{}, fmt.Errorf("read report: %w", err)
	}

	var rr model.RunReport
	if err := json.Unmarshal(data, &rr); err != nil {
		return model.RunReport{}, fmt.Errorf("unmarshal report: %w", err)
	}

	return rr, nil
}
