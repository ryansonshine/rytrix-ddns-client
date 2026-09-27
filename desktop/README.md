# rytrix DDNS desktop app

A menu bar (macOS) and system tray (Windows, Linux) app that keeps a rytrix DDNS hostname pointed at this computer.
It uses the same updater and config file as the `rytrix-ddns` command-line client, so either one can set up the other.

Built with [Wails v3](https://v3.wails.io) (Go and a React/Tailwind UI in the system webview).

## Develop

Needs Go 1.26, Node 22+, and on Linux `libgtk-3-dev libwebkit2gtk-4.1-dev`.

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.26
wails3 dev              # live-reloading window
wails3 task package     # .app/.dmg on macOS, installer on Windows, AppImage and .deb on Linux
```
