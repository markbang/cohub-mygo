# Cohub MyGo

[English](README.md) · 简体中文

用 [MyGo](https://mygo.egoist.dev) 构建的 [Cohub](https://cohub.live) 桌面客户端，GPU 绘制，不使用 WebView。

## 下载

| 平台 | x64 | ARM64 |
| --- | --- | --- |
| macOS | [DMG](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo-0.0.1-amd64.dmg) | [DMG](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo-0.0.1-arm64.dmg) |
| Windows | [安装器](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo-setup-0.0.1-amd64.exe) | [安装器](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo-setup-0.0.1-arm64.exe) |
| Linux | [deb](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo_0.0.1_amd64.deb) · [tar.gz](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo-0.0.1-linux-amd64.tar.gz) | [deb](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo_0.0.1_arm64.deb) · [tar.gz](https://github.com/markbang/cohub-mygo/releases/download/v0.0.1/cohub-mygo-0.0.1-linux-arm64.tar.gz) |

macOS / Linux 也可一行安装：

```sh
curl -fsSL https://github.com/markbang/cohub-mygo/releases/latest/download/install.sh | sh
```

Windows 在 PowerShell 里：

```powershell
irm https://github.com/markbang/cohub-mygo/releases/latest/download/install.ps1 | iex
```

应用没有签名。从浏览器下载的话，第一次打开时放行一次即可：Windows 点 **更多信息 → 仍要运行**；macOS 到 **系统设置 → 隐私与安全性 → 仍要打开**。

## TODO

- [x] 缓存数据，加快启动。
- [x] 用 WebRTC 显示实时画面。
- [x] 在 macOS 和 Windows 上验证 GPU 渲染。
