package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cldixon/jernel/internal/config"
)

// State holds the daemon's runtime state
type State struct {
	PID              int       `json:"pid"`
	StartedAt        time.Time `json:"started_at"`
	NextTrigger      time.Time `json:"next_trigger"`
	EntriesGenerated int       `json:"entries_generated"`
	LastEntryAt      time.Time `json:"last_entry_at,omitempty"`
	LastPersona      string    `json:"last_persona,omitempty"`
}

// StatePath returns the path to the daemon state file
func StatePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.state"), nil
}

// LoadState reads the daemon state from disk
func LoadState() (*State, error) {
	path, err := StatePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read state: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}

	return &state, nil
}

// SaveState writes the daemon state to disk
func SaveState(state *State) error {
	path, err := StatePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	return nil
}

// RemoveState removes the state file
func RemoveState() error {
	path, err := StatePath()
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
