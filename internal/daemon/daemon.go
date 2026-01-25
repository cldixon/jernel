package daemon

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/cldixon/jernel/internal/config"
	"github.com/cldixon/jernel/internal/entry"
)

// Daemon manages autonomous journal entry generation
type Daemon struct {
	cfg      *config.Config
	state    *State
	shutdown chan struct{}
	done     chan struct{}
	logger   *log.Logger
}

// New creates a new daemon instance
func New(cfg *config.Config) *Daemon {
	return &Daemon{
		cfg:      cfg,
		shutdown: make(chan struct{}),
		done:     make(chan struct{}),
		logger:   log.New(os.Stdout, "[jernel-daemon] ", log.LstdFlags),
	}
}

// Start begins the daemon's main loop
func (d *Daemon) Start(ctx context.Context) error {
	// Check if already running
	running, pid, err := IsRunning()
	if err != nil {
		return fmt.Errorf("failed to check running state: %w", err)
	}
	if running {
		return fmt.Errorf("daemon already running with PID %d", pid)
	}

	// Write PID file
	if err := WritePID(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Initialize state
	nextTrigger, err := CalculateNextTrigger(d.cfg.Daemon.Rate, d.cfg.Daemon.RatePeriod)
	if err != nil {
		RemovePID()
		return fmt.Errorf("failed to calculate next trigger: %w", err)
	}

	d.state = &State{
		PID:              os.Getpid(),
		StartedAt:        time.Now(),
		NextTrigger:      nextTrigger,
		EntriesGenerated: 0,
	}

	if err := SaveState(d.state); err != nil {
		RemovePID()
		return fmt.Errorf("failed to save initial state: %w", err)
	}

	d.logger.Printf("Daemon started (PID: %d)", d.state.PID)
	d.logger.Printf("Rate: %d entries per %s", d.cfg.Daemon.Rate, d.cfg.Daemon.RatePeriod)
	d.logger.Printf("Next entry scheduled for: %s", d.state.NextTrigger.Format(time.RFC1123))

	// Run main loop
	go d.run(ctx)

	return nil
}

// run is the main daemon loop
func (d *Daemon) run(ctx context.Context) {
	defer close(d.done)
	defer d.cleanup()

	for {
		// Calculate time until next trigger
		waitDuration := time.Until(d.state.NextTrigger)
		if waitDuration < 0 {
			waitDuration = 0
		}

		d.logger.Printf("Waiting %s until next entry...", waitDuration.Round(time.Second))

		select {
		case <-ctx.Done():
			d.logger.Println("Context cancelled, shutting down...")
			return
		case <-d.shutdown:
			d.logger.Println("Shutdown signal received...")
			return
		case <-time.After(waitDuration):
			// Time to generate an entry
			if err := d.generateEntry(ctx); err != nil {
				d.logger.Printf("Error generating entry: %v", err)
			}

			// Schedule next trigger
			nextTrigger, err := CalculateNextTrigger(d.cfg.Daemon.Rate, d.cfg.Daemon.RatePeriod)
			if err != nil {
				d.logger.Printf("Error calculating next trigger: %v", err)
				continue
			}

			d.state.NextTrigger = nextTrigger
			if err := SaveState(d.state); err != nil {
				d.logger.Printf("Error saving state: %v", err)
			}

			d.logger.Printf("Next entry scheduled for: %s", d.state.NextTrigger.Format(time.RFC1123))
		}
	}
}

// generateEntry creates a new journal entry
func (d *Daemon) generateEntry(ctx context.Context) error {
	// Select persona
	personaName := d.selectPersona()

	d.logger.Printf("Generating entry with persona: %s", personaName)

	// Generate entry using the entry package
	result, err := entry.Generate(ctx, d.cfg, personaName)
	if err != nil {
		return err
	}

	// Update state
	d.state.EntriesGenerated++
	d.state.LastEntryAt = time.Now()
	d.state.LastPersona = personaName

	if err := SaveState(d.state); err != nil {
		d.logger.Printf("Warning: failed to save state: %v", err)
	}

	d.logger.Printf("Entry #%d created (ID: %d, persona: %s)",
		d.state.EntriesGenerated, result.Entry.ID, personaName)

	return nil
}

// selectPersona chooses a persona for the next entry
func (d *Daemon) selectPersona() string {
	personas := d.cfg.Daemon.Personas

	// If no personas configured, use default
	if len(personas) == 0 {
		return d.cfg.DefaultPersona
	}

	// Random selection from configured personas
	if len(personas) == 1 {
		return personas[0]
	}

	idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(personas))))
	if err != nil {
		// Fallback to first persona on error
		return personas[0]
	}

	return personas[idx.Int64()]
}

// Stop signals the daemon to shut down gracefully
func (d *Daemon) Stop() {
	close(d.shutdown)
}

// Wait blocks until the daemon has fully stopped
func (d *Daemon) Wait() {
	<-d.done
}

// cleanup removes PID and state files on shutdown
func (d *Daemon) cleanup() {
	d.logger.Println("Cleaning up...")

	if err := RemovePID(); err != nil {
		d.logger.Printf("Warning: failed to remove PID file: %v", err)
	}

	if err := RemoveState(); err != nil {
		d.logger.Printf("Warning: failed to remove state file: %v", err)
	}

	d.logger.Println("Daemon stopped")
}
