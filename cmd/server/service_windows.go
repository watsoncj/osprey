//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

func installService(listen, dataDir, apiKey string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	binArgs := fmt.Sprintf(`"%s" -listen %s -data "%s"`, exe, listen, dataDir)
	if apiKey != "" {
		binArgs += " -api-key " + apiKey
	}

	cmd := exec.Command("sc", "create", serviceName,
		"binPath=", binArgs,
		"start=", "auto",
		"DisplayName=", "Osprey Server",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sc create: %s: %w", out, err)
	}

	// Configure recovery: restart after 10s/30s/60s, including on clean exit
	// (needed for self-update which exits with code 0).
	cmd = exec.Command("sc", "failure", serviceName, "reset=", "3600", "actions=", "restart/10000/restart/30000/restart/60000")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Warning: sc failure config: %s: %v", out, err)
	}
	cmd = exec.Command("sc", "failureflag", serviceName, "1")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Warning: sc failureflag: %s: %v", out, err)
	}

	cmd = exec.Command("sc", "start", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sc start: %s: %w", out, err)
	}
	return nil
}

func uninstallService() error {
	_ = exec.Command("sc", "stop", serviceName).Run()
	cmd := exec.Command("sc", "delete", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sc delete: %s: %w", out, err)
	}
	return nil
}
