package xcollection

import "sync"

type MUSlice[V any] struct {
	store []V
	mu    *sync.RWMutex
}

func NewMUSlice[V any]() *MUSlice[V] {
	return &MUSlice[V]{
		store: make([]V, 0),
		mu:    &sync.RWMutex{},
	}
}

func (m *MUSlice[V]) Add(value V) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.store = append(m.store, value)
}

func (m *MUSlice[V]) GetLast(n int) []V {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if n > len(m.store) {
		return m.store
	}

	return m.store[len(m.store)-n:]
}

func (m *MUSlice[V]) GetFirst(n int) []V {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if n > len(m.store) {
		return m.store
	}

	return m.store[:len(m.store)-n]
}
