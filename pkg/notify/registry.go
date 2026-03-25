package notify

import (
	"fmt"
	"sort"
	"sync"
)

// AdapterFactory creates an Adapter from a string config map.
type AdapterFactory func(cfg map[string]string) (Adapter, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]AdapterFactory{}
)

// Register adds an adapter factory under the given name.
// It panics if name is already registered (fail-fast on duplicate init).
func Register(name string, factory AdapterFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, ok := registry[name]; ok {
		panic(fmt.Sprintf("notify: adapter %q already registered", name))
	}
	registry[name] = factory
}

// NewFromConfig looks up a registered factory by name and creates an adapter.
func NewFromConfig(name string, cfg map[string]string) (Adapter, error) {
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("notify: unknown adapter %q", name)
	}
	return factory(cfg)
}

// RegisteredNames returns a sorted list of all registered adapter names.
func RegisteredNames() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
