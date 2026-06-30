package state

import (
	"sort"
	"strconv"
	"sync"
	"time"
)

type UPS struct {
	Name        string
	Description string
	Vars        map[string]string
	UpdatedAt   time.Time
}

type Store struct {
	mu  sync.RWMutex
	ups map[string]*UPS
}

func New() *Store { return &Store{ups: map[string]*UPS{}} }

func (s *Store) Upsert(name, desc string, vars map[string]string) {
	cp := map[string]string{}
	for k, v := range vars {
		cp[k] = v
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ups[name] = &UPS{Name: name, Description: desc, Vars: cp, UpdatedAt: time.Now()}
}

func (s *Store) Names() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.ups))
	for n := range s.ups {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (s *Store) Get(name string) (*UPS, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.ups[name]
	if !ok {
		return nil, false
	}
	cp := &UPS{Name: u.Name, Description: u.Description, UpdatedAt: u.UpdatedAt, Vars: map[string]string{}}
	for k, v := range u.Vars {
		cp.Vars[k] = v
	}
	return cp, true
}

func (s *Store) Snapshot() []UPS {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.ups))
	for name := range s.ups {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]UPS, 0, len(names))
	for _, name := range names {
		u := s.ups[name]
		cp := UPS{
			Name:        u.Name,
			Description: u.Description,
			UpdatedAt:   u.UpdatedAt,
			Vars:        map[string]string{},
		}
		for k, v := range u.Vars {
			cp.Vars[k] = v
		}
		out = append(out, cp)
	}
	return out
}

func FormatInt(n int) string { return strconv.Itoa(n) }
