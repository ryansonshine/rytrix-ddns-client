package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Run as root, install writes a system unit that starts at boot. Otherwise it writes a
// user unit, which only runs at boot once lingering is enabled for the user.
func unitPath() (path string, system bool, err error) {
	if os.Geteuid() == 0 {
		return "/etc/systemd/system/" + Name + ".service", true, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", false, err
	}
	return filepath.Join(dir, "systemd", "user", Name+".service"), false, nil
}

func systemctl(system bool, args ...string) error {
	if !system {
		args = append([]string{"--user"}, args...)
	}
	return run("systemctl", args...)
}

func Install(exe, configPath string) (string, error) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return "", errors.New("systemd isn't available here; start `rytrix-ddns run` from your init system or cron (@reboot) instead")
	}
	path, system, err := unitPath()
	if err != nil {
		return "", err
	}

	wantedBy := "default.target"
	if system {
		wantedBy = "multi-user.target"
	}
	quoted := []string{systemdQuote(exe)}
	for _, a := range runArgs(configPath) {
		quoted = append(quoted, systemdQuote(a))
	}
	unit := fmt.Sprintf(`[Unit]
Description=%s
Wants=network-online.target
After=network-online.target

[Service]
ExecStart=%s
Restart=always
RestartSec=30

[Install]
WantedBy=%s
`, DisplayName, strings.Join(quoted, " "), wantedBy)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(unit), 0o644); err != nil {
		return "", err
	}
	if err := systemctl(system, "daemon-reload"); err != nil {
		return "", err
	}
	if err := systemctl(system, "enable", "--now", Name); err != nil {
		return "", err
	}

	if system {
		return fmt.Sprintf("Installed and started %s. Logs: journalctl -u %s", path, Name), nil
	}
	return fmt.Sprintf("Installed and started %s. Logs: journalctl --user -u %s\n"+
		"To keep it running after you log out and start it at boot, run: sudo loginctl enable-linger %s",
		path, Name, os.Getenv("USER")), nil
}

func Uninstall() (string, error) {
	path, system, err := unitPath()
	if err != nil {
		return "", err
	}
	_ = systemctl(system, "disable", "--now", Name)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	_ = systemctl(system, "daemon-reload")
	return "Stopped and removed " + path + ".", nil
}

// systemdQuote quotes an ExecStart argument, since paths may contain spaces.
func systemdQuote(s string) string {
	if !strings.ContainsAny(s, " \t\"'\\$%") {
		return s
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `$$`, `%`, `%%`)
	return `"` + r.Replace(s) + `"`
}

func RunManaged(func(context.Context)) (bool, error) { return false, nil }
