package store

import (
	"strings"
	"sync"
	"time"
)

type Entry struct {
	Value     string
	ExpiresAt int64
	LastAccess int64
}

type Store struct {
	mu   sync.RWMutex
	data map[string]Entry
	maxKeys int // 0 means no limit
	evictions int64 // ccount for evicated keys
	reads  int64
	writes int64
}

// Stats returns basic stats for INFO command.
type Stats struct {
	Keys      int   `json:"keys"`
	MaxKeys   int   `json:"max_keys"`
	Evictions int64 `json:"evictions"`
	Reads     int64 `json:"reads"`
	Writes    int64 `json:"writes"`
}


func New() *Store {
	return &Store{
		data: make(map[string]Entry),
		maxKeys: 0, // no limit by default; we'll control via command
	}
}

// SetMaxKeys sets a soft limit on number of keys. 0 means no limit.
func (s *Store) SetMaxKeys(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxKeys = n
}

func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Stats{
		Keys:      len(s.data),
		MaxKeys:   s.maxKeys,
		Evictions: s.evictions,
		Reads:     s.reads,
		Writes:    s.writes,
	}
}

// set stores a va,lue without a TTL(no expiry)
func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()

	// If key is new, enforce capacity
	if _, exists := s.data[key]; !exists {
		s.ensureCapacity()
	}
	s.data[key] = Entry{Value: value, ExpiresAt: 0,LastAccess: now}
	s.writes++
}

// setwithttl sets key with ttl in seconds.
func (s *Store) Setwithttl(key, value string, ttlSeconds int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()

	if _, exists := s.data[key]; !exists {
		s.ensureCapacity()
	}

	var exp int64 = 0
	if ttlSeconds > 0 {
		exp = time.Now().Unix() + ttlSeconds
	}
	s.data[key] = Entry{Value: value, ExpiresAt: exp,LastAccess: now}
	s.writes++
}

// get returns a value if present and not expired
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()

	defer s.mu.RUnlock()
	e, ok := s.data[key]
	if !ok {
		s.reads++
		return "", false
	}

	// Check if expired (and has an expiry)
	if e.ExpiresAt != 0 && e.ExpiresAt < time.Now().Unix() {
		return "", false
	}
	e.LastAccess = time.Now().Unix()
	s.data[key] = e
	s.reads++
	return e.Value, true
}

// Del key if it exist and return whether it was removed.
func (s *Store) Del(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; ok {
		delete(s.data, key)
		s.writes++
		return true
	}
	return false
}

// Expire sets a new TTl for a key. Returns true if updaed
func (s *Store) Expires(key string, ttlSeconds int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e, ok := s.data[key]; ok {
		if ttlSeconds <= 0 {
			e.ExpiresAt = 0
		} else {
			e.ExpiresAt = time.Now().Unix() + ttlSeconds
		}
		s.data[key] = e
		s.writes++
		return true
	}
	return false
}

// TTL returns remaining time-to-live in seconds.
// -1 if key exists and has no TTL
// -2 if key does not exist or is expired
func (s *Store) TTL(key string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.data[key]
	if !ok {
		return -2
	}
	if e.ExpiresAt == 0 {
		return -1
	}
	if time.Now().Unix() > e.ExpiresAt {
		return -2
	}
	return e.ExpiresAt - time.Now().Unix()
}

// Cleanup expired removes expired keys
func (s *Store) CleanupExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for i, e := range s.data {
		if e.ExpiresAt != 0 && e.ExpiresAt < time.Now().Unix() {
			delete(s.data, i)
			removed++
			s.evictions++
		}
	}
	return removed
}

// keys return a snapshot of all keys(just for debugging)
func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]string, 0, len(s.data))
	for k := range s.data {
		res = append(res, k)
	}
	return res
}

// HelpText returns a small help message for the client.
func HelpText() string {
	lines := []string{
		"Supported commands (simple text protocol):",
		"  SET key value           - set value for key (no TTL)",
		"  SETEX key ttl value     - set value with TTL in seconds",
		"  GET key                 - get value for key",
		"  DEL key                 - delete key",
		"  EXISTS key              - check if key exists",
		"  TTL key                 - get remaining TTL (seconds)",
		"  INCR key                - increment integer value (init 0 if missing)",
		"  DECR key                - decrement integer value (init 0 if missing)",
		"  CONFIG MAXKEYS n        - set max allowed keys (0 = unlimited)",
		"  INFO                    - show basic stats (keys, evictions, reads, writes)",
		"  KEYS                    - list all keys",
		"  PING [msg]              - ping or echo message",
		"  HELP                    - show this help",
		"  QUIT                    - close connection",
	}
	return strings.Join(lines, "\n")
}
