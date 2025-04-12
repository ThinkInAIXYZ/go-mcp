package pkg

import "sync"

type SyncMap[V any] struct {
	m sync.Map
}

func (m *SyncMap[V]) Delete(key string) {
	m.m.Delete(key)
}

func (m *SyncMap[V]) Load(key string) (value V, ok bool) {
	v, ok := m.m.Load(key)
	if !ok {
		return value, ok
	}
	return v.(V), ok
}

func (m *SyncMap[V]) LoadAndDelete(key string) (value V, loaded bool) {
	v, loaded := m.m.LoadAndDelete(key)
	if !loaded {
		return value, loaded
	}
	return v.(V), loaded
}

func (m *SyncMap[V]) LoadOrStore(key string, value V) (actual V, loaded bool) {
	a, loaded := m.m.LoadOrStore(key, value)
	return a.(V), loaded
}

func (m *SyncMap[V]) Range(f func(key string, value V) bool) {
	m.m.Range(func(key, value any) bool { return f(key.(string), value.(V)) })
}

func (m *SyncMap[V]) Store(key string, value V) {
	m.m.Store(key, value)
}

func (m *SyncMap[V]) IsEmpty() bool {
	isEmpty := true
	m.m.Range(func(_, _ any) bool {
		isEmpty = false
		return false
	})
	return isEmpty
}
