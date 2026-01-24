package cmd

import (
	"fmt"
	"strconv"

	"github.com/cldixon/jernel/internal/store"
	"github.com/spf13/cobra"
)

var (
	listFlag          bool
	limitFlag         int
	personaFilterFlag string
)

var readCmd = &cobra.Command{
	Use:   "read [id]",
	Short: "Read journal entries",
	Long: `Read journal entries. Without arguments, shows the most recent entry.
Use --list to see multiple entries, or provide an ID to read a specific entry.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := store.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer db.Close()

		// List mode
		if listFlag {
			var entries []*store.Entry
			if personaFilterFlag != "" {
				entries, err = db.ListByPersona(personaFilterFlag, limitFlag)
			} else {
				entries, err = db.List(limitFlag)
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
		}

		// Single entry mode
		var entry *store.Entry
		if len(args) > 0 {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid entry ID: %s", args[0])
			}
			entry, err = db.GetByID(id)
			if err != nil {
				return err
			}
		} else {
			// Get most recent
			entries, err := db.List(1)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Println("No entries found. Create one with 'jernel new'")
				return nil
			}
			entry = entries[0]
		}

		printEntry(entry)
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
	rootCmd.AddCommand(readCmd)
	readCmd.Flags().BoolVarP(&listFlag, "list", "l", false, "List entries instead of showing content")
	readCmd.Flags().IntVarP(&limitFlag, "limit", "n", 10, "Number of entries to list")
	readCmd.Flags().StringVar(&personaFilterFlag, "persona", "", "Filter by persona")
}
