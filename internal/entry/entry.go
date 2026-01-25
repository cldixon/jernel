package entry

import (
	"context"
	"fmt"

	"github.com/cldixon/jernel/internal/config"
	"github.com/cldixon/jernel/internal/llm"
	"github.com/cldixon/jernel/internal/metrics"
	"github.com/cldixon/jernel/internal/persona"
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

	// Generate entry via LLM
	client, err := llm.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	result, err := client.GenerateEntry(ctx, p.Description, snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to generate entry: %w", err)
	}

	// Save to database
	db, err := store.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

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
