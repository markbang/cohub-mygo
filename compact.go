package main

import (
	"github.com/egoist/mygo/ui"
)

// compactView is the web's compact shell, below 768 DIPs: one column, a
// header of the tab's own, and the tab bar at the bottom while no chat or
// Space is open.
func (a *app) compactView(c *ui.Context) {
	p := a.pal
	ui.Column(c).Fill().Background(p.bgPrimary).Children(func() {
		switch {
		case a.selSession != "":
			ui.Column(c).Grow(1).MinHeight(0).Background(p.bgContent).Children(func() { a.sessionView(c) })
			return
		case a.selSpace != "":
			ui.Column(c).Grow(1).MinHeight(0).Background(p.bgContent).Children(func() { a.newChatView(c) })
			return
		}
		ui.Column(c).Grow(1).MinHeight(0).Children(func() {
			switch a.rail {
			case railSpaces:
				a.spacesView(c)
			case railAccount:
				a.accountTab(c)
			default:
				a.compactChats(c)
			}
		})
		a.tabBar(c)
	})
}

func (a *app) compactChats(c *ui.Context) {
	p := a.pal
	bar := c.TitleBar()
	ui.Row(c).Height(52).Shrink(0).Gap(10).AlignItems(ui.Center).Padding(0, 10+bar.Right, 0, 14+bar.Left).DragWindow().Children(func() {
		a.logo(c, 30)
		ui.Text(c, "对话").FontSize(17).FontWeight(600).Grow(1)
		for _, it := range []struct{ icon, label string }{{"search", "搜索"}, {"list-filter", "筛选"}} {
			b := ui.ButtonBase(c.Key(it.label)).Size(36, 36).Center().Radius(8).Label(it.label).FocusRing(false).Children(func() {
				ui.Icon(c, icon(it.icon)).Size(20, 20).TextColor(p.textSecond)
			})
			if b.Pressed() {
				b.Background(p.hover)
			}
		}
		add := ui.ButtonBase(c.Key("new")).Size(36, 36).Center().Radius(8).Label("新建对话").FocusRing(false).Children(func() {
			ui.Icon(c, icon("plus")).Size(22, 22).TextColor(p.brand)
		})
		if add.Clicked() {
			a.newChat()
		}
	})
	ui.ScrollHorizontal(c.Key("chips")).Shrink(0).Scrollbars(false).Padding(2, 10, 8, 10).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Row(c).Gap(4).Children(func() {
			a.spaceChip(c, "", "全部", "")
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
		row := ui.Row(c.Key(s.ID)).Gap(12).AlignItems(ui.Center).Padding(10, 14).Children(func() {
			name, av := "", ""
			if s.Space != nil {
				name, av = s.Space.Name, s.Space.avatarURL()
			}
			a.avatar(c, av, name, 44, 10)
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, s.displayTitle()).FontSize(16).FontWeight(500).SingleLine().Grow(1).MinWidth(0)
					ui.Text(c, relTime(s.when(), now)).FontSize(12).Font(fontMono).TextColor(p.textTertiary).Shrink(0)
				})
				ui.Row(c).Gap(6).AlignItems(ui.Center).Height(20).Children(func() {
					if s.running() {
						a.runningPill(c, s.ActiveTurn.StartedAt)
						ui.Spacer(c)
						return
					}
					ui.Text(c, preview(s.LatestMessageText)).FontSize(14).TextColor(p.textTertiary).SingleLine().Grow(1).MinWidth(0)
					if a.unread(s) {
						a.dot(c, p.errorC, 7)
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
}

func (a *app) accountTab(c *ui.Context) {
	p := a.pal
	name, av, mail := "未登录", "", ""
	if a.me != nil && a.me.Profile != nil {
		name, av, mail = a.me.Profile.name(), a.me.Profile.AvatarURL, a.me.Email
	}
	ui.Column(c).Grow(1).Padding(48, 20).Gap(12).AlignItems(ui.Center).Children(func() {
		a.avatar(c, av, name, 72, 36)
		ui.Text(c, name).FontSize(20).FontWeight(600)
		ui.Text(c, mail).FontSize(13).TextColor(p.textTertiary)
		ui.Box(c).Height(12)
		ui.Segmented(c, (*int)(&a.themeMode), "跟随系统", "浅色", "深色").Label("外观")
	})
}

// tabBar is the bottom bar of the compact shell, as MobileTabBar.svelte.
func (a *app) tabBar(c *ui.Context) {
	p := a.pal
	tabs := []struct {
		icon, label string
		r           rail
	}{{"message-square", "对话", railChats}, {"layers", "Space", railSpaces}, {"user", "账户", railAccount}}
	ui.Row(c).Height(58).Shrink(0).AlignItems(ui.Stretch).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Background(p.bgPrimary).Children(func() {
		for _, t := range tabs {
			on := a.rail == t.r
			b := ui.ButtonBase(c.Key(t.label)).Grow(1).Basis(0).Column().AlignItems(ui.Center).Justify(ui.Center).Gap(3).FocusRing(false).Children(func() {
				col := p.textTertiary
				if on {
					col = p.brand
				}
				if t.r == railAccount && a.me != nil && a.me.Profile != nil {
					a.avatar(c, a.me.Profile.AvatarURL, a.me.Profile.name(), 22, 11)
				} else {
					ui.Icon(c, icon(t.icon)).Size(22, 22).TextColor(col)
				}
				ui.Text(c, t.label).FontSize(11).TextColor(col)
			})
			if b.Clicked() {
				a.rail = t.r
			}
		}
	})
}
