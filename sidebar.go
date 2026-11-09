package main

import (
	"context"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// sidebar is the web's left column: the rail of modes on top, then what the
// route's area lists, and the account at the bottom.
func (a *app) sidebar(c *ui.Context) {
	p := a.pal
	ui.Column(c).Width(sidebarWidth).Shrink(0).Background(p.bgPrimary).BorderWidth(0, 1, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		a.sidebarTop(c)
		switch a.route.area() {
		case areaSpace:
			a.spaceSidebar(c)
		case areaSpaces:
			a.spacesNavSidebar(c)
		case areaSettings:
			a.settingsNav(c)
		default:
			a.chatsSidebar(c)
		}
		a.accountRow(c)
	})
}

func (a *app) sidebarTop(c *ui.Context) {
	p := a.pal
	bar := c.TitleBar()
	ar := a.route.area()
	ui.Row(c).Height(headerHeight).Shrink(0).AlignItems(ui.Center).Gap(2).Padding(0, 8, 0, 12+bar.Left).DragWindow().Children(func() {
		a.logo(c, 28)
		ui.Spacer(c)
		if a.iconButton(c.Key("rail-chats"), "message-square", "对话", ar == areaChats).Clicked() {
			a.go_(route{kind: routeSessions})
		}
		if a.iconButton(c.Key("rail-spaces"), "layers", "Space", ar == areaSpaces).Clicked() {
			a.go_(route{kind: routeSpaces})
		}
		search := ui.ButtonBase(c.Key("search")).Row().Gap(4).AlignItems(ui.Center).Height(26).Padding(0, 8).Radius(6).Background(p.hover).Label("搜索 (⌘K)").FocusRing(false).Children(func() {
			ui.Icon(c, icon("search")).Size(13, 13).TextColor(p.textTertiary)
			ui.Text(c, "⌘K").FontSize(10).TextColor(p.textTertiary)
		})
		if search.Hovered() {
			search.Background(p.hoverStrong)
		}
		if search.Clicked() {
			a.palette.open(paletteAll)
		}
		if a.iconButton(c.Key("collapse"), "panel-left", "收起侧栏 (⌘B)", false).Clicked() {
			a.sidebarOpen = false
		}
	})
}

func (a *app) chatsSidebar(c *ui.Context) {
	p := a.pal
	ui.Row(c).Height(38).Shrink(0).AlignItems(ui.Center).Gap(2).Padding(4, 8, 0, 12).Children(func() {
		ui.Text(c, "对话").FontSize(14).FontWeight(600).Grow(1)
		a.iconButton(c.Key("filter"), "list-filter", "筛选", false)
		add := ui.ButtonBase(c.Key("new-chat")).Size(28, 28).Center().Radius(6).Label("新建对话 (⌘O)").FocusRing(false).Children(func() {
			ui.Icon(c, icon("plus")).Size(16, 16).TextColor(p.brand)
		})
		if add.Hovered() {
			add.Background(p.brandMuted)
		}
		if add.Clicked() {
			a.newChat()
		}
	})
	ui.ScrollHorizontal(c.Key("chips")).Shrink(0).Scrollbars(false).Padding(6, 6, 10, 6).Children(func() {
		ui.Row(c).Gap(2).Children(func() {
			a.spaceChip(c, "", "全部", "")
			for _, sp := range a.chatSpaces() {
				a.spaceChip(c, sp.ID, sp.Name, sp.avatar())
			}
		})
	})
	list := a.visibleSessions()
	a.sessionList.Key = func(i int) any { return list[i].ID }
	ui.List(c.Key("sessions"), &a.sessionList, len(list), func(i int) {
		a.sessionRow(c, list[i])
	}).Grow(1).Padding(0, 6, 6, 6).Gap(0).Children(func() {
		if len(list) == 0 {
			if a.status != "" && a.me == nil {
				ui.Row(c).Gap(6).Padding(16, 10).Children(func() {
					a.spinner(c, 12, p.textTertiary)
					ui.Text(c, "加载对话…").FontSize(12).TextColor(p.textTertiary)
				})
				return
			}
			ui.Text(c, "还没有对话").FontSize(12).TextColor(p.textTertiary).Padding(16, 10)
		}
	})
}

func (a *app) spaceChip(c *ui.Context, id, name, avatar string) {
	p := a.pal
	on := a.chip == id
	b := ui.ButtonBase(c.Key("chip-"+id)).Row().Gap(5).AlignItems(ui.Center).Height(26).Padding(0, 6).Radius(6).Label(name).FocusRing(false).Children(func() {
		if id != "" {
			a.avatar(c, avatar, name, 16, 4)
		}
		col := p.textSecond
		if on {
			col = p.brand
		}
		ui.Text(c, name).FontSize(12).TextColor(col).SingleLine().MaxWidth(120)
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

// sessionRow is a chat in the list, as SessionRow.svelte: the Space's
// picture, the title and time, then how long a running turn has run and
// the latest reply.
func (a *app) sessionRow(c *ui.Context, s *Session) {
	p := a.pal
	now := a.now()
	row := ui.Row(c.Key(s.ID)).Gap(10).AlignItems(ui.Center).Height(52).Padding(0, 6).Radius(6).Children(func() {
		name, av := a.spaceName(s.SpaceID)
		a.avatar(c, av, name, 36, 8)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, s.displayTitle()).FontSize(14).FontWeight(500).SingleLine().Grow(1).MinWidth(0)
				ui.Text(c, relTime(s.when(), now)).FontSize(11).TextColor(p.textTertiary).Shrink(0)
			})
			ui.Row(c).Gap(6).AlignItems(ui.Center).Height(18).Children(func() {
				if s.running() {
					a.runningPill(c, s.ActiveTurn.StartedAt)
				}
				ui.Text(c, preview(s.LatestMessageText)).FontSize(12).TextColor(p.textTertiary).SingleLine().Grow(1).MinWidth(0)
				if a.unread(s) {
					a.dot(c, p.errorC, 6)
				}
			})
		})
	})
	a.pressable(row, s.ID == a.route.sessionID())
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
	ui.Row(c).Gap(2).AlignItems(ui.Center).Padding(1, 5).Radius(4).Background(p.brandMuted).Shrink(0).Children(func() {
		for i := 0; i < 3; i++ {
			a.dot(c.Key(i), p.brand, 3).Opacity(0.55)
		}
		label := "运行中"
		if !since.IsZero() {
			label += " " + elapsed(since, a.now())
			c.After(time.Second)
		}
		ui.Text(c, label).FontSize(11).TextColor(p.textSecond).SingleLine().MarginStart(4)
	})
}

var sidebarSpaceNav = []struct{ icon, label string }{
	{"clock", "最近"}, {"layout-grid", "全部 Space"}, {"user", "我的"}, {"pin", "置顶"}, {"archive", "已归档"},
}

// navRow is a row of the sidebars' navigation: an icon and a label, the
// chosen one on the active tint.
func (a *app) navRow(c *ui.Context, iconName, label string, on bool, height float32, fn func()) {
	p := a.pal
	row := ui.Row(c.Key(label)).Gap(10).AlignItems(ui.Center).Height(height).Padding(0, 8).Radius(6).Children(func() {
		col := p.textSecond
		if on {
			col = p.text
		}
		ui.Icon(c, icon(iconName)).Size(15, 15).TextColor(col)
		t := ui.Text(c, label).FontSize(13).TextColor(col).SingleLine()
		if on {
			t.FontWeight(600)
		}
	})
	a.pressable(row, on)
	row.OnClick(fn)
}

// spacesNavSidebar is the Space directory's filters.
func (a *app) spacesNavSidebar(c *ui.Context) {
	ui.Column(c).Grow(1).Padding(8, 6).Gap(2).BorderWidth(1, 0, 0, 0).BorderColor(a.pal.borderSubtle).Children(func() {
		for i, it := range sidebarSpaceNav {
			a.navRow(c, it.icon, it.label, a.spacesNav == i, 36, func() { a.spacesNav = i; a.spaceList = ui.ListState{} })
		}
	})
}

// spaceSidebar is one Space's: its switcher, a new chat, its tools, its
// labels and its chats by source.
func (a *app) spaceSidebar(c *ui.Context) {
	p := a.pal
	spaceID := a.route.space
	name, av := a.spaceName(spaceID)
	if name == "" {
		name = "Space"
	}
	ui.Column(c).Shrink(0).BorderWidth(1, 0, 1, 0).BorderColor(p.borderSubtle).Padding(0, 6).Children(func() {
		sw := ui.Row(c.Key("switcher")).Gap(8).AlignItems(ui.Center).Height(46).Padding(0, 6).Radius(6).Children(func() {
			a.avatar(c, av, name, 26, 6)
			ui.Text(c, name).FontSize(14).FontWeight(600).SingleLine().Grow(1).MinWidth(0)
			ui.Icon(c, icon("chevron-down")).Size(14, 14).TextColor(p.textTertiary)
		})
		a.pressable(sw, false)
		sw.OnClick(func() { a.palette.open(paletteSpaces) })
	})
	ui.Scroll(c.Key("space-side")).Grow(1).Children(func() {
		ui.Column(c).Padding(8, 6, 8, 6).Gap(1).Children(func() {
			nb := ui.Row(c.Key("new")).Gap(6).AlignItems(ui.Center).Height(32).Padding(0, 8).Radius(7).Background(p.brandMuted).Border(1, p.brandBorder).Margin(0, 0, 4, 0).Cursor(ui.CursorPointer).Children(func() {
				ui.Icon(c, icon("plus")).Size(14, 14).TextColor(p.brand)
				ui.Text(c, "新建对话").FontSize(13).TextColor(p.brand).Grow(1)
				ui.Text(c, "⌘O").FontSize(10).TextColor(p.brand).Padding(1, 4).Radius(4).Border(1, p.brandBorder)
			})
			if nb.Hovered() {
				nb.Background(p.brand.Alpha(0.15))
			}
			nb.OnClick(func() { a.go_(route{kind: routeSpace, space: spaceID}) })
			r := a.route
			a.navRow(c, "settings", "设置", r.kind == routeSpaceSettings, 33, func() { a.go_(route{kind: routeSpaceSettings, space: spaceID, sub: "profile"}) })
			a.navRow(c, "activity", "活动", r.kind == routeSpaceActivity, 33, func() { a.go_(route{kind: routeSpaceActivity, space: spaceID}) })
			a.navRow(c, "save", "新建存档", r.kind == routeSpaceCheckpoint && r.sub == "new", 33, func() {
				a.go_(route{kind: routeSpaceCheckpoint, space: spaceID, sub: "new"})
			})
			ui.Box(c).Height(14)
			a.labelsSection(c, spaceID)
			a.chatsSection(c, spaceID)
		})
	})
}

func (a *app) sectionHead(c *ui.Context, key, iconName, label string, extra func()) bool {
	p := a.pal
	open := a.groupsOpen[key]
	head := ui.Row(c.Key("head-"+key)).Gap(8).AlignItems(ui.Center).Height(28).Padding(0, 6).Radius(6).Children(func() {
		name := "chevron-down"
		if !open {
			name = "chevron-up"
		}
		ui.Icon(c, icon(name)).Size(12, 12).TextColor(p.textTertiary)
		ui.Icon(c, icon(iconName)).Size(14, 14).TextColor(p.placeholder)
		ui.Text(c, label).FontSize(11).TextColor(p.placeholder).Grow(1)
		if extra != nil {
			extra()
		}
	})
	a.pressable(head, false)
	head.OnClick(func() { a.groupsOpen[key] = !open })
	return open
}

func (a *app) labelsSection(c *ui.Context, spaceID string) {
	p := a.pal
	a.once("labels:"+spaceID, func(ctx context.Context) {
		labels, err := a.api.labels(ctx, spaceID)
		if err == nil {
			a.update(func() { a.labels[spaceID] = labels })
		}
	})
	if !a.sectionHead(c, "labels", "tag", "标签", func() {
		ui.Icon(c, icon("plus")).Size(12, 12).TextColor(p.placeholder)
	}) {
		return
	}
	var user []*Label
	for _, l := range a.labels[spaceID] {
		if l.Source != "system" {
			user = append(user, l)
		}
	}
	if len(user) == 0 {
		ui.Text(c, "暂无标签").FontSize(12).TextColor(p.textTertiary).Padding(6, 24)
		return
	}
	for _, l := range user {
		a.labelRow(c, spaceID, l, l.Name, 0)
	}
}

// chatsSection lists the Space's chats: by source, by user and channel,
// then all of them, as Sidebar.svelte's chatsSection.
func (a *app) chatsSection(c *ui.Context, spaceID string) {
	p := a.pal
	ui.Box(c).Height(8)
	if !a.sectionHead(c, "chats", "message-square", "对话", nil) {
		return
	}
	for _, l := range a.labels[spaceID] {
		switch {
		case strings.HasPrefix(l.SystemKey, "session-source:"):
			for _, child := range l.Children {
				a.labelRow(c, spaceID, child, "Source/"+child.Name, 0)
			}
		}
	}
	for _, l := range a.labels[spaceID] {
		if strings.HasPrefix(l.SystemKey, "session-channel:") || strings.HasPrefix(l.SystemKey, "session-user:") {
			a.labelRow(c, spaceID, l, l.Name, 0)
		}
	}
	key := "all:" + spaceID
	open, ok := a.groupsOpen[key]
	if !ok {
		open = true
	}
	head := ui.Row(c.Key("all")).Gap(6).AlignItems(ui.Center).Height(30).Padding(0, 6).Radius(6).Margin(4, 0, 0, 0).Children(func() {
		name := "chevron-down"
		if !open {
			name = "chevron-right"
		}
		ui.Icon(c, icon(name)).Size(12, 12).TextColor(p.textTertiary)
		ui.Text(c, "全部").FontSize(13).TextColor(p.textSecond)
	})
	a.pressable(head, false)
	head.OnClick(func() { a.groupsOpen[key] = !open })
	if !open {
		return
	}
	list := a.spaceSessions[spaceID]
	if list == nil {
		ui.Row(c).Padding(8).Children(func() { a.spinner(c, 12, p.textTertiary) })
		return
	}
	for _, s := range list {
		a.spaceSessionRow(c, s, 0)
	}
}

func (a *app) spaceSessionRow(c *ui.Context, s *Session, indent float32) {
	p := a.pal
	row := ui.Row(c.Key(s.ID)).Gap(6).AlignItems(ui.Center).Height(30).Padding(0, 6, 0, 6+indent).Radius(6).Children(func() {
		ui.Text(c, s.displayTitle()).FontSize(13).SingleLine().Grow(1).MinWidth(0)
		if src := sourceLabel(s.Source); src != "" {
			ui.Text(c, src).FontSize(10).FontWeight(500).TextColor(p.textTertiary).Padding(1, 5).Radius(3).Background(p.hoverStrong).Shrink(0)
		}
		switch {
		case s.running():
			a.spinner(c, 11, p.brand)
		default:
			ui.Text(c, relTime(s.when(), a.now())).FontSize(10).TextColor(p.placeholder).Shrink(0)
		}
		if a.unread(s) {
			a.dot(c, p.errorC, 6)
		}
	})
	a.pressable(row, s.ID == a.route.sessionID())
	row.OnClick(func() { a.openSession(s) })
}

// labelRow is a label of the tree, which opens into the chats it holds.
func (a *app) labelRow(c *ui.Context, spaceID string, l *Label, ref string, depth int) {
	p := a.pal
	key := "label:" + l.ID
	open := a.groupsOpen[key]
	row := ui.Row(c.Key(l.ID)).Gap(6).AlignItems(ui.Center).Height(30).Padding(0, 6, 0, 6+float32(depth)*14).Radius(6).Children(func() {
		name := "chevron-right"
		if open {
			name = "chevron-down"
		}
		ui.Icon(c, icon(name)).Size(12, 12).TextColor(p.textTertiary)
		ui.Text(c, a.labelName(l)).FontSize(13).TextColor(p.textSecond).SingleLine()
	})
	a.pressable(row, false)
	row.OnClick(func() { a.groupsOpen[key] = !open })
	if !open {
		return
	}
	for _, child := range l.Children {
		a.labelRow(c, spaceID, child, ref+"/"+child.Name, depth+1)
	}
	items, ok := a.labelItems[key]
	a.once("items:"+l.ID, func(ctx context.Context) {
		got, err := a.api.labelItems(ctx, spaceID, ref)
		a.update(func() {
			if err != nil {
				got = []LabelItem{}
			}
			a.labelItems[key] = got
		})
	})
	if !ok {
		ui.Row(c).Padding(6, 24).Children(func() { a.spinner(c, 12, p.textTertiary) })
		return
	}
	for _, it := range items {
		if it.ResourceType != "session" {
			continue
		}
		if s := a.session(it.ResourceRef); s != nil {
			a.spaceSessionRow(c, s, float32(depth+1)*14)
		}
	}
}

// labelName names a system label as the sidebar does: a user's label by
// their name.
func (a *app) labelName(l *Label) string {
	if strings.HasPrefix(l.SystemKey, "session-user:") && l.Depth > 0 {
		if a.me != nil && a.me.Profile != nil && a.me.UUID == l.Name {
			return a.me.Profile.name()
		}
	}
	return l.Name
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
	case "space_hook":
		return "Hook"
	}
	return strings.ToUpper(src[:1]) + src[1:]
}

var settingsSections = []struct{ id, icon, label string }{
	{"general", "settings", "常规设置"},
	{"activity", "activity", "活动"},
	{"referrals", "gift", "邀请奖励"},
	{"billing", "credit-card", "账单"},
	{"rules", "square-pen", "用户规则"},
	{"channels", "network", "渠道"},
}

// settingsNav is the account settings' sidebar.
func (a *app) settingsNav(c *ui.Context) {
	ui.Column(c).Grow(1).Padding(8, 6).Gap(2).BorderWidth(1, 0, 0, 0).BorderColor(a.pal.borderSubtle).Children(func() {
		a.navRow(c, "arrow-left", "返回", false, 34, func() { a.goBackFromSettings() })
		ui.Box(c).Height(10)
		for _, s := range settingsSections {
			a.navRow(c, s.icon, s.label, a.route.sub == s.id, 38, func() { a.go_(route{kind: routeSettings, sub: s.id}) })
		}
	})
}

func (a *app) goBackFromSettings() {
	for i := len(a.back) - 1; i >= 0; i-- {
		if a.back[i].kind != routeSettings {
			r := a.back[i]
			a.back = a.back[:i]
			a.show(r)
			return
		}
	}
	a.show(route{kind: routeSessions})
}

// accountRow is the signed-in user, whose menu holds the settings and
// the appearance.
func (a *app) accountRow(c *ui.Context) {
	p := a.pal
	name, av := "未登录", ""
	if a.me != nil && a.me.Profile != nil {
		name, av = a.me.Profile.name(), a.me.Profile.AvatarURL
	}
	ui.Row(c).Height(46).Shrink(0).Gap(6).AlignItems(ui.Center).Padding(0, 6).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		who := ui.ButtonBase(c.Key("account")).Row().Gap(8).AlignItems(ui.Center).Grow(1).Height(34).Padding(0, 6).Radius(6).Label("账户").FocusRing(false).Children(func() {
			a.avatar(c, av, name, 22, 11)
			ui.Text(c, name).FontSize(13).SingleLine().Grow(1)
			ui.Icon(c, icon("chevron-down")).Size(13, 13).TextColor(p.textTertiary)
		})
		switch {
		case a.route.area() == areaSettings || a.userMenu:
			who.Background(p.active)
		case who.Hovered():
			who.Background(p.hover)
		}
		if who.Clicked() {
			a.userMenu = !a.userMenu
		}
		items := []menuItem{
			{icon: "settings", label: "设置", run: func() { a.go_(route{kind: routeSettings, sub: "general"}) }},
			{icon: "credit-card", label: "账单", run: func() { a.go_(route{kind: routeSettings, sub: "billing"}) }},
			{},
			{icon: "sun", label: "浅色", checked: a.theme == "light", run: func() { a.setTheme("light") }},
			{icon: "moon", label: "深色", checked: a.theme == "dark", run: func() { a.setTheme("dark") }},
			{icon: "monitor", label: "跟随系统", checked: a.theme == "system", run: func() { a.setTheme("system") }},
		}
		a.menu(c, who, &a.userMenu, items)
		a.iconButton(c.Key("help"), "circle-help", "帮助", false)
	})
}
