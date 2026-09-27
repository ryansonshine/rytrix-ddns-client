// Command rytrix-ddns keeps a rytrix DDNS hostname pointed at this machine's public IP.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/ryansonshine/rytrix-ddns-client/internal/config"
	"github.com/ryansonshine/rytrix-ddns-client/internal/service"
	"github.com/ryansonshine/rytrix-ddns-client/internal/updater"
	"golang.org/x/term"
)

var version = "dev"

const usage = `rytrix-ddns keeps a rytrix DDNS hostname pointed at this machine's public IP.

Usage:
  rytrix-ddns setup       Save your hostname and token, and check they work
  rytrix-ddns update      Send one update now
  rytrix-ddns run         Keep updating in the foreground (what the service runs)
  rytrix-ddns install     Start automatically at boot or login
  rytrix-ddns uninstall   Stop starting automatically
  rytrix-ddns version     Print the version

Every command accepts --config PATH (default %s).
`

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}

	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "setup":
		err = setup(args)
	case "update":
		err = update(args)
	case "run":
		err = runCmd(args)
	case "install":
		err = install(args)
	case "uninstall":
		err = uninstall(args)
	case "version", "--version", "-v":
		fmt.Println("rytrix-ddns", version)
	case "help", "--help", "-h":
		printUsage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		printUsage(os.Stderr)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func printUsage(w io.Writer) {
	path, _ := config.DefaultPath()
	fmt.Fprintf(w, usage, path)
}

// flags parses the options shared by every command.
func flags(name string, args []string, extra func(*flag.FlagSet)) (string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	// Services run without $HOME, so there may be no default; they always pass --config.
	defaultPath, defaultErr := config.DefaultPath()
	path := fs.String("config", defaultPath, "path to the config file")
	if extra != nil {
		extra(fs)
	}
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if *path == "" {
		return "", fmt.Errorf("no --config given and no default location: %w", defaultErr)
	}
	return filepath.Abs(*path)
}

func setup(args []string) error {
	var server, hostname, token string
	var interval time.Duration
	path, err := flags("setup", args, func(fs *flag.FlagSet) {
		fs.StringVar(&server, "server", config.DefaultServer, "the rytrix DDNS server")
		fs.StringVar(&hostname, "hostname", "", "the host to keep updated, e.g. home or home.d.rytrix.com")
		fs.StringVar(&token, "token", "", "the host's token from the dashboard")
		fs.DurationVar(&interval, "interval", 5*time.Minute, "how often to check the public IP")
	})
	if err != nil {
		return err
	}

	in := bufio.NewReader(os.Stdin)
	if hostname == "" {
		if hostname, err = prompt(in, "Hostname (e.g. home.d.rytrix.com): "); err != nil {
			return err
		}
	}
	if token == "" {
		if token, err = promptSecret(in, "Token: "); err != nil {
			return err
		}
	}

	cfg := &config.Config{Server: server, Hostname: hostname, Token: strings.TrimSpace(token), Interval: config.Duration{Duration: interval}}
	if err := config.Save(path, cfg); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := updater.New(cfg, version, log.New(io.Discard, "", 0)).Update(ctx, true)
	if err != nil {
		return fmt.Errorf("saved %s, but the test update failed: %w", path, err)
	}

	fmt.Printf("Saved %s.\n%s now points at %s.\n\nNext: run `rytrix-ddns install` to keep it updated automatically.\n", path, cfg.Hostname, res.IP)
	return nil
}

func update(args []string) error {
	path, err := flags("update", args, nil)
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := updater.New(cfg, version, log.New(io.Discard, "", 0)).Update(ctx, true)
	if err != nil {
		return err
	}
	if res.Changed {
		fmt.Printf("Updated %s to %s.\n", cfg.Hostname, res.IP)
	} else {
		fmt.Printf("%s already points at %s.\n", cfg.Hostname, res.IP)
	}
	return nil
}

func runCmd(args []string) error {
	path, err := flags("run", args, nil)
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	out, closeLog := logOutput(path)
	defer closeLog()
	logger := log.New(out, "", log.LstdFlags)
	u := updater.New(cfg, version, logger)

	if managed, err := service.RunManaged(u.Run); managed || err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	u.Run(ctx)
	return nil
}

// logOutput also writes to a file next to the config on Windows, where a service has no
// console. The file is emptied once it passes 5 MB.
func logOutput(configPath string) (io.Writer, func()) {
	if runtime.GOOS != "windows" {
		return os.Stderr, func() {}
	}
	path := filepath.Join(filepath.Dir(configPath), "rytrix-ddns.log")
	if info, err := os.Stat(path); err == nil && info.Size() > 5<<20 {
		os.Truncate(path, 0)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return os.Stderr, func() {}
	}
	return io.MultiWriter(os.Stderr, f), func() { f.Close() }
}

func install(args []string) error {
	path, err := flags("install", args, nil)
	if err != nil {
		return err
	}
	if _, err := config.Load(path); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	msg, err := service.Install(exe, path)
	if err != nil {
		return err
	}
	fmt.Println(msg)
	return nil
}

func uninstall(args []string) error {
	if _, err := flags("uninstall", args, nil); err != nil {
		return err
	}
	msg, err := service.Uninstall()
	if err != nil {
		return err
	}
	fmt.Println(msg)
	return nil
}

func prompt(in *bufio.Reader, label string) (string, error) {
	fmt.Print(label)
	line, err := in.ReadString('\n')
	if err != nil && !(errors.Is(err, io.EOF) && line != "") {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// promptSecret hides the token as it's typed when stdin is a terminal.
func promptSecret(in *bufio.Reader, label string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return prompt(in, label)
	}
	fmt.Print(label)
	b, err := term.ReadPassword(fd)
	fmt.Println()
	return strings.TrimSpace(string(b)), err
}
