package engine

import (
	"testing"
	"time"
)

func TestGateCacheGetSet(t *testing.T) {
	cache := NewGateResultCache()

	// Initially empty.
	_, ok := cache.Get("go test ./...", "abc123")
	if ok {
		t.Fatal("expected cache miss on empty cache")
	}

	// Set and retrieve.
	result := CachedGateResult{
		Passed:    true,
		Output:    "PASS",
		CachedAt:  time.Now(),
		InputHash: "abc123",
	}
	cache.Set("go test ./...", "abc123", result)

	got, ok := cache.Get("go test ./...", "abc123")
	if !ok {
		t.Fatal("expected cache hit after Set")
	}
	if !got.Passed {
		t.Fatal("expected Passed=true")
	}
	if got.Output != "PASS" {
		t.Fatalf("expected Output=PASS, got %s", got.Output)
	}
}

func TestGateCacheMissForDifferentInputHash(t *testing.T) {
	cache := NewGateResultCache()

	cache.Set("go test ./...", "hash-v1", CachedGateResult{Passed: true, Output: "ok"})

	// Same command, different hash should miss.
	_, ok := cache.Get("go test ./...", "hash-v2")
	if ok {
		t.Fatal("expected cache miss for different inputHash")
	}

	// Same hash, different command should miss.
	_, ok = cache.Get("go vet ./...", "hash-v1")
	if ok {
		t.Fatal("expected cache miss for different command")
	}
}

func TestGateCacheInvalidate(t *testing.T) {
	cache := NewGateResultCache()

	cache.Set("cmd1", "h1", CachedGateResult{Passed: true})
	cache.Set("cmd2", "h2", CachedGateResult{Passed: false})

	cache.Invalidate()

	_, ok1 := cache.Get("cmd1", "h1")
	_, ok2 := cache.Get("cmd2", "h2")
	if ok1 || ok2 {
		t.Fatal("expected all entries cleared after Invalidate")
	}
}

func TestGateCacheInvalidateCommand(t *testing.T) {
	cache := NewGateResultCache()

	cache.Set("go test ./...", "h1", CachedGateResult{Passed: true})
	cache.Set("go test ./...", "h2", CachedGateResult{Passed: false})
	cache.Set("go vet ./...", "h1", CachedGateResult{Passed: true})

	cache.InvalidateCommand("go test ./...")

	_, ok := cache.Get("go test ./...", "h1")
	if ok {
		t.Fatal("expected invalidated command entry to be gone")
	}
	_, ok = cache.Get("go test ./...", "h2")
	if ok {
		t.Fatal("expected invalidated command entry to be gone")
	}

	// Other command should remain.
	_, ok = cache.Get("go vet ./...", "h1")
	if !ok {
		t.Fatal("expected unrelated command entry to remain")
	}
}

func TestGateCacheSetAutoTimestamp(t *testing.T) {
	cache := NewGateResultCache()

	before := time.Now()
	cache.Set("cmd", "h", CachedGateResult{Passed: true})
	after := time.Now()

	got, ok := cache.Get("cmd", "h")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.CachedAt.Before(before) || got.CachedAt.After(after) {
		t.Fatalf("CachedAt %v not between %v and %v", got.CachedAt, before, after)
	}
}
