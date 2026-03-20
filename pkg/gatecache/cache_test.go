package gatecache

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCache_PutGet(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, DefaultTTL)

	key := CacheKey{
		HeadCommit:   "abc123",
		StagedStat:   "",
		UnstagedStat: "",
		Command:      "",
	}
	result := CachedResult{
		GateType: "build",
		Command:  "go build ./...",
		Passed:   true,
		ExitCode: 0,
		Stdout:   "build output",
		Stderr:   "",
	}

	if err := c.Put(key, "build", result); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	got, ok := c.Get(key, "build")
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if got.GateType != "build" {
		t.Errorf("GateType mismatch: got %q, want %q", got.GateType, "build")
	}
	if !got.Passed {
		t.Error("expected Passed=true")
	}
	if got.Stdout != "build output" {
		t.Errorf("Stdout mismatch: got %q, want %q", got.Stdout, "build output")
	}
	if !got.FromCache {
		t.Error("expected FromCache=true after Put")
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	dir := t.TempDir()
	ttl := 50 * time.Millisecond
	c := New(dir, ttl)

	key := CacheKey{HeadCommit: "deadbeef"}
	// Set CachedAt to already-expired timestamp
	result := CachedResult{
		GateType: "test",
		Passed:   true,
		CachedAt: time.Now().Add(-100 * time.Millisecond),
	}

	if err := c.Put(key, "test", result); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	got, ok := c.Get(key, "test")
	if ok {
		t.Errorf("expected cache miss due to TTL expiry, but got hit: %+v", got)
	}
}

func TestCache_TTLNotExpired(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, 5*time.Minute)

	key := CacheKey{HeadCommit: "fresh123"}
	result := CachedResult{
		GateType: "lint",
		Passed:   true,
	}

	if err := c.Put(key, "lint", result); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	got, ok := c.Get(key, "lint")
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if got.GateType != "lint" {
		t.Errorf("GateType mismatch: got %q", got.GateType)
	}
}

func TestCache_Invalidate(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, DefaultTTL)

	key := CacheKey{HeadCommit: "abc"}
	result := CachedResult{GateType: "build", Passed: true}

	if err := c.Put(key, "build", result); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Verify cache file was created
	cachePath := filepath.Join(dir, cacheFileName)
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("expected cache file to exist before Invalidate")
	}

	if err := c.Invalidate(); err != nil {
		t.Fatalf("Invalidate failed: %v", err)
	}

	// Entries should be gone
	if _, ok := c.Get(key, "build"); ok {
		t.Error("expected cache miss after Invalidate, got hit")
	}

	// Cache file should be removed
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("expected cache file to be removed after Invalidate")
	}
}

func TestCache_InvalidateNonExistentFile(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, DefaultTTL)

	// Invalidate with no cache file should not error
	if err := c.Invalidate(); err != nil {
		t.Errorf("Invalidate with no cache file should not error: %v", err)
	}
}

func TestCache_Persistence(t *testing.T) {
	dir := t.TempDir()
	c1 := New(dir, DefaultTTL)

	key := CacheKey{
		HeadCommit:   "persist123",
		StagedStat:   "1 file changed",
		UnstagedStat: "",
		Command:      "",
	}
	result := CachedResult{
		GateType: "vet",
		Command:  "go vet ./...",
		Passed:   true,
		ExitCode: 0,
		Stdout:   "vet ok",
		Stderr:   "",
	}

	if err := c1.Put(key, "vet", result); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Create a new cache instance from the same directory — it should load from disk
	c2 := New(dir, DefaultTTL)

	got, ok := c2.Get(key, "vet")
	if !ok {
		t.Fatal("expected cache hit after reload from disk, got miss")
	}
	if got.GateType != "vet" {
		t.Errorf("GateType mismatch after reload: got %q", got.GateType)
	}
	if !got.Passed {
		t.Error("expected Passed=true after reload")
	}
	if got.Stdout != "vet ok" {
		t.Errorf("Stdout mismatch after reload: got %q", got.Stdout)
	}
	if !got.FromCache {
		t.Error("expected FromCache=true after reload")
	}
}

func TestCache_MultipleGateTypes(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, DefaultTTL)

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
		if err := c.Put(key, g.gateType, r); err != nil {
			t.Fatalf("Put(%q) failed: %v", g.gateType, err)
		}
	}

	for _, g := range gates {
		got, ok := c.Get(key, g.gateType)
		if !ok {
			t.Errorf("expected hit for gate %q, got miss", g.gateType)
			continue
		}
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

	c := New(dir, DefaultTTL)
	key, err := c.BuildKey(dir)
	if err != nil {
		t.Fatalf("BuildKey failed: %v", err)
	}

	if key.HeadCommit == "" {
		t.Error("HeadCommit should not be empty")
	}
	if len(key.HeadCommit) != 40 {
		t.Errorf("HeadCommit should be a 40-char SHA, got %q (len=%d)", key.HeadCommit, len(key.HeadCommit))
	}

	// Build a second key from the same state — it should produce the same hash
	key2, err := c.BuildKey(dir)
	if err != nil {
		t.Fatalf("second BuildKey failed: %v", err)
	}
	if hashKey(key) != hashKey(key2) {
		t.Error("same repo state should produce same hash")
	}

	// Modify a file (unstaged) and verify the key changes
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}
	key3, err := c.BuildKey(dir)
	if err != nil {
		t.Fatalf("third BuildKey failed: %v", err)
	}
	if hashKey(key) == hashKey(key3) {
		t.Error("different repo state should produce different hash")
	}
}

func TestCache_CommandDifferentiation(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, DefaultTTL)

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
	result := CachedResult{GateType: "build", Passed: true}
	if err := c.Put(key1, "build", result); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if _, ok := c.Get(key2, "build"); ok {
		t.Error("expected cache miss for key2 (different Command), but got hit")
	}
	if _, ok := c.Get(key1, "build"); !ok {
		t.Error("expected cache hit for key1, got miss")
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

	c := New(dir, DefaultTTL)

	key1, err := c.BuildKeyForGate(dir, "go test ./...")
	if err != nil {
		t.Fatalf("BuildKeyForGate failed: %v", err)
	}
	if key1.Command != "go test ./..." {
		t.Errorf("Command field not set: got %q", key1.Command)
	}

	key2, err := c.BuildKeyForGate(dir, "go test -count=1 ./...")
	if err != nil {
		t.Fatalf("BuildKeyForGate (second) failed: %v", err)
	}
	if key2.Command != "go test -count=1 ./..." {
		t.Errorf("Command field not set: got %q", key2.Command)
	}

	// Different commands must produce different hashes
	if hashKey(key1) == hashKey(key2) {
		t.Error("different gate commands should produce different hashes")
	}

	// Same command must produce the same hash
	key3, err := c.BuildKeyForGate(dir, "go test ./...")
	if err != nil {
		t.Fatalf("BuildKeyForGate (third) failed: %v", err)
	}
	if hashKey(key1) != hashKey(key3) {
		t.Error("same gate command should produce the same hash")
	}
}
