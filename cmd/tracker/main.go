//go:build !windows

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	stopCh := make(chan struct{})

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		sig := <-sigCh
		log.Printf("received signal %s -- shutting down gracefully", sig)
		close(stopCh)
	}()

	runTracker(stopCh)
}
