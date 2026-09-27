// Package service installs the client so it starts on boot or login: a launchd agent on
// macOS, a systemd unit on Linux, and a Windows service on Windows.
package service

import (
	"os/exec"
	"strings"
)

const (
	Name        = "rytrix-ddns"
	DisplayName = "rytrix DDNS client"
	Description = "Keeps a rytrix DDNS hostname pointed at this machine's public IP."
)

// runArgs is the command line every service definition starts the client with.
func runArgs(configPath string) []string {
	return []string{"run", "--config", configPath}
}

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return &commandError{cmd: name + " " + strings.Join(args, " "), out: strings.TrimSpace(string(out)), err: err}
	}
	return nil
}

type commandError struct {
	cmd, out string
	err      error
}

func (e *commandError) Error() string {
	if e.out != "" {
		return e.cmd + ": " + e.out
	}
	return e.cmd + ": " + e.err.Error()
}
