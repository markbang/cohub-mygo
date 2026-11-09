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
	a.applyTheme(c)

	c.OnShortcut(ui.Cmd, ui.KeyB, func() { a.sidebarOpen = !a.sidebarOpen })
	c.OnShortcut(ui.Cmd, ui.KeyO, func() { a.newChat() })
	c.OnShortcut(ui.Cmd, ui.KeyK, func() { a.palette.toggle(paletteAll) })
	c.OnShortcut(ui.Cmd, ui.KeyComma, func() { a.go_(route{kind: routeSettings, sub: "general"}) })
	c.OnShortcut(ui.Cmd, ui.KeyBracketLeft, func() { a.goBack() })

	w, _ := c.Size()
	if a.route.kind < 0 {
		a.compact = w < compactWidth
		a.navigate("/sessions")
	}
	compact := w < compactWidth
	if compact != a.compact {
		a.compact = compact
		if !compact && a.route.kind == routeSessions {
			a.show(a.route)
		}
	}
	if a.compact {
		a.compactView(c)
		a.paletteView(c)
		return
	}
	p := a.pal
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		if a.sidebarOpen {
			a.sidebar(c)
		}
		bg := p.bgPrimary
		switch a.route.kind {
		case routeSession, routeSpaceSession, routeSpace, routeSpaceNewSession,
			routeSpaceFiles, routeSpaceCheckpoint, routeSpaceTask, routeSpaceCron:
			bg = p.chatBg.Over(p.bgPrimary)
		}
		ui.Column(c).Grow(1).MinWidth(0).Background(bg).Children(func() {
			a.page(c)
		})
		if a.hasWindow() {
			a.windowPane(c)
		}
		if a.panelOpen && a.showsPanel() && w >= 1100 {
			a.rightPanel(c)
		}
	})
	a.paletteView(c)
}

// showsPanel tells whether the route has the panel beside it: chats and
// a Space's new chat.
func (a *app) showsPanel() bool {
	switch a.route.kind {
	case routeSession, routeSpaceSession, routeSpace, routeSpaceNewSession,
		routeSpaceFiles, routeSpaceCheckpoint, routeSpaceTask, routeSpaceCron:
		return true
	}
	return false
}

func (a *app) hasWindow() bool {
	switch a.route.kind {
	case routeSpaceFiles, routeSpaceCheckpoint, routeSpaceTask, routeSpaceCron:
		return true
	}
	return false
}

// page builds the main column of the route.
func (a *app) page(c *ui.Context) {
	switch a.route.kind {
	case routeSession, routeSpaceSession:
		a.sessionView(c)
	case routeSpace, routeSpaceNewSession:
		a.spaceHome(c)
	case routeSpaces, routeNewSpace:
		a.spacesView(c)
	case routeSpaceSettings:
		a.spaceSettings(c)
	case routeSpaceActivity:
		a.activityPage(c, a.route.space)
	case routeSpaceWallet:
		a.walletPage(c)
	case routeSpaceFiles, routeSpaceCheckpoint, routeSpaceTask, routeSpaceCron:
		a.spaceHome(c)
	case routeSettings:
		a.settingsPage(c)
	default:
		a.emptyView(c)
	}
}

// newChat starts a chat: in the route's Space, else in one the picker
// asks for.
func (a *app) newChat() {
	if sp := a.currentSpace(); sp != "" && a.route.area() == areaSpace {
		a.go_(route{kind: routeSpace, space: sp})
		return
	}
	a.go_(route{kind: routeNewSession})
}

// header is the 40 DIP bar above a main column, which drags the window.
func (a *app) header(c *ui.Context, fn func()) {
	p := a.pal
	bar := c.TitleBar()
	right := float32(12)
	if !a.panelOpen || !a.showsPanel() || a.compact {
		right += bar.Right
	}
	ui.Row(c).Height(headerHeight).Shrink(0).Gap(4).AlignItems(ui.Center).
		Padding(0, right, 0, 12).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).
		DragWindow().Children(fn)
}

func (a *app) emptyView(c *ui.Context) {
	p := a.pal
	a.header(c, func() {
		a.headerLead(c)
		ui.Text(c, "对话").FontSize(13).Grow(1)
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
	bar := c.TitleBar()
	tabs := []struct{ icon, label string }{{"message-square", "本对话"}, {"copy", "文件"}, {"images", "媒体"}, {"layout-template", "应用"}}
	ui.Column(c).Width(panelWidth).Shrink(0).Background(p.bgPrimary).BorderWidth(0, 0, 0, 1).BorderColor(p.borderSubtle).Children(func() {
		ui.Row(c).Height(headerHeight).Shrink(0).AlignItems(ui.Center).Padding(0, 8+bar.Right, 0, 8).Gap(2).DragWindow().Children(func() {
			for i, t := range tabs {
				if i > 0 {
					ui.Box(c).Width(1).Height(12).Background(p.borderSubtle).Margin(0, 3)
				}
				on := a.panelTab == i
				b := ui.ButtonBase(c.Key(t.label)).Row().Gap(5).AlignItems(ui.Center).Height(26).Padding(0, 6).Radius(6).Label(t.label).FocusRing(false).Children(func() {
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
		ui.Column(c).Grow(1).MinHeight(0).Children(func() { a.panelContent(c) })
	})
}
