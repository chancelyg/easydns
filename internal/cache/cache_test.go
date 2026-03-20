package cache

import (
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		limit   int
		wantErr bool
	}{
		{"valid limit", 100, false},
		{"zero limit", 0, true},
		{"negative limit", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.limit)
			if (err != nil) != tt.wantErr {
				t.Errorf("New(%d) error = %v, wantErr %v", tt.limit, err, tt.wantErr)
			}
		})
	}
}

func TestDNSCache_AddAndGet(t *testing.T) {
	cache, err := New(10)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	cache.Add("key1", "value1")

	val, found := cache.Get("key1")
	if !found {
		t.Fatal("expected to find key1")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %v", val)
	}

	_, found = cache.Get("nonexistent")
	if found {
		t.Error("expected not to find nonexistent key")
	}
}

func TestDNSCache_Remove(t *testing.T) {
	cache, err := New(10)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	cache.Add("key1", "value1")
	if _, found := cache.Get("key1"); !found {
		t.Fatal("expected to find key1 before removal")
	}

	cache.Remove("key1")
	if _, found := cache.Get("key1"); found {
		t.Error("expected key1 to be removed")
	}
}

func TestDNSCache_LRUEviction(t *testing.T) {
	cache, err := New(3)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	cache.Add("key1", "value1")
	cache.Add("key2", "value2")
	cache.Add("key3", "value3")

	for _, key := range []string{"key1", "key2", "key3"} {
		if _, found := cache.Get(key); !found {
			t.Errorf("expected to find %s", key)
		}
	}

	cache.Add("key4", "value4")

	if _, found := cache.Get("key1"); found {
		t.Error("expected key1 to be evicted")
	}

	for _, key := range []string{"key2", "key3", "key4"} {
		if _, found := cache.Get(key); !found {
			t.Errorf("expected to find %s", key)
		}
	}
}

func TestDNSCache_ConcurrentAccess(t *testing.T) {
	cache, err := New(100)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := (id*100+j)%1000 + 1
				cache.Add(key, key)
				cache.Get(key)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
