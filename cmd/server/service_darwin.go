//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const plistLabel = "com.osprey.server"

func plistPath() string {
	return filepath.Join("/Library/LaunchDaemons", plistLabel+".plist")
}

func installService(listen, dataDir, apiKey string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	apiKeyArgs := ""
	if apiKey != "" {
		apiKeyArgs = fmt.Sprintf(`
        <string>-api-key</string>
        <string>%s</string>`, apiKey)
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>-listen</string>
        <string>%s</string>
        <string>-data</string>
        <string>%s</string>%s
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardErrorPath</key>
    <string>/var/log/osprey-server.log</string>
</dict>
</plist>`, plistLabel, exe, listen, dataDir, apiKeyArgs)

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
