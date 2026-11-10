# Cohub MyGo

English · [简体中文](README.zh-CN.md)

A [Cohub](https://cohub.live) desktop client built with [MyGo](https://mygo.egoist.dev), rendered on the GPU without a WebView.

## Download

| Platform | x64 | ARM64 |
| --- | --- | --- |
| macOS | [DMG](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo-0.0.1-amd64.dmg) | [DMG](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo-0.0.1-arm64.dmg) |
| Windows | [Installer](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo-setup-0.0.1-amd64.exe) | [Installer](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo-setup-0.0.1-arm64.exe) |
| Linux | [deb](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo_0.0.1_amd64.deb) · [tar.gz](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo-0.0.1-linux-amd64.tar.gz) | [deb](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo_0.0.1_arm64.deb) · [tar.gz](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo-0.0.1-linux-arm64.tar.gz) |

Or install on macOS / Linux:

```sh
curl -fsSL https://github.com/markbang/cohub-mygo/releases/latest/download/install.sh | sh
```

## TODO

- [x] Cache data for faster startup.
- [ ] Sync read/unread status across devices.
- [ ] Use WebRTC for live displays.
- [ ] Test GPU rendering on macOS and Windows.
- [ ] Add macOS notarization and Windows code signing.
