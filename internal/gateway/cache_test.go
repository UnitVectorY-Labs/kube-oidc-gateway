package gateway

import (
	"testing"
	"time"
)

func TestCache(t *testing.T) {
	t.Run("Get from empty cache returns false", func(t *testing.T) {
		cache := NewCache(60 * time.Second)
		_, found := cache.Get("test-key")
		if found {
			t.Error("Expected cache miss for non-existent key")
		}
	})

	t.Run("Set and Get returns cached value", func(t *testing.T) {
		cache := NewCache(60 * time.Second)
		testData := []byte(`{"test": "data"}`)

		cache.Set("test-key", testData)

		result, found := cache.Get("test-key")
		if !found {
			t.Error("Expected cache hit after Set")
		}
		if string(result) != string(testData) {
			t.Errorf("Expected %s, got %s", testData, result)
		}
	})

	t.Run("Cache expires after TTL", func(t *testing.T) {
		cache := NewCache(100 * time.Millisecond)
		testData := []byte(`{"test": "data"}`)

		cache.Set("test-key", testData)

		// Should be cached immediately
		_, found := cache.Get("test-key")
		if !found {
			t.Error("Expected cache hit immediately after Set")
		}

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)

		// Should be expired
		_, found = cache.Get("test-key")
		if found {
			t.Error("Expected cache miss after TTL expiration")
		}
	})

	t.Run("Cache stores multiple keys separately", func(t *testing.T) {
		cache := NewCache(60 * time.Second)
		data1 := []byte(`{"key": "1"}`)
		data2 := []byte(`{"key": "2"}`)

		cache.Set("key1", data1)
		cache.Set("key2", data2)

		result1, _ := cache.Get("key1")
		result2, _ := cache.Get("key2")

		if string(result1) != string(data1) {
			t.Errorf("Key1: expected %s, got %s", data1, result1)
		}
		if string(result2) != string(data2) {
			t.Errorf("Key2: expected %s, got %s", data2, result2)
		}
	})

	t.Run("GetStale returns expired cache entries", func(t *testing.T) {
		cache := NewCache(100 * time.Millisecond)
		testData := []byte(`{"test": "stale"}`)

		cache.Set("test-key", testData)

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)

		// Regular Get should fail
		_, found := cache.Get("test-key")
		if found {
			t.Error("Expected cache miss after TTL expiration")
		}

		// GetStale should succeed
		result, found := cache.GetStale("test-key")
		if !found {
			t.Error("Expected GetStale to return expired entry")
		}
		if string(result) != string(testData) {
			t.Errorf("Expected %s, got %s", testData, result)
		}
	})

	t.Run("GetStale returns false for non-existent keys", func(t *testing.T) {
		cache := NewCache(60 * time.Second)
		_, found := cache.GetStale("non-existent")
		if found {
			t.Error("Expected GetStale to return false for non-existent key")
		}
	})
}
