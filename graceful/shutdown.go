package graceful

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Logger interface {
	Infof(format string, args ...any)
}

// ShutDownSlowly will shutdown the process after delay time
// delay: time to wait before shutdown
// It will listen to SIGTERM, SIGINT, os.Interrupt signal
// It will log the signal received and the shutdown process
// It will log the delay time before shutdown
//
// Example:
//
//	graceful.ShutDownSlowly(logger, 5 * time.Second)
func ShutDownSlowly(logger Logger, delay time.Duration) {
	q := make(chan os.Signal, 1)
	signal.Notify(q, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	logger.Infof("receive signal: %v\n", <-q)
	logger.Infof("Shut down slowly in %v\n", delay)
	time.Sleep(delay)
}
