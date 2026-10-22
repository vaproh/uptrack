package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadState reads state.json. A missing file yields empty state.
func LoadState(path string) (*State, error) {
	s := &State{Apps: map[string]AppState{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}
	err = json.Unmarshal(raw, s)
	if err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	if s.Apps == nil {
		s.Apps = map[string]AppState{}
	}
	return s, nil
}

// SaveState writes state.json atomically.
func SaveState(path string, s *State) error {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	raw = append(raw, '\n')
	err = writeAtomic(path, raw, 0o644)
	if err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}
