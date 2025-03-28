package pkg

import "sync"

// SyncMap 是对sync.Map的泛型包装，提供类型安全的操作
type SyncMap[K comparable, V any] struct {
	m sync.Map
}

// Delete 删除指定键的元素
func (m *SyncMap[K, V]) Delete(key K) {
	m.m.Delete(key)
}

// Load 获取指定键的值
func (m *SyncMap[K, V]) Load(key K) (value V, ok bool) {
	v, ok := m.m.Load(key)
	if !ok {
		return value, ok
	}
	return v.(V), ok
}

// LoadAndDelete 获取指定键的值并删除该键
func (m *SyncMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	v, loaded := m.m.LoadAndDelete(key)
	if !loaded {
		return value, loaded
	}
	return v.(V), loaded
}

// LoadOrStore 如果键存在则返回已有的值，否则存储提供的值
func (m *SyncMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	a, loaded := m.m.LoadOrStore(key, value)
	return a.(V), loaded
}

// Range 遍历所有键值对
func (m *SyncMap[K, V]) Range(f func(key K, value V) bool) {
	m.m.Range(func(key, value any) bool {
		return f(key.(K), value.(V))
	})
}

// Store 存储键值对
func (m *SyncMap[K, V]) Store(key K, value V) {
	m.m.Store(key, value)
}
