package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func serviceArgs(serverURL, hostname string, interval, lookback time.Duration, apiKey string) []string {
	args := []string{"-server", serverURL, "-hostname", hostname, "-interval", interval.String(), "-lookback", lookback.String()}
	if apiKey != "" {
		args = append(args, "-api-key", apiKey)
	}
	return args
}

func installService(serverURL, hostname string, interval, lookback time.Duration, apiKey string) error {
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
	}, serviceArgs(serverURL, hostname, interval, lookback, apiKey)...)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()

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
