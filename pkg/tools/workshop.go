package tools

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// DefaultWorkshop is the default implementation of the Workshop interface.
// It provides thread-safe tool registration and lookup with namespace filtering.
type DefaultWorkshop struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewWorkshop creates and returns a new empty Workshop.
func NewWorkshop() Workshop {
	return &DefaultWorkshop{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the workshop. Returns an error if a tool with the
// same name already exists.
func (w *DefaultWorkshop) Register(tool Tool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.tools[tool.Name]; exists {
		return fmt.Errorf("tool %q already registered", tool.Name)
	}

	w.tools[tool.Name] = tool
	return nil
}

// Get retrieves a tool by exact name. Returns the tool and true if found,
// or an empty Tool and false if not found.
func (w *DefaultWorkshop) Get(name string) (Tool, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	tool, ok := w.tools[name]
	return tool, ok
}

// All returns a slice of all registered tools, sorted by name for determinism.
func (w *DefaultWorkshop) All() []Tool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	result := make([]Tool, 0, len(w.tools))
	for _, tool := range w.tools {
		result = append(result, tool)
	}

	// Sort by name for deterministic ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// Namespace returns all tools whose names start with the given prefix,
// sorted by name for determinism. This enables namespace filtering for
// agent-specific tool subsets (e.g., "file:" for file tools).
func (w *DefaultWorkshop) Namespace(prefix string) []Tool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	result := make([]Tool, 0)
	for _, tool := range w.tools {
		if strings.HasPrefix(tool.Name, prefix) {
			result = append(result, tool)
		}
	}

	// Sort by name for deterministic ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}
