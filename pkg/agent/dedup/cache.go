// Package dedup provides a content-hash cache for deduplicating file reads.
// When an agent reads the same file content multiple times, the cache returns
// a compact summary instead of the full content, reducing token usage.
package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Entry records the hash and metadata for a cached file read.
type Entry struct {
	ContentHash string
	LineCount   int
	ReadAt      time.Time
}

// Stats holds dedup metrics for inclusion in completion reports.
type Stats struct {
	Hits                int
	Misses              int
	TokensSavedEstimate int
}

// Cache is a per-agent content-hash cache for file read dedup.
// It is safe for concurrent use.
type Cache struct {
	mu      sync.Mutex
	entries map[string]Entry
	hits    int
	misses  int
	// hitLines accumulates total line counts for all hits (for token estimate)
	hitLines int
}

// New creates a new empty Cache.
func New() *Cache {
	return &Cache{
		entries: make(map[string]Entry),
	}
}

// hashContent computes the sha256 hex digest of content.
func hashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// countLines counts the number of lines in content.
func countLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	n := strings.Count(string(content), "\n")
	// If content doesn't end in newline, the last line still counts
	if len(content) > 0 && content[len(content)-1] != '\n' {
		n++
	}
	return n
}

// Check compares content against the cached hash for path.
// Returns (true, summary) if content is unchanged since last read.
// Returns (false, "") if this is a new read or content changed.
// Updates the cache entry on miss.
func (c *Cache) Check(path string, content []byte) (deduped bool, summary string) {
	hash := hashContent(content)

	c.mu.Lock()
	defer c.mu.Unlock()

	existing, found := c.entries[path]
	if found && existing.ContentHash == hash {
		// Cache hit: content unchanged
		c.hits++
		c.hitLines += existing.LineCount
		shortHash := hash[:8]
		return true, fmt.Sprintf("[file unchanged since last read — %d lines, hash %s]", existing.LineCount, shortHash)
	}

	// Cache miss: update entry
	lineCount := countLines(content)
	c.entries[path] = Entry{
		ContentHash: hash,
		LineCount:   lineCount,
		ReadAt:      time.Now(),
	}
	c.misses++
	return false, ""
}

// Invalidate removes the cache entry for path.
// Called when the agent writes or edits a file.
func (c *Cache) Invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, path)
}

// Stats returns current dedup metrics.
func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{
		Hits:                c.hits,
		Misses:              c.misses,
		TokensSavedEstimate: c.hitLines * 4,
	}
}
