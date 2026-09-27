// Package updater keeps a rytrix DDNS host pointed at this machine's public IP.
package updater

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ryansonshine/rytrix-ddns-client/internal/config"
)

// refreshEvery re-sends an unchanged IP so the server never expires the host.
const refreshEvery = 24 * time.Hour

// ErrConfig means the server rejected the hostname or token; retrying won't help.
var ErrConfig = errors.New("configuration rejected by server")

type Updater struct {
	cfg    *config.Config
	client *http.Client
	logger *log.Logger
	ua     string

	lastIP     string
	lastUpdate time.Time
}

func New(cfg *config.Config, version string, logger *log.Logger) *Updater {
	return &Updater{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
		logger: logger,
		ua:     "rytrix-ddns/" + version,
	}
}

// Result is what one Update call did.
type Result struct {
	IP      string
	Changed bool
	Skipped bool
}

// Update sends an update when the public IP changed or the last one is older than a day.
func (u *Updater) Update(ctx context.Context, force bool) (Result, error) {
	ip, err := u.PublicIP(ctx)
	if err != nil {
		u.logger.Printf("could not check public IP (%v); updating anyway", err)
	}

	if !force && ip != "" && ip == u.lastIP && time.Since(u.lastUpdate) < refreshEvery {
		return Result{IP: ip, Skipped: true}, nil
	}

	status, err := u.send(ctx)
	if err != nil {
		return Result{}, err
	}

	fields := strings.Fields(status)
	code := fields[0]
	switch code {
	case "good", "nochg":
		if len(fields) > 1 {
			ip = fields[1]
		}
		u.lastIP, u.lastUpdate = ip, time.Now()
		return Result{IP: ip, Changed: code == "good"}, nil
	case "badauth", "nohost", "notfqdn", "numhost":
		return Result{}, fmt.Errorf("%w: %s (check the hostname and token)", ErrConfig, code)
	default:
		return Result{}, fmt.Errorf("server answered %q", status)
	}
}

// PublicIP asks the server which address it sees us on.
func (u *Updater) PublicIP(ctx context.Context) (string, error) {
	body, _, err := u.get(ctx, u.cfg.Server+"/ip", false)
	return strings.TrimSpace(body), err
}

func (u *Updater) send(ctx context.Context) (string, error) {
	q := url.Values{"hostname": {u.cfg.Hostname}}
	body, code, err := u.get(ctx, u.cfg.Server+"/nic/update?"+q.Encode(), true)
	if err != nil && code != http.StatusUnauthorized {
		return "", err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return "", fmt.Errorf("empty response (HTTP %d)", code)
	}
	return body, nil
}

func (u *Updater) get(ctx context.Context, target string, auth bool) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("User-Agent", u.ua)
	if auth {
		req.SetBasicAuth(u.cfg.Hostname, u.cfg.Token)
	}

	res, err := u.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()

	b, err := io.ReadAll(io.LimitReader(res.Body, 4096))
	if err != nil {
		return "", res.StatusCode, err
	}
	if res.StatusCode != http.StatusOK {
		return string(b), res.StatusCode, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return string(b), res.StatusCode, nil
}

// Run updates on every interval until ctx ends. Failures back off up to the interval;
// a rejected hostname or token is retried hourly in case it's fixed on the server.
func (u *Updater) Run(ctx context.Context) {
	u.logger.Printf("keeping %s up to date via %s every %s", u.cfg.Hostname, u.cfg.Server, u.cfg.Interval)

	wait := time.Duration(0)
	backoff := 15 * time.Second
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}

		res, err := u.Update(ctx, false)
		switch {
		case errors.Is(err, ErrConfig):
			u.logger.Printf("update failed: %v; retrying in 1h", err)
			wait = time.Hour
		case err != nil:
			u.logger.Printf("update failed: %v; retrying in %s", err, backoff)
			wait = backoff
			backoff = min(backoff*2, u.cfg.Interval.Duration)
		default:
			if res.Changed {
				u.logger.Printf("updated %s to %s", u.cfg.Hostname, res.IP)
			} else if !res.Skipped {
				u.logger.Printf("confirmed %s at %s", u.cfg.Hostname, res.IP)
			}
			wait = u.cfg.Interval.Duration
			backoff = 15 * time.Second
		}
	}
}
