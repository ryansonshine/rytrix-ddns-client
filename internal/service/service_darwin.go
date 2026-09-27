package service

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const label = "com.rytrix.ddns"

// Run with sudo, install writes a LaunchDaemon that starts at boot, for a Mac nobody logs
// in to. Otherwise it writes a LaunchAgent, which needs no sudo but only runs while the
// user is logged in.
func system() bool { return os.Geteuid() == 0 }

func plistPath() (string, error) {
	if system() {
		return "/Library/LaunchDaemons/" + label + ".plist", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
}

func logPath() string {
	if system() {
		return "/Library/Logs/rytrix-ddns.log"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Logs", "rytrix-ddns.log")
}

func kind() string {
	if system() {
		return "boot daemon"
	}
	return "login agent"
}

func Install(exe, configPath string) (string, error) {
	path, err := plistPath()
	if err != nil {
		return "", err
	}

	var args bytes.Buffer
	for _, a := range append([]string{exe}, runArgs(configPath)...) {
		args.WriteString("\n    <string>")
		xml.EscapeText(&args, []byte(a))
		args.WriteString("</string>")
	}
	var logFile bytes.Buffer
	xml.EscapeText(&logFile, []byte(logPath()))

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
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, label, args.String(), logFile.String(), logFile.String())

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	// Reinstalling replaces a running agent, so drop any old one first.
	_ = run("launchctl", "bootout", domain()+"/"+label)
	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return "", err
	}
	if err := run("launchctl", "bootstrap", domain(), path); err != nil {
		return "", err
	}
	return fmt.Sprintf("Installed a %s (%s). Logs go to %s.", kind(), path, logPath()), nil
}

func Uninstall() (string, error) {
	path, err := plistPath()
	if err != nil {
		return "", err
	}
	_ = run("launchctl", "bootout", domain()+"/"+label)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return "Removed the " + kind() + ".", nil
}

func domain() string {
	if system() {
		return "system"
	}
	return "gui/" + strconv.Itoa(os.Getuid())
}

// RunManaged is only needed on Windows, where the service manager drives the process.
func RunManaged(func(context.Context)) (bool, error) { return false, nil }
