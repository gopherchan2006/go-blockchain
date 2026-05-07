package main

import (
	"os"
	"sync"
	"time"
)

type debugEntry struct {
	Value     any       `json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type DebugStore struct {
	mu      sync.RWMutex
	entries map[string]debugEntry
}

var dbg = &DebugStore{entries: make(map[string]debugEntry)}

func DebugEnabled() bool {
	return os.Getenv("DEBUG") == "1"
}

func (d *DebugStore) Set(key string, value any) {
	if !DebugEnabled() {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries[key] = debugEntry{Value: value, UpdatedAt: time.Now()}
}

func (d *DebugStore) Del(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.entries, key)
}

func (d *DebugStore) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries = make(map[string]debugEntry)
}

func (d *DebugStore) snapshot() map[string]debugEntry {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make(map[string]debugEntry, len(d.entries))
	for k, v := range d.entries {
		out[k] = v
	}
	return out
}
