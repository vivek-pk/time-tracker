//go:build windows

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/windows/svc"
)

// timeTrackerService implements svc.Handler for the Windows SCM.
type timeTrackerService struct{}

func (s *timeTrackerService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		runTracker(stopCh)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				close(stopCh)
				<-doneCh
				return
			}
		case <-doneCh:
			return
		}
	}
}

func main() {
	// Detect whether we are running as a Windows Service or interactively.
	isService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("failed to determine if running as service: %v", err)
	}

	if isService {
		// Running under the SCM -- hand off to the service handler.
		err = svc.Run("TimeTracker", &timeTrackerService{})
		if err != nil {
			log.Fatalf("service failed: %v", err)
		}
		return
	}

	// Running interactively (e.g. from a terminal).
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
