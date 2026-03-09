//go:build linux

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

const (
	unitPath   = "/etc/systemd/system/osprey-server.service"
	installDir = "/opt/osprey"
	ospreyUser = "osprey"
)

func installService(listen, dataDir, apiKey string) error {
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
	dstExe := filepath.Join(installDir, "osprey-server")
	if err := copyFile(srcExe, dstExe); err != nil {
		return fmt.Errorf("copy binary: %w", err)
	}
	if err := os.Chmod(dstExe, 0o755); err != nil {
		return fmt.Errorf("chmod binary: %w", err)
	}

	// Data directory defaults to /opt/osprey/data.
	if dataDir == "./data" {
		dataDir = filepath.Join(installDir, "data")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	// Chown install dir tree to osprey user.
	chown := exec.Command("chown", "-R", ospreyUser+":"+ospreyUser, installDir)
	if out, err := chown.CombinedOutput(); err != nil {
		return fmt.Errorf("chown: %s: %w", out, err)
	}

	// Build ExecStart command.
	execStart := fmt.Sprintf("%s -listen %s -data %s", dstExe, listen, dataDir)
	if apiKey != "" {
		execStart += " -api-key " + apiKey
	}

	unit := fmt.Sprintf(`[Unit]
Description=Osprey Server
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

	openFirewall(listen)
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

// openFirewall attempts to allow the listen port through the firewall.
// It tries firewalld first, then falls back to ufw. Failures are logged
// but not treated as fatal since the firewall may not be active.
func openFirewall(listen string) {
	port := listen
	if i := strings.LastIndex(listen, ":"); i >= 0 {
		port = listen[i+1:]
	}

	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		cmd := exec.Command("firewall-cmd", "--permanent", "--add-port="+port+"/tcp")
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("firewall-cmd add-port: %s: %v", out, err)
		} else {
			_ = exec.Command("firewall-cmd", "--reload").Run()
			log.Printf("Firewall: opened port %s/tcp (firewalld)", port)
		}
		return
	}

	if _, err := exec.LookPath("ufw"); err == nil {
		cmd := exec.Command("ufw", "allow", port+"/tcp")
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("ufw allow: %s: %v", out, err)
		} else {
			log.Printf("Firewall: opened port %s/tcp (ufw)", port)
		}
	}
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
