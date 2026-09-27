package updater

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ryansonshine/rytrix-ddns-client/internal/config"
)

type fakeServer struct {
	ip       atomic.Value
	serverIP string
	token    string
	updates  atomic.Int32
}

func newFake(t *testing.T, token string) (*fakeServer, *httptest.Server) {
	f := &fakeServer{token: token}
	f.ip.Store("198.51.100.1")
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ip":
			io.WriteString(w, f.ip.Load().(string))
		case "/nic/update":
			user, pass, ok := r.BasicAuth()
			if !ok || user != r.URL.Query().Get("hostname") || pass != f.token {
				io.WriteString(w, "badauth")
				return
			}
			f.updates.Add(1)
			ip := f.ip.Load().(string)
			if ip == f.serverIP {
				io.WriteString(w, "nochg "+ip)
				return
			}
			f.serverIP = ip
			io.WriteString(w, "good "+ip)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func newUpdater(srv *httptest.Server, token string) *Updater {
	cfg := &config.Config{Server: srv.URL, Hostname: "home", Token: token, Interval: config.Duration{Duration: time.Minute}}
	u := New(cfg, "test", log.New(io.Discard, "", 0))
	u.client = srv.Client()
	return u
}

func TestUpdateOnlyWhenTheIPChanges(t *testing.T) {
	fake, srv := newFake(t, "secret")
	u := newUpdater(srv, "secret")
	ctx := context.Background()

	res, err := u.Update(ctx, false)
	if err != nil || !res.Changed || res.IP != "198.51.100.1" {
		t.Fatalf("first update: %+v, %v", res, err)
	}

	res, err = u.Update(ctx, false)
	if err != nil || !res.Skipped {
		t.Fatalf("unchanged IP should skip: %+v, %v", res, err)
	}

	fake.ip.Store("198.51.100.2")
	res, err = u.Update(ctx, false)
	if err != nil || !res.Changed || res.IP != "198.51.100.2" {
		t.Fatalf("changed IP: %+v, %v", res, err)
	}

	if got := fake.updates.Load(); got != 2 {
		t.Fatalf("server saw %d updates, want 2", got)
	}
}

func TestForcedUpdateReportsNoChange(t *testing.T) {
	_, srv := newFake(t, "secret")
	u := newUpdater(srv, "secret")
	ctx := context.Background()

	if _, err := u.Update(ctx, true); err != nil {
		t.Fatal(err)
	}
	res, err := u.Update(ctx, true)
	if err != nil || res.Changed || res.Skipped {
		t.Fatalf("forced repeat should be sent and report no change: %+v, %v", res, err)
	}
}

func TestStaleUpdateIsResent(t *testing.T) {
	fake, srv := newFake(t, "secret")
	u := newUpdater(srv, "secret")
	ctx := context.Background()

	if _, err := u.Update(ctx, false); err != nil {
		t.Fatal(err)
	}
	u.lastUpdate = time.Now().Add(-25 * time.Hour)
	if res, err := u.Update(ctx, false); err != nil || res.Skipped {
		t.Fatalf("a day-old update should be refreshed: %+v, %v", res, err)
	}
	if got := fake.updates.Load(); got != 2 {
		t.Fatalf("server saw %d updates, want 2", got)
	}
}

func TestBadTokenIsAConfigError(t *testing.T) {
	_, srv := newFake(t, "secret")
	u := newUpdater(srv, "wrong")

	_, err := u.Update(context.Background(), false)
	if !errors.Is(err, ErrConfig) {
		t.Fatalf("got %v, want ErrConfig", err)
	}
}

func TestRunStopsWithTheContext(t *testing.T) {
	fake, srv := newFake(t, "secret")
	u := newUpdater(srv, "secret")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		u.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for fake.updates.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run didn't return after cancel")
	}
	if fake.updates.Load() == 0 {
		t.Fatal("Run never sent an update")
	}
}
