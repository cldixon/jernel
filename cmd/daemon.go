package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cldixon/jernel/internal/config"
	"github.com/cldixon/jernel/internal/daemon"
	"github.com/spf13/cobra"
)

// Flags for daemon start command
var (
	daemonRate       int
	daemonRatePeriod string
	daemonPersonas   string
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the jernel daemon for autonomous entry generation",
	Long: `The daemon runs in the background and generates journal entries
at random intervals based on the configured rate.

Configure defaults in ~/.config/jernel/config.yaml:

  daemon:
    rate: 3           # entries per period
    rate_period: day  # hour, day, or week
    personas:         # personas to randomly select from
      - default
      - dramatic

Or override with flags: jernel daemon start --rate 5 --rate-period day`,
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon in foreground",
	Long: `Start the jernel daemon in the foreground. The daemon will generate
journal entries at random intervals based on your configuration.

Use Ctrl+C to stop the daemon gracefully.

Flags override config.yaml settings for this run only.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Apply flag overrides
		if cmd.Flags().Changed("rate") {
			cfg.Daemon.Rate = daemonRate
		}
		if cmd.Flags().Changed("rate-period") {
			cfg.Daemon.RatePeriod = daemonRatePeriod
		}
		if cmd.Flags().Changed("personas") {
			if daemonPersonas != "" {
				cfg.Daemon.Personas = strings.Split(daemonPersonas, ",")
			} else {
				cfg.Daemon.Personas = []string{}
			}
		}

		// Create daemon
		d := daemon.New(cfg)

		// Set up signal handling
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigChan
			fmt.Println("\nReceived shutdown signal...")
			d.Stop()
			cancel()
		}()

		// Start daemon
		if err := d.Start(ctx); err != nil {
			return err
		}

		// Wait for daemon to finish
		d.Wait()

		return nil
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		running, pid, err := daemon.IsRunning()
		if err != nil {
			return fmt.Errorf("failed to check daemon status: %w", err)
		}

		if !running {
			fmt.Println("Daemon is not running")
			return nil
		}

		fmt.Printf("Stopping daemon (PID: %d)...\n", pid)

		if err := daemon.StopRunning(); err != nil {
			return fmt.Errorf("failed to stop daemon: %w", err)
		}

		// Wait for process to exit
		for i := 0; i < 30; i++ {
			running, _, _ = daemon.IsRunning()
			if !running {
				fmt.Println("Daemon stopped")
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}

		return fmt.Errorf("daemon did not stop in time")
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Check if running
		running, pid, err := daemon.IsRunning()
		if err != nil {
			return fmt.Errorf("failed to check daemon status: %w", err)
		}

		fmt.Println("Daemon Configuration:")
		fmt.Printf("  Rate:        %d per %s\n", cfg.Daemon.Rate, cfg.Daemon.RatePeriod)
		if len(cfg.Daemon.Personas) > 0 {
			fmt.Printf("  Personas:    %v\n", cfg.Daemon.Personas)
		} else {
			fmt.Printf("  Personas:    [%s] (default)\n", cfg.DefaultPersona)
		}
		fmt.Println()

		if !running {
			fmt.Println("Status: NOT RUNNING")
			return nil
		}

		fmt.Printf("Status: RUNNING (PID: %d)\n", pid)

		// Load state for more details
		state, err := daemon.LoadState()
		if err != nil {
			fmt.Printf("  (could not load state: %v)\n", err)
			return nil
		}

		if state != nil {
			fmt.Printf("  Started:     %s\n", state.StartedAt.Format(time.RFC1123))
			fmt.Printf("  Next entry:  %s\n", state.NextTrigger.Format(time.RFC1123))
			fmt.Printf("  Entries:     %d generated\n", state.EntriesGenerated)
			if !state.LastEntryAt.IsZero() {
				fmt.Printf("  Last entry:  %s (persona: %s)\n",
					state.LastEntryAt.Format(time.RFC1123), state.LastPersona)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)

	// Flags for daemon start
	daemonStartCmd.Flags().IntVar(&daemonRate, "rate", 0, "Number of entries per period (overrides config)")
	daemonStartCmd.Flags().StringVar(&daemonRatePeriod, "rate-period", "", "Period for rate: hour, day, or week (overrides config)")
	daemonStartCmd.Flags().StringVar(&daemonPersonas, "personas", "", "Comma-separated list of personas (overrides config)")
}
