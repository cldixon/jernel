package cmd

import (
	"context"
	"fmt"

	"github.com/cldixon/jernel/internal/config"
	"github.com/cldixon/jernel/internal/entry"
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

		fmt.Printf("Creating a new jernel entry with persona: %s\n\n", personaName)
		fmt.Println("Gathering system metrics and generating entry...")

		// Generate entry using the entry package
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

func init() {
	rootCmd.AddCommand(newCmd)
	newCmd.Flags().StringVarP(&personaFlag, "persona", "p", "", "Persona to use (defaults to config setting)")
}
