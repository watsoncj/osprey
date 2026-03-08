//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
)

const unitPath = "/etc/systemd/system/osprey-server.service"

func installService(listen, dataDir, apiKey string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	cmd := fmt.Sprintf("%s -listen %s -data %s", exe, listen, dataDir)
	if apiKey != "" {
		cmd += " -api-key " + apiKey
	}

	unit := fmt.Sprintf(`[Unit]
Description=Osprey Server
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
`, cmd)

	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}

	for _, args := range [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", serviceName},
		{"systemctl", "start", serviceName},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %s: %w", args[0], out, err)
		}
	}
	return nil
}

func uninstallService() error {
	_ = exec.Command("systemctl", "stop", serviceName).Run()
	_ = exec.Command("systemctl", "disable", serviceName).Run()
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit file: %w", err)
	}
	_ = exec.Command("systemctl", "daemon-reload").Run()
	return nil
}
