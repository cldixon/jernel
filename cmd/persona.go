package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cldixon/jernel/internal/persona"
	"github.com/cldixon/jernel/internal/store"
	"github.com/spf13/cobra"
)

var personaCmd = &cobra.Command{
	Use:   "persona",
	Short: "Manage personas",
	Long:  `Create, list, and delete personas. Personas define the voice and style for journal entries.`,
}

var personaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available personas",
	RunE: func(cmd *cobra.Command, args []string) error {
		names, err := persona.List()
		if err != nil {
			return fmt.Errorf("failed to list personas: %w", err)
		}

		if len(names) == 0 {
			fmt.Println("No personas found.")
			fmt.Println("Create one with 'jernel persona create <name>'")
			return nil
		}

		fmt.Println("Available personas:")
		for _, name := range names {
			p, err := persona.Get(name)
			if err != nil {
				fmt.Printf("  %s\n", name)
				continue
			}
			// Show first 50 chars of description
			desc := p.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			desc = strings.ReplaceAll(desc, "\n", " ")
			fmt.Printf("  %s - %s\n", name, desc)
		}
		return nil
	},
}

var personaCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new persona",
	Long: `Create a new persona file with a template. The file will be opened in your
default editor ($EDITOR) for you to customize.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Sanitize name
		name = strings.ToLower(strings.ReplaceAll(name, " ", "_"))

		path, err := persona.Create(name)
		if err != nil {
			return err
		}

		fmt.Printf("Created persona file: %s\n", path)
		fmt.Println("Edit this file to define the persona's voice and personality.")
		return nil
	},
}

var personaDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a persona",
	Long:  `Delete a persona and optionally all associated journal entries.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Check persona exists
		if _, err := persona.Get(name); err != nil {
			return fmt.Errorf("persona '%s' not found", name)
		}

		// Count associated entries
		db, err := store.Open()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}

		entryCount, err := db.CountByPersona(name)
		if err != nil {
			db.Close()
			return fmt.Errorf("failed to count entries: %w", err)
		}
		db.Close()

		// Confirm with user
		if entryCount > 0 {
			fmt.Printf("This will delete persona '%s' and %d associated %s.\n",
				name, entryCount, pluralize(entryCount, "entry", "entries"))
		} else {
			fmt.Printf("This will delete persona '%s'.\n", name)
		}
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

		// Delete entries first
		if entryCount > 0 {
			db, err = store.Open()
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			deleted, err := db.DeleteByPersona(name)
			db.Close()
			if err != nil {
				return fmt.Errorf("failed to delete entries: %w", err)
			}
			fmt.Printf("Deleted %d %s.\n", deleted, pluralize(int(deleted), "entry", "entries"))
		}

		// Delete persona file
		if err := persona.Delete(name); err != nil {
			return err
		}

		fmt.Printf("Deleted persona '%s'.\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(personaCmd)
	personaCmd.AddCommand(personaListCmd)
	personaCmd.AddCommand(personaCreateCmd)
	personaCmd.AddCommand(personaDeleteCmd)
}
