package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"time"
)

const (
	unitPath   = "/etc/systemd/system/osprey-agent.service"
	installDir = "/opt/osprey"
	ospreyUser = "osprey"
)

func installService(serverURL, hostname string, interval, lookback time.Duration, apiKey, spoolDir, logFile string, skipVerify bool) error {
	// Create osprey system user if it doesn't exist.
	if _, err := user.Lookup(ospreyUser); err != nil {
		cmd := exec.Command("useradd", "--system", "--no-create-home", "--shell", "/usr/sbin/nologin", ospreyUser)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("create user: %s: %w", out, err)
		}
		log.Printf("Created system user %s", ospreyUser)
	}

	// Create install directory and copy binary.
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}

	srcExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	dstExe := filepath.Join(installDir, "osprey-agent")
	if err := copyFile(srcExe, dstExe); err != nil {
		return fmt.Errorf("copy binary: %w", err)
	}
	if err := os.Chmod(dstExe, 0o755); err != nil {
		return fmt.Errorf("chmod binary: %w", err)
	}

	// Default spool directory under install dir.
	if spoolDir == "" || spoolDir == "./spool" {
		spoolDir = filepath.Join(installDir, "spool")
	}
	if err := os.MkdirAll(spoolDir, 0o755); err != nil {
		return fmt.Errorf("create spool dir: %w", err)
	}

	// Chown install dir tree to osprey user.
	chown := exec.Command("chown", "-R", ospreyUser+":"+ospreyUser, installDir)
	if out, err := chown.CombinedOutput(); err != nil {
		return fmt.Errorf("chown: %s: %w", out, err)
	}

	// Build ExecStart command.
	execStart := fmt.Sprintf("%s -server %s -hostname %s -interval %s -lookback %s -spool %s",
		dstExe, serverURL, hostname, interval, lookback, spoolDir)
	if apiKey != "" {
		execStart += " -api-key " + apiKey
	}
	if logFile != "" {
		execStart += " -logfile " + logFile
	}
	if skipVerify {
		execStart += " -skip-verify"
	}

	unit := fmt.Sprintf(`[Unit]
Description=Osprey Agent
After=network.target

[Service]
Type=simple
User=%s
Group=%s
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
`, ospreyUser, ospreyUser, installDir, execStart)

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

// copyFile copies src to dst by reading and writing the content.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
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

func isWindowsService() bool                            { return false }
func runWindowsService(func(ctx context.Context)) error { return nil }
