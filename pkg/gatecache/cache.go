package gatecache

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
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

// GetData wraps successful cache hits.
type GetData struct {
	Result    CachedResult `json:"result"`
	Key       string       `json:"key"`        // hex hash of CacheKey
	FromCache bool         `json:"from_cache"` // always true for Get
}

// PutData is returned on successful Put operations.
type PutData struct {
	Key       string    // hex of hashed CacheKey
	GateType  string
	Timestamp time.Time
}

// InvalidateData is returned on successful Invalidate operations.
type InvalidateData struct {
	ClearedCount int
}

// IsExpired reports whether this cached result has exceeded its TTL.
func (r *CachedResult) IsExpired(cacheTTL time.Duration) bool {
	ttl := cacheTTL
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
func New(ctx context.Context, stateDir string, ttl time.Duration) *Cache {
	if ttl == 0 {
		ttl = DefaultTTL
	}
	c := &Cache{
		stateDir: stateDir,
		ttl:      ttl,
		entries:  make(map[string]map[string]CachedResult),
	}
	c.mu.Lock()
	_ = c.load(ctx) // ignore load errors; start with empty cache
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
func (c *Cache) BuildKey(ctx context.Context, repoDir string) result.Result[CacheKey] {
	if err := ctx.Err(); err != nil {
		return result.NewFailure[CacheKey]([]result.SAWError{
			result.NewFatal("CACHE_BUILD_KEY_CANCELLED", fmt.Sprintf("gatecache: context cancelled: %v", err)).WithCause(err),
		})
	}

	headCommit, err := git.RunOutput(repoDir, "rev-parse", "HEAD")
	if err != nil {
		return result.NewFailure[CacheKey]([]result.SAWError{
			result.NewFatal("CACHE_BUILD_KEY_FAILED", fmt.Sprintf("gatecache: get HEAD: %v", err)).WithCause(err),
		})
	}

	stagedStat, err := git.RunOutput(repoDir, "diff", "--cached", "--stat")
	if err != nil {
		return result.NewFailure[CacheKey]([]result.SAWError{
			result.NewFatal("CACHE_BUILD_KEY_FAILED", fmt.Sprintf("gatecache: get staged stat: %v", err)).WithCause(err),
		})
	}

	unstagedStat, err := git.RunOutput(repoDir, "diff", "--stat")
	if err != nil {
		return result.NewFailure[CacheKey]([]result.SAWError{
			result.NewFatal("CACHE_BUILD_KEY_FAILED", fmt.Sprintf("gatecache: get unstaged stat: %v", err)).WithCause(err),
		})
	}

	key := CacheKey{
		HeadCommit:   strings.TrimSpace(headCommit),
		StagedStat:   strings.TrimSpace(stagedStat),
		UnstagedStat: strings.TrimSpace(unstagedStat),
	}
	return result.NewSuccess(key)
}

// BuildKeyForGate computes a CacheKey for the repository at repoDir
// combined with the given gate command string. The command is included in the
// key so that changing a gate's command (e.g. adding a flag) causes a cache miss.
func (c *Cache) BuildKeyForGate(ctx context.Context, repoDir string, command string) result.Result[CacheKey] {
	r := c.BuildKey(ctx, repoDir)
	if r.IsFatal() {
		return r
	}
	key := r.GetData()
	key.Command = command
	return result.NewSuccess(key)
}

// Get returns the cached result for (key, gateType) if it exists and has not expired.
// Returns Result[GetData] with code="CACHE_MISS" (non-fatal) on miss or expiry.
func (c *Cache) Get(ctx context.Context, key CacheKey, gateType string) result.Result[GetData] {
	c.mu.RLock()
	defer c.mu.RUnlock()

	h := hashKey(key)
	inner, ok := c.entries[h]
	if !ok {
		return result.NewFailure[GetData]([]result.SAWError{
			result.NewWarning("CACHE_MISS", "cache entry not found or expired"),
		})
	}
	r, ok := inner[gateType]
	if !ok {
		return result.NewFailure[GetData]([]result.SAWError{
			result.NewWarning("CACHE_MISS", "cache entry not found or expired"),
		})
	}
	if r.IsExpired(c.ttl) {
		return result.NewFailure[GetData]([]result.SAWError{
			result.NewWarning("CACHE_MISS", "cache entry not found or expired"),
		})
	}
	return result.NewSuccess(GetData{
		Result:    r,
		Key:       h,
		FromCache: true,
	})
}

// Put stores result under (key, gateType) and persists the cache to disk.
func (c *Cache) Put(ctx context.Context, key CacheKey, gateType string, r CachedResult) result.Result[PutData] {
	if err := ctx.Err(); err != nil {
		return result.NewFailure[PutData]([]result.SAWError{
			result.NewFatal("CACHE_PUT_CANCELLED", fmt.Sprintf("gatecache: context cancelled: %v", err)).WithCause(err),
		})
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	h := hashKey(key)
	if c.entries[h] == nil {
		c.entries[h] = make(map[string]CachedResult)
	}
	r.FromCache = true
	if r.CachedAt.IsZero() {
		r.CachedAt = time.Now()
	}
	c.entries[h][gateType] = r
	if err := c.save(ctx); err != nil {
		return result.NewFailure[PutData]([]result.SAWError{
			result.NewFatal("CACHE_PUT_FAILED", fmt.Sprintf("gatecache: save cache: %v", err)).WithCause(err),
		})
	}
	return result.NewSuccess(PutData{
		Key:       h,
		GateType:  gateType,
		Timestamp: r.CachedAt,
	})
}

// Invalidate clears all in-memory entries and removes the cache file from disk.
func (c *Cache) Invalidate(ctx context.Context) result.Result[InvalidateData] {
	if err := ctx.Err(); err != nil {
		return result.NewFailure[InvalidateData]([]result.SAWError{
			result.NewFatal("CACHE_INVALIDATE_CANCELLED", fmt.Sprintf("gatecache: context cancelled: %v", err)).WithCause(err),
		})
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Count entries before clearing
	count := 0
	for _, inner := range c.entries {
		count += len(inner)
	}

	c.entries = make(map[string]map[string]CachedResult)
	path := filepath.Join(c.stateDir, cacheFileName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return result.NewFailure[InvalidateData]([]result.SAWError{
			result.NewFatal("CACHE_INVALIDATE_FAILED", fmt.Sprintf("gatecache: remove cache file: %v", err)).WithCause(err),
		})
	}
	return result.NewSuccess(InvalidateData{ClearedCount: count})
}

// load reads the cache file from disk into c.entries.
// MUST be called with c.mu held (write lock).
func (c *Cache) load(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("gatecache: load cancelled: %w", err)
	}
	path := filepath.Join(c.stateDir, cacheFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file yet; that's fine
		}
		return fmt.Errorf("CACHE_LOAD_FAILED: read cache file: %w", err)
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return fmt.Errorf("CACHE_LOAD_FAILED: parse cache file: %w", err)
	}
	if cf.Entries != nil {
		c.entries = cf.Entries
	}
	return nil
}

// save writes c.entries to the cache file on disk.
// MUST be called with c.mu held (lock already acquired by caller).
func (c *Cache) save(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("gatecache: save cancelled: %w", err)
	}
	if err := os.MkdirAll(c.stateDir, 0755); err != nil {
		return fmt.Errorf("CACHE_SAVE_FAILED: create state dir: %w", err)
	}
	cf := cacheFile{Entries: c.entries}
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return fmt.Errorf("CACHE_SAVE_FAILED: marshal cache: %w", err)
	}
	path := filepath.Join(c.stateDir, cacheFileName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("CACHE_SAVE_FAILED: write cache file: %w", err)
	}
	return nil
}
