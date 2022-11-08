package concurrentmap

import "sync"

type ConcurrentMap interface {
	// Store sets the value for a key.
	Store(key, value interface{})

	// Load returns the value stored in the map for a key, or nil if no
	Load(key interface{}) (value interface{}, ok bool)

	// LoadOrStore returns the existing value for the key if present.
	LoadOrStore(key, value interface{}) (actual interface{}, loaded bool)

	// LoadAndDelete deletes the value for a key, returning the previous value if any.
	LoadAndDelete(key interface{}) (value interface{}, loaded bool)

	// Delete deletes the value for a key.
	Delete(key interface{})

	// Range calls f sequentially for each key and value present in the map.
	Range(f func(key, value interface{}) bool)

	// Clear deletes all elements from the map.
	Clear()

	// Len returns the number of elements within the map.
	Len() int

	// Keys returns all keys in the map.
	Keys() []interface{}
}

type concurrentMap struct {
	// The map to store the data
	m map[interface{}]interface{}

	// The lock to protect the map
	lock sync.RWMutex
}

func (m *concurrentMap) Store(key, value interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.m[key] = value
}

func (m *concurrentMap) Load(key interface{}) (value interface{}, ok bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	value, ok = m.m[key]

	return
}

func (m *concurrentMap) LoadOrStore(key, value interface{}) (actual interface{}, loaded bool) {
	m.lock.Lock()
	defer m.lock.Unlock()

	actual, loaded = m.m[key]
	if !loaded {
		m.m[key] = value
		actual = value
	}

	return
}

func (m *concurrentMap) LoadAndDelete(key interface{}) (value interface{}, loaded bool) {
	m.lock.Lock()
	defer m.lock.Unlock()

	value, loaded = m.m[key]
	if loaded {
		delete(m.m, key)
	}

	return
}

func (m *concurrentMap) Delete(key interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.m, key)
}

func (m *concurrentMap) Range(f func(key, value interface{}) bool) {
	keys := m.Keys()

	for _, key := range keys {
		load, _ := m.Load(key)

		if !f(key, load) {
			break
		}
	}
}

func (m *concurrentMap) Clear() {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.m = make(map[interface{}]interface{})
}

func (m *concurrentMap) Len() int {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return len(m.m)
}

func (m *concurrentMap) Keys() []interface{} {
	m.lock.RLock()
	defer m.lock.RUnlock()

	keys := make([]interface{}, len(m.m))
	i := 0

	for key := range m.m {
		keys[i] = key
		i++
	}

	return keys
}

func NewConcurrentMap() ConcurrentMap {
	return &concurrentMap{
		m:    make(map[interface{}]interface{}),
		lock: sync.RWMutex{},
	}
}
