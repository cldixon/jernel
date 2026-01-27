package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/cldixon/jernel/internal/config"
	"github.com/cldixon/jernel/internal/metrics"
	_ "github.com/mattn/go-sqlite3"
)

// Entry represents a saved journal entry
type Entry struct {
	ID              int64
	Persona         string
	Content         string
	CreatedAt       time.Time
	ModelID         string
	MessageID       string
	MetricsSnapshot *metrics.Snapshot
}

// Store handles persistence of journal entries
type Store struct {
	db *sql.DB
}

// DBPath returns the path to the database file
func DBPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "jernel.db"), nil
}

// Open creates or opens the database
func Open() (*Store, error) {
	path, err := DBPath()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate creates the schema if it doesn't exist
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		persona TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		model_id TEXT NOT NULL,
		message_id TEXT NOT NULL,
		metrics_snapshot TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_entries_created_at ON entries(created_at);
	CREATE INDEX IF NOT EXISTS idx_entries_persona ON entries(persona);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	return nil
}

// Save persists a new journal entry
func (s *Store) Save(persona string, content string, modelID string, messageID string, snapshot *metrics.Snapshot) (*Entry, error) {
	metricsJSON, err := snapshot.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize metrics: %w", err)
	}

	result, err := s.db.Exec(`
		INSERT INTO entries (persona, content, created_at, model_id, message_id, metrics_snapshot)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		persona,
		content,
		snapshot.Timestamp,
		modelID,
		messageID,
		metricsJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to save entry: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get entry id: %w", err)
	}

	return &Entry{
		ID:              id,
		Persona:         persona,
		Content:         content,
		CreatedAt:       snapshot.Timestamp,
		ModelID:         modelID,
		MessageID:       messageID,
		MetricsSnapshot: snapshot,
	}, nil
}

// GetByID retrieves a single entry by ID
func (s *Store) GetByID(id int64) (*Entry, error) {
	row := s.db.QueryRow(`
		SELECT id, persona, content, created_at, model_id, message_id, metrics_snapshot
		FROM entries
		WHERE id = ?
	`, id)

	return scanEntry(row)
}

// List retrieves entries with optional limit, newest first
func (s *Store) List(limit int) ([]*Entry, error) {
	rows, err := s.db.Query(`
		SELECT id, persona, content, created_at, model_id, message_id, metrics_snapshot
		FROM entries
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list entries: %w", err)
	}
	defer rows.Close()

	return scanEntries(rows)
}

// ListByPersona retrieves entries for a specific persona
func (s *Store) ListByPersona(persona string, limit int) ([]*Entry, error) {
	rows, err := s.db.Query(`
		SELECT id, persona, content, created_at, model_id, message_id, metrics_snapshot
		FROM entries
		WHERE persona = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, persona, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list entries: %w", err)
	}
	defer rows.Close()

	return scanEntries(rows)
}

// CountByPersona returns the number of entries for a specific persona
func (s *Store) CountByPersona(persona string) (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM entries WHERE persona = ?
	`, persona).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count entries: %w", err)
	}
	return count, nil
}

// DeleteByPersona removes all entries for a specific persona
func (s *Store) DeleteByPersona(persona string) (int64, error) {
	result, err := s.db.Exec(`
		DELETE FROM entries WHERE persona = ?
	`, persona)
	if err != nil {
		return 0, fmt.Errorf("failed to delete entries: %w", err)
	}
	return result.RowsAffected()
}

// DeleteAll removes all entries from the database
func (s *Store) DeleteAll() (int64, error) {
	result, err := s.db.Exec(`DELETE FROM entries`)
	if err != nil {
		return 0, fmt.Errorf("failed to delete entries: %w", err)
	}
	return result.RowsAffected()
}

// scanner interface for both *sql.Row and *sql.Rows
type scanner interface {
	Scan(dest ...any) error
}

func scanEntry(s scanner) (*Entry, error) {
	var e Entry
	var metricsJSON sql.NullString
	err := s.Scan(
		&e.ID,
		&e.Persona,
		&e.Content,
		&e.CreatedAt,
		&e.ModelID,
		&e.MessageID,
		&metricsJSON,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("entry not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan entry: %w", err)
	}

	if metricsJSON.Valid {
		snapshot, err := metrics.SnapshotFromJSON(metricsJSON.String)
		if err != nil {
			return nil, fmt.Errorf("failed to parse metrics: %w", err)
		}
		e.MetricsSnapshot = snapshot
	}

	return &e, nil
}

func scanEntries(rows *sql.Rows) ([]*Entry, error) {
	var entries []*Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating entries: %w", err)
	}

	return entries, nil
}
