package mempool

import "sync"

// Manager holds a collection of named Mempools, typically one per active
// stream. Safe for concurrent use.
type Manager struct {
	mu    sync.RWMutex
	pools map[string]*Mempool
}

// NewManager creates a Manager ready to use.
func NewManager() *Manager {
	return &Manager{
		pools: map[string]*Mempool{},
	}
}

// Get returns the Mempool for the given name, or nil if it doesn't exist.
func (m *Manager) Get(name string) *Mempool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pools[name]
}

// GetOrCreate returns the Mempool for the given name, creating it if needed.
func (m *Manager) GetOrCreate(name string) *Mempool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.pools[name]; ok {
		return p
	}
	p := New()
	m.pools[name] = p
	return p
}

// Remove closes and removes the Mempool for the given name.
func (m *Manager) Remove(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.pools[name]; ok {
		p.Close()
		delete(m.pools, name)
	}
}

// Names returns the names of all active mempools.
func (m *Manager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.pools))
	for name := range m.pools {
		names = append(names, name)
	}
	return names
}
