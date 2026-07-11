package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"lasordning/series"
)

// cache.go persists fetched full-series book lists to a JSON file in the app
// directory, so the "what to download next" list is available offline (the
// user fetches everything while online, then browses on the go).

const cacheFile = "lasordning_series.json"

// SeriesCache is an in-memory + on-disk map of series name -> full book list.
type SeriesCache struct {
	mu   sync.Mutex
	path string
	data map[string]series.FullSeries
}

// LoadCache reads the cache file next to the executable. A missing/corrupt file
// yields an empty cache (not an error) so first run and offline both work.
func LoadCache(dir string) *SeriesCache {
	c := &SeriesCache{
		path: filepath.Join(dir, cacheFile),
		data: map[string]series.FullSeries{},
	}
	b, err := os.ReadFile(c.path)
	if err != nil {
		return c
	}
	var m map[string]series.FullSeries
	if json.Unmarshal(b, &m) == nil && m != nil {
		c.data = m
	}
	return c
}

// Get returns the cached full series for a name (ok=false if not cached).
func (c *SeriesCache) Get(name string) (series.FullSeries, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fs, ok := c.data[name]
	return fs, ok
}

// Put stores a full series and flushes the whole cache to disk. Flush errors
// are returned but the in-memory copy is always updated.
func (c *SeriesCache) Put(fs series.FullSeries) error {
	c.mu.Lock()
	c.data[fs.Name] = fs
	// snapshot under lock, write outside
	b, err := json.MarshalIndent(c.data, "", "  ")
	c.mu.Unlock()
	if err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}

// Count returns how many series are cached (for status display).
func (c *SeriesCache) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.data)
}
