package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// isWindowsService reports whether the process is running as a Windows service.
func isWindowsService() bool {
	is, _ := svc.IsWindowsService()
	return is
}

// agentService implements svc.Handler for the Windows SCM.
type agentService struct {
	daemonFn func(ctx context.Context)
}

func (s *agentService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.daemonFn(ctx)
		close(done)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		c := <-r
		switch c.Cmd {
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			cancel()
			<-done
			return false, 0
		case svc.Interrogate:
			changes <- c.CurrentStatus
		}
	}
}

// runWindowsService starts the process as a Windows service, using daemonFn
// as the main work loop. daemonFn must respect ctx cancellation to allow
// graceful shutdown.
func runWindowsService(daemonFn func(ctx context.Context)) error {
	return svc.Run(serviceName, &agentService{daemonFn: daemonFn})
}

func serviceArgs(serverURL, hostname string, interval, lookback time.Duration, apiKey, spoolDir, logFile string, skipVerify bool) []string {
	args := []string{"-server", serverURL, "-hostname", hostname, "-interval", interval.String(), "-lookback", lookback.String()}
	if apiKey != "" {
		args = append(args, "-api-key", apiKey)
	}
	if spoolDir != "" && spoolDir != "./spool" {
		args = append(args, "-spool", spoolDir)
	}
	if logFile != "" {
		args = append(args, "-logfile", logFile)
	}
	if skipVerify {
		args = append(args, "-skip-verify")
	}
	return args
}

func installService(serverURL, hostname string, interval, lookback time.Duration, apiKey, spoolDir, logFile string, skipVerify bool) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", serviceName)
	}

	s, err = m.CreateService(serviceName, exe, mgr.Config{
		DisplayName: "Osprey Agent",
		StartType:   mgr.StartAutomatic,
		Description: "Osprey browser history monitoring agent",
	}, serviceArgs(serverURL, hostname, interval, lookback, apiKey, spoolDir, logFile, skipVerify)...)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()

	// Restart the service after 10 seconds if it exits for any reason
	// (including clean exit from self-update).
	if err := s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 10 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
	}, 3600); err != nil {
		log.Printf("Warning: could not set recovery actions: %v", err)
	}
	// Apply recovery actions even on clean (non-crash) exits so that
	// self-update can exit(0) and the SCM restarts the new binary.
	if err := s.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		log.Printf("Warning: could not enable recovery on non-crash failures: %v", err)
	}

	if err := s.Start(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}
	return nil
}

func uninstallService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer s.Close()

	if _, err := s.Control(svc.Stop); err != nil {
		log.Printf("Warning: could not stop service: %v", err)
	}

	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	return nil
}
