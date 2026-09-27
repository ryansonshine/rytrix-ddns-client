package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ryansonshine/rytrix-ddns-client/internal/config"
	"github.com/ryansonshine/rytrix-ddns-client/internal/updater"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Status is everything the window and tray show.
type Status struct {
	Configured   bool   `json:"configured"`
	Hostname     string `json:"hostname"`
	Domain       string `json:"domain"`
	Server       string `json:"server"`
	IP           string `json:"ip"`
	LastChecked  string `json:"lastChecked"`
	Error        string `json:"error"`
	Busy         bool   `json:"busy"`
	StartAtLogin bool   `json:"startAtLogin"`
	Version      string `json:"version"`
}

func (s Status) FQDN() string {
	if s.Hostname == "" {
		return ""
	}
	return s.Hostname + "." + s.Domain
}

// DDNS runs the updater in the background and is bound to the frontend, so its exported
// methods are callable from the settings window.
type DDNS struct {
	mu       sync.Mutex
	path     string
	cfg      *config.Config
	up       *updater.Updater
	status   Status
	wake     chan bool
	onChange func(Status)
	logger   *log.Logger
}

func NewDDNS(path string, onChange func(Status)) *DDNS {
	d := &DDNS{
		path:     path,
		wake:     make(chan bool, 1),
		onChange: onChange,
		logger:   log.New(os.Stderr, "", log.LstdFlags),
		status:   Status{Domain: "d.rytrix.com", Server: config.DefaultServer, Version: version},
	}
	if cfg, err := config.Load(path); err == nil {
		d.useConfig(cfg)
	}
	d.status.StartAtLogin = autostartEnabled()
	return d
}

func (d *DDNS) useConfig(cfg *config.Config) {
	d.cfg = cfg
	d.up = updater.New(cfg, version, d.logger)
	d.status.Configured = true
	d.status.Hostname = cfg.Hostname
	d.status.Server = cfg.Server
}

// run checks on every interval and whenever UpdateNow or Save asks it to.
func (d *DDNS) run(ctx context.Context) {
	go d.lookUpDomain(ctx)
	backoff := 15 * time.Second
	wait := time.Duration(0)
	for {
		force := false
		select {
		case <-ctx.Done():
			return
		case force = <-d.wake:
		case <-time.After(wait):
		}

		interval, err := d.check(ctx, force)
		switch {
		case errors.Is(err, updater.ErrConfig):
			wait = time.Hour
		case err != nil:
			wait = backoff
			backoff = min(backoff*2, interval)
		default:
			wait = interval
			backoff = 15 * time.Second
		}
	}
}

func (d *DDNS) check(ctx context.Context, force bool) (time.Duration, error) {
	d.mu.Lock()
	up, cfg := d.up, d.cfg
	if up == nil {
		d.mu.Unlock()
		return time.Hour, nil
	}
	d.status.Busy = true
	d.mu.Unlock()
	d.changed()

	res, err := up.Update(ctx, force)

	d.mu.Lock()
	d.status.Busy = false
	d.status.LastChecked = time.Now().Format(time.RFC3339)
	if err != nil {
		d.status.Error = err.Error()
	} else {
		d.status.Error = ""
		if res.IP != "" {
			d.status.IP = res.IP
		}
	}
	d.mu.Unlock()
	d.changed()
	return cfg.Interval.Duration, err
}

func (d *DDNS) changed() {
	if d.onChange != nil {
		d.onChange(d.GetStatus())
	}
}

// lookUpDomain asks the server for its zone, so the app shows the right FQDN.
func (d *DDNS) lookUpDomain(ctx context.Context) {
	d.mu.Lock()
	server := d.status.Server
	d.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server+"/api/session", nil)
	if err != nil {
		return
	}
	res, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	var body struct {
		Domain string `json:"domain"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&body) == nil && body.Domain != "" {
		d.mu.Lock()
		d.status.Domain = body.Domain
		d.mu.Unlock()
		d.changed()
	}
}

func (d *DDNS) GetStatus() Status {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.status
}

// Save checks the hostname and token with a real update before keeping them.
func (d *DDNS) Save(hostname, token string) (Status, error) {
	cfg := &config.Config{Hostname: hostname, Token: token}
	if err := cfg.Prepare(); err != nil {
		return d.GetStatus(), err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := updater.New(cfg, version, d.logger).Update(ctx, true)
	if err != nil {
		return d.GetStatus(), err
	}
	if err := config.Save(d.path, cfg); err != nil {
		return d.GetStatus(), err
	}

	d.mu.Lock()
	d.useConfig(cfg)
	d.status.IP = res.IP
	d.status.Error = ""
	d.status.LastChecked = time.Now().Format(time.RFC3339)
	d.mu.Unlock()
	d.changed()
	d.poke(false)
	return d.GetStatus(), nil
}

// UpdateNow sends an update even if the IP hasn't changed.
func (d *DDNS) UpdateNow() Status {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	d.check(ctx, true)
	return d.GetStatus()
}

func (d *DDNS) SetStartAtLogin(enabled bool) (Status, error) {
	exe, err := os.Executable()
	if err != nil {
		return d.GetStatus(), err
	}
	if err := setAutostart(enabled, exe); err != nil {
		return d.GetStatus(), err
	}
	d.mu.Lock()
	d.status.StartAtLogin = autostartEnabled()
	d.mu.Unlock()
	d.changed()
	return d.GetStatus(), nil
}

// Forget deletes the saved hostname and token. The host itself stays registered.
func (d *DDNS) Forget() (Status, error) {
	if err := os.Remove(d.path); err != nil && !os.IsNotExist(err) {
		return d.GetStatus(), err
	}
	d.mu.Lock()
	d.cfg, d.up = nil, nil
	d.status = Status{Domain: d.status.Domain, Server: config.DefaultServer, Version: version, StartAtLogin: d.status.StartAtLogin}
	d.mu.Unlock()
	d.changed()
	return d.GetStatus(), nil
}

// CopyHostname puts the full hostname, e.g. home.d.rytrix.com, on the clipboard.
func (d *DDNS) CopyHostname() bool {
	fqdn := d.GetStatus().FQDN()
	return fqdn != "" && application.Get().Clipboard.SetText(fqdn)
}

func (d *DDNS) OpenDashboard() {
	application.Get().Browser.OpenURL(d.GetStatus().Server)
}

func (d *DDNS) poke(force bool) {
	select {
	case d.wake <- force:
	default:
	}
}
