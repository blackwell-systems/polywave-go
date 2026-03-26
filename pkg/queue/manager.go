package queue

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"gopkg.in/yaml.v3"
)

// Manager manages the IMPL queue directory (docs/IMPL/queue/).
type Manager struct {
	repoPath string
}

// NewManager creates a new Manager rooted at the given repoPath.
func NewManager(repoPath string) *Manager {
	return &Manager{repoPath: repoPath}
}

// queueDir returns the absolute path to docs/IMPL/queue/.
func (m *Manager) queueDir() string {
	return protocol.IMPLQueueDir(m.repoPath)
}

// completeDir returns the absolute path to docs/IMPL/complete/.
func (m *Manager) completeDir() string {
	return protocol.IMPLCompleteDir(m.repoPath)
}

// slugFromTitle generates a URL-safe slug from a title string.
// Converts to lowercase, replaces spaces/non-alphanumeric with hyphens,
// collapses multiple hyphens, and trims leading/trailing hyphens.
func slugFromTitle(title string) string {
	s := strings.ToLower(title)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "item"
	}
	return s
}

// Add writes item as YAML to docs/IMPL/queue/{priority:03d}-{slug}.yaml.
// Auto-generates slug from title if item.Slug is empty.
// Sets status to "queued" if empty.
func (m *Manager) Add(item Item) error {
	if item.Slug == "" {
		item.Slug = slugFromTitle(item.Title)
	}
	if item.Status == "" {
		item.Status = "queued"
	}

	dir := m.queueDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("queue.Manager.Add: create queue dir: %w", err)
	}

	filename := fmt.Sprintf("%03d-%s.yaml", item.Priority, item.Slug)
	path := filepath.Join(dir, filename)

	if err := protocol.SaveYAML(path, &item); err != nil {
		return fmt.Errorf("queue.Manager.Add: %w", err)
	}
	return nil
}

// List reads all YAML files in docs/IMPL/queue/, sorts by priority (ascending),
// and returns the slice. FilePath is populated on each item.
func (m *Manager) List() ([]Item, error) {
	dir := m.queueDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Item{}, nil
		}
		return nil, fmt.Errorf("queue.Manager.List: read dir: %w", err)
	}

	var items []Item
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		path := filepath.Join(dir, name)
		item, err := protocol.LoadYAML[Item](path)
		if err != nil {
			return nil, fmt.Errorf("queue.Manager.List: %w", err)
		}
		item.FilePath = path
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Priority < items[j].Priority
	})

	return items, nil
}

// implSlug is a minimal struct for extracting feature_slug from IMPL YAML files.
type implSlug struct {
	FeatureSlug string `yaml:"feature_slug"`
}

// completedSlugs returns the set of slugs that are "complete" by scanning
// both the queue directory and docs/IMPL/complete/.
func (m *Manager) completedSlugs() (map[string]bool, error) {
	completed := make(map[string]bool)

	// Check queue items for complete status
	queueItems, err := m.List()
	if err != nil {
		return nil, err
	}
	for _, item := range queueItems {
		if item.Status == "complete" && item.Slug != "" {
			completed[item.Slug] = true
		}
	}

	// Check docs/IMPL/complete/ directory for IMPL YAML files.
	// IMPL files use feature_slug (not slug), so we extract that field.
	// We also derive slug from filename pattern IMPL-{slug}.yaml as fallback.
	completeDir := m.completeDir()
	entries, err := os.ReadDir(completeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return completed, nil
		}
		return nil, fmt.Errorf("queue.Manager.completedSlugs: read complete dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}

		// Try to extract feature_slug from file contents
		path := filepath.Join(completeDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		// In-memory unmarshal: data already read above; cannot use LoadYAML (no file path).
		var is implSlug
		if err := yaml.Unmarshal(data, &is); err == nil && is.FeatureSlug != "" {
			completed[is.FeatureSlug] = true
			continue
		}

		// Fallback: derive slug from filename IMPL-{slug}.yaml
		base := strings.TrimSuffix(name, ".yaml")
		if strings.HasPrefix(base, "IMPL-") {
			completed[strings.TrimPrefix(base, "IMPL-")] = true
		}
	}

	return completed, nil
}

// Next returns the highest-priority (lowest number) item whose status is
// "queued" AND all depends_on slugs have status "complete". Returns nil if
// no eligible item exists.
func (m *Manager) Next() (*Item, error) {
	items, err := m.List()
	if err != nil {
		return nil, err
	}

	completed, err := m.completedSlugs()
	if err != nil {
		return nil, err
	}

	for i := range items {
		item := &items[i]
		if item.Status != "queued" {
			continue
		}
		eligible := true
		for _, dep := range item.DependsOn {
			if !completed[dep] {
				eligible = false
				break
			}
		}
		if eligible {
			return item, nil
		}
	}
	return nil, nil
}

// UpdateStatus updates the status field in the queue item's YAML file
// identified by slug.
func (m *Manager) UpdateStatus(slug string, status string) error {
	dir := m.queueDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("queue.Manager.UpdateStatus: read dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		path := filepath.Join(dir, name)
		item, err := protocol.LoadYAML[Item](path)
		if err != nil {
			return fmt.Errorf("queue.Manager.UpdateStatus: %w", err)
		}
		if item.Slug == slug {
			item.Status = status
			if err := protocol.SaveYAML(path, &item); err != nil {
				return fmt.Errorf("queue.Manager.UpdateStatus: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("queue.Manager.UpdateStatus: slug %q not found in queue", slug)
}
