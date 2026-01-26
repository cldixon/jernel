package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cldixon/jernel/internal/config"
	"github.com/cldixon/jernel/internal/entry"
	"github.com/cldixon/jernel/internal/store"
	"github.com/spf13/cobra"
)

var entryCmd = &cobra.Command{
	Use:   "entry",
	Short: "Manage journal entries",
	Long:  `Create, list, and read journal entries.`,
}

// Flags for entry create
var entryCreatePersonaFlag string

var entryCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new journal entry",
	Long:  `Generate a new journal entry using system metrics and the specified persona.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		personaName := entryCreatePersonaFlag
		if personaName == "" {
			personaName = cfg.DefaultPersona
		}

		fmt.Printf("Creating a new jernel entry with persona: %s\n\n", personaName)
		fmt.Println("Gathering system metrics and generating entry...")

		result, err := entry.Generate(ctx, cfg, personaName)
		if err != nil {
			return err
		}

		fmt.Printf("\n  Uptime:  %s\n", result.Snapshot.Uptime)
		fmt.Printf("  CPU:     %.1f%%\n", result.Snapshot.CPUPercent)
		fmt.Printf("  Memory:  %.1f%%\n", result.Snapshot.MemoryPercent)
		fmt.Printf("  Disk:    %.1f%%\n\n", result.Snapshot.DiskPercent)

		fmt.Println("---")
		fmt.Println(result.Entry.Content)
		fmt.Println("---")
		fmt.Printf("\nSaved as entry #%d\n", result.Entry.ID)

		return nil
	},
}

// Flags for entry list
var entryListLimitFlag int
var entryListPersonaFlag string

var entryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List journal entries",
	Long:  `List journal entries with optional filtering by persona.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := store.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer db.Close()

		var entries []*store.Entry
		if entryListPersonaFlag != "" {
			entries, err = db.ListByPersona(entryListPersonaFlag, entryListLimitFlag)
		} else {
			entries, err = db.List(entryListLimitFlag)
		}
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			fmt.Println("No entries found.")
			return nil
		}

		for _, e := range entries {
			fmt.Printf("#%d [%s] %s\n", e.ID, e.Persona, e.CreatedAt.Format("Jan 02, 2006 3:04 PM"))
		}
		return nil
	},
}

var entryReadCmd = &cobra.Command{
	Use:   "read [id]",
	Short: "Read a journal entry",
	Long:  `Read a specific journal entry by ID, or the most recent entry if no ID is provided.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := store.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer db.Close()

		var e *store.Entry
		if len(args) > 0 {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid entry ID: %s", args[0])
			}
			e, err = db.GetByID(id)
			if err != nil {
				return err
			}
		} else {
			entries, err := db.List(1)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Println("No entries found. Create one with 'jernel entry create'")
				return nil
			}
			e = entries[0]
		}

		printEntry(e)
		return nil
	},
}

func printEntry(e *store.Entry) {
	fmt.Printf("Entry #%d\n", e.ID)
	fmt.Printf("Persona: %s\n", e.Persona)
	fmt.Printf("Date: %s\n", e.CreatedAt.Format("Monday, January 02, 2006 at 3:04 PM"))
	fmt.Printf("Model: %s\n", e.ModelID)
	if e.MetricsSnapshot != nil {
		m := e.MetricsSnapshot
		fmt.Printf("System: CPU %.1f%% | Memory %.1f%% | Disk %.1f%% | Uptime %s\n",
			m.CPUPercent, m.MemoryPercent, m.DiskPercent, m.Uptime)
	}
	fmt.Println()
	fmt.Println("---")
	fmt.Println(e.Content)
	fmt.Println("---")
}

func init() {
	rootCmd.AddCommand(entryCmd)

	// entry create
	entryCmd.AddCommand(entryCreateCmd)
	entryCreateCmd.Flags().StringVarP(&entryCreatePersonaFlag, "persona", "p", "", "Persona to use (defaults to config setting)")

	// entry list
	entryCmd.AddCommand(entryListCmd)
	entryListCmd.Flags().IntVarP(&entryListLimitFlag, "limit", "n", 10, "Number of entries to list")
	entryListCmd.Flags().StringVarP(&entryListPersonaFlag, "persona", "p", "", "Filter by persona")

	// entry read
	entryCmd.AddCommand(entryReadCmd)
}
