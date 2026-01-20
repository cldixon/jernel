package cmd

import (
	"fmt"
	"os"

	"github.com/cldixon/jernel/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "jernel",
	Short: "A journal for your machine's soul",
	Long:  `jernel gives your computer a voice by translating system metrics into personal journal entries.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return config.Init()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
