package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	in := &Config{Hostname: "Home.d.rytrix.com", Token: "tok"}
	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}

	out, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if out.Server != DefaultServer || out.Hostname != "home" || out.Token != "tok" || out.Interval.Duration != 5*time.Minute {
		t.Fatalf("round trip: %+v", out)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("config holds the token, perms = %o, want 600", perm)
		}
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]Config{
		"http server":  {Server: "http://ddns.rytrix.com", Hostname: "home", Token: "t", Interval: Duration{time.Minute}},
		"no hostname":  {Server: DefaultServer, Token: "t", Interval: Duration{time.Minute}},
		"no token":     {Server: DefaultServer, Hostname: "home", Interval: Duration{time.Minute}},
		"too frequent": {Server: DefaultServer, Hostname: "home", Token: "t", Interval: Duration{time.Second}},
	}
	for name, c := range cases {
		if err := c.Validate(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestLoadMissingFileSaysToRunSetup(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil || !strings.Contains(err.Error(), "setup") {
		t.Fatalf("got %v", err)
	}
}
