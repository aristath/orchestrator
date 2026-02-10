package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aristath/orchestrator/internal/backend"
	"github.com/aristath/orchestrator/internal/config"
	"github.com/aristath/orchestrator/internal/events"
	"github.com/aristath/orchestrator/internal/tui"
)

func main() {
	// Create signal-aware context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Create ProcessManager for subprocess tracking
	pm := backend.NewProcessManager()

	// Load configuration
	cfg, err := config.LoadDefault()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Determine config paths
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		os.Exit(1)
	}
	globalPath := filepath.Join(homeDir, ".orchestrator", "config.json")
	projectPath := filepath.Join(".orchestrator", "config.json")

	// Create event bus
	bus := events.NewEventBus()
	defer bus.Close()

	// Create TUI model
	model := tui.New(bus, cfg, globalPath, projectPath)

	// Start Bubble Tea program in a goroutine so main can handle shutdown
	p := tea.NewProgram(model, tea.WithAltScreen())

	errChan := make(chan error, 1)
	go func() {
		_, err := p.Run()
		errChan <- err
	}()

	// TODO: Wire ParallelRunner here when DAG execution is integrated with TUI.
	// The runner will publish real events to the bus.
	// For now, TUI starts empty and waits for events.

	// Handle shutdown
	select {
	case err := <-errChan:
		// Normal TUI exit (user pressed 'q' or TUI finished)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		// Signal received (Ctrl+C or SIGTERM)
		// Call stop() to restore default signal handling (double Ctrl+C = force exit)
		stop()

		log.Println("Shutdown signal received, cleaning up...")

		// Kill all tracked subprocesses
		if err := pm.KillAll(); err != nil {
			log.Printf("Error killing subprocesses: %v", err)
		}

		// Quit the TUI
		p.Quit()

		// Wait for TUI to exit with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		select {
		case err := <-errChan:
			if err != nil {
				log.Printf("TUI exit error: %v", err)
			}
		case <-shutdownCtx.Done():
			log.Println("Shutdown timeout exceeded, forcing exit")
		}
	}

	log.Println("Shutdown complete")
}
