package decoder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// ytCache is a simple on-disk JSON cache mapping YouTube video IDs to titles.
type ytCache struct {
	mu    sync.Mutex
	path  string
	items map[string]string
}

func newYTCache() *ytCache {
	c := &ytCache{items: make(map[string]string)}

	dir, err := os.UserCacheDir()
	if err != nil {
		return c
	}
	cacheDir := filepath.Join(dir, "browser-forensics")
	_ = os.MkdirAll(cacheDir, 0o755)
	c.path = filepath.Join(cacheDir, "youtube-titles.json")

	c.load()
	return c
}

func (c *ytCache) load() {
	if c.path == "" {
		return
	}
	data, err := os.ReadFile(c.path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &c.items)
}

func (c *ytCache) save() {
	if c.path == "" {
		return
	}
	data, err := json.Marshal(c.items)
	if err != nil {
		return
	}
	_ = os.WriteFile(c.path, data, 0o644)
}

func (c *ytCache) get(videoID string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	title, ok := c.items[videoID]
	return title, ok
}

func (c *ytCache) set(videoID, title string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[videoID] = title
	c.save()
}
