package app

import (
	"log"
	"runtime"
	"time"

	"github.com/egoist/mygo/ui"
)

// The panes' widths and the room they keep, as the web's layout store.
const (
	sidebarWidth = 240
	sidebarMin   = 220
	sidebarMax   = 420
	panelWidth   = 320
	panelMin     = 260
	panelMax     = 520
	mainWidth    = 400 // beside a window
	mainMin      = 320
	windowMin    = 280
	headerHeight = 40
	compactWidth = 768
)

func clampPane(v, def, lo, hi float32) float32 {
	if v <= 0 {
		return def
	}
	return min(max(v, lo), hi)
}

// view builds the window: the sidebar, the main column and the right
// panel on desktops, as the web app's (app) layout; one column with a tab
// bar below 768 DIPs, as its compact shell.
func (a *app) view(c *ui.Context) {
	if a.perf && !a.firstFrame {
		a.firstFrame = true
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		log.Printf("perf: first frame in %s, heap %.1f MB, %d goroutines",
			time.Since(a.startAt).Round(time.Millisecond), float64(m.HeapAlloc)/(1<<20), runtime.NumGoroutine())
	}
	a.applyTheme(c)

	c.OnShortcut(toggleMods(), ui.KeyLeft, func() { a.sidebarOpen = !a.sidebarOpen })
	c.OnShortcut(ui.Cmd, ui.KeyN, func() { a.newChat() })
	c.OnShortcut(ui.Cmd, ui.KeyO, func() { a.newChat() })
	c.OnShortcut(ui.Cmd, ui.KeyK, func() { a.palette.toggle(paletteAll) })
	c.OnShortcut(ui.Cmd, ui.KeyF, func() {
		if a.route.sessionID() != "" {
			a.find.open = true
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyComma, func() { a.go_(route{kind: routeSettings, sub: "general"}) })
	c.OnShortcut(ui.Cmd, ui.KeyB, func() { a.sidebarOpen = !a.sidebarOpen })
	c.OnShortcut(ui.Cmd, ui.KeyBracketLeft, func() { a.goBack() })
	c.OnShortcut(ui.Cmd, ui.KeyBracketRight, func() { a.goForward() })
	if runtime.GOOS != "darwin" {
		// Where going back is Alt+Left, as in browsers and file managers.
		c.OnShortcut(ui.Alt, ui.KeyLeft, func() { a.goBack() })
		c.OnShortcut(ui.Alt, ui.KeyRight, func() { a.goForward() })
	}
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyM, func() { a.menus["model"] = true })
	c.OnShortcut(toggleMods(), ui.KeyRight, func() { a.panelOpen = !a.panelOpen })
	a.pageKeys(c)

	w, h := c.Size()
	a.width, a.height = w, h
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
	if a.login.needed {
		a.loginView(c)
		return
	}
	if a.route.kind == routeInvite {
		// A public page, full window as the web's.
		ui.Column(c).Fill().Background(a.pal.bgPrimary).Children(func() { a.invitePage(c) })
		a.toastView(c)
		a.portToastView(c)
		a.publishDialog(c)
		return
	}
	if a.compact {
		a.compactView(c)
		a.paletteView(c)
		a.confirmView(c)
		a.toastView(c)
		a.portToastView(c)
		a.publishDialog(c)
		return
	}
	p := a.pal
	win, wide := a.hasWindow(), a.windowWide && a.hasWindow()
	panel := a.panelOpen && a.showsPanel() && w >= 1100
	// body is the room right of the sidebar, as the web's workspace body.
	body := w - 52
	if a.sidebarOpen {
		body = w - a.sidebarW
	}
	right := float32(0)
	if panel {
		right = a.panelW
	}
	main := min(a.mainW, max(mainMin, body-right-windowMin))
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		if a.sidebarOpen {
			a.sidebar(c)
			if w >= 960 {
				a.gutter(c, m("nav_resize_sidebar"), a.sidebarW, -1, func(start, off float32, done bool) {
					a.sidebarW = min(max(min(start+off, w-640), sidebarMin), sidebarMax)
					if done {
						a.prefs.SidebarWidth = a.sidebarW
						a.prefs.save()
					}
				})
			}
		} else {
			a.collapsedRail(c)
		}
		bg := p.bgPrimary
		switch a.route.kind {
		case routeSession, routeSpaceSession, routeSpace, routeSpaceNewSession,
			routeSpaceCheckpoint, routeSpaceTask, routeSpaceCron, routeSpaceApp:
			bg = p.chatBg.Over(p.bgPrimary)
		}
		if !wide {
			// Beside a window, the main column keeps its width and the
			// window takes the rest, as the web's.
			col := ui.Column(c).MinWidth(0).Background(bg)
			if win {
				col.Width(main).Shrink(0)
			} else {
				col.Grow(1)
			}
			col.Children(func() {
				a.page(c)
				if a.route.area() == areaSpace {
					a.danmakuLayer(c)
				}
			})
		}
		if win {
			if !wide {
				a.gutter(c, m("window_resize_panel"), main, 0, func(start, off float32, done bool) {
					// Let go 120 DIPs past the narrowest, the main column
					// gives the window its room.
					if done && start+off < min(start, mainMin)-120 {
						a.mainW, a.windowWide = start, true
						return
					}
					a.mainW = min(max(start+off, mainMin), max(mainMin, body-right-windowMin))
					if done {
						a.prefs.MainWidth = a.mainW
						a.prefs.save()
					}
				})
			}
			a.windowPane(c)
		}
		if panel {
			a.gutter(c, m("side_panel_resize"), a.panelW, 0, func(start, off float32, done bool) {
				room := body
				if !wide {
					room -= mainMin
				}
				if win {
					room -= windowMin
				}
				a.panelW = min(max(min(start-off, room), panelMin), panelMax)
				if done {
					a.prefs.PanelWidth = a.panelW
					a.prefs.save()
				}
			})
			a.rightPanel(c)
		}
	})
	a.noticesView(c)
	a.paletteView(c)
	a.helpView(c)
	a.feedbackView(c)
	a.confirmView(c)
	a.toastView(c)
	a.portToastView(c)
	a.publishDialog(c)
	a.openWithDialog(c)
}

// paneDrag is a drag of the edge between two panes: the edge, the pane's
// width as it began and how far the pointer has gone since.
type paneDrag struct {
	edge       string
	start, off float32
}

// gutter is ResizeHandle.svelte, the edge between two panes: a strip 12
// DIPs wide over it that the pointer drags, or the arrows move by 16 DIPs
// (64 with Shift), showing its line and grip while hovered or dragged.
// resize gets the pane's width as the drag began, how far it went since
// along the pointer's way, and whether it ended. line is where the panes'
// border is from the edge.
func (a *app) gutter(c *ui.Context, label string, width, line float32, resize func(start, off float32, done bool)) {
	p := a.pal
	ui.Box(c.Key("gutter:" + label)).Width(0).Shrink(0).ZIndex(1).Children(func() {
		h := ui.ButtonBase(c.Key("handle")).Absolute().Top(0).Bottom(0).Left(-6).Width(12).
			Cursor(ui.CursorResizeEW).Label(label).FocusRing(false)
		d := a.paneDrag
		mine := d != nil && d.edge == label
		dx, _, held := h.Dragged()
		switch {
		case held:
			if !mine {
				d, mine = &paneDrag{edge: label, start: width}, true
				a.paneDrag = d
			}
			if dx != 0 {
				d.off += dx
				resize(d.start, d.off, false)
			}
		case mine:
			a.paneDrag, mine = nil, false
			resize(d.start, d.off, true)
		}
		for _, k := range []struct {
			mods ui.Modifiers
			key  ui.Key
			off  float32
		}{{0, ui.KeyLeft, -16}, {0, ui.KeyRight, 16}, {ui.Shift, ui.KeyLeft, -64}, {ui.Shift, ui.KeyRight, 64}} {
			h.OnShortcut(k.mods, k.key, func() { resize(width, k.off, true) })
		}
		lit := mine || h.FocusVisible()
		if !lit && !h.Hovered() {
			return
		}
		grip, alpha := p.textTertiary, float32(0.6)
		if lit {
			grip, alpha = p.textSecond, 1
		}
		fade := min(64/max(a.height, 1), 0.5)
		h.Children(func() {
			ui.Box(c).Absolute().Top(0).Bottom(0).Left(6 + line).Width(1).Opacity(alpha).
				LinearGradient(ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
					{Color: p.borderStrong.Alpha(0), At: 0}, {Color: p.borderStrong, At: fade},
					{Color: p.borderStrong, At: 1 - fade}, {Color: p.borderStrong.Alpha(0), At: 1},
				}})
			ui.Box(c).Attach(ui.AnchorCenter, ui.AnchorCenter).Size(4, 32).Radius(2).Background(grip).Opacity(alpha)
		})
	})
}

// toastView is .toast-surface: what went wrong, for five seconds.
func (a *app) toastView(c *ui.Context) {
	if a.toast == "" {
		return
	}
	if time.Since(a.toastAt) > 5*time.Second {
		a.toast = ""
		return
	}
	c.After(time.Second)
	p := a.pal
	ui.Row(c.Key("toast")).Attach(ui.AnchorBottom, ui.AnchorBottom).Bottom(24).MaxWidth(480).Gap(8).AlignItems(ui.Center).
		Padding(10, 14).Radius(10).Background(p.bgElevated).Border(1, p.borderSubtle).Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, 0.15)).Children(func() {
		if a.toastOK {
			ui.Icon(c, icon("circle-check")).Size(14, 14).TextColor(p.success)
		} else {
			ui.Icon(c, icon("circle-alert")).Size(14, 14).TextColor(p.errorC)
		}
		ui.Text(c, a.toast).FontSize(12).TextColor(p.text).MaxLines(3)
	})
}

// showsPanel tells whether the route has the panel beside it: chats and
// a Space's new chat.
func (a *app) showsPanel() bool {
	switch a.route.kind {
	case routeSession, routeSpaceSession, routeSpace, routeSpaceNewSession,
		routeSpaceCheckpoint, routeSpaceTask, routeSpaceCron, routeSpaceApp:
		return true
	}
	return false
}

func (a *app) hasWindow() bool {
	switch a.route.kind {
	case routeSpaceCheckpoint, routeSpaceTask, routeSpaceCron, routeSpaceApp:
		return true
	}
	return a.route.win != "" && a.route.area() == areaSpace
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
	case routeSpaceCheckpoint, routeSpaceTask, routeSpaceCron, routeSpaceApp:
		a.spaceHome(c)
	case routeSettings:
		a.settingsPage(c)
	case routeInvite:
		a.invitePage(c)
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
	width := a.panelW
	if a.compact {
		width = 0
	}
	tabs := []struct{ icon, label string }{{"message-square", m("side_panel_tab_session")}, {"files", m("files_title")}, {"images", m("side_panel_tab_media")}, {"panels-top-left", m("side_panel_tab_apps")}}
	if a.can(a.currentSpace(), "cronjob.view") {
		tabs = append(tabs, struct{ icon, label string }{"clock", m("side_panel_tab_scheduled")})
	}
	// A tab the Space does not offer (yet: its access may be loading)
	// shows the chat's outputs, and comes back once it does.
	tab := a.panelTab
	if tab >= len(tabs) {
		tab = 0
	}
	// The tab bar drops its icons once the tabs overflow it.
	icons := len(tabs) <= 4
	// The panel shows the page's content color through; the file tree
	// lays its own primary background over the whole panel.
	bg := p.bgContent
	if tab == 1 {
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
				on := tab == i
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
		ui.Column(c).Grow(1).MinHeight(0).Children(func() { a.panelContent(c, tab) })
	})
}

// notify shows a toast that something went well.
func (a *app) notify(msg string) {
	a.toast, a.toastAt, a.toastOK = msg, time.Now(), true
}
