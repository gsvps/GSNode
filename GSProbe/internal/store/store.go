package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"gsprobe/internal/model"
)

type Store struct {
	root string
	mu   sync.RWMutex
}

func New(root string) (*Store, error) {
	p := filepath.Join(root, "reports")
	if err := os.MkdirAll(p, 0755); err != nil {
		return nil, err
	}
	return &Store{root: p}, nil
}
func (s *Store) DataRoot() string { return filepath.Dir(s.root) }
func (s *Store) Save(r model.Report) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, e := json.MarshalIndent(r, "", "  ")
	if e != nil {
		return e
	}
	tmp := filepath.Join(s.root, r.ID+".tmp")
	dst := filepath.Join(s.root, r.ID+".json")
	if e = os.WriteFile(tmp, b, 0644); e != nil {
		return e
	}
	return os.Rename(tmp, dst)
}
func (s *Store) Get(id string) (model.Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var r model.Report
	b, e := os.ReadFile(filepath.Join(s.root, filepath.Base(id)+".json"))
	if e != nil {
		return r, e
	}
	e = json.Unmarshal(b, &r)
	return r, e
}
func (s *Store) List() ([]model.Report, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []model.Report{}
	for _, x := range entries {
		if x.IsDir() || filepath.Ext(x.Name()) != ".json" {
			continue
		}
		var r model.Report
		b, er := os.ReadFile(filepath.Join(s.root, x.Name()))
		if er == nil && json.Unmarshal(b, &r) == nil {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out, nil
}
func (s *Store) Latest() (model.Report, error) {
	rows, e := s.List()
	if e != nil {
		return model.Report{}, e
	}
	if len(rows) == 0 {
		return model.Report{}, errors.New("no reports")
	}
	return rows[0], nil
}
