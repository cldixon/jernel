package cmd

import (
	"context"
	"fmt"

	"github.com/cldixon/jernel/internal/config"
	"github.com/cldixon/jernel/internal/llm"
	"github.com/cldixon/jernel/internal/metrics"
	"github.com/cldixon/jernel/internal/persona"
	"github.com/cldixon/jernel/internal/store"
	"github.com/spf13/cobra"
)

var personaFlag string

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new journal entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load config
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Determine which persona to use
		personaName := personaFlag
		if personaName == "" {
			personaName = cfg.DefaultPersona
		}

		// Load persona
		p, err := persona.Get(personaName)
		if err != nil {
			return err
		}

		fmt.Printf("Creating a new jernel entry with persona: %s\n\n", p.Name)

		// Gather metrics
		fmt.Println("Gathering system metrics...")
		snapshot, err := metrics.Gather()
		if err != nil {
			return fmt.Errorf("failed to gather metrics: %w", err)
		}

		fmt.Printf("  Uptime:  %s\n", snapshot.Uptime)
		fmt.Printf("  CPU:     %.1f%%\n", snapshot.CPUPercent)
		fmt.Printf("  Memory:  %.1f%%\n", snapshot.MemoryPercent)
		fmt.Printf("  Disk:    %.1f%%\n\n", snapshot.DiskPercent)

		// Generate entry
		fmt.Println("Generating journal entry...\n")
		client, err := llm.NewClient(cfg)
		if err != nil {
			return err
		}

		result, err := client.GenerateEntry(ctx, p.Description, snapshot)
		if err != nil {
			return err
		}

		// Save to database
		db, err := store.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer db.Close()

		entry, err := db.Save(p.Name, result.Content, result.ModelID, result.MessageID, snapshot)
		if err != nil {
			return fmt.Errorf("failed to save entry: %w", err)
		}

		fmt.Println("---")
		fmt.Println(result.Content)
		fmt.Println("---")
		fmt.Printf("\nSaved as entry #%d\n", entry.ID)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
	newCmd.Flags().StringVarP(&personaFlag, "persona", "p", "", "Persona to use (defaults to config setting)")
}
