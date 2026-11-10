package app

import (
	"github.com/egoist/mygo/ui"
)

// compactView is the web's compact shell, below 768 DIPs: one column, a
// header of the page's own, and the tab bar under the lists.
func (a *app) compactView(c *ui.Context) {
	p := a.pal
	ui.Column(c).Fill().Background(p.bgPrimary).Children(func() {
		tabBar := false
		ui.Column(c).Grow(1).MinHeight(0).Background(p.bgContent).Children(func() {
			switch a.route.kind {
			case routeSessions, routeNewSession:
				a.compactChats(c)
				tabBar = true
			case routeSpaces, routeNewSpace:
				ui.Column(c).Grow(1).MinHeight(0).Background(p.bgPrimary).Children(func() { a.spacesView(c) })
				tabBar = true
			case routeSettings:
				a.settingsPage(c)
				tabBar = a.route.sub == "general"
			default:
				a.page(c)
			}
		})
		if tabBar {
			a.tabBar(c)
		}
	})
	if a.compactPanel && a.showsPanel() {
		ui.Column(c.Key("sheet")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(p.bgPrimary).Children(func() { a.rightPanel(c) })
	}
	if a.compactNav {
		a.drawer(c)
	}
}

// compactListHeader is ListHeader.svelte below lg: the brand tile that
// opens the drawer, a 15px title, and 40px actions.
func (a *app) compactListHeader(c *ui.Context, bar ui.TitleBar, title string, actions func()) {
	ui.Row(c.Key("list-header")).Height(44).Shrink(0).Gap(4).AlignItems(ui.Center).Padding(0, 6+bar.Right, 0, 11+bar.Left).DragWindow().Children(func() {
		b := ui.ButtonBase(c.Key("brand")).Size(40, 40).Center().Radius(8).Margin(0, 2, 0, 0).Label(m("sessions_open_nav")).FocusRing(false).Children(func() {
			a.logo(c, 30)
		})
		if b.Clicked() {
			a.compactNav = true
		}
		ui.Text(c, title).FontSize(15).FontWeight(600).Grow(1).MinWidth(0).SingleLine()
		a.inHeader = true
		actions()
		a.inHeader = false
	})
}

func (a *app) compactChats(c *ui.Context) {
	p := a.pal
	bar := c.TitleBar()
	ui.Column(c).Grow(1).MinHeight(0).Background(p.bgPrimary).Children(func() {
		a.compactListHeader(c, bar, m("sidebar_chats"), func() {
			if a.iconButton(c.Key("search"), "search", m("list_search"), false).Clicked() {
				a.palette.open(paletteAll)
			}
			a.sourcePicker(c)
			add := ui.ButtonBase(c.Key("new")).Size(40, 40).Center().Radius(8).Label(m("sidebar_new_chat")).FocusRing(false).Children(func() {
				ui.Icon(c, icon("plus")).Size(20, 20).TextColor(p.brand)
			})
			if add.Clicked() {
				a.newChat()
			}
		})
		a.chipBar(c)
		list := a.visibleSessions()
		now := a.now()
		a.sessionList.Key = func(i int) any { return list[i].ID }
		ui.List(c.Key("compact-sessions"), &a.sessionList, len(list), func(i int) {
			s := list[i]
			row := ui.Row(c.Key(s.ID)).Gap(12).AlignItems(ui.Center).Height(60).Padding(0, 16).Radius(8).Children(func() {
				name, av := a.spaceName(s.SpaceID)
				a.avatarSeed(c, av, name, s.SpaceID, 40, 10)
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Height(20).Children(func() {
						ui.Text(c, chatTitle(s)).FontSize(16).FontWeight(500).SingleLine().Grow(1).MinWidth(0)
						ui.Text(c, relTime(s.When(), now)).FontSize(12).TextColor(p.placeholder).Shrink(0)
					})
					ui.Row(c).Gap(8).AlignItems(ui.Center).Height(18).Children(func() {
						if s.Running() {
							a.runningPill(c, s.ActiveTurn.StartedAt)
						}
						ui.Text(c, preview(s.LatestMessageText)).FontSize(14).TextColor(p.textTertiary.MixOklab(p.placeholder, 0.4)).SingleLine().Grow(1).MinWidth(0)
						if a.unread(s) {
							a.dot(c, p.errorC, 6)
						}
					})
				})
			})
			row.Cursor(ui.CursorPointer)
			if row.Pressed() {
				row.Background(p.hover)
			}
			row.OnClick(func() { a.openSession(s) })
		}).Grow(1).Padding(6, 6)
	})
}

// drawer is the compact shell's sidebar, sliding over the page.
func (a *app) drawer(c *ui.Context) {
	p := a.pal
	ui.Overlay(c, func() {
		ui.Box(c.Key("scrim")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(p.overlay()).OnClick(func() { a.compactNav = false })
		ui.Column(c.Key("drawer")).Absolute().Top(0).Left(0).Bottom(0).Width(300).Background(p.bgPrimary).Shadow(0, 0, 24, 0, ui.RGBA(0, 0, 0, 0.2)).Children(func() {
			a.sidebarTop(c)
			switch a.route.area() {
			case areaSpace:
				a.spaceSidebar(c)
			default:
				a.chatsSidebar(c)
			}
			a.accountRow(c)
		})
	})
}

// tabBar is the bottom bar of the compact shell, as MobileTabBar.svelte.
func (a *app) tabBar(c *ui.Context) {
	p := a.pal
	ar := a.route.area()
	tabs := []struct {
		icon, label string
		on          bool
		to          route
	}{
		{"message-square", m("sidebar_chats"), ar == areaChats, route{kind: routeSessions}},
		{"layers", "Space", ar == areaSpaces, route{kind: routeSpaces}},
		{"user", m("nav_tab_account"), ar == areaSettings, route{kind: routeSettings, sub: "general"}},
	}
	ui.Row(c).Height(56).Shrink(0).AlignItems(ui.Stretch).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Background(p.bgPrimary).Children(func() {
		for _, t := range tabs {
			b := ui.ButtonBase(c.Key(t.label)).Grow(1).Basis(0).Column().AlignItems(ui.Center).Justify(ui.Center).Gap(3).Label(t.label).FocusRing(false).Children(func() {
				col := p.textTertiary
				if t.on {
					col = p.brand
				}
				if t.icon == "user" && a.me != nil && a.me.Profile != nil {
					a.avatar(c, a.me.Profile.AvatarURL, a.me.Profile.Name(), 24, 12)
				} else {
					ui.Icon(c, icon(t.icon)).Size(24, 24).TextColor(col)
				}
				ui.Text(c, t.label).FontSize(11).TextColor(col)
			})
			if b.Clicked() {
				a.go_(t.to)
			}
		}
	})
}
