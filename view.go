package main

import (
	"github.com/egoist/mygo/ui"
)

const (
	sidebarWidth = 240
	panelWidth   = 320
	headerHeight = 40
	compactWidth = 768
)

// view builds the window: the sidebar, the main column and the right
// panel on desktops, as the web app's (app) layout; one column with a tab
// bar below 768 DIPs, as its compact shell.
func (a *app) view(c *ui.Context) {
	dark := c.Theme().Dark
	switch a.themeMode {
	case themeLight:
		dark = false
	case themeDark:
		dark = true
	}
	if dark != a.pal.dark || a.pal.bgPrimary == (ui.Color{}) {
		if dark {
			a.pal = darkPalette()
		} else {
			a.pal = lightPalette()
		}
	}
	a.pal.apply(c)
	a.md.Theme = a.markdownTheme(c)

	c.OnShortcut(ui.Cmd, ui.KeyB, func() { a.sidebarOpen = !a.sidebarOpen })
	c.OnShortcut(ui.Cmd, ui.KeyO, func() { a.newChat() })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyL, func() { a.themeMode = (a.themeMode + 1) % 3 })

	w, _ := c.Size()
	a.compact = w < compactWidth
	if !a.compact && a.rail == railAccount {
		a.rail = railChats
	}
	if !a.compact && a.rail == railChats && a.selSession == "" && !a.autoPicked && len(a.sessions) > 0 {
		// /sessions opens the latest chat, as the web does.
		a.autoPicked = true
		a.openSession(a.sessions[0])
	}
	if a.compact {
		a.compactView(c)
		return
	}
	p := a.pal
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		if a.sidebarOpen {
			a.sidebar(c)
		}
		ui.Column(c).Grow(1).MinWidth(0).Background(p.bgContent).Children(func() {
			switch {
			case a.selSession != "":
				a.sessionView(c)
			case a.selSpace != "":
				a.newChatView(c)
			case a.rail == railSpaces:
				a.spacesView(c)
			default:
				a.emptyView(c)
			}
		})
		if a.panelOpen && (a.selSession != "" || a.selSpace != "") && w >= 1100 {
			a.rightPanel(c)
		}
	})
}

func (a *app) newChat() {
	current := a.session(a.selSession)
	a.selSession = ""
	if a.selSpace == "" {
		if s := current; s != nil {
			a.selSpace = s.SpaceID
		} else if a.chip != "" {
			a.selSpace = a.chip
		} else if len(a.spaces) > 0 {
			a.selSpace = a.spaces[0].ID
		}
	}
}

// header is the 40 DIP bar above every main column, which drags the window.
func (a *app) header(c *ui.Context, fn func()) {
	p := a.pal
	bar := c.TitleBar()
	ui.Row(c).Height(headerHeight).Shrink(0).Gap(8).AlignItems(ui.Center).
		Padding(0, 12+bar.Right, 0, 16).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).
		DragWindow().Children(fn)
}

func (a *app) emptyView(c *ui.Context) {
	p := a.pal
	a.header(c, func() {
		ui.Text(c, "对话").FontWeight(500).Grow(1)
	})
	ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
		if a.status != "" && a.me == nil {
			a.spinner(c, 18, p.textTertiary)
			ui.Text(c, a.status).TextColor(p.textTertiary).FontSize(13)
			return
		}
		ui.Text(c, "选择一个对话，或新建一个").TextColor(p.textTertiary).FontSize(13)
	})
}

// rightPanel is the panel beside a chat: what it made and changed.
func (a *app) rightPanel(c *ui.Context) {
	p := a.pal
	tabs := []struct{ icon, label string }{{"message-square", "本对话"}, {"copy", "文件"}, {"images", "媒体"}, {"layout-template", "应用"}}
	ui.Column(c).Width(panelWidth).Shrink(0).Background(p.bgPrimary).BorderWidth(0, 0, 0, 1).BorderColor(p.borderSubtle).Children(func() {
		ui.Row(c).Height(headerHeight).Shrink(0).AlignItems(ui.Center).Padding(0, 8).Gap(2).DragWindow().Children(func() {
			for i, t := range tabs {
				if i > 0 {
					ui.Box(c).Width(1).Height(12).Background(p.borderSubtle).Margin(0, 4)
				}
				on := a.panelTab == i
				b := ui.ButtonBase(c.Key(t.label)).Row().Gap(5).AlignItems(ui.Center).Padding(4, 6).Radius(6).FocusRing(false).Children(func() {
					col := p.textTertiary
					if on {
						col = p.text
					}
					ui.Icon(c, icon(t.icon)).Size(13, 13).TextColor(col)
					txt := ui.Text(c, t.label).FontSize(12).TextColor(col).SingleLine()
					if on {
						txt.FontWeight(600)
					}
				})
				if b.Hovered() && !on {
					b.Background(p.hover)
				}
				if b.Clicked() {
					a.panelTab = i
				}
			}
		})
		ui.Column(c).Grow(1).Padding(16).Children(func() {
			msg := []string{"本对话中 Agent 生成的媒体和改动的文件会显示在这里。", "Space 的文件会显示在这里。", "生成的图片和视频会显示在这里。", "发布的应用会显示在这里。"}[a.panelTab]
			if a.selSession == "" && a.panelTab == 0 {
				msg = "开始对话后，这里会显示它生成的媒体和改动的文件。"
			}
			ui.Text(c, msg).FontSize(12).LineHeight(1.6).TextColor(p.textTertiary)
		})
	})
}
