package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	mu   sync.RWMutex
	data map[string]map[string]any
	dir  string
}

func NewStore(dir string) *Store {
	_ = os.MkdirAll(dir, 0o755)
	return &Store{
		data: make(map[string]map[string]any),
		dir:  dir,
	}
}

// --------------------
// Public API
// --------------------

// Get liefert einen Wert und lädt den Namespace bei Bedarf
func (s *Store) Get(ns string, key string) (any, bool) {
	s.ensureNamespaceLoaded(ns)

	s.mu.RLock()
	defer s.mu.RUnlock()

	nsMap := s.data[ns]
	v, ok := nsMap[key]
	return v, ok
}

// Set speichert einen Wert, lädt bei Bedarf den Namespace und schreibt die Datei zurück
func (s *Store) Set(ns string, key string, value any) error {
	s.ensureNamespaceLoaded(ns)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[ns]; !ok {
		s.data[ns] = make(map[string]any)
	}
	s.data[ns][key] = value
	return s.saveNamespace(ns)
}

// Cleanup removes the entire storage directory and all its contents
func (s *Store) Cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear in-memory data
	s.data = make(map[string]map[string]any)

	// Remove the storage directory
	return os.RemoveAll(s.dir)
}

// --------------------
// Internal helpers
// --------------------

// ensureNamespaceLoaded sorgt dafür, dass ein Namespace einmalig von Disk geladen wird
func (s *Store) ensureNamespaceLoaded(ns string) {
	s.mu.RLock()
	_, ok := s.data[ns]
	s.mu.RUnlock()
	if ok {
		return // schon im Speicher
	}

	// Jetzt exklusiv prüfen und laden
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[ns]; ok {
		return // anderer Goroutine war schneller
	}

	m, err := s.loadNamespace(ns)
	if err != nil {
		// Datei nicht gefunden → leerer Namespace
		m = make(map[string]any)
	}
	s.data[ns] = m
}

func (s *Store) saveNamespace(ns string) error {
	path := filepath.Join(s.dir, string(ns)+".json")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(s.data[ns])
}

func (s *Store) loadNamespace(ns string) (map[string]any, error) {
	path := filepath.Join(s.dir, string(ns)+".json")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var m map[string]any
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}
