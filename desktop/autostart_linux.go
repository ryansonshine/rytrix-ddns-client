package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func autostartPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "autostart", "rytrix-ddns.desktop"), nil
}

// setAutostart writes an XDG autostart entry, which GNOME, KDE and most desktops read.
func setAutostart(enabled bool, exe string) error {
	path, err := autostartPath()
	if err != nil {
		return err
	}
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	// An AppImage runs from a temporary mount, so start the AppImage file itself.
	if appimage := os.Getenv("APPIMAGE"); appimage != "" {
		exe = appimage
	}
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=rytrix DDNS
Comment=Keeps your rytrix DDNS hostname pointed at this computer
Exec="%s"
X-GNOME-Autostart-enabled=true
`, strings.ReplaceAll(exe, `"`, `\"`))

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(entry), 0o644)
}

func autostartEnabled() bool {
	path, err := autostartPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}
