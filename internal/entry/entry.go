package entry

import (
	"context"
	"fmt"

	"github.com/cldixon/jernel/internal/config"
	"github.com/cldixon/jernel/internal/llm"
	"github.com/cldixon/jernel/internal/metrics"
	"github.com/cldixon/jernel/internal/persona"
	"github.com/cldixon/jernel/internal/prompt"
	"github.com/cldixon/jernel/internal/store"
)

// Result contains the generated entry and associated metadata
type Result struct {
	Entry    *store.Entry
	Persona  *persona.Persona
	Snapshot *metrics.Snapshot
}

// Generate creates a new journal entry with the given persona
// It gathers metrics, calls the LLM, and saves to the database
func Generate(ctx context.Context, cfg *config.Config, personaName string) (*Result, error) {
	// Load persona
	p, err := persona.Get(personaName)
	if err != nil {
		return nil, fmt.Errorf("failed to load persona: %w", err)
	}

	// Gather metrics
	snapshot, err := metrics.Gather()
	if err != nil {
		return nil, fmt.Errorf("failed to gather metrics: %w", err)
	}

	// Open database early to fetch previous entries for context
	db, err := store.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Fetch previous entries for context continuity
	var previousEntries []prompt.PreviousEntry
	if cfg.ContextEntries > 0 {
		recentEntries, err := db.ListByPersona(p.Name, cfg.ContextEntries)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch previous entries: %w", err)
		}
		for _, e := range recentEntries {
			previousEntries = append(previousEntries, prompt.PreviousEntry{
				Date:    e.CreatedAt.Format("Monday, January 2, 2006 at 3:04 PM"),
				Content: e.Content,
			})
		}
	}

	// Generate entry via LLM
	client, err := llm.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	result, err := client.GenerateEntry(ctx, p.Description, snapshot, previousEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to generate entry: %w", err)
	}

	// Save to database
	entry, err := db.Save(p.Name, result.Content, result.ModelID, result.MessageID, snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to save entry: %w", err)
	}

	return &Result{
		Entry:    entry,
		Persona:  p,
		Snapshot: snapshot,
	}, nil
}
