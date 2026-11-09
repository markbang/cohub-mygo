package main

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
	if a.compactNav {
		a.drawer(c)
	}
}

func (a *app) compactChats(c *ui.Context) {
	p := a.pal
	bar := c.TitleBar()
	ui.Column(c).Grow(1).MinHeight(0).Background(p.bgPrimary).Children(func() {
		ui.Row(c).Height(52).Shrink(0).Gap(6).AlignItems(ui.Center).Padding(0, 8+bar.Right, 0, 14+bar.Left).DragWindow().Children(func() {
			a.logo(c, 30)
			ui.Text(c, m("sidebar_chats")).FontSize(17).FontWeight(600).Grow(1).MarginStart(6)
			for _, it := range []struct{ icon, label string }{{"search", m("list_search")}, {"list-filter", m("x_filter")}} {
				b := ui.ButtonBase(c.Key(it.label)).Size(40, 40).Center().Radius(8).Label(it.label).FocusRing(false).Children(func() {
					ui.Icon(c, icon(it.icon)).Size(20, 20).TextColor(p.textSecond)
				})
				if b.Pressed() {
					b.Background(p.hover)
				}
				if it.icon == "search" && b.Clicked() {
					a.palette.open(paletteAll)
				}
			}
			add := ui.ButtonBase(c.Key("new")).Size(40, 40).Center().Radius(8).Label(m("sidebar_new_chat")).FocusRing(false).Children(func() {
				ui.Icon(c, icon("plus")).Size(22, 22).TextColor(p.brand)
			})
			if add.Clicked() {
				a.newChat()
			}
		})
		ui.ScrollHorizontal(c.Key("chips")).Shrink(0).Scrollbars(false).Padding(2, 12, 10, 12).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
			ui.Row(c).Gap(4).Children(func() {
				a.spaceChip(c, "", m("sidebar_all"), "")
				for _, sp := range a.chatSpaces() {
					a.spaceChip(c, sp.ID, sp.Name, sp.avatar())
				}
			})
		})
		list := a.visibleSessions()
		now := a.now()
		a.sessionList.Key = func(i int) any { return list[i].ID }
		ui.List(c.Key("compact-sessions"), &a.sessionList, len(list), func(i int) {
			s := list[i]
			row := ui.Row(c.Key(s.ID)).Gap(12).AlignItems(ui.Center).Height(68).Padding(0, 20).Children(func() {
				name, av := a.spaceName(s.SpaceID)
				a.avatarSeed(c, av, name, s.SpaceID, 40, 10)
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, s.displayTitle()).FontSize(16).FontWeight(500).SingleLine().Grow(1).MinWidth(0)
						ui.Text(c, relTime(s.when(), now)).FontSize(12).TextColor(p.textTertiary).Shrink(0)
					})
					ui.Row(c).Gap(6).AlignItems(ui.Center).Height(20).Children(func() {
						if s.running() {
							a.runningPill(c, s.ActiveTurn.StartedAt)
						}
						ui.Text(c, preview(s.LatestMessageText)).FontSize(14).TextColor(p.textTertiary).SingleLine().Grow(1).MinWidth(0)
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
		}).Grow(1)
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
					a.avatar(c, a.me.Profile.AvatarURL, a.me.Profile.name(), 24, 12)
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
