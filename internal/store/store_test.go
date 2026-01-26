package store

import (
	"os"
	"testing"
	"time"

	"github.com/cldixon/jernel/internal/metrics"
)

// setupTestDB creates a temporary database for testing
func setupTestDB(t *testing.T) (*Store, func()) {
	t.Helper()

	// Create temp home directory
	tmpHome, err := os.MkdirTemp("", "jernel-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Override HOME
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)

	// Create config directory (required for DBPath)
	configDir := tmpHome + "/.config/jernel"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Open database
	store, err := Open()
	if err != nil {
		os.Setenv("HOME", origHome)
		os.RemoveAll(tmpHome)
		t.Fatalf("failed to open store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.Setenv("HOME", origHome)
		os.RemoveAll(tmpHome)
	}

	return store, cleanup
}

// createTestSnapshot creates a minimal metrics snapshot for testing
func createTestSnapshot() *metrics.Snapshot {
	return &metrics.Snapshot{
		Timestamp:     time.Now(),
		Uptime:        time.Hour * 24,
		CPUPercent:    25.5,
		MemoryPercent: 60.0,
		MemoryUsed:    8 * 1024 * 1024 * 1024,  // 8GB
		MemoryTotal:   16 * 1024 * 1024 * 1024, // 16GB
		DiskPercent:   45.0,
		DiskUsed:      500 * 1024 * 1024 * 1024,  // 500GB
		DiskTotal:     1024 * 1024 * 1024 * 1024, // 1TB
		MachineType:   metrics.MachineTypeLaptop,
		TimeOfDay:     metrics.TimeOfDayAfternoon,
	}
}

// TestStoreOpenCreatesMigration verifies that Open() creates the database
// and runs migrations successfully.
func TestStoreOpenCreatesMigration(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	// Verify we can query the entries table (proves migration ran)
	var count int
	err := store.db.QueryRow("SELECT COUNT(*) FROM entries").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query entries table: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 entries in fresh db, got %d", count)
	}

	// Verify indexes exist
	rows, err := store.db.Query(`
		SELECT name FROM sqlite_master
		WHERE type='index' AND tbl_name='entries'
	`)
	if err != nil {
		t.Fatalf("failed to query indexes: %v", err)
	}
	defer rows.Close()

	indexes := make(map[string]bool)
	for rows.Next() {
		var name string
		rows.Scan(&name)
		indexes[name] = true
	}

	if !indexes["idx_entries_created_at"] {
		t.Error("missing index: idx_entries_created_at")
	}
	if !indexes["idx_entries_persona"] {
		t.Error("missing index: idx_entries_persona")
	}
}

// TestStoreSaveAndRetrieve verifies the full save/retrieve cycle
// including metrics JSON serialization.
func TestStoreSaveAndRetrieve(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	snapshot := createTestSnapshot()

	// Save an entry
	entry, err := store.Save(
		"test_persona",
		"This is my journal entry content.",
		"claude-3-test",
		"msg_12345",
		snapshot,
	)
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	if entry.ID == 0 {
		t.Error("expected non-zero ID after save")
	}
	if entry.Persona != "test_persona" {
		t.Errorf("expected persona 'test_persona', got %q", entry.Persona)
	}

	// Retrieve by ID
	retrieved, err := store.GetByID(entry.ID)
	if err != nil {
		t.Fatalf("GetByID() failed: %v", err)
	}

	if retrieved.Content != "This is my journal entry content." {
		t.Errorf("content mismatch: %q", retrieved.Content)
	}
	if retrieved.ModelID != "claude-3-test" {
		t.Errorf("model_id mismatch: %q", retrieved.ModelID)
	}
	if retrieved.MessageID != "msg_12345" {
		t.Errorf("message_id mismatch: %q", retrieved.MessageID)
	}

	// Verify metrics were deserialized
	if retrieved.MetricsSnapshot == nil {
		t.Fatal("MetricsSnapshot is nil after retrieval")
	}
	if retrieved.MetricsSnapshot.CPUPercent != 25.5 {
		t.Errorf("CPUPercent mismatch: got %.1f", retrieved.MetricsSnapshot.CPUPercent)
	}
	if retrieved.MetricsSnapshot.MachineType != metrics.MachineTypeLaptop {
		t.Errorf("MachineType mismatch: got %v", retrieved.MetricsSnapshot.MachineType)
	}
}

// TestStoreListOrdering verifies entries are returned newest-first.
func TestStoreListOrdering(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	// Create entries with different timestamps
	times := []time.Time{
		time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 3, 10, 0, 0, 0, time.UTC),
	}

	for i, ts := range times {
		snapshot := createTestSnapshot()
		snapshot.Timestamp = ts
		_, err := store.Save("persona", "Entry "+string(rune('A'+i)), "model", "msg", snapshot)
		if err != nil {
			t.Fatalf("failed to save entry %d: %v", i, err)
		}
	}

	// List should return newest first
	entries, err := store.List(10)
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// First entry should be the newest (Jan 3)
	if entries[0].Content != "Entry C" {
		t.Errorf("expected newest entry first, got %q", entries[0].Content)
	}
	// Last entry should be the oldest (Jan 1)
	if entries[2].Content != "Entry A" {
		t.Errorf("expected oldest entry last, got %q", entries[2].Content)
	}
}

// TestStoreListByPersona verifies filtering by persona works correctly.
func TestStoreListByPersona(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	snapshot := createTestSnapshot()

	// Create entries for different personas
	store.Save("alice", "Alice entry 1", "model", "msg1", snapshot)
	store.Save("bob", "Bob entry 1", "model", "msg2", snapshot)
	store.Save("alice", "Alice entry 2", "model", "msg3", snapshot)
	store.Save("bob", "Bob entry 2", "model", "msg4", snapshot)

	// List Alice's entries
	aliceEntries, err := store.ListByPersona("alice", 10)
	if err != nil {
		t.Fatalf("ListByPersona() failed: %v", err)
	}

	if len(aliceEntries) != 2 {
		t.Errorf("expected 2 alice entries, got %d", len(aliceEntries))
	}

	for _, e := range aliceEntries {
		if e.Persona != "alice" {
			t.Errorf("got entry with persona %q in alice list", e.Persona)
		}
	}

	// Count verification
	count, err := store.CountByPersona("bob")
	if err != nil {
		t.Fatalf("CountByPersona() failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected bob count 2, got %d", count)
	}
}

// TestStoreDeleteOperations verifies delete functionality.
func TestStoreDeleteOperations(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	snapshot := createTestSnapshot()

	// Create test data
	store.Save("delete_me", "Entry 1", "model", "msg1", snapshot)
	store.Save("delete_me", "Entry 2", "model", "msg2", snapshot)
	store.Save("keep_me", "Entry 3", "model", "msg3", snapshot)

	// Delete by persona
	deleted, err := store.DeleteByPersona("delete_me")
	if err != nil {
		t.Fatalf("DeleteByPersona() failed: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	// Verify deletion
	count, _ := store.CountByPersona("delete_me")
	if count != 0 {
		t.Errorf("expected 0 entries after delete, got %d", count)
	}

	// Verify other entries preserved
	count, _ = store.CountByPersona("keep_me")
	if count != 1 {
		t.Errorf("expected keep_me entry preserved, got count %d", count)
	}

	// Test DeleteAll
	store.Save("another", "Entry 4", "model", "msg4", snapshot)
	deleted, err = store.DeleteAll()
	if err != nil {
		t.Fatalf("DeleteAll() failed: %v", err)
	}
	if deleted != 2 { // keep_me + another
		t.Errorf("expected 2 deleted by DeleteAll, got %d", deleted)
	}

	// Verify all gone
	entries, _ := store.List(100)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after DeleteAll, got %d", len(entries))
	}
}

// TestStoreGetByIDNotFound verifies proper error handling for missing entries.
func TestStoreGetByIDNotFound(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := store.GetByID(99999)
	if err == nil {
		t.Error("expected error for non-existent ID, got nil")
	}
}
