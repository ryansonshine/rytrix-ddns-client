# rytrix-ddns

The desktop client for [rytrix DDNS](https://ddns.rytrix.com). It keeps a hostname such as `home.d.rytrix.com`
pointed at your public IP, and runs in the background on macOS, Windows and Linux.

It checks your IP every 5 minutes and sends an update only when it changes, plus once a day so the host never
expires.

## Install

**macOS and Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/ryansonshine/rytrix-ddns-client/main/install.sh | sh
```

**Windows** (PowerShell)

```powershell
irm https://raw.githubusercontent.com/ryansonshine/rytrix-ddns-client/main/install.ps1 | iex
```

Both scripts download the latest release, check its SHA-256 checksum, and put `rytrix-ddns` on your PATH. You can
also download an archive from [Releases](https://github.com/ryansonshine/rytrix-ddns-client/releases) yourself.

## Set up

Register a host at [ddns.rytrix.com](https://ddns.rytrix.com) and copy its token. Then:

```sh
rytrix-ddns setup      # asks for the hostname and token, then sends a test update
rytrix-ddns install    # starts it automatically from now on
```

`install` uses each system's own mechanism:

| OS | How it runs | Logs |
| --- | --- | --- |
| macOS | a login agent (`~/Library/LaunchAgents/com.rytrix.ddns.plist`) | `~/Library/Logs/rytrix-ddns.log` |
| Linux | a systemd unit. System-wide with `sudo`, per-user otherwise | `journalctl -u rytrix-ddns` |
| Windows | a Windows service. Run `install` from an administrator prompt | `%AppData%\rytrix-ddns\rytrix-ddns.log` |

On Linux without systemd (Unraid, Alpine), start `rytrix-ddns run` from your init system or a `@reboot` cron entry.

## Commands

| Command | What it does |
| --- | --- |
| `setup` | Saves the hostname and token (`--hostname`, `--token`, `--interval` skip the prompts) |
| `update` | Sends one update now |
| `run` | Keeps updating in the foreground. This is what the service runs |
| `install` / `uninstall` | Starts or stops running automatically |
| `version` | Prints the version |

The settings live in your user config directory as `rytrix-ddns/config.json`, readable only by you because it holds
the token. Pass `--config PATH` to any command to use another file.

## Notes

- Releases aren't code-signed. The install scripts download with `curl` and PowerShell, which don't trigger the
  macOS Gatekeeper or Windows SmartScreen prompts that a browser download does.
- The client only talks to the server over HTTPS.
