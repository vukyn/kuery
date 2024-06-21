package graceful

import (
	"github.com/vukyn/kuery/log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func ShutDownSlowly(delay time.Duration) {
	log := log.New("graceful", "ShutDownSlowly")

	q := make(chan os.Signal, 1)
	signal.Notify(q, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	log.Debug("receive signal: %v", <-q)
	log.Debug("ShutDownSlowly")
	time.Sleep(delay)
}
