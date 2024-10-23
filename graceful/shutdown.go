package graceful

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ShutDownSlowly will shutdown the process after delay time
// delay: time to wait before shutdown
// It will listen to SIGTERM, SIGINT, os.Interrupt signal
// It will log the signal received and the shutdown process
// It will log the delay time before shutdown
//
// Example:
//
//	graceful.ShutDownSlowly(5 * time.Second)
func ShutDownSlowly(delay time.Duration) {
	q := make(chan os.Signal, 1)
	signal.Notify(q, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	log.Printf("receive signal: %v\n", <-q)
	log.Printf("Shut down slowly in %v\n", delay)
	time.Sleep(delay)
}
