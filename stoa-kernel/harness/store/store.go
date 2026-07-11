// Package store is the event_harness's own model-provider config: which models the
// orchestrator can drive, and where their keys live. This is the config that was removed
// from stag (the gate holds no keys) — it belongs HERE, on the orchestrator side.
// A JSON file, not SQLite: the orchestrator's config is small and human-editable.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
)

// Model is a connected model provider. A key is supplied directly (APIKey, dev) or by
// naming an env var (APIKeyEnv); APIKey wins. The API never echoes a stored key.
type Model struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"` // "claude" | "openai"
	BaseURL   string `json:"baseUrl"`
	Model     string `json:"model"`
	APIKey    string `json:"apiKey,omitempty"`
	APIKeyEnv string `json:"apiKeyEnv,omitempty"`
}

// Key resolves the usable secret: stored directly, else the named env var.
func (m Model) Key() string {
	if m.APIKey != "" {
		return m.APIKey
	}
	if m.APIKeyEnv != "" {
		return os.Getenv(m.APIKeyEnv)
	}
	return ""
}

// Store is a JSON-file-backed set of models, keyed by name.
type Store struct {
	path string
	mu   sync.Mutex
}

func Open(path string) *Store { return &Store{path: path} }

func (s *Store) load() ([]Model, error) {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ms []Model
	if len(b) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(b, &ms); err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	return ms, nil
}

func (s *Store) save(ms []Model) error {
	sort.Slice(ms, func(i, j int) bool { return ms[i].Name < ms[j].Name })
	b, err := json.MarshalIndent(ms, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o600) // 0600: it may hold keys
}

func (s *Store) List() ([]Model, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Store) Get(name string) (Model, bool, error) {
	ms, err := s.List()
	if err != nil {
		return Model{}, false, err
	}
	for _, m := range ms {
		if m.Name == name {
			return m, true, nil
		}
	}
	return Model{}, false, nil
}

// Put upserts by name. An empty APIKey preserves the existing stored key (edit without
// re-entering it).
func (s *Store) Put(m Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ms, err := s.load()
	if err != nil {
		return err
	}
	replaced := false
	for i := range ms {
		if ms[i].Name == m.Name {
			if m.APIKey == "" {
				m.APIKey = ms[i].APIKey // preserve
			}
			ms[i] = m
			replaced = true
			break
		}
	}
	if !replaced {
		ms = append(ms, m)
	}
	return s.save(ms)
}

func (s *Store) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ms, err := s.load()
	if err != nil {
		return err
	}
	out := ms[:0]
	for _, m := range ms {
		if m.Name != name {
			out = append(out, m)
		}
	}
	return s.save(out)
}
