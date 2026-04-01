package queue

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"gopkg.in/yaml.v3"
)

func TestAdd_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	item := Item{
		Title:              "Dark Mode",
		Priority:           1,
		Slug:               "dark-mode",
		FeatureDescription: "Add dark mode support",
	}
	res := mgr.Add(item)
	if res.IsFatal() {
		t.Fatalf("Add: %v", res.Errors[0].Message)
	}

	path := filepath.Join(dir, "docs", "IMPL", "queue", "001-dark-mode.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}

	var loaded Item
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.Title != "Dark Mode" {
		t.Errorf("title = %q, want %q", loaded.Title, "Dark Mode")
	}
	if loaded.Status != "queued" {
		t.Errorf("status = %q, want %q", loaded.Status, "queued")
	}
	if loaded.Slug != "dark-mode" {
		t.Errorf("slug = %q, want %q", loaded.Slug, "dark-mode")
	}
}

func TestAdd_SlugInResult(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	item := Item{
		Title:    "My Feature",
		Priority: 3,
	}
	res := mgr.Add(item)
	if res.IsFatal() {
		t.Fatalf("Add: %v", res.Errors[0].Message)
	}

	d := res.GetData()
	if d.Slug != "my-feature" {
		t.Errorf("AddData.Slug = %q, want %q", d.Slug, "my-feature")
	}
	if d.FilePath == "" {
		t.Error("AddData.FilePath should not be empty")
	}
}

func TestAdd_AutoSlug(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	item := Item{
		Title:    "My Cool Feature!",
		Priority: 5,
	}
	res := mgr.Add(item)
	if res.IsFatal() {
		t.Fatalf("Add: %v", res.Errors[0].Message)
	}

	path := filepath.Join(dir, "docs", "IMPL", "queue", "005-my-cool-feature.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var loaded Item
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.Slug != "my-cool-feature" {
		t.Errorf("slug = %q, want %q", loaded.Slug, "my-cool-feature")
	}
}

func TestList_SortsByPriority(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Add items out of order
	items := []Item{
		{Title: "Third", Priority: 3, Slug: "third"},
		{Title: "First", Priority: 1, Slug: "first"},
		{Title: "Second", Priority: 2, Slug: "second"},
	}
	for _, item := range items {
		res := mgr.Add(item)
		if res.IsFatal() {
			t.Fatalf("Add %q: %v", item.Title, res.Errors[0].Message)
		}
	}

	listRes := mgr.List()
	if listRes.IsFatal() {
		t.Fatalf("List: %v", listRes.Errors[0].Message)
	}
	listed := listRes.GetData().Items
	if len(listed) != 3 {
		t.Fatalf("len = %d, want 3", len(listed))
	}
	if listed[0].Title != "First" {
		t.Errorf("listed[0].Title = %q, want %q", listed[0].Title, "First")
	}
	if listed[1].Title != "Second" {
		t.Errorf("listed[1].Title = %q, want %q", listed[1].Title, "Second")
	}
	if listed[2].Title != "Third" {
		t.Errorf("listed[2].Title = %q, want %q", listed[2].Title, "Third")
	}

	// Verify FilePath is populated
	for _, item := range listed {
		if item.FilePath == "" {
			t.Errorf("FilePath is empty for %q", item.Title)
		}
	}
}

func TestList_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	listRes := mgr.List()
	if listRes.IsFatal() {
		t.Fatalf("List: %v", listRes.Errors[0].Message)
	}
	listed := listRes.GetData().Items
	if len(listed) != 0 {
		t.Errorf("len = %d, want 0", len(listed))
	}
}

func TestNext_ReturnsHighestPriority(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	items := []Item{
		{Title: "Low", Priority: 10, Slug: "low"},
		{Title: "High", Priority: 1, Slug: "high"},
		{Title: "Mid", Priority: 5, Slug: "mid"},
	}
	for _, item := range items {
		res := mgr.Add(item)
		if res.IsFatal() {
			t.Fatalf("Add: %v", res.Errors[0].Message)
		}
	}

	nextRes := mgr.Next()
	if nextRes.IsFatal() {
		t.Fatalf("Next: %v", nextRes.Errors[0].Message)
	}
	next := nextRes.GetData()
	if next.Slug != "high" {
		t.Errorf("slug = %q, want %q", next.Slug, "high")
	}
}

func TestNext_QueueEmpty(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	nextRes := mgr.Next()
	if !nextRes.IsFatal() {
		t.Fatal("expected Fatal result when queue is empty")
	}
	if len(nextRes.Errors) == 0 {
		t.Fatal("expected errors in Fatal result")
	}
	if nextRes.Errors[0].Code != result.CodeQueueEmpty {
		t.Errorf("error code = %q, want %q", nextRes.Errors[0].Code, result.CodeQueueEmpty)
	}
}

func TestNext_SkipsDependencies(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Item A has no deps (priority 2)
	// Item B depends on "prereq" which is not complete (priority 1)
	items := []Item{
		{Title: "B", Priority: 1, Slug: "b", DependsOn: []string{"prereq"}},
		{Title: "A", Priority: 2, Slug: "a"},
	}
	for _, item := range items {
		res := mgr.Add(item)
		if res.IsFatal() {
			t.Fatalf("Add: %v", res.Errors[0].Message)
		}
	}

	nextRes := mgr.Next()
	if nextRes.IsFatal() {
		t.Fatalf("Next: %v", nextRes.Errors[0].Message)
	}
	// B has unmet dep, so A should be returned even though B has higher priority
	next := nextRes.GetData()
	if next.Slug != "a" {
		t.Errorf("slug = %q, want %q", next.Slug, "a")
	}
}

func TestNext_NoneEligible(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// All items have unmet dependencies
	items := []Item{
		{Title: "A", Priority: 1, Slug: "a", DependsOn: []string{"missing"}},
		{Title: "B", Priority: 2, Slug: "b", DependsOn: []string{"also-missing"}},
	}
	for _, item := range items {
		res := mgr.Add(item)
		if res.IsFatal() {
			t.Fatalf("Add: %v", res.Errors[0].Message)
		}
	}

	nextRes := mgr.Next()
	if !nextRes.IsFatal() {
		t.Errorf("expected Fatal when no items eligible, got Code=%q", nextRes.Code)
	}
	if nextRes.Errors[0].Code != result.CodeQueueEmpty {
		t.Errorf("error code = %q, want %q", nextRes.Errors[0].Code, result.CodeQueueEmpty)
	}
}

func TestNext_DependencyMetByCompleteQueueItem(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// prereq is complete in the queue
	items := []Item{
		{Title: "Prereq", Priority: 1, Slug: "prereq", Status: "complete"},
		{Title: "Dependent", Priority: 2, Slug: "dependent", DependsOn: []string{"prereq"}},
	}
	for _, item := range items {
		res := mgr.Add(item)
		if res.IsFatal() {
			t.Fatalf("Add: %v", res.Errors[0].Message)
		}
	}

	nextRes := mgr.Next()
	if nextRes.IsFatal() {
		t.Fatalf("Next: %v", nextRes.Errors[0].Message)
	}
	next := nextRes.GetData()
	if next.Slug != "dependent" {
		t.Errorf("slug = %q, want %q", next.Slug, "dependent")
	}
}

func TestNext_DependencyMetByCompleteDir(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Create an IMPL file in docs/IMPL/complete/ with feature_slug matching the dep
	completeDir := filepath.Join(dir, "docs", "IMPL", "complete")
	if err := os.MkdirAll(completeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	implContent := "feature_slug: prereq\ntitle: Prereq Feature\nstate: COMPLETE\n"
	if err := os.WriteFile(filepath.Join(completeDir, "IMPL-prereq.yaml"), []byte(implContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Add item that depends on "prereq"
	item := Item{Title: "Dependent", Priority: 1, Slug: "dependent", DependsOn: []string{"prereq"}}
	res := mgr.Add(item)
	if res.IsFatal() {
		t.Fatalf("Add: %v", res.Errors[0].Message)
	}

	nextRes := mgr.Next()
	if nextRes.IsFatal() {
		t.Fatalf("Next: %v", nextRes.Errors[0].Message)
	}
	next := nextRes.GetData()
	if next.Slug != "dependent" {
		t.Errorf("slug = %q, want %q", next.Slug, "dependent")
	}
}

func TestUpdateStatus(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	item := Item{Title: "Test", Priority: 1, Slug: "test"}
	if addRes := mgr.Add(item); addRes.IsFatal() {
		t.Fatalf("Add: %v", addRes.Errors[0].Message)
	}

	updateRes := mgr.UpdateStatus("test", "in_progress")
	if updateRes.IsFatal() {
		t.Fatalf("UpdateStatus: %v", updateRes.Errors[0].Message)
	}

	// Verify returned data
	d := updateRes.GetData()
	if d.Slug != "test" {
		t.Errorf("UpdateStatusData.Slug = %q, want %q", d.Slug, "test")
	}
	if d.NewStatus != "in_progress" {
		t.Errorf("UpdateStatusData.NewStatus = %q, want %q", d.NewStatus, "in_progress")
	}

	listRes := mgr.List()
	if listRes.IsFatal() {
		t.Fatalf("List: %v", listRes.Errors[0].Message)
	}
	listed := listRes.GetData().Items
	if len(listed) != 1 {
		t.Fatalf("len = %d, want 1", len(listed))
	}
	if listed[0].Status != "in_progress" {
		t.Errorf("status = %q, want %q", listed[0].Status, "in_progress")
	}
}

func TestUpdateStatus_ReturnsCorrectNewStatus(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	item := Item{Title: "Status Test", Priority: 1, Slug: "status-test"}
	if addRes := mgr.Add(item); addRes.IsFatal() {
		t.Fatalf("Add: %v", addRes.Errors[0].Message)
	}

	updateRes := mgr.UpdateStatus("status-test", "complete")
	if updateRes.IsFatal() {
		t.Fatalf("UpdateStatus: %v", updateRes.Errors[0].Message)
	}

	d := updateRes.GetData()
	if d.NewStatus != "complete" {
		t.Errorf("UpdateStatusData.NewStatus = %q, want %q", d.NewStatus, "complete")
	}
}

func TestUpdateStatus_NotFound(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Create queue dir but no items
	queueDir := filepath.Join(dir, "docs", "IMPL", "queue")
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		t.Fatal(err)
	}

	updateRes := mgr.UpdateStatus("nonexistent", "complete")
	if !updateRes.IsFatal() {
		t.Fatal("expected Fatal result for nonexistent slug")
	}
	if updateRes.Errors[0].Code != result.CodeQueueStatusUpdateFailed {
		t.Errorf("error code = %q, want %q", updateRes.Errors[0].Code, result.CodeQueueStatusUpdateFailed)
	}
}

func TestSlugFromTitle(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Dark Mode", "dark-mode"},
		{"My Cool Feature!", "my-cool-feature"},
		{"  spaces  everywhere  ", "spaces-everywhere"},
		{"UPPERCASE", "uppercase"},
		{"a---b", "a-b"},
		{"", "item"},
		{"!!!@@@", "item"},
	}
	for _, tt := range tests {
		got := slugFromTitle(tt.title)
		if got != tt.want {
			t.Errorf("slugFromTitle(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}
