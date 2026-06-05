package graceful

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/vukyn/kuery/log"
)

var (
	DefaultSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM, os.Interrupt}
)

// ShutdownHandler defines the interface for components that need graceful shutdown
type ShutdownHandler interface {
	Shutdown(ctx context.Context) error
}

// ShutdownOptions configures the graceful shutdown behavior
type ShutdownOptions struct {
	// Timeout for the shutdown process
	Timeout time.Duration
	// Delay between shutdown steps
	StepDelay time.Duration
	// Whether to log shutdown progress
	Verbose bool
	// Custom signal handlers
	Signals []os.Signal
	// Logger for logging shutdown progress
	Logger log.SimpleLogger
}

// DefaultShutdownOptions returns default shutdown configuration
func DefaultShutdownOptions() *ShutdownOptions {
	return &ShutdownOptions{
		Timeout:   30 * time.Second,
		StepDelay: 100 * time.Millisecond,
		Verbose:   true,
		Signals:   DefaultSignals,
	}
}

// GracefulShutdown waits for kill signals and performs graceful shutdown
func GracefulShutdown(handlers []ShutdownHandler, opts *ShutdownOptions) error {
	if opts == nil {
		opts = DefaultShutdownOptions()
	}
	if opts.Logger == nil {
		opts.Logger = log.New()
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	// Create signal channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, opts.Signals...)

	if opts.Verbose {
		opts.Logger.Infof("Waiting for shutdown signal (timeout: %v)...", opts.Timeout.Seconds())
	}

	// Wait for signal
	<-sigChan

	if opts.Verbose {
		opts.Logger.Infof("Received shutdown signal, starting graceful shutdown...")
	}

	// Perform shutdown with slow return
	return performShutdown(ctx, handlers, opts)
}

// performShutdown executes the actual shutdown process with slow return
func performShutdown(ctx context.Context, handlers []ShutdownHandler, opts *ShutdownOptions) error {
	var wg sync.WaitGroup
	errors := make(chan error, len(handlers))

	// Shutdown each handler with delay
	for i, handler := range handlers {
		wg.Add(1)
		go func(index int, h ShutdownHandler) {
			defer wg.Done()

			if opts.Verbose {
				opts.Logger.Infof("Shutting down handler %d...", index+1)
			}

			// Add delay between shutdowns for slow return
			time.Sleep(opts.StepDelay * time.Duration(index+1))

			if err := h.Shutdown(ctx); err != nil {
				errors <- fmt.Errorf("handler %d shutdown error: %w", index+1, err)
			} else if opts.Verbose {
				opts.Logger.Infof("Handler %d shutdown completed", index+1)
			}
		}(i, handler)
	}

	// Wait for all handlers to complete
	wg.Wait()
	close(errors)

	// Collect any errors
	var shutdownErrors []error
	for err := range errors {
		shutdownErrors = append(shutdownErrors, err)
	}

	// Final delay for slow return
	if opts.Verbose {
		opts.Logger.Infof("Finalizing shutdown...")
	}
	time.Sleep(opts.StepDelay * 2)

	if len(shutdownErrors) > 0 {
		if opts.Verbose {
			opts.Logger.Errorf("Shutdown completed with %d errors", len(shutdownErrors))
		}
		return fmt.Errorf("shutdown errors: %v", shutdownErrors)
	}

	if opts.Verbose {
		opts.Logger.Infof("Graceful shutdown completed successfully")
	}
	return nil
}

// SimpleShutdown provides a simple shutdown function for basic use cases
func SimpleShutdown(timeout time.Duration) error {
	opts := &ShutdownOptions{
		Timeout:   timeout,
		StepDelay: 200 * time.Millisecond,
		Verbose:   true,
	}
	return GracefulShutdown(nil, opts)
}

// ShutdownWithCallback allows custom shutdown logic
func ShutdownWithCallback(callback func(context.Context) error, opts *ShutdownOptions) error {
	if opts == nil {
		opts = DefaultShutdownOptions()
	}
	if opts.Logger == nil {
		opts.Logger = log.New()
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, opts.Signals...)

	if opts.Verbose {
		opts.Logger.Infof("Waiting for shutdown signal (timeout: %vs)...", opts.Timeout.Seconds())
	}

	<-sigChan

	if opts.Verbose {
		opts.Logger.Infof("Received shutdown signal, executing callback...")
	}

	// Execute callback with delay for slow return
	time.Sleep(opts.StepDelay)

	if err := callback(ctx); err != nil {
		return fmt.Errorf("shutdown callback error: %w", err)
	}

	// Final delay
	time.Sleep(opts.StepDelay)

	if opts.Verbose {
		opts.Logger.Infof("Shutdown callback completed successfully")
	}
	return nil
}
