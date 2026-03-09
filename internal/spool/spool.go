package spool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/watsoncj/osprey/internal/model"
)

// Spool manages locally queued reports that failed to upload.
type Spool struct {
	Dir string
}

// Save writes a submission to the spool directory for later retry.
func (s *Spool) Save(hostname string, sub model.Submission) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("create spool dir: %w", err)
	}

	id := fmt.Sprintf("%s_%s", hostname, sub.ScannedAt.UTC().Format("20060102T150405Z"))
	path := filepath.Join(s.Dir, id+".json")

	data, err := json.Marshal(sub)
	if err != nil {
		return fmt.Errorf("marshal submission: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// Entry is a spooled submission with its file path.
type Entry struct {
	Path       string
	Hostname   string
	Submission model.Submission
}

// List returns all spooled reports, oldest first.
func (s *Spool) List() ([]Entry, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result []Entry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || e.Name() == "last_scan.json" {
			continue
		}

		path := filepath.Join(s.Dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var sub model.Submission
		if err := json.Unmarshal(data, &sub); err != nil {
			continue
		}

		// Extract hostname from filename: {hostname}_{timestamp}.json
		name := strings.TrimSuffix(e.Name(), ".json")
		parts := strings.SplitN(name, "_", 2)
		hostname := parts[0]

		result = append(result, Entry{
			Path:       path,
			Hostname:   hostname,
			Submission: sub,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Submission.ScannedAt.Before(result[j].Submission.ScannedAt)
	})

	return result, nil
}

// Remove deletes a spooled report after successful upload.
func (s *Spool) Remove(path string) error {
	return os.Remove(path)
}
