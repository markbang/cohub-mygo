package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// sidebar is the web's left column: the rail of modes on top, then the
// chats, the Space directory or one Space, and the account at the bottom.
func (a *app) sidebar(c *ui.Context) {
	p := a.pal
	ui.Column(c).Width(sidebarWidth).Shrink(0).Background(p.bgPrimary).BorderWidth(0, 1, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		a.sidebarTop(c)
		switch {
		case a.rail == railSpaces && a.selSpace != "":
			a.spaceSidebar(c)
		case a.rail == railSpaces:
			a.spacesNavSidebar(c)
		default:
			a.chatsSidebar(c)
		}
		a.accountRow(c)
	})
}

func (a *app) sidebarTop(c *ui.Context) {
	p := a.pal
	bar := c.TitleBar()
	ui.Row(c).Height(headerHeight).Shrink(0).AlignItems(ui.Center).Gap(4).Padding(0, 8, 0, 12+bar.Left).DragWindow().Children(func() {
		a.logo(c, 28)
		ui.Spacer(c)
		if a.iconButton(c.Key("rail-chats"), "message-square", "对话", a.rail == railChats).Clicked() {
			a.rail = railChats
			a.selSpace = ""
		}
		if a.iconButton(c.Key("rail-spaces"), "layers", "Space", a.rail == railSpaces).Clicked() {
			a.rail, a.selSpace = railSpaces, ""
		}
		search := ui.ButtonBase(c.Key("search")).Row().Gap(4).AlignItems(ui.Center).Height(24).Padding(0, 8).Radius(6).Background(p.hover).Label("搜索").FocusRing(false).Children(func() {
			ui.Icon(c, icon("search")).Size(13, 13).TextColor(p.textTertiary)
			ui.Text(c, "⌘K").FontSize(11).Font(fontMono).TextColor(p.textTertiary)
		})
		if search.Hovered() {
			search.Background(p.hoverStrong)
		}
		if search.Clicked() {
			a.rail, a.selSpace, a.selSession = railSpaces, "", ""
		}
		if a.iconButton(c.Key("collapse"), "panel-left", "收起侧栏 (⌘B)", false).Clicked() {
			a.sidebarOpen = false
		}
	})
}

func (a *app) chatsSidebar(c *ui.Context) {
	p := a.pal
	ui.Row(c).Height(36).Shrink(0).AlignItems(ui.Center).Gap(2).Padding(0, 8, 0, 14).Children(func() {
		ui.Text(c, "对话").FontWeight(600).Grow(1)
		a.iconButton(c.Key("filter"), "list-filter", "筛选", false)
		add := a.iconButton(c.Key("new-chat"), "plus", "新建对话 (⌘O)", false)
		if add.Clicked() {
			a.newChat()
		}
	})
	// Space chips: all, then the Spaces with recent chats.
	ui.ScrollHorizontal(c.Key("chips")).Shrink(0).Scrollbars(false).Padding(4, 6, 8, 6).Children(func() {
		ui.Row(c).Gap(2).Children(func() {
			a.spaceChip(c, "", "全部", "")
			for _, sp := range a.chatSpaces() {
				a.spaceChip(c, sp.ID, sp.Name, sp.avatar())
			}
		})
	})
	list := a.visibleSessions()
	a.sessionList.Key = func(i int) any { return list[i].ID }
	ui.List(c, &a.sessionList, len(list), func(i int) {
		a.sessionRow(c, list[i])
	}).Grow(1).Padding(0, 6, 6, 6).Gap(2).Children(func() {
		if len(list) == 0 {
			msg := "还没有对话"
			if a.me == nil && a.status != "" {
				msg = a.status
			}
			ui.Text(c, msg).FontSize(12).TextColor(p.textTertiary).Padding(16, 10)
		}
	})
}

func (a *app) spaceChip(c *ui.Context, id, name, avatar string) {
	p := a.pal
	on := a.chip == id
	b := ui.ButtonBase(c.Key("chip-"+id)).Row().Gap(5).AlignItems(ui.Center).Height(24).Padding(0, 7).Radius(6).FocusRing(false).Children(func() {
		if id != "" {
			a.avatar(c, avatar, name, 16, 4)
		}
		col := p.textTertiary
		if on {
			col = p.brand
		}
		ui.Text(c, name).FontSize(12).TextColor(col).SingleLine().MaxWidth(110)
	})
	switch {
	case on:
		b.Background(p.brandMuted)
	case b.Hovered():
		b.Background(p.hover)
	}
	if b.Clicked() {
		a.chip = id
		a.sessionList = ui.ListState{}
	}
}

// sessionRow is a chat in the list: the Space's picture, the title and
// time, then the latest reply or how long the running turn has run.
func (a *app) sessionRow(c *ui.Context, s *Session) {
	p := a.pal
	now := a.now()
	row := ui.Row(c.Key(s.ID)).Gap(10).AlignItems(ui.Center).Padding(8, 6).Radius(6).Children(func() {
		name, av := "", ""
		if s.Space != nil {
			name, av = s.Space.Name, s.Space.avatarURL()
		} else if sp := a.space(s.SpaceID); sp != nil {
			name, av = sp.Name, sp.avatar()
		}
		a.avatar(c, av, name, 36, 8)
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, s.displayTitle()).FontSize(14).FontWeight(500).SingleLine().Grow(1).MinWidth(0)
				ui.Text(c, relTime(s.when(), now)).FontSize(11).Font(fontMono).TextColor(p.textTertiary).Shrink(0)
			})
			ui.Row(c).Gap(6).AlignItems(ui.Center).Height(17).Children(func() {
				if s.running() {
					a.runningPill(c, s.ActiveTurn.StartedAt)
					ui.Spacer(c)
					return
				}
				ui.Text(c, preview(s.LatestMessageText)).FontSize(12).TextColor(p.textTertiary).SingleLine().Grow(1).MinWidth(0)
				if a.unread(s) {
					a.dot(c, p.errorC, 6)
				}
			})
		})
	})
	a.pressable(row, s.ID == a.selSession)
	row.OnClick(func() { a.openSession(s) })
}

func (s *sessionSpace) avatarURL() string {
	if s == nil || s.PublicProfile == nil {
		return ""
	}
	return s.PublicProfile.AvatarURL
}

// runningPill shows that a turn runs, and for how long.
func (a *app) runningPill(c *ui.Context, since time.Time) {
	p := a.pal
	ui.Row(c).Gap(3).AlignItems(ui.Center).Padding(1, 6).Radius(4).Background(p.brandMuted).Shrink(0).Children(func() {
		for i := 0; i < 3; i++ {
			a.dot(c.Key(i), p.brand, 3).Opacity(1 - 0.3*float32(i))
		}
		label := "运行中"
		if !since.IsZero() {
			label += " " + elapsed(since, a.now())
			c.After(time.Second)
		}
		ui.Text(c, label).FontSize(11).TextColor(p.brand).SingleLine().MarginStart(2)
	})
}

var sidebarSpaceNav = []struct{ icon, label string }{
	{"clock", "最近"}, {"layout-grid", "全部 Space"}, {"user", "我的"}, {"pin", "置顶"}, {"archive", "已归档"},
}

// spacesNavSidebar is the Space directory's filters.
func (a *app) spacesNavSidebar(c *ui.Context) {
	p := a.pal
	ui.Column(c).Grow(1).Padding(8, 6).Gap(2).Children(func() {
		for i, it := range sidebarSpaceNav {
			on := a.spacesNav == i
			row := ui.Row(c.Key(it.label)).Gap(10).AlignItems(ui.Center).Height(34).Padding(0, 8).Radius(6).Children(func() {
				col := p.textSecond
				if on {
					col = p.text
				}
				ui.Icon(c, icon(it.icon)).Size(15, 15).TextColor(col)
				t := ui.Text(c, it.label).FontSize(13).TextColor(col)
				if on {
					t.FontWeight(600)
				}
			})
			a.pressable(row, on)
			row.OnClick(func() { a.spacesNav = i; a.spaceList = ui.ListState{} })
		}
	})
}

// spaceSidebar is one Space's: its switcher, a new chat, its tools and
// its chats.
func (a *app) spaceSidebar(c *ui.Context) {
	p := a.pal
	sp := a.space(a.selSpace)
	name, av := "Space", ""
	if sp != nil {
		name, av = sp.Name, sp.avatar()
	}
	ui.Column(c).Shrink(0).Padding(4, 6, 6, 6).Gap(2).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		sw := ui.Row(c.Key("switcher")).Gap(8).AlignItems(ui.Center).Height(40).Padding(0, 6).Radius(6).Children(func() {
			a.avatar(c, av, name, 26, 6)
			ui.Text(c, name).FontWeight(600).SingleLine().Grow(1).MinWidth(0)
			ui.Icon(c, icon("chevron-down")).Size(14, 14).TextColor(p.textTertiary)
		})
		a.pressable(sw, false)
		sw.OnClick(func() { a.selSpace, a.selSession = "", "" })
		nb := ui.Row(c.Key("new")).Gap(6).AlignItems(ui.Center).Height(32).Padding(0, 8).Radius(7).Background(p.brandMuted).Border(1, p.brandBorder).Margin(4, 0).Cursor(ui.CursorPointer).Children(func() {
			ui.Icon(c, icon("plus")).Size(14, 14).TextColor(p.brand)
			ui.Text(c, "新建对话").FontSize(13).TextColor(p.brand).Grow(1)
			ui.Text(c, "⌘O").FontSize(10).Font(fontMono).TextColor(p.brand).Padding(1, 4).Radius(4).Border(1, p.brandBorder)
		})
		if nb.Hovered() {
			nb.Background(p.brand.Alpha(0.15))
		}
		nb.OnClick(func() { a.selSession = "" })
		for _, it := range []struct{ icon, label string }{{"settings", "设置"}, {"activity", "活动"}, {"save", "新建存档"}} {
			row := ui.Row(c.Key(it.label)).Gap(10).AlignItems(ui.Center).Height(30).Padding(0, 8).Radius(6).Children(func() {
				ui.Icon(c, icon(it.icon)).Size(14, 14).TextColor(p.textSecond)
				ui.Text(c, it.label).FontSize(13).TextColor(p.textSecond)
			})
			a.pressable(row, false)
		}
	})
	ui.Row(c).Height(30).Shrink(0).Gap(6).AlignItems(ui.Center).Padding(8, 14, 0, 14).Children(func() {
		ui.Icon(c, icon("chevron-down")).Size(12, 12).TextColor(p.textTertiary)
		ui.Icon(c, icon("message-square")).Size(13, 13).TextColor(p.textTertiary)
		ui.Text(c, "对话").FontSize(12).TextColor(p.textTertiary)
	})
	list := a.spaceSessions[a.selSpace]
	ui.List(c.Key("space-sessions-"+a.selSpace), nil, len(list), func(i int) {
		s := list[i]
		row := ui.Row(c.Key(s.ID)).Gap(6).AlignItems(ui.Center).Height(30).Padding(0, 8).Radius(6).Children(func() {
			ui.Text(c, s.displayTitle()).FontSize(13).SingleLine().Grow(1).MinWidth(0)
			if src := sourceLabel(s.Source); src != "" {
				ui.Text(c, src).FontSize(10).FontWeight(500).Font(fontMono).TextColor(p.textTertiary).Padding(1, 5).Radius(3).Background(p.hoverStrong).Shrink(0)
			}
			ui.Text(c, relTime(s.when(), a.now())).FontSize(10.5).Font(fontMono).TextColor(p.textTertiary).Shrink(0)
			switch {
			case s.running():
				a.spinner(c, 10, p.brand)
			case a.unread(s):
				a.dot(c, p.errorC, 6)
			}
		})
		a.pressable(row, s.ID == a.selSession)
		row.OnClick(func() { a.openSession(s) })
	}).Grow(1).Padding(0, 6, 6, 6).Gap(1).Children(func() {
		if list == nil {
			ui.Row(c).Padding(12).Children(func() { a.spinner(c, 14, p.textTertiary) })
		}
	})
}

func sourceLabel(src string) string {
	switch src {
	case "cli":
		return "CLI"
	case "web", "":
		return ""
	case "public_api":
		return "API"
	case "scheduled_task":
		return "定时"
	}
	return strings.ToUpper(src[:1]) + src[1:]
}

// accountRow is the signed-in user, whose menu holds the appearance.
func (a *app) accountRow(c *ui.Context) {
	p := a.pal
	name, av := "未登录", ""
	if a.me != nil && a.me.Profile != nil {
		name, av = a.me.Profile.name(), a.me.Profile.AvatarURL
	}
	ui.Row(c).Height(46).Shrink(0).Gap(6).AlignItems(ui.Center).Padding(0, 8).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		who := ui.ButtonBase(c.Key("account")).Row().Gap(8).AlignItems(ui.Center).Grow(1).Height(32).Padding(0, 6).Radius(6).FocusRing(false).Children(func() {
			a.avatar(c, av, name, 20, 10)
			ui.Text(c, name).FontSize(13).SingleLine().Grow(1)
			ui.Icon(c, icon("chevron-down")).Size(13, 13).TextColor(p.textTertiary)
		})
		if who.Hovered() {
			who.Background(p.hover)
		}
		if who.Clicked() {
			a.userMenu = !a.userMenu
		}
		ui.Popover(c, who, &a.userMenu, func() {
			ui.Column(c).Gap(2).MinWidth(180).Children(func() {
				ui.Text(c, "外观").FontSize(11).TextColor(p.textTertiary).Padding(2, 6)
				for i, label := range []string{"跟随系统", "浅色", "深色"} {
					row := ui.Row(c.Key(label)).Gap(8).AlignItems(ui.Center).Height(28).Padding(0, 6).Radius(5).Children(func() {
						ui.Text(c, label).FontSize(13).Grow(1)
						if a.themeMode == themeMode(i) {
							ui.Icon(c, icon("check")).Size(14, 14).TextColor(p.brand)
						}
					})
					a.pressable(row, false)
					row.OnClick(func() { a.themeMode = themeMode(i); a.userMenu = false })
				}
				if a.status != "" {
					ui.Text(c, a.status).FontSize(11).TextColor(p.textTertiary).Padding(6).MaxWidth(220)
				}
			})
		})
		a.iconButton(c.Key("help"), "circle-help", "帮助", false)
	})
}
