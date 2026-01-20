package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/cldixon/jernel/internal/config"
	"github.com/cldixon/jernel/internal/metrics"
)

// Entry represents a saved journal entry
type Entry struct {
	ID            int64
	Persona       string
	Content       string
	CreatedAt     time.Time
	Uptime        string
	CPUPercent    float64
	MemoryPercent float64
	DiskPercent   float64
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
		uptime TEXT,
		cpu_percent REAL,
		memory_percent REAL,
		disk_percent REAL
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
func (s *Store) Save(persona string, content string, snapshot *metrics.Snapshot) (*Entry, error) {
	result, err := s.db.Exec(`
		INSERT INTO entries (persona, content, created_at, uptime, cpu_percent, memory_percent, disk_percent)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		persona,
		content,
		snapshot.Timestamp,
		snapshot.Uptime.String(),
		snapshot.CPUPercent,
		snapshot.MemoryPercent,
		snapshot.DiskPercent,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to save entry: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get entry id: %w", err)
	}

	return &Entry{
		ID:            id,
		Persona:       persona,
		Content:       content,
		CreatedAt:     snapshot.Timestamp,
		Uptime:        snapshot.Uptime.String(),
		CPUPercent:    snapshot.CPUPercent,
		MemoryPercent: snapshot.MemoryPercent,
		DiskPercent:   snapshot.DiskPercent,
	}, nil
}

// GetByID retrieves a single entry by ID
func (s *Store) GetByID(id int64) (*Entry, error) {
	row := s.db.QueryRow(`
		SELECT id, persona, content, created_at, uptime, cpu_percent, memory_percent, disk_percent
		FROM entries
		WHERE id = ?
	`, id)

	return scanEntry(row)
}

// List retrieves entries with optional limit, newest first
func (s *Store) List(limit int) ([]*Entry, error) {
	rows, err := s.db.Query(`
		SELECT id, persona, content, created_at, uptime, cpu_percent, memory_percent, disk_percent
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
		SELECT id, persona, content, created_at, uptime, cpu_percent, memory_percent, disk_percent
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

// scanner interface for both *sql.Row and *sql.Rows
type scanner interface {
	Scan(dest ...any) error
}

func scanEntry(s scanner) (*Entry, error) {
	var e Entry
	err := s.Scan(
		&e.ID,
		&e.Persona,
		&e.Content,
		&e.CreatedAt,
		&e.Uptime,
		&e.CPUPercent,
		&e.MemoryPercent,
		&e.DiskPercent,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("entry not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan entry: %w", err)
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
