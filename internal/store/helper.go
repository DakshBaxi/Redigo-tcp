package store

// ensureCapacity is called before inserting a new key.
// If maxKeys > 0 and we're at capacity, it evicts one key (random for now).
func (s *Store) ensureCapacity() {
	if s.maxKeys <= 0 {
		return
	}
	if len(s.data) < s.maxKeys {
		return
	}

	// Find LRU (smallest LastAccess)
	var lruKey string
	var lruTime int64
	first := true

	// Simple random eviction: pick the first key in map iteration.
	for k,e := range s.data {
		if first || e.LastAccess < lruTime {
			lruKey = k
			lruTime = e.LastAccess
			first = false
		}
	}
		if !first {
		delete(s.data, lruKey)
		s.evictions++
	}
}
