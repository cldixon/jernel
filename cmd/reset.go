package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cldixon/jernel/internal/store"
	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Delete all journal entries",
	Long:  `Permanently deletes all journal entries from the database. This action cannot be undone.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get entry count first
		db, err := store.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}

		entries, err := db.List(10000)
		if err != nil {
			db.Close()
			return fmt.Errorf("failed to count entries: %w", err)
		}
		count := len(entries)
		db.Close()

		if count == 0 {
			fmt.Println("No entries to delete.")
			return nil
		}

		// Confirm with user
		fmt.Printf("This will permanently delete %d journal %s.\n", count, pluralize(count, "entry", "entries"))
		fmt.Print("Type 'yes' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		if strings.TrimSpace(strings.ToLower(input)) != "yes" {
			fmt.Println("Aborted.")
			return nil
		}

		// Delete all entries
		db, err = store.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer db.Close()

		deleted, err := db.DeleteAll()
		if err != nil {
			return fmt.Errorf("failed to delete entries: %w", err)
		}

		fmt.Printf("Deleted %d %s.\n", deleted, pluralize(int(deleted), "entry", "entries"))
		return nil
	},
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
