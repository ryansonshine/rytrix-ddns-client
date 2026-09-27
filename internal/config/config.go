// Package config loads and saves the client's settings file.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultServer = "https://ddns.rytrix.com"

type Config struct {
	Server   string   `json:"server"`
	Hostname string   `json:"hostname"`
	Token    string   `json:"token"`
	Interval Duration `json:"interval"`
}

// Duration marshals as a Go duration string such as "5m".
type Duration struct{ time.Duration }

func (d Duration) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = v
	return nil
}

// DefaultPath is <user config dir>/rytrix-ddns/config.json.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rytrix-ddns", "config.json"), nil
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no config at %s: run `rytrix-ddns setup` first", path)
	}
	if err != nil {
		return nil, err
	}

	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	c.applyDefaults()
	return &c, c.Validate()
}

// Save writes the file readable by its owner only, because it holds the token.
func Save(path string, c *Config) error {
	c.applyDefaults()
	if err := c.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

func (c *Config) applyDefaults() {
	if c.Server == "" {
		c.Server = DefaultServer
	}
	c.Server = strings.TrimRight(c.Server, "/")
	c.Hostname = ShortHostname(c.Hostname)
	if c.Interval.Duration == 0 {
		c.Interval.Duration = 5 * time.Minute
	}
}

func (c *Config) Validate() error {
	u, err := url.Parse(c.Server)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("server must be an https:// URL, got %q", c.Server)
	}
	if c.Hostname == "" {
		return errors.New("hostname is required")
	}
	if c.Token == "" {
		return errors.New("token is required")
	}
	if c.Interval.Duration < time.Minute {
		return errors.New("interval must be at least 1m")
	}
	return nil
}

// ShortHostname accepts "home" or "home.d.rytrix.com" and returns "home".
func ShortHostname(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if i := strings.IndexByte(name, '.'); i >= 0 {
		name = name[:i]
	}
	return name
}
