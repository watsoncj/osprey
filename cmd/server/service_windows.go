//go:build windows

package main

import (
	"fmt"
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
