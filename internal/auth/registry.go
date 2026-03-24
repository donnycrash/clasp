package auth

import (
	"fmt"
	"sort"
	"sync"
)

var (
	mu        sync.RWMutex
	providers = map[string]func() Provider{}
)

// Register adds a provider factory to the global registry. It is typically
// called from an init() function in the provider's source file.
func Register(name string, factory func() Provider) {
	mu.Lock()
	defer mu.Unlock()
	providers[name] = factory
}

// Get returns a new instance of the named provider.
func Get(name string) (Provider, error) {
	mu.RLock()
	defer mu.RUnlock()

	factory, ok := providers[name]
	if !ok {
		return nil, fmt.Errorf("auth: unknown provider %q", name)
	}
	return factory(), nil
}

// List returns the names of all registered providers in sorted order.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
