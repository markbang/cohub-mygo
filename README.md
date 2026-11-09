# cohub-mygo

这是 Cohub 桌面端的原型：用 [MyGo](https://mygo.egoist.dev) 的 native UI 按 Cohub web 的 `(app)` 布局和外观重新实现，界面全部由 MyGo 在 GPU 上绘制（macOS 用 Metal，Windows 用 D3D11，Linux 用 OpenGL）。

- 不使用 WebView、HTML 或 JS，纯 Go 实现，不依赖 cgo。
- 账号沿用 `cohub` CLI 的登录状态（`~/.config/cohub/auth.json`，token 过期时调用 `cohub auth token` 刷新），和现有桌面外壳一致。
- 没有登录时显示演示数据。

| 浅色 | 深色 | 窄窗口（< 768） |
|---|---|---|
| ![](screenshots/desktop-chat-light.png) | ![](screenshots/desktop-chat-dark.png) | ![](screenshots/compact-chats.png) |

## 运行

```sh
go run .                       # 使用 cohub CLI 的账号
COHUB_DEMO=1 go run .          # 演示数据
go tool mygo dev               # 改代码后自动重启
go tool mygo build             # 打包 dmg / 安装器 / AppImage
```

需要 Go 1.27+。Linux 上还需要 GTK 3。

## 已复刻的部分

| web | 这里 |
|---|---|
| 左上 rail：Logo、对话 / Space 切换、⌘K、收起侧栏 | `sidebar.go` `sidebarTop` |
| 对话侧栏：Space chips、会话行（头像、标题、时间、预览、运行中 pill、未读红点）、账户行 | `chatsSidebar` / `sessionRow` / `accountRow` |
| Space 目录：最近 / 全部 / 我的 / 置顶 / 已归档 | `spacesNavSidebar`、`spaces.go` |
| 单个 Space：切换器、新建对话 ⌘O、设置 / 活动 / 存档、会话列表（来源标签） | `spaceSidebar`、`newChatView` |
| 会话：面包屑头部、用户气泡、步骤 / 工具摘要、Markdown 回复（表格、代码高亮）、模型和 token 统计、流式运行中状态 | `session.go` |
| Composer：多行输入（Enter 发送，Shift+Enter 换行）、附件、harness / 模型 chip、语音、发送 / 停止 | `composer` |
| 右侧面板：本对话 / 文件 / 媒体 / 应用 | `view.go` `rightPanel` |
| 窄窗口：顶部标题、chips、大号会话行、底部 tab 栏（对话 / Space / 账户），全屏会话页 | `compact.go` |
| 主题：light / dark design tokens（oklch）、Geist 字体、Lucide 图标 | `theme.go`、`assets.go` |

数据来自 `/api/me`、`/api/spaces`、`/api/me/sessions`、`/api/spaces/:id/sessions`、`/api/sessions/:id/turns` 和 `turns/stream-snapshot`。发送调用 `POST /api/spaces/:id/prompt`，停止调用 `POST /api/sessions/:id/abort`。

## 还没做

- **实时同步**：现在是轮询，正在运行的会话每 2 秒拉一次 turns 和 stream snapshot，列表每 16 秒刷新一次。web 用的是 realtime WebSocket（`packages/sdk/src/websocket.ts`）。
- **本地缓存**：没有首屏缓存。web 端有 IndexedDB 缓存。
- **未读状态**：只保存在本地（`~/.config/cohub-mygo/seen.json`），没有和服务端同步。
- **以下功能没有实现**：工具调用详情展开、附件上传、模型和 harness 选择、⌘K 搜索、Board、文件树与预览、设置、标签、存档、i18n（目前只有中文）。

## 测试

```sh
CGO_ENABLED=0 go test ./...                                   # 无窗口测试（ui.Tester）
SNAPSHOT_DIR=screenshots go test ./...                        # 输出演示数据截图
COHUB_LIVE=1 SNAPSHOT_DIR=screenshots go test -run Live ./... # 用真实账号截图（只留在本地，不提交）
COHUB_CAPTURE=/tmp/shot.png go run .                          # 真窗口运行 12 秒后截图并退出
```

## 资源

- `fonts/`：Geist 和 Geist Mono，SIL OFL 1.1，见 `fonts/OFL.txt`。
- `icons/`：Lucide v1.70，ISC 许可。
