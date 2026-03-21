package engine

import (
	"sync"
	"time"
)

// GateResultCache stores pass/fail results for verification gates keyed by
// (command + input file hashes). This ensures repeated gate runs within the
// same wave execution reuse prior results (E38).
type GateResultCache struct {
	mu      sync.RWMutex
	entries map[string]CachedGateResult
}

// CachedGateResult holds the cached outcome of a verification gate execution.
type CachedGateResult struct {
	Passed    bool      `json:"passed"`
	Output    string    `json:"output"`
	CachedAt  time.Time `json:"cached_at"`
	InputHash string    `json:"input_hash"` // hash of files that affect this gate
}

// NewGateResultCache creates a new empty gate result cache.
func NewGateResultCache() *GateResultCache {
	return &GateResultCache{
		entries: make(map[string]CachedGateResult),
	}
}

// cacheKey generates a unique key from command and input hash.
func cacheKey(command, inputHash string) string {
	return command + "\x00" + inputHash
}

// Get retrieves a cached gate result. Returns the result and true if found,
// or a zero value and false if not cached.
func (c *GateResultCache) Get(command string, inputHash string) (CachedGateResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result, ok := c.entries[cacheKey(command, inputHash)]
	return result, ok
}

// Set stores a gate result in the cache.
func (c *GateResultCache) Set(command string, inputHash string, result CachedGateResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	result.InputHash = inputHash
	if result.CachedAt.IsZero() {
		result.CachedAt = time.Now()
	}
	c.entries[cacheKey(command, inputHash)] = result
}

// Invalidate clears all cached gate results. This should be called when
// files change (e.g., after format gate runs in fix mode, new wave merges).
func (c *GateResultCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]CachedGateResult)
}

// InvalidateCommand removes all cached results for a specific command,
// regardless of input hash.
func (c *GateResultCache) InvalidateCommand(command string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prefix := command + "\x00"
	for k := range c.entries {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.entries, k)
		}
	}
}
