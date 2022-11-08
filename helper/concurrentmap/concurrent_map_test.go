package concurrentmap

import (
	"testing"
)

func TestConcurrentMap(t *testing.T) {
	cmap := NewConcurrentMap()

	const key = "key"

	const value = "value"

	// test store
	cmap.Store(key, value)

	if v, ok := cmap.Load(key); !ok || v != value {
		t.Error("Store() failed")
	}

	// test load or store
	if v, ok := cmap.LoadOrStore(key, "value2"); !ok || v != value {
		t.Error("LoadOrStore() failed")
	}

	// test load and delete
	if v, ok := cmap.LoadAndDelete(key); !ok || v != value {
		t.Error("LoadAndDelete() failed")
	}

	// test delete
	cmap.Store(key, value)
	cmap.Delete(key)

	if v, ok := cmap.Load(key); ok && v != nil {
		t.Error("Delete() failed")
	}

	// test range
	cmap.Store("key1", "value1")
	cmap.Store("key2", "value2")
	cmap.Store("key3", "value3")

	cmap.Range(func(key, value interface{}) bool {
		if key == "key1" && value != "value1" {
			t.Error("Range() failed")
		}
		if key == "key2" && value != "value2" {
			t.Error("Range() failed")
		}
		if key == "key3" && value != "value3" {
			t.Error("Range() failed")
		}

		// test re-entrant deadlock
		k, _ := key.(string)
		v, _ := value.(string)
		cmap.Store(k+"-re", v+"-re")

		return true
	})

	// test clear
	cmap.Clear()

	if v, ok := cmap.Load("key1"); ok && v != nil {
		t.Error("clear failed")
	}

	// test len
	cmap.Store("key1", "value1")
	cmap.Store("key2", "value2")
	cmap.Store("key3", "value3")

	if cmap.Len() != 3 {
		t.Error("Len() failed")
	}

	// test keys
	keys := cmap.Keys()
	if len(keys) != 3 {
		t.Error("Keys() failed")
	}
}
