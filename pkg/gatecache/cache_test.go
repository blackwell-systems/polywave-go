package gatecache

import (
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestCache_PutGet(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	c := New(ctx, dir, DefaultTTL)

	key := CacheKey{
		HeadCommit:   "abc123",
		StagedStat:   "",
		UnstagedStat: "",
		Command:      "",
	}
	r := CachedResult{
		GateType: "build",
		Command:  "go build ./...",
		Passed:   true,
		ExitCode: 0,
		Stdout:   "build output",
		Stderr:   "",
	}

	putResult := c.Put(ctx, key, "build", r)
	if putResult.IsFatal() {
		t.Fatalf("Put failed: %v", putResult.Errors)
	}

	getResult := c.Get(ctx, key, "build")
	if !getResult.IsSuccess() {
		t.Fatal("expected cache hit, got CACHE_MISS")
	}
	if !getResult.IsSuccess() {
		t.Fatalf("Get failed: %v", getResult.Errors)
	}
	data := getResult.GetData()
	got := data.Result
	if got.GateType != "build" {
		t.Errorf("GateType mismatch: got %q, want %q", got.GateType, "build")
	}
	if !got.Passed {
		t.Error("expected Passed=true")
	}
	if got.Stdout != "build output" {
		t.Errorf("Stdout mismatch: got %q, want %q", got.Stdout, "build output")
	}
	if !data.FromCache {
		t.Error("expected FromCache=true after Put")
	}
}

func TestCache_Put_ReturnsSuccessWithCorrectGateType(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	c := New(ctx, dir, DefaultTTL)

	key := CacheKey{HeadCommit: "abc123"}
	r := CachedResult{
		GateType: "lint",
		Passed:   true,
	}

	putResult := c.Put(ctx, key, "lint", r)
	if !putResult.IsSuccess() {
		t.Fatalf("expected Put to return Success, got code=%q errors=%v", putResult.Code, putResult.Errors)
	}
	data := putResult.GetData()
	if data.GateType != "lint" {
		t.Errorf("PutData.GateType: got %q, want %q", data.GateType, "lint")
	}
	if data.Key == "" {
		t.Error("PutData.Key should not be empty")
	}
	if data.Timestamp.IsZero() {
		t.Error("PutData.Timestamp should not be zero")
	}
}

func TestCache_Get_ReturnsGetDataStruct(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	c := New(ctx, dir, DefaultTTL)

	key := CacheKey{HeadCommit: "abc123"}
	r := CachedResult{
		GateType: "test",
		Passed:   true,
		Command:  "go test",
	}

	putResult := c.Put(ctx, key, "test", r)
	if putResult.IsFatal() {
		t.Fatalf("Put failed: %v", putResult.Errors)
	}

	getResult := c.Get(ctx, key, "test")
	if !getResult.IsSuccess() {
		t.Fatalf("expected success, got code=%q errors=%v", getResult.Code, getResult.Errors)
	}
	data := getResult.GetData()
	if data.Key == "" {
		t.Error("Key should not be empty")
	}
	if !data.FromCache {
		t.Error("FromCache should be true for Get")
	}
	if data.Result.GateType != "test" {
		t.Errorf("Result.GateType: got %q, want %q", data.Result.GateType, "test")
	}
}

func TestCache_Get_ReturnsMissCodeOnExpiry(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	ttl := 50 * time.Millisecond
	c := New(ctx, dir, ttl)

	key := CacheKey{HeadCommit: "deadbeef"}
	// Set CachedAt to already-expired timestamp
	r := CachedResult{
		GateType: "test",
		Passed:   true,
		CachedAt: time.Now().Add(-100 * time.Millisecond),
	}

	putResult := c.Put(ctx, key, "test", r)
	if putResult.IsFatal() {
		t.Fatalf("Put failed: %v", putResult.Errors)
	}

	getResult := c.Get(ctx, key, "test")
	if getResult.IsSuccess() {
		t.Errorf("expected CACHE_MISS on expiry but got a hit: %v", getResult.GetData())
	}
	if len(getResult.Errors) == 0 || getResult.Errors[0].Code != result.CodeCacheMiss {
		t.Errorf("expected CACHE_MISS error code, got %v", getResult.Errors)
	}
	if len(getResult.Errors) > 0 && getResult.Errors[0].Severity == "fatal" {
		t.Error("CACHE_MISS should have warning severity, not fatal")
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	ttl := 50 * time.Millisecond
	c := New(ctx, dir, ttl)

	key := CacheKey{HeadCommit: "deadbeef"}
	// Set CachedAt to already-expired timestamp
	r := CachedResult{
		GateType: "test",
		Passed:   true,
		CachedAt: time.Now().Add(-100 * time.Millisecond),
	}

	putResult := c.Put(ctx, key, "test", r)
	if putResult.IsFatal() {
		t.Fatalf("Put failed: %v", putResult.Errors)
	}

	getResult := c.Get(ctx, key, "test")
	if getResult.IsSuccess() {
		t.Errorf("expected CACHE_MISS due to TTL expiry but got a hit: %v", getResult.GetData())
	}
	if len(getResult.Errors) == 0 || getResult.Errors[0].Code != result.CodeCacheMiss {
		t.Errorf("expected CACHE_MISS error code, got %v", getResult.Errors)
	}
}

func TestCache_TTLNotExpired(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	c := New(ctx, dir, 5*time.Minute)

	key := CacheKey{HeadCommit: "fresh123"}
	r := CachedResult{
		GateType: "lint",
		Passed:   true,
	}

	putResult := c.Put(ctx, key, "lint", r)
	if putResult.IsFatal() {
		t.Fatalf("Put failed: %v", putResult.Errors)
	}

	getResult := c.Get(ctx, key, "lint")
	if !getResult.IsSuccess() {
		t.Fatal("expected cache hit, got CACHE_MISS")
	}
	if !getResult.IsSuccess() {
		t.Fatalf("Get failed: %v", getResult.Errors)
	}
	got := getResult.GetData().Result
	if got.GateType != "lint" {
		t.Errorf("GateType mismatch: got %q", got.GateType)
	}
}

func TestCache_Invalidate(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	c := New(ctx, dir, DefaultTTL)

	key := CacheKey{HeadCommit: "abc"}
	r := CachedResult{GateType: "build", Passed: true}

	putResult := c.Put(ctx, key, "build", r)
	if putResult.IsFatal() {
		t.Fatalf("Put failed: %v", putResult.Errors)
	}

	// Verify cache file was created
	cachePath := filepath.Join(dir, cacheFileName)
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("expected cache file to exist before Invalidate")
	}

	invResult := c.Invalidate(ctx)
	if invResult.IsFatal() {
		t.Fatalf("Invalidate failed: %v", invResult.Errors)
	}

	// Entries should be gone
	getResult := c.Get(ctx, key, "build")
	if getResult.IsSuccess() {
		t.Errorf("expected CACHE_MISS after Invalidate, got code=%q", getResult.Code)
	}

	// Cache file should be removed
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("expected cache file to be removed after Invalidate")
	}
}

func TestCache_Invalidate_ReturnsSuccessWithClearedCount(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	c := New(ctx, dir, DefaultTTL)

	// Put multiple entries
	key := CacheKey{HeadCommit: "abc"}
	for _, gt := range []string{"build", "test", "lint"} {
		putResult := c.Put(ctx, key, gt, CachedResult{GateType: gt, Passed: true})
		if putResult.IsFatal() {
			t.Fatalf("Put(%q) failed: %v", gt, putResult.Errors)
		}
	}

	invResult := c.Invalidate(ctx)
	if !invResult.IsSuccess() {
		t.Fatalf("expected Invalidate to return Success, got code=%q errors=%v", invResult.Code, invResult.Errors)
	}
	data := invResult.GetData()
	if data.ClearedCount <= 0 {
		t.Errorf("expected ClearedCount > 0, got %d", data.ClearedCount)
	}
}

func TestCache_InvalidateNonExistentFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	c := New(ctx, dir, DefaultTTL)

	// Invalidate with no cache file should not error
	invResult := c.Invalidate(ctx)
	if invResult.IsFatal() {
		t.Errorf("Invalidate with no cache file should not error: %v", invResult.Errors)
	}
}

func TestCache_Persistence(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	c1 := New(ctx, dir, DefaultTTL)

	key := CacheKey{
		HeadCommit:   "persist123",
		StagedStat:   "1 file changed",
		UnstagedStat: "",
		Command:      "",
	}
	r := CachedResult{
		GateType: "vet",
		Command:  "go vet ./...",
		Passed:   true,
		ExitCode: 0,
		Stdout:   "vet ok",
		Stderr:   "",
	}

	putResult := c1.Put(ctx, key, "vet", r)
	if putResult.IsFatal() {
		t.Fatalf("Put failed: %v", putResult.Errors)
	}

	// Create a new cache instance from the same directory — it should load from disk
	c2 := New(ctx, dir, DefaultTTL)

	getResult := c2.Get(ctx, key, "vet")
	if !getResult.IsSuccess() {
		t.Fatal("expected cache hit after reload from disk, got CACHE_MISS")
	}
	if !getResult.IsSuccess() {
		t.Fatalf("Get failed: %v", getResult.Errors)
	}
	data := getResult.GetData()
	got := data.Result
	if got.GateType != "vet" {
		t.Errorf("GateType mismatch after reload: got %q", got.GateType)
	}
	if !got.Passed {
		t.Error("expected Passed=true after reload")
	}
	if got.Stdout != "vet ok" {
		t.Errorf("Stdout mismatch after reload: got %q", got.Stdout)
	}
	if !data.FromCache {
		t.Error("expected FromCache=true after reload")
	}
}

func TestCache_MultipleGateTypes(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	c := New(ctx, dir, DefaultTTL)

	key := CacheKey{HeadCommit: "multi"}
	gates := []struct {
		gateType string
		passed   bool
	}{
		{"build", true},
		{"test", false},
		{"lint", true},
	}

	for _, g := range gates {
		r := CachedResult{GateType: g.gateType, Passed: g.passed}
		putResult := c.Put(ctx, key, g.gateType, r)
		if putResult.IsFatal() {
			t.Fatalf("Put(%q) failed: %v", g.gateType, putResult.Errors)
		}
	}

	for _, g := range gates {
		getResult := c.Get(ctx, key, g.gateType)
		if !getResult.IsSuccess() {
			t.Errorf("expected hit for gate %q, got CACHE_MISS", g.gateType)
			continue
		}
		if !getResult.IsSuccess() {
			t.Errorf("Get failed for gate %q: %v", g.gateType, getResult.Errors)
			continue
		}
		got := getResult.GetData().Result
		if got.Passed != g.passed {
			t.Errorf("gate %q: Passed mismatch: got %v, want %v", g.gateType, got.Passed, g.passed)
		}
	}
}

func TestBuildKey(t *testing.T) {
	// Initialize a temp git repo so BuildKey can run real git commands
	dir := t.TempDir()

	gitCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	gitCmd("init")
	gitCmd("config", "user.email", "test@test.com")
	gitCmd("config", "user.name", "Test")

	// Create an initial commit
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	gitCmd("add", ".")
	gitCmd("commit", "-m", "initial")

	ctx := context.Background()
	c := New(ctx, dir, DefaultTTL)
	keyResult := c.BuildKey(ctx, dir)
	if keyResult.IsFatal() {
		t.Fatalf("BuildKey failed: %v", keyResult.Errors)
	}
	key := keyResult.GetData()

	if key.HeadCommit == "" {
		t.Error("HeadCommit should not be empty")
	}
	if len(key.HeadCommit) != 40 {
		t.Errorf("HeadCommit should be a 40-char SHA, got %q (len=%d)", key.HeadCommit, len(key.HeadCommit))
	}

	// Build a second key from the same state — it should produce the same hash
	keyResult2 := c.BuildKey(ctx, dir)
	if keyResult2.IsFatal() {
		t.Fatalf("second BuildKey failed: %v", keyResult2.Errors)
	}
	key2 := keyResult2.GetData()
	if hashKey(key) != hashKey(key2) {
		t.Error("same repo state should produce same hash")
	}

	// Modify a file (unstaged) and verify the key changes
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}
	keyResult3 := c.BuildKey(ctx, dir)
	if keyResult3.IsFatal() {
		t.Fatalf("third BuildKey failed: %v", keyResult3.Errors)
	}
	key3 := keyResult3.GetData()
	if hashKey(key) == hashKey(key3) {
		t.Error("different repo state should produce different hash")
	}
}

func TestBuildKey_ReturnsSuccessWithNonEmptyHeadCommit(t *testing.T) {
	dir := t.TempDir()

	gitCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	gitCmd("init")
	gitCmd("config", "user.email", "test@test.com")
	gitCmd("config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	gitCmd("add", ".")
	gitCmd("commit", "-m", "initial")

	ctx := context.Background()
	c := New(ctx, dir, DefaultTTL)
	keyResult := c.BuildKey(ctx, dir)
	if !keyResult.IsSuccess() {
		t.Fatalf("expected BuildKey to return Success, got code=%q errors=%v", keyResult.Code, keyResult.Errors)
	}
	data := keyResult.GetData()
	if data.HeadCommit == "" {
		t.Error("expected non-empty HeadCommit in BuildKey result")
	}
}

func TestCache_CommandDifferentiation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	c := New(ctx, dir, DefaultTTL)

	// Two keys identical except for Command field
	key1 := CacheKey{
		HeadCommit:   "sha1abc",
		StagedStat:   "",
		UnstagedStat: "",
		Command:      "go build ./...",
	}
	key2 := CacheKey{
		HeadCommit:   "sha1abc",
		StagedStat:   "",
		UnstagedStat: "",
		Command:      "go build -race ./...",
	}

	// The hash digests must differ
	if hashKey(key1) == hashKey(key2) {
		t.Error("different Command strings should produce different hash keys")
	}

	// Storing under key1 should not be retrievable via key2
	r := CachedResult{GateType: "build", Passed: true}
	putResult := c.Put(ctx, key1, "build", r)
	if putResult.IsFatal() {
		t.Fatalf("Put failed: %v", putResult.Errors)
	}

	getResult2 := c.Get(ctx, key2, "build")
	if getResult2.IsSuccess() {
		t.Error("expected CACHE_MISS for key2 (different Command), but got hit")
	}
	getResult1 := c.Get(ctx, key1, "build")
	if !getResult1.IsSuccess() {
		t.Error("expected cache hit for key1, got CACHE_MISS")
	}
}

func TestBuildKeyForGate(t *testing.T) {
	// Initialize a temp git repo
	dir := t.TempDir()

	gitCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	gitCmd("init")
	gitCmd("config", "user.email", "test@test.com")
	gitCmd("config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	gitCmd("add", ".")
	gitCmd("commit", "-m", "initial")

	ctx := context.Background()
	c := New(ctx, dir, DefaultTTL)

	r1 := c.BuildKeyForGate(ctx, dir, "go test ./...")
	if r1.IsFatal() {
		t.Fatalf("BuildKeyForGate failed: %v", r1.Errors)
	}
	key1 := r1.GetData()
	if key1.Command != "go test ./..." {
		t.Errorf("Command field not set: got %q", key1.Command)
	}

	r2 := c.BuildKeyForGate(ctx, dir, "go test -count=1 ./...")
	if r2.IsFatal() {
		t.Fatalf("BuildKeyForGate (second) failed: %v", r2.Errors)
	}
	key2 := r2.GetData()
	if key2.Command != "go test -count=1 ./..." {
		t.Errorf("Command field not set: got %q", key2.Command)
	}

	// Different commands must produce different hashes
	if hashKey(key1) == hashKey(key2) {
		t.Error("different gate commands should produce different hashes")
	}

	// Same command must produce the same hash
	r3 := c.BuildKeyForGate(ctx, dir, "go test ./...")
	if r3.IsFatal() {
		t.Fatalf("BuildKeyForGate (third) failed: %v", r3.Errors)
	}
	key3 := r3.GetData()
	if hashKey(key1) != hashKey(key3) {
		t.Error("same gate command should produce the same hash")
	}
}

// TestCacheThreadSafety launches 10 goroutines that call Get/Put concurrently.
// Run with `go test -race` to detect data races.
func TestCacheThreadSafety(t *testing.T) {
	ctx := context.Background()
	cache := New(ctx, t.TempDir(), DefaultTTL)
	key := CacheKey{HeadCommit: "abc123", Command: "go test"}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Concurrent writes
			r := CachedResult{
				GateType: fmt.Sprintf("type-%d", idx),
				Passed:   true,
			}
			_ = cache.Put(ctx, key, fmt.Sprintf("type-%d", idx), r)
			// Concurrent reads
			_ = cache.Get(ctx, key, fmt.Sprintf("type-%d", idx))
		}(i)
	}
	wg.Wait()

	// Verify all entries exist after concurrent writes
	for i := 0; i < 10; i++ {
		getResult := cache.Get(ctx, key, fmt.Sprintf("type-%d", i))
		if !getResult.IsSuccess() {
			t.Errorf("expected gate type-%d to exist after concurrent Put", i)
		}
	}
}

// TestConcurrentInvalidate verifies that Invalidate can be called safely
// while other goroutines are calling Get/Put. This should not panic.
func TestConcurrentInvalidate(t *testing.T) {
	ctx := context.Background()
	cache := New(ctx, t.TempDir(), DefaultTTL)
	key := CacheKey{HeadCommit: "invalidate123"}

	var wg sync.WaitGroup

	// Launch 5 goroutines that continuously Get/Put
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				r := CachedResult{
					GateType: fmt.Sprintf("gate-%d-%d", idx, j),
					Passed:   true,
				}
				_ = cache.Put(ctx, key, r.GateType, r)
				_ = cache.Get(ctx, key, r.GateType)
			}
		}(i)
	}

	// Launch 1 goroutine that calls Invalidate multiple times
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			_ = cache.Invalidate(ctx)
		}
	}()

	wg.Wait()
	// If we reach here without panicking, the test passed
}

// TestConcurrentPut verifies that multiple goroutines can Put different gate
// results to the same cache key without corrupting the entries map.
func TestConcurrentPut(t *testing.T) {
	ctx := context.Background()
	cache := New(ctx, t.TempDir(), DefaultTTL)
	key := CacheKey{HeadCommit: "concurrent-put-123"}

	const numGoroutines = 20
	var wg sync.WaitGroup

	// Each goroutine writes a unique gate type
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r := CachedResult{
				GateType: fmt.Sprintf("parallel-gate-%d", idx),
				Passed:   idx%2 == 0, // Alternate passed/failed
			}
			putResult := cache.Put(ctx, key, r.GateType, r)
			if putResult.IsFatal() {
				t.Errorf("Put failed for gate %d: %v", idx, putResult.Errors)
			}
		}(i)
	}

	wg.Wait()

	// Verify all gate types exist and have correct values
	for i := 0; i < numGoroutines; i++ {
		gateType := fmt.Sprintf("parallel-gate-%d", i)
		getResult := cache.Get(ctx, key, gateType)
		if !getResult.IsSuccess() {
			t.Errorf("expected gate %q to exist, got CACHE_MISS", gateType)
			continue
		}
		if !getResult.IsSuccess() {
			t.Errorf("Get failed for gate %q: %v", gateType, getResult.Errors)
			continue
		}
		got := getResult.GetData().Result
		expectedPassed := i%2 == 0
		if got.Passed != expectedPassed {
			t.Errorf("gate %q: expected Passed=%v, got %v", gateType, expectedPassed, got.Passed)
		}
	}
}
