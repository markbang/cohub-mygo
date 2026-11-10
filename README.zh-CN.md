# Cohub MyGo

[English](README.md) · 简体中文

用 [MyGo](https://mygo.egoist.dev) 构建的 [Cohub](https://cohub.live) 桌面客户端，GPU 绘制，不使用 WebView。

## 下载

| 平台 | x64 | ARM64 |
| --- | --- | --- |
| macOS | [DMG](https://github.com/markbang/cohub-mygo/releases/download/v0.0.3/cohub-mygo-0.0.3-amd64.dmg) | [DMG](https://github.com/markbang/cohub-mygo/releases/download/v0.0.3/cohub-mygo-0.0.3-arm64.dmg) |
| Windows | [安装器](https://github.com/markbang/cohub-mygo/releases/download/v0.0.3/cohub-mygo-setup-0.0.3-amd64.exe) | [安装器](https://github.com/markbang/cohub-mygo/releases/download/v0.0.3/cohub-mygo-setup-0.0.3-arm64.exe) |
| Linux | [deb](https://github.com/markbang/cohub-mygo/releases/download/v0.0.3/cohub-mygo_0.0.3_amd64.deb) · [tar.gz](https://github.com/markbang/cohub-mygo/releases/download/v0.0.3/cohub-mygo-0.0.3-linux-amd64.tar.gz) | [deb](https://github.com/markbang/cohub-mygo/releases/download/v0.0.3/cohub-mygo_0.0.3_arm64.deb) · [tar.gz](https://github.com/markbang/cohub-mygo/releases/download/v0.0.3/cohub-mygo-0.0.3-linux-arm64.tar.gz) |

macOS / Linux 也可一行安装：

```sh
curl -fsSL https://github.com/markbang/cohub-mygo/releases/latest/download/install.sh | sh
```

Windows 在 PowerShell 里：

```powershell
irm https://github.com/markbang/cohub-mygo/releases/latest/download/install.ps1 | iex
```

应用会在后台检查更新并提示安装；账户菜单里有 **检查更新…**，macOS 的应用菜单里也有。`.deb` 包由包管理器更新。0.0.2 及之前的版本不会自动更新，需要手动装一次 0.0.3 或更新的版本。

应用没有签名。从浏览器下载的话，第一次打开时放行一次即可：Windows 点 **更多信息 → 仍要运行**；macOS 到 **系统设置 → 隐私与安全性 → 仍要打开**。

## TODO

- [x] 缓存数据，加快启动。
- [x] 用 WebRTC 显示实时画面。
- [x] 在 macOS 和 Windows 上验证 GPU 渲染。
