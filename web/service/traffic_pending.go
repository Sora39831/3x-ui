package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type TrafficDelta struct {
	Kind      string `json:"kind"`
	InboundID int    `json:"inboundId"`
	Email     string `json:"email"`
	UpDelta   int64  `json:"upDelta"`
	DownDelta int64  `json:"downDelta"`
}

const (
	TrafficDeltaKindClient      = "client"
	TrafficDeltaKindInboundOnly = "inbound_only"
)

type TrafficPendingStore struct {
	path string
	mu   sync.Mutex
}

func NewTrafficPendingStore(path string) *TrafficPendingStore {
	return &TrafficPendingStore{path: path}
}

func (s *TrafficPendingStore) Load() ([]TrafficDelta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadUnlocked()
}

func (s *TrafficPendingStore) Save(deltas []TrafficDelta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveUnlocked(deltas)
}

func (s *TrafficPendingStore) Merge(newDeltas []TrafficDelta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.loadUnlocked()
	if err != nil {
		return err
	}

	index := map[string]int{}
	for i, delta := range current {
		index[deltaKey(delta.Kind, delta.InboundID, delta.Email)] = i
	}

	for _, delta := range newDeltas {
		if delta.Kind == "" {
			delta.Kind = TrafficDeltaKindClient
		}
		key := deltaKey(delta.Kind, delta.InboundID, delta.Email)
		if idx, ok := index[key]; ok {
			current[idx].UpDelta += delta.UpDelta
			current[idx].DownDelta += delta.DownDelta
			continue
		}
		index[key] = len(current)
		current = append(current, delta)
	}

	return s.saveUnlocked(current)
}

func (s *TrafficPendingStore) Take() ([]TrafficDelta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	deltas, err := s.loadUnlocked()
	if err != nil || len(deltas) == 0 {
		return deltas, err
	}
	if err := s.saveUnlocked([]TrafficDelta{}); err != nil {
		return nil, err
	}
	return deltas, nil
}

func (s *TrafficPendingStore) loadUnlocked() ([]TrafficDelta, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return []TrafficDelta{}, nil
	}
	if err != nil {
		return nil, err
	}

	var deltas []TrafficDelta
	if err := json.Unmarshal(data, &deltas); err != nil {
		return nil, err
	}
	if deltas == nil {
		return []TrafficDelta{}, nil
	}
	return deltas, nil
}

func (s *TrafficPendingStore) saveUnlocked(deltas []TrafficDelta) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(deltas, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func deltaKey(kind string, inboundID int, email string) string {
	return fmt.Sprintf("%s:%d:%s", kind, inboundID, email)
}
