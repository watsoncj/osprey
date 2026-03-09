package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const plistLabel = "com.osprey.agent"

func plistPath() string {
	return filepath.Join("/Library/LaunchDaemons", plistLabel+".plist")
}

func installService(serverURL, hostname string, interval, lookback time.Duration, apiKey, spoolDir, logFile string, skipVerify bool) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	args := []string{exe, "-server", serverURL, "-hostname", hostname, "-interval", interval.String(), "-lookback", lookback.String()}
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

	var argStrings string
	for _, a := range args {
		argStrings += fmt.Sprintf("\n        <string>%s</string>", a)
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>%s
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardErrorPath</key>
    <string>/var/log/osprey-agent.log</string>
</dict>
</plist>`, plistLabel, argStrings)

	if err := os.WriteFile(plistPath(), []byte(plist), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	cmd := exec.Command("launchctl", "load", plistPath())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load: %s: %w", out, err)
	}
	return nil
}

func uninstallService() error {
	_ = exec.Command("launchctl", "unload", plistPath()).Run()
	if err := os.Remove(plistPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}
	return nil
}

func isWindowsService() bool                          { return false }
func runWindowsService(func(ctx context.Context)) error { return nil }
