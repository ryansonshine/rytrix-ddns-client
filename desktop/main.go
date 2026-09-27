// Command RytrixDDNS is the tray app for rytrix DDNS.
package main

import (
	"context"
	"embed"
	"log"
	"runtime"
	"time"

	"github.com/ryansonshine/rytrix-ddns-client/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

var version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

//go:embed icons/tray-template.png
var trayTemplateIcon []byte

//go:embed icons/tray.png
var trayIcon []byte

func init() {
	application.RegisterEvent[Status]("status")
}

func main() {
	path, err := config.DefaultPath()
	if err != nil {
		log.Fatal(err)
	}

	var app *application.App
	var tray *trayMenu
	var showWindow func()
	ddns := NewDDNS(path, func(s Status) {
		if app == nil {
			return
		}
		app.Event.Emit("status", s)
		if tray != nil {
			application.InvokeSync(func() { tray.update(s) })
		}
	})

	app = application.New(application.Options{
		Name:        "rytrix DDNS",
		Description: "Keeps your rytrix DDNS hostname pointed at this computer's public IP",
		Services:    []application.Service{application.NewService(ddns)},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac: application.MacOptions{
			// A menu bar app: no Dock icon, and closing the window doesn't quit.
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
		// Opening the app again brings up the settings window instead of a second copy.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.rytrix.ddns.desktop",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if showWindow != nil {
					showWindow()
				}
			},
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "settings",
		Title:            "rytrix DDNS",
		Width:            460,
		Height:           620,
		DisableResize:    true,
		Hidden:           ddns.GetStatus().Configured,
		BackgroundColour: application.NewRGB(11, 16, 32),
		URL:              "/",
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 40,
		},
		Windows: application.WindowsWindow{HiddenOnTaskbar: false},
	})
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		window.Hide()
		e.Cancel()
	})

	showWindow = func() {
		window.Show()
		window.Focus()
	}

	tray = newTrayMenu(app, ddns, showWindow)
	tray.update(ddns.GetStatus())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ddns.run(ctx)

	// The menu shows the last check time, so refresh it now and then.
	go func() {
		for range time.Tick(time.Minute) {
			s := ddns.GetStatus()
			application.InvokeSync(func() { tray.update(s) })
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

type trayMenu struct {
	tray      *application.SystemTray
	menu      *application.Menu
	host      *application.MenuItem
	detail    *application.MenuItem
	startItem *application.MenuItem
	copyItem  *application.MenuItem
}

func newTrayMenu(app *application.App, ddns *DDNS, showWindow func()) *trayMenu {
	t := &trayMenu{tray: app.SystemTray.New(), menu: app.NewMenu()}
	if runtime.GOOS == "darwin" {
		t.tray.SetTemplateIcon(trayTemplateIcon)
	} else {
		t.tray.SetIcon(trayIcon)
	}

	t.host = t.menu.Add("").SetEnabled(false)
	t.detail = t.menu.Add("").SetEnabled(false)
	t.menu.AddSeparator()
	t.copyItem = t.menu.Add("Copy hostname").OnClick(func(*application.Context) { ddns.CopyHostname() })
	t.menu.Add("Update now").OnClick(func(*application.Context) { go ddns.UpdateNow() })
	t.menu.Add("Settings…").OnClick(func(*application.Context) { showWindow() })
	t.menu.Add("Open dashboard").OnClick(func(*application.Context) { ddns.OpenDashboard() })
	t.menu.AddSeparator()
	t.startItem = t.menu.AddCheckbox("Start at login", ddns.GetStatus().StartAtLogin).OnClick(func(ctx *application.Context) {
		go ddns.SetStartAtLogin(ctx.ClickedMenuItem().Checked())
	})
	t.menu.AddSeparator()
	t.menu.Add("Quit rytrix DDNS").OnClick(func(*application.Context) { app.Quit() })

	t.tray.SetMenu(t.menu)
	return t
}

func (t *trayMenu) update(s Status) {
	switch {
	case !s.Configured:
		t.host.SetLabel("Not set up yet")
		t.detail.SetLabel("Open Settings to add a host")
	case s.Error != "":
		t.host.SetLabel("⚠ " + s.FQDN())
		t.detail.SetLabel(shorten(s.Error, 60))
	case s.Busy:
		t.host.SetLabel(s.FQDN())
		t.detail.SetLabel("Checking…")
	default:
		t.host.SetLabel("● " + s.FQDN())
		t.detail.SetLabel(detailLine(s))
	}
	t.startItem.SetChecked(s.StartAtLogin)
	t.copyItem.SetEnabled(s.Configured)
	t.menu.Update()

	tip := "rytrix DDNS"
	if s.Configured {
		tip = s.FQDN() + " → " + orDash(s.IP)
	}
	t.tray.SetTooltip(tip)
}

func detailLine(s Status) string {
	line := orDash(s.IP)
	if checked, err := time.Parse(time.RFC3339, s.LastChecked); err == nil {
		line += " · checked " + checked.Format("15:04")
	}
	return line
}

func orDash(s string) string {
	if s == "" {
		return "–"
	}
	return s
}

func shorten(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
