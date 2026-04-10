package xcollection

import "sync"

type MUMap[K comparable, V any] struct {
	store map[K]V
	mu    *sync.RWMutex
}

func NewMUMap[K comparable, V any]() *MUMap[K, V] {
	return &MUMap[K, V]{
		store: make(map[K]V),
		mu:    &sync.RWMutex{},
	}
}

func (m *MUMap[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	v, ok := m.store[key]
	return v, ok
}

func (m *MUMap[K, V]) Set(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.store[key] = value
}
