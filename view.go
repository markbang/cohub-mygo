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

	c.OnShortcut(ui.Ctrl|ui.Alt, ui.KeyLeft, func() { a.sidebarOpen = !a.sidebarOpen })
	c.OnShortcut(ui.Cmd, ui.KeyO, func() { a.newChat() })
	c.OnShortcut(ui.Cmd, ui.KeyK, func() { a.palette.toggle(paletteAll) })
	c.OnShortcut(ui.Cmd, ui.KeyComma, func() { a.go_(route{kind: routeSettings, sub: "general"}) })
	c.OnShortcut(ui.Cmd, ui.KeyBracketLeft, func() { a.goBack() })

	w, _ := c.Size()
	a.width = w
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
		} else {
			a.collapsedRail(c)
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
	case routeSpaces:
		a.spacesView(c)
	case routeNewSpace:
		a.newSpacePage(c)
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
	h, left := float32(headerHeight), float32(12)
	if a.compact {
		// ColumnHeader's 2.75rem, with header-action's 40px targets.
		h, left, right = 44, 6, 6+bar.Right
	}
	ui.Row(c).Height(h).Shrink(0).Gap(4).AlignItems(ui.Center).
		Padding(0, right, 0, left).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).
		DragWindow().Children(func() {
		a.inHeader = true
		fn()
		a.inHeader = false
	})
}

func (a *app) emptyView(c *ui.Context) {
	p := a.pal
	a.header(c, func() {
		a.headerLead(c)
		ui.Text(c, m("sidebar_chats")).FontSize(13).Grow(1)
	})
	ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
		if a.status != "" && a.me == nil {
			a.spinner(c, 18, p.textTertiary)
			ui.Text(c, a.status).TextColor(p.textTertiary).FontSize(13)
			return
		}
		ui.Text(c, m("x_select_or_new")).TextColor(p.textTertiary).FontSize(13)
	})
}

// rightPanel is the panel beside a chat: what it made and changed.
func (a *app) rightPanel(c *ui.Context) {
	p := a.pal
	bar := c.TitleBar()
	width := float32(panelWidth)
	if a.compact {
		width = 0
	}
	tabs := []struct{ icon, label string }{{"message-square", m("side_panel_tab_session")}, {"files", m("files_title")}, {"images", m("side_panel_tab_media")}, {"panels-top-left", m("side_panel_tab_apps")}}
	if a.can(a.currentSpace(), "cronjob.view") {
		tabs = append(tabs, struct{ icon, label string }{"clock", m("side_panel_tab_scheduled")})
	}
	if a.panelTab >= len(tabs) {
		a.panelTab = 0
	}
	// The tab bar drops its icons once the tabs overflow it.
	icons := len(tabs) <= 4
	// The panel shows the page's content color through; the file tree
	// lays its own primary background over the whole panel.
	bg := p.bgContent
	if a.panelTab == 1 {
		bg = p.bgPrimary
	}
	col := ui.Column(c).Shrink(0).Background(bg).BorderWidth(0, 0, 0, 1).BorderColor(p.borderSubtle)
	if width > 0 {
		col.Width(width)
	} else {
		col.Grow(1).MinHeight(0)
	}
	col.Children(func() {
		ui.Row(c).Height(headerHeight).Shrink(0).AlignItems(ui.Center).Padding(0, 8+bar.Right, 0, 8).Gap(2).DragWindow().Children(func() {
			for i, t := range tabs {
				if i == 1 {
					ui.Box(c).Width(1).Height(14).Background(p.borderSubtle).Margin(0, 4)
				}
				on := a.panelTab == i
				// .tab-key: 28px, 11px medium, the chosen one on a raised pill.
				b := ui.ButtonBase(c.Key(t.label)).Row().Gap(6).AlignItems(ui.Center).Height(28).Padding(0, 8).Radius(5).Label(t.label).FocusRing(false)
				hover := b.Hovered()
				b.Children(func() {
					col := p.textTertiary
					switch {
					case on:
						col = p.text
					case hover:
						col = p.textSecond
					}
					if icons {
						ui.Icon(c, icon(t.icon)).Size(14, 14).TextColor(col)
					}
					ui.Text(c, t.label).FontSize(11).FontWeight(500).TextColor(col).SingleLine()
				})
				if on {
					b.Background(p.tabKeyBg)
				}
				if b.Clicked() {
					a.panelTab = i
				}
			}
			if a.compact {
				ui.Spacer(c)
				if a.iconButton(c.Key("close-panel"), "x", m("side_panel_hide"), false).Clicked() {
					a.compactPanel = false
				}
			}
		})
		ui.Column(c).Grow(1).MinHeight(0).Children(func() { a.panelContent(c) })
	})
}
