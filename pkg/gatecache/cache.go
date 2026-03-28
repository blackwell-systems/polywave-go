package gatecache

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
)

const (
	// DefaultTTL is the default cache entry lifetime.
	DefaultTTL = 5 * time.Minute

	// cacheFileName is the name of the JSON cache file within the state directory.
	cacheFileName = "gate-cache.json"
)

// CacheKey uniquely identifies a snapshot of the repository's working state.
// HeadCommit + StagedStat + UnstagedStat capture repository state.
// Command is the gate command string; it ensures that changing the command
// (e.g. "go test ./..." -> "go test -count=1 ./...") invalidates the cache.
type CacheKey struct {
	HeadCommit   string `json:"head_commit"`
	StagedStat   string `json:"staged_stat"`
	UnstagedStat string `json:"unstaged_stat"`
	Command      string `json:"command"`
}

// CachedResult mirrors protocol.GateResult fields plus cache metadata.
type CachedResult struct {
	GateType  string        `json:"gate_type"`
	Command   string        `json:"command"`
	Passed    bool          `json:"passed"`
	ExitCode  int           `json:"exit_code"`
	Stdout    string        `json:"stdout"`
	Stderr    string        `json:"stderr"`
	FromCache bool          `json:"from_cache"`
	CachedAt  time.Time     `json:"cached_at"`
	TTL       time.Duration `json:"-"`
}

// IsExpired reports whether this cached result has exceeded its TTL.
func (r *CachedResult) IsExpired() bool {
	ttl := r.TTL
	if ttl == 0 {
		ttl = DefaultTTL
	}
	return time.Since(r.CachedAt) > ttl
}

// cacheFile is the on-disk format for the cache.
type cacheFile struct {
	Entries map[string]map[string]CachedResult `json:"entries"`
}

// Cache is an in-memory + file-backed cache for quality gate results.
// The mu field protects concurrent access to entries map during parallel gate execution.
type Cache struct {
	stateDir string
	ttl      time.Duration
	mu       sync.RWMutex // Protects entries map
	entries  map[string]map[string]CachedResult
}

// New creates a new Cache backed by stateDir. It loads any existing cache data
// from disk. If ttl is zero, DefaultTTL is used.
func New(stateDir string, ttl time.Duration) *Cache {
	if ttl == 0 {
		ttl = DefaultTTL
	}
	c := &Cache{
		stateDir: stateDir,
		ttl:      ttl,
		entries:  make(map[string]map[string]CachedResult),
	}
	c.mu.Lock()
	_ = c.load() // ignore load errors; start with empty cache
	c.mu.Unlock()
	return c
}

// hashKey returns the md5 hex digest of the combined CacheKey fields.
func hashKey(key CacheKey) string {
	h := md5.New()
	_, _ = fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s",
		key.HeadCommit, key.StagedStat, key.UnstagedStat, key.Command)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// BuildKey computes a CacheKey for the repository at repoDir by running git
// commands to capture HEAD commit and diff stats.
func (c *Cache) BuildKey(repoDir string) (CacheKey, error) {
	headCommit, err := git.RunOutput(repoDir, "rev-parse", "HEAD")
	if err != nil {
		return CacheKey{}, fmt.Errorf("gatecache: get HEAD: %w", err)
	}

	stagedStat, err := git.RunOutput(repoDir, "diff", "--cached", "--stat")
	if err != nil {
		return CacheKey{}, fmt.Errorf("gatecache: get staged stat: %w", err)
	}

	unstagedStat, err := git.RunOutput(repoDir, "diff", "--stat")
	if err != nil {
		return CacheKey{}, fmt.Errorf("gatecache: get unstaged stat: %w", err)
	}

	return CacheKey{
		HeadCommit:   strings.TrimSpace(headCommit),
		StagedStat:   strings.TrimSpace(stagedStat),
		UnstagedStat: strings.TrimSpace(unstagedStat),
	}, nil
}

// BuildKeyForGate computes a CacheKey for the repository at repoDir
// combined with the given gate command string. The command is included in the
// key so that changing a gate's command (e.g. adding a flag) causes a cache miss.
func (c *Cache) BuildKeyForGate(repoDir string, command string) (CacheKey, error) {
	key, err := c.BuildKey(repoDir)
	if err != nil {
		return CacheKey{}, err
	}
	key.Command = command
	return key, nil
}

// Get returns the cached result for (key, gateType) if it exists and has not
// expired. The returned CachedResult has its TTL field populated from the cache.
func (c *Cache) Get(key CacheKey, gateType string) (*CachedResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	h := hashKey(key)
	inner, ok := c.entries[h]
	if !ok {
		return nil, false
	}
	result, ok := inner[gateType]
	if !ok {
		return nil, false
	}
	// Populate TTL for expiry check
	result.TTL = c.ttl
	if result.IsExpired() {
		return nil, false
	}
	return &result, true
}

// Put stores result under (key, gateType) and persists the cache to disk.
func (c *Cache) Put(key CacheKey, gateType string, result CachedResult) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	h := hashKey(key)
	if c.entries[h] == nil {
		c.entries[h] = make(map[string]CachedResult)
	}
	result.FromCache = true
	if result.CachedAt.IsZero() {
		result.CachedAt = time.Now()
	}
	c.entries[h][gateType] = result
	return c.save()
}

// Invalidate clears all in-memory entries and removes the cache file from disk.
func (c *Cache) Invalidate() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]map[string]CachedResult)
	path := filepath.Join(c.stateDir, cacheFileName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("gatecache: remove cache file: %w", err)
	}
	return nil
}

// load reads the cache file from disk into c.entries.
// MUST be called with c.mu held (write lock).
func (c *Cache) load() error {
	path := filepath.Join(c.stateDir, cacheFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file yet; that's fine
		}
		return fmt.Errorf("gatecache: read cache file: %w", err)
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return fmt.Errorf("gatecache: parse cache file: %w", err)
	}
	if cf.Entries != nil {
		c.entries = cf.Entries
	}
	return nil
}

// save writes c.entries to the cache file on disk.
// MUST be called with c.mu held (lock already acquired by caller).
func (c *Cache) save() error {
	if err := os.MkdirAll(c.stateDir, 0755); err != nil {
		return fmt.Errorf("gatecache: create state dir: %w", err)
	}
	cf := cacheFile{Entries: c.entries}
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return fmt.Errorf("gatecache: marshal cache: %w", err)
	}
	path := filepath.Join(c.stateDir, cacheFileName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("gatecache: write cache file: %w", err)
	}
	return nil
}
