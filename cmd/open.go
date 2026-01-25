package cmd

import (
	"fmt"

	"github.com/cldixon/jernel/internal/store"
	"github.com/cldixon/jernel/internal/tui"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open the interactive journal viewer",
	Long:  `Opens your journal in an interactive terminal UI to browse and read entries.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := store.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer db.Close()

		entries, err := db.List(100)
		if err != nil {
			return fmt.Errorf("failed to load entries: %w", err)
		}

		return tui.Run(entries)
	},
}

func init() {
	rootCmd.AddCommand(openCmd)
}
