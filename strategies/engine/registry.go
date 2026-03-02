package engine

import (
	"fmt"
	"sort"
	"sync"
)

var (
	registryMu sync.RWMutex
	registry   = map[string]Strategy{}
)

func Register(s Strategy) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[s.ID()] = s
}

func MustGet(id string) (Strategy, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	s, ok := registry[id]
	if !ok {
		return nil, fmt.Errorf("strategy not found: %s", id)
	}
	return s, nil
}

func ListIDs() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
