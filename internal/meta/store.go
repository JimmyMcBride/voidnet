package meta

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

const SchemaVersion = 1

type State struct {
	SchemaVersion     int      `json:"schema_version"`
	UnlockedStarters  []string `json:"unlocked_starters"`
	UnlockedModifiers []string `json:"unlocked_modifiers"`
	UnlockedTraits    []string `json:"unlocked_traits"`
	Milestones        []string `json:"milestones"`
}

type Store struct {
	path string
}

func DefaultPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(root, "system-breakers", "meta.json"), nil
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func DefaultState() State {
	return normalize(State{
		SchemaVersion:     SchemaVersion,
		UnlockedStarters:  []string{"firewall"},
		UnlockedModifiers: []string{"single"},
		UnlockedTraits:    []string{"volatile", "persistent", "overclocked", "encrypted"},
		Milestones:        nil,
	})
}

func (s *Store) Load() (State, error) {
	if s.path == "" {
		return DefaultState(), nil
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultState(), nil
		}
		return State{}, fmt.Errorf("read meta state: %w", err)
	}

	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		return State{}, fmt.Errorf("decode meta state: %w", err)
	}

	return normalize(state), nil
}

func (s *Store) Save(state State) error {
	if s.path == "" {
		return nil
	}

	state = normalize(state)
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create meta dir: %w", err)
	}

	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode meta state: %w", err)
	}

	if err := os.WriteFile(s.path, raw, 0o644); err != nil {
		return fmt.Errorf("write meta state: %w", err)
	}
	return nil
}

func (s State) HasMilestone(id string) bool {
	return slices.Contains(s.Milestones, id)
}

func (s *State) AddMilestone(id string) bool {
	if slices.Contains(s.Milestones, id) {
		return false
	}
	s.Milestones = append(s.Milestones, id)
	slices.Sort(s.Milestones)
	return true
}

func (s *State) UnlockStarter(id string) bool {
	return appendUnique(&s.UnlockedStarters, id)
}

func (s *State) UnlockModifier(id string) bool {
	return appendUnique(&s.UnlockedModifiers, id)
}

func (s *State) UnlockTrait(id string) bool {
	return appendUnique(&s.UnlockedTraits, id)
}

func appendUnique(dst *[]string, id string) bool {
	if slices.Contains(*dst, id) {
		return false
	}
	*dst = append(*dst, id)
	slices.Sort(*dst)
	return true
}

func normalize(state State) State {
	if state.SchemaVersion == 0 {
		state.SchemaVersion = SchemaVersion
	}
	if len(state.UnlockedStarters) == 0 {
		state.UnlockedStarters = DefaultState().UnlockedStarters
	}
	if len(state.UnlockedModifiers) == 0 {
		state.UnlockedModifiers = DefaultState().UnlockedModifiers
	}
	if len(state.UnlockedTraits) == 0 {
		state.UnlockedTraits = DefaultState().UnlockedTraits
	}
	slices.Sort(state.UnlockedStarters)
	slices.Sort(state.UnlockedModifiers)
	slices.Sort(state.UnlockedTraits)
	slices.Sort(state.Milestones)
	return state
}
