package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupTestEnv creates a temporary home directory for testing
func setupTestEnv(t *testing.T) func() {
	t.Helper()

	tmpHome, err := os.MkdirTemp("", "jernel-daemon-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)

	// Create config directory
	configDir := filepath.Join(tmpHome, ".config", "jernel")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	return func() {
		os.Setenv("HOME", origHome)
		os.RemoveAll(tmpHome)
	}
}

// TestPeriodToDuration verifies period string conversion.
func TestPeriodToDuration(t *testing.T) {
	tests := []struct {
		period   string
		expected time.Duration
		wantErr  bool
	}{
		{"hour", time.Hour, false},
		{"day", 24 * time.Hour, false},
		{"week", 7 * 24 * time.Hour, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.period, func(t *testing.T) {
			d, err := PeriodToDuration(tc.period)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for period %q", tc.period)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if d != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, d)
			}
		})
	}
}

// TestCalculateNextIntervalRange verifies intervals are within expected bounds.
func TestCalculateNextIntervalRange(t *testing.T) {
	tests := []struct {
		name        string
		rate        int
		period      string
		expectedAvg time.Duration
	}{
		{"3 per day", 3, "day", 8 * time.Hour},
		{"1 per hour", 1, "hour", time.Hour},
		{"6 per day", 6, "day", 4 * time.Hour},
		{"14 per week", 14, "week", 12 * time.Hour},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate min/max bounds (0.5x to 1.5x average)
			minBound := tc.expectedAvg / 2
			maxBound := tc.expectedAvg + tc.expectedAvg/2

			// Run multiple iterations to test randomness stays in bounds
			for i := 0; i < 100; i++ {
				interval, err := CalculateNextInterval(tc.rate, tc.period)
				if err != nil {
					t.Fatalf("CalculateNextInterval failed: %v", err)
				}

				if interval < minBound {
					t.Errorf("interval %v below minimum %v", interval, minBound)
				}
				if interval > maxBound {
					t.Errorf("interval %v above maximum %v", interval, maxBound)
				}
			}
		})
	}
}

// TestCalculateNextIntervalRandomness verifies intervals have variance.
func TestCalculateNextIntervalRandomness(t *testing.T) {
	// Generate many intervals and check they're not all identical
	seen := make(map[time.Duration]bool)

	for i := 0; i < 50; i++ {
		interval, err := CalculateNextInterval(3, "day")
		if err != nil {
			t.Fatalf("CalculateNextInterval failed: %v", err)
		}
		seen[interval] = true
	}

	// Should have multiple different values (randomness working)
	if len(seen) < 10 {
		t.Errorf("expected variance in intervals, only got %d unique values", len(seen))
	}
}

// TestCalculateNextIntervalInvalidInputs verifies error handling.
func TestCalculateNextIntervalInvalidInputs(t *testing.T) {
	tests := []struct {
		name   string
		rate   int
		period string
	}{
		{"zero rate", 0, "day"},
		{"negative rate", -1, "day"},
		{"invalid period", 3, "invalid"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CalculateNextInterval(tc.rate, tc.period)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// TestCalculateNextTrigger verifies trigger time is in the future.
func TestCalculateNextTrigger(t *testing.T) {
	now := time.Now()

	trigger, err := CalculateNextTrigger(3, "day")
	if err != nil {
		t.Fatalf("CalculateNextTrigger failed: %v", err)
	}

	if trigger.Before(now) {
		t.Error("next trigger should be in the future")
	}

	// Should be within reasonable bounds (4-12 hours for 3/day)
	minExpected := now.Add(4 * time.Hour)
	maxExpected := now.Add(12 * time.Hour)

	if trigger.Before(minExpected) {
		t.Errorf("trigger %v too soon (min %v)", trigger, minExpected)
	}
	if trigger.After(maxExpected) {
		t.Errorf("trigger %v too late (max %v)", trigger, maxExpected)
	}
}

// TestStateSaveAndLoad verifies state serialization round-trip.
func TestStateSaveAndLoad(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	original := &State{
		PID:              12345,
		StartedAt:        time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		NextTrigger:      time.Date(2025, 1, 15, 18, 30, 0, 0, time.UTC),
		EntriesGenerated: 5,
		LastEntryAt:      time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC),
		LastPersona:      "test_persona",
	}

	// Save state
	if err := SaveState(original); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Load state
	loaded, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	// Verify all fields
	if loaded.PID != original.PID {
		t.Errorf("PID mismatch: expected %d, got %d", original.PID, loaded.PID)
	}
	if !loaded.StartedAt.Equal(original.StartedAt) {
		t.Errorf("StartedAt mismatch: expected %v, got %v", original.StartedAt, loaded.StartedAt)
	}
	if !loaded.NextTrigger.Equal(original.NextTrigger) {
		t.Errorf("NextTrigger mismatch: expected %v, got %v", original.NextTrigger, loaded.NextTrigger)
	}
	if loaded.EntriesGenerated != original.EntriesGenerated {
		t.Errorf("EntriesGenerated mismatch: expected %d, got %d", original.EntriesGenerated, loaded.EntriesGenerated)
	}
	if !loaded.LastEntryAt.Equal(original.LastEntryAt) {
		t.Errorf("LastEntryAt mismatch")
	}
	if loaded.LastPersona != original.LastPersona {
		t.Errorf("LastPersona mismatch: expected %q, got %q", original.LastPersona, loaded.LastPersona)
	}
}

// TestStateLoadMissing verifies LoadState handles missing file gracefully.
func TestStateLoadMissing(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	state, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState should not error on missing file: %v", err)
	}
	if state != nil {
		t.Error("expected nil state for missing file")
	}
}

// TestStateRemove verifies RemoveState cleans up the state file.
func TestStateRemove(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Create state
	if err := SaveState(&State{PID: 1}); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Remove it
	if err := RemoveState(); err != nil {
		t.Fatalf("RemoveState failed: %v", err)
	}

	// Verify it's gone
	state, _ := LoadState()
	if state != nil {
		t.Error("state should be nil after removal")
	}

	// Remove again should not error
	if err := RemoveState(); err != nil {
		t.Errorf("RemoveState on missing file should not error: %v", err)
	}
}

// TestPIDFileOperations verifies PID file write/read/remove cycle.
func TestPIDFileOperations(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Write PID (uses current process)
	if err := WritePID(); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	// Read PID
	pid, err := ReadPID()
	if err != nil {
		t.Fatalf("ReadPID failed: %v", err)
	}

	expectedPID := os.Getpid()
	if pid != expectedPID {
		t.Errorf("PID mismatch: expected %d, got %d", expectedPID, pid)
	}

	// Remove PID
	if err := RemovePID(); err != nil {
		t.Fatalf("RemovePID failed: %v", err)
	}

	// Read should fail now
	_, err = ReadPID()
	if err == nil {
		t.Error("ReadPID should fail after removal")
	}

	// Remove again should not error
	if err := RemovePID(); err != nil {
		t.Errorf("RemovePID on missing file should not error: %v", err)
	}
}

// TestIsRunningDetectsCurrentProcess verifies IsRunning correctly identifies
// if a daemon process exists.
func TestIsRunningDetectsCurrentProcess(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// No PID file - should not be running
	running, pid, err := IsRunning()
	if err != nil {
		t.Fatalf("IsRunning failed: %v", err)
	}
	if running {
		t.Error("should not be running without PID file")
	}

	// Write our own PID (simulates running daemon)
	if err := WritePID(); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	// Now should detect as running (our process exists)
	running, pid, err = IsRunning()
	if err != nil {
		t.Fatalf("IsRunning failed: %v", err)
	}
	if !running {
		t.Error("should detect current process as running")
	}
	if pid != os.Getpid() {
		t.Errorf("PID mismatch: expected %d, got %d", os.Getpid(), pid)
	}
}

// TestIsRunningCleansStaleFile verifies stale PID files are cleaned up.
func TestIsRunningCleansStaleFile(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	// Write a PID that definitely doesn't exist
	path, _ := PIDPath()
	// Use a very high PID that's unlikely to exist
	if err := os.WriteFile(path, []byte("999999999"), 0644); err != nil {
		t.Fatalf("failed to write stale PID: %v", err)
	}

	// IsRunning should detect it's not actually running
	running, _, err := IsRunning()
	if err != nil {
		t.Fatalf("IsRunning failed: %v", err)
	}
	if running {
		t.Error("should not detect non-existent process as running")
	}

	// Stale PID file should have been cleaned up
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("stale PID file should have been removed")
	}
}
