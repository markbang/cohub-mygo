# Cohub MyGo

English · [简体中文](README.zh-CN.md)

A [Cohub](https://cohub.live) desktop client built with [MyGo](https://mygo.egoist.dev), rendered on the GPU without a WebView.

## Download

| Platform | x64 | ARM64 |
| --- | --- | --- |
| macOS | [DMG](https://github.com/markbang/cohub-mygo/releases/download/v0.0.2/cohub-mygo-0.0.2-amd64.dmg) | [DMG](https://github.com/markbang/cohub-mygo/releases/download/v0.0.2/cohub-mygo-0.0.2-arm64.dmg) |
| Windows | [Installer](https://github.com/markbang/cohub-mygo/releases/download/v0.0.2/cohub-mygo-setup-0.0.2-amd64.exe) | [Installer](https://github.com/markbang/cohub-mygo/releases/download/v0.0.2/cohub-mygo-setup-0.0.2-arm64.exe) |
| Linux | [deb](https://github.com/markbang/cohub-mygo/releases/download/v0.0.2/cohub-mygo_0.0.2_amd64.deb) · [tar.gz](https://github.com/markbang/cohub-mygo/releases/download/v0.0.2/cohub-mygo-0.0.2-linux-amd64.tar.gz) | [deb](https://github.com/markbang/cohub-mygo/releases/download/v0.0.2/cohub-mygo_0.0.2_arm64.deb) · [tar.gz](https://github.com/markbang/cohub-mygo/releases/download/v0.0.2/cohub-mygo-0.0.2-linux-arm64.tar.gz) |

Or install on macOS / Linux:

```sh
curl -fsSL https://github.com/markbang/cohub-mygo/releases/latest/download/install.sh | sh
```

On Windows, in PowerShell:

```powershell
irm https://github.com/markbang/cohub-mygo/releases/latest/download/install.ps1 | iex
```

The apps aren't signed. If you download from the browser, allow the app the first time you open it: on Windows, **More info → Run anyway**; on macOS, **System Settings → Privacy & Security → Open Anyway**.

## TODO

- [x] Cache data for faster startup.
- [x] Use WebRTC for live displays.
- [x] Test GPU rendering on macOS and Windows.
