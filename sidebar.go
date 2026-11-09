package main

import (
	"context"
	"net/url"
	"regexp"
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
		if a.iconButton(c.Key("rail-chats"), "message-square", m("sidebar_chats"), ar == areaChats).Clicked() {
			a.go_(route{kind: routeSessions})
		}
		if a.iconButton(c.Key("rail-spaces"), "layers", "Space", ar == areaSpaces).Clicked() {
			a.go_(route{kind: routeSpaces})
		}
		search := ui.ButtonBase(c.Key("search")).Row().Gap(4).AlignItems(ui.Center).Height(26).Padding(0, 8).Radius(6).Background(p.hover).Label(m("x_search_k")).FocusRing(false).Children(func() {
			ui.Icon(c, icon("search")).Size(13, 13).TextColor(p.textTertiary)
			ui.Text(c, "⌘K").FontSize(10).TextColor(p.textTertiary)
		})
		if search.Hovered() {
			search.Background(p.hoverStrong)
		}
		if search.Clicked() {
			a.palette.open(paletteAll)
		}
		if a.iconButton(c.Key("collapse"), "panel-left-close", m("sidebar_collapse"), false).Clicked() {
			a.sidebarOpen = false
		}
	})
}

func (a *app) chatsSidebar(c *ui.Context) {
	p := a.pal
	ui.Row(c).Height(40).Shrink(0).AlignItems(ui.Center).Gap(2).Padding(0, 6, 0, 12).Children(func() {
		ui.Text(c, m("nav_tab_chats")).FontSize(13).FontWeight(600).Grow(1)
		a.sourcePicker(c)
		add := ui.ButtonBase(c.Key("new-chat")).Size(28, 28).Center().Radius(6).Label(m("x_new_chat_k")).FocusRing(false).Children(func() {
			ui.Icon(c, icon("plus")).Size(16, 16).TextColor(p.brand)
		})
		if add.Hovered() {
			add.Background(p.brandMuted)
		}
		if add.Clicked() {
			a.newChat()
		}
	})
	a.chipBar(c)
	list := a.visibleSessions()
	a.sessionList.Key = func(i int) any { return list[i].ID }
	ui.List(c.Key("sessions"), &a.sessionList, len(list), func(i int) {
		a.sessionRow(c, list[i])
	}).Grow(1).Padding(6, 6, 6, 6).Gap(0).Children(func() {
		if len(list) == 0 {
			if a.status != "" && a.me == nil {
				ui.Row(c).Gap(6).Padding(16, 10).Children(func() {
					a.spinner(c, 12, p.textTertiary)
					ui.Text(c, m("x_loading_chats")).FontSize(12).TextColor(p.textTertiary)
				})
				return
			}
			ui.Text(c, m("x_no_chats")).FontSize(12).TextColor(p.textTertiary).Padding(16, 10)
		}
	})
}

// chipBar is ChatsPane's FilterBar: All, then a chip per Space with chats.
// FilterBar is 44px with a 16px inset below lg, 36px and 6px above.
func (a *app) chipBar(c *ui.Context) {
	h, x := float32(36), float32(6)
	if a.compact {
		h, x = 44, 16
	}
	ui.ScrollHorizontal(c.Key("chips")).Height(h).Shrink(0).Scrollbars(false).Padding(0, x).
		BorderWidth(0, 0, 1, 0).BorderColor(a.pal.borderSubtle).Children(func() {
		ui.Row(c).Height(h).Gap(4).AlignItems(ui.Center).Children(func() {
			a.spaceChip(c, "", m("chats_space_all"), "")
			for _, sp := range a.chatSpaces() {
				a.spaceChip(c, sp.ID, sp.Name, sp.avatar())
			}
		})
	})
}

// spaceChip is FilterChip.svelte: 32px tall below lg, 28px above.
func (a *app) spaceChip(c *ui.Context, id, name, avatar string) {
	p := a.pal
	on := a.chip == id
	h, r, x := float32(28), float32(6), float32(6)
	if a.compact {
		h, r, x = 32, 7, 8
	}
	b := ui.ButtonBase(c.Key("chip-"+id)).Row().Gap(6).AlignItems(ui.Center).Height(h).Padding(0, x).Radius(r).Label(name).FocusRing(false).Children(func() {
		if id != "" {
			a.avatarSeed(c, avatar, name, id, 16, 5)
		}
		col := p.textTertiary
		if on {
			col = p.brandMutedFg
		}
		ui.Text(c, name).FontSize(12).FontWeight(500).TextColor(col).SingleLine().MaxWidth(160)
		if on && id != "" {
			ui.Icon(c, icon("x")).Size(12, 12).TextColor(col.Alpha(0.7))
		}
	})
	switch {
	case on:
		b.Background(p.brandMuted)
	case b.Hovered():
		b.Background(p.hover)
	}
	if b.Clicked() {
		if on && id != "" {
			id = ""
		}
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
		a.avatarSeed(c, av, name, s.SpaceID, 36, 8)
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
	a.rowActions(c, row, s, false)
}

// rowActions are a chat row's hover buttons, as SessionRow.svelte: insert
// its reference (in a Space), rename it, archive or restore it. A rename
// edits the title in a box over the row.
func (a *app) rowActions(c *ui.Context, row ui.Element, s *Session, inSpace bool) {
	p := a.pal
	if a.rowRenaming == s.ID {
		row.Children(func() {
			ui.Row(c.Key("rename")).Absolute().Top(0).Left(0).Right(0).Bottom(0).AlignItems(ui.Center).Padding(0, 6).Background(p.sidebarBg).Children(func() {
				in := ui.TextInputBase(c.Key("in"), &a.renameText).FontSize(13).Height(26).Padding(0, 6).Radius(4).Background(p.hoverStrong).Grow(1).AutoFocus().Label(m("sidebar_session_name"))
				in.OnShortcut(0, ui.KeyEscape, func() { a.rowRenaming = "" })
				if in.Submitted() {
					t := strings.TrimSpace(a.renameText)
					a.rowRenaming = ""
					if t != s.Title {
						old := s.Title
						s.Title = t
						a.act(func(ctx context.Context) error {
							var title any = t
							if t == "" {
								title = nil
							}
							err := a.api.do(ctx, "PATCH", "/api/sessions/"+s.ID, map[string]any{"title": title}, nil)
							if err != nil {
								a.update(func() { s.Title = old })
							}
							return err
						})
					}
				}
			})
		})
		return
	}
	if !row.Hovered() || a.compact {
		return
	}
	row.Children(func() {
		ui.Row(c.Key("actions")).Absolute().Top(0).Bottom(0).Right(6).Gap(2).AlignItems(ui.Center).Children(func() {
			btn := func(key, ic, label string) bool {
				b := ui.ButtonBase(c.Key(key)).Size(24, 24).Center().Radius(5).Background(p.sidebarBg).Label(label).Tooltip(label).FocusRing(false).Children(func() {
					ui.Icon(c, icon(ic)).Size(13, 13).TextColor(p.textTertiary)
				})
				if b.Hovered() {
					b.Background(p.hoverStrong)
				}
				return b.Clicked()
			}
			if inSpace && btn("insert", "text-cursor-input", m("common_insert")) {
				a.insertReference("/sessions/" + s.ID + ".jsonl")
			}
			if btn("rename", "pencil", m("common_rename")) {
				a.rowRenaming, a.renameText = s.ID, s.Title
			}
			archived := s.ArchivedAt != nil
			if archived || !s.running() {
				ic, label := "archive", m("session_archive")
				if archived {
					ic, label = "archive-restore", m("session_unarchive")
				}
				if btn("archive", ic, label) {
					a.setSessionArchived(s, !archived)
				}
			}
		})
	})
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
		label := m("session_activity_running")
		if !since.IsZero() {
			label += " " + elapsed(since, a.now())
			c.After(time.Second)
		}
		ui.Text(c, label).FontSize(11).TextColor(p.textSecond).SingleLine().MarginStart(4)
	})
}

var sidebarSpaceNav = []struct{ icon, label string }{
	{"clock", "spaces_section_recent"}, {"layout-grid", "spaces_section_all"}, {"user", "x_mine"}, {"pin", "spaces_section_pinned"}, {"archive", "spaces_archived"},
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

// actionRow is a Space's action under its new chat: a small icon and a
// 12 px label, tertiary until hovered.
func (a *app) actionRow(c *ui.Context, iconName, label string, fn func()) {
	p := a.pal
	row := ui.Row(c.Key(label)).Gap(8).AlignItems(ui.Center).Height(30).Padding(0, 6).Radius(5).Children(func() {
		ui.Icon(c, icon(iconName)).Size(14, 14).TextColor(p.textTertiary)
		ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(p.textTertiary).SingleLine()
	})
	if row.Hovered() {
		row.Background(p.hover)
	}
	row.Cursor(ui.CursorPointer).OnClick(fn)
}

// spacesNavSidebar is the Space directory's filters.
func (a *app) spacesNavSidebar(c *ui.Context) {
	ui.Column(c).Grow(1).Padding(8, 6).Gap(2).BorderWidth(1, 0, 0, 0).BorderColor(a.pal.borderSubtle).Children(func() {
		for i, it := range sidebarSpaceNav {
			a.navRow(c, it.icon, m(it.label), a.spacesNav == i, 36, func() { a.spacesNav = i; a.spaceList = ui.ListState{} })
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
			a.avatarSeed(c, av, name, spaceID, 26, 6)
			ui.Text(c, name).FontSize(13).FontWeight(500).SingleLine().Grow(1).MinWidth(0)
			ui.Icon(c, icon("chevron-down")).Size(14, 14).TextColor(p.textTertiary)
		})
		a.pressable(sw, false)
		sw.OnClick(func() { a.palette.open(paletteSpaces) })
	})
	ui.Scroll(c.Key("space-side")).Grow(1).Children(func() {
		ui.Column(c).Padding(8, 6, 8, 6).Gap(1).Children(func() {
			nb := ui.Row(c.Key("new")).Gap(6).AlignItems(ui.Center).Height(32).Padding(0, 8).Radius(7).Background(p.brandMuted).Border(1, p.brandBorder).Margin(0, 0, 4, 0).Cursor(ui.CursorPointer).Children(func() {
				ui.Icon(c, icon("plus")).Size(14, 14).TextColor(p.brand)
				ui.Text(c, m("sidebar_new_chat")).FontSize(12).FontWeight(500).TextColor(p.brand).Grow(1)
				ui.Text(c, "⌘O").FontSize(10).TextColor(p.brand).Padding(1, 4).Radius(4).Border(1, p.brandBorder)
			})
			if nb.Hovered() {
				nb.Background(p.brand.Alpha(0.15))
			}
			nb.OnClick(func() { a.go_(route{kind: routeSpace, space: spaceID}) })
			a.actionRow(c, "settings", m("nav_settings"), func() { a.go_(route{kind: routeSpaceSettings, space: spaceID, sub: "profile"}) })
			a.actionRow(c, "activity", m("nav_activity"), func() { a.go_(route{kind: routeSpaceActivity, space: spaceID}) })
			a.actionRow(c, "save", m("sidebar_new_save"), func() {
				a.go_(route{kind: routeSpaceCheckpoint, space: spaceID, sub: "new"})
			})
			ui.Box(c).Height(14)
			a.labelsSection(c, spaceID)
			a.chatsSection(c, spaceID)
			a.spaceSections(c, spaceID)
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
	if !a.sectionHead(c, "labels", "tag", m("sidebar_labels"), func() {
		add := ui.ButtonBase(c.Key("new-label")).Size(18, 18).Center().Radius(4).Label(m("sidebar_new_label")).FocusRing(false).Children(func() {
			ui.Icon(c, icon("plus")).Size(12, 12).TextColor(p.placeholder)
		})
		if add.Clicked() {
			a.newLabelFor, a.newLabelName = spaceID, ""
			a.groupsOpen["labels"] = true
		}
	}) {
		return
	}
	if a.newLabelFor == spaceID {
		ui.Row(c.Key("new-label-row")).Height(30).Padding(0, 6, 0, 24).AlignItems(ui.Center).Children(func() {
			in := ui.TextInputBase(c.Key("name"), &a.newLabelName).Placeholder(m("sidebar_new_label")).PlaceholderColor(p.placeholder).
				FontSize(12).Height(24).Padding(0, 6).Radius(4).Border(1, p.brand).Grow(1).AutoFocus().Label(m("sidebar_new_label"))
			in.OnShortcut(0, ui.KeyEscape, func() { a.newLabelFor = "" })
			if in.Submitted() {
				name := strings.TrimSpace(a.newLabelName)
				a.newLabelFor = ""
				if name != "" {
					a.act(func(ctx context.Context) error {
						return a.api.do(ctx, "POST", "/api/spaces/"+spaceID+"/labels", map[string]any{"labelRef": name}, nil)
					}, "labels:"+spaceID)
				}
			}
		})
	}
	var user []*Label
	for _, l := range a.labels[spaceID] {
		if l.Source != "system" {
			user = append(user, l)
		}
	}
	if len(user) == 0 && a.newLabelFor != spaceID {
		ui.Text(c, m("sidebar_no_labels")).FontSize(12).TextColor(p.textTertiary).Padding(6, 24)
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
	if !a.sectionHead(c, "chats", "message-square", m("sidebar_chats"), nil) {
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
		ui.Text(c, m("sidebar_all")).FontSize(13).TextColor(p.textSecond)
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
	limit, ok := a.pages.json["more:"+spaceID].(int)
	if !ok {
		limit = 20
	}
	for i, r := range forkTree(list, a.forks[spaceID]) {
		if i >= limit {
			more := ui.ButtonBase(c.Key("more")).Height(28).Radius(6).Center().FocusRing(false).Children(func() {
				ui.Text(c, m("sidebar_show_more")).FontSize(12).TextColor(p.textTertiary)
			})
			if more.Hovered() {
				more.Background(p.hover)
			}
			if more.Clicked() {
				a.pages.json["more:"+spaceID] = limit + 20
			}
			break
		}
		a.spaceSessionTreeRow(c, r)
	}
}

func (a *app) spaceSessionRow(c *ui.Context, s *Session, indent float32) {
	a.spaceSessionTreeRow(c, treeRow{s: s})
}

func (a *app) spaceSessionTreeRow(c *ui.Context, tr treeRow) {
	p := a.pal
	s := tr.s
	on := s.ID == a.route.sessionID()
	indent := float32(0)
	if tr.depth > 0 {
		indent = float32(11+(tr.depth-1)*12+7) - 6
	}
	row := ui.Column(c.Key(s.ID)).Gap(2).Justify(ui.Center).MinHeight(30).Padding(5, 6, 5, 6+indent).Radius(6).Children(func() {
		a.treeGuides(c, tr)
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			t := ui.Text(c, tr.title()).FontSize(13).TextColor(p.textTertiary).SingleLine().Grow(1).MinWidth(0)
			if on {
				t.TextColor(p.text).FontWeight(500)
			}
			if src := sourceLabel(s.Source); src != "" {
				ui.Text(c, src).FontSize(10).FontWeight(500).TextColor(p.textTertiary).Padding(1, 5).Radius(3).Background(p.hoverStrong).Shrink(0)
			}
			ui.Text(c, relTime(s.when(), a.now())).FontSize(10).TextColor(p.placeholder).Shrink(0)
			if a.unread(s) {
				a.dot(c, p.errorC, 6)
			}
		})
		if s.running() {
			ui.Row(c).Children(func() { a.runningPill(c, s.ActiveTurn.StartedAt) })
		}
	})
	if row.Hovered() && !on {
		row.Background(p.hover)
	}
	if on {
		row.Background(p.active)
	}
	row.Cursor(ui.CursorPointer)
	row.OnClick(func() { a.openSession(s) })
	a.rowActions(c, row, s, true)
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
	if l.Source == "user" && row.Hovered() {
		row.Children(func() {
			del := ui.ButtonBase(c.Key("del")).Absolute().Top(5).Right(6).Size(20, 20).Center().Radius(4).Background(p.sidebarBg).Label(m("common_delete")).FocusRing(false).Children(func() {
				ui.Icon(c, icon("trash-2")).Size(12, 12).TextColor(p.textTertiary)
			})
			if del.Clicked() {
				name := a.labelName(l)
				a.confirm(m("common_delete"), m("sidebar_delete_label_confirm", "label", name), m("common_delete"), func() {
					a.act(func(ctx context.Context) error {
						return a.api.do(ctx, "DELETE", "/api/spaces/"+spaceID+"/labels/by-ref?ref="+url.QueryEscape(ref), nil, nil)
					}, "labels:"+spaceID)
				})
			}
		})
	}
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
	k := sourceKey(src)
	if k == "web" {
		return ""
	}
	return sourceName(k)
}

// sourceKey is a chat's source as the inbox sorts them, as the server's
// resolveSessionSourceKey: "channel:feishu" is Feishu's, an unknown one
// "other".
func sourceKey(src string) string {
	raw := strings.ToLower(strings.TrimSpace(src))
	raw = regexp.MustCompile(`[\s-]+`).ReplaceAllString(raw, "_")
	switch raw {
	case "", "web_app":
		return "web"
	}
	known := func(k string) bool { return k == "other" || sourceName(k) != "" }
	if known(raw) {
		return raw
	}
	ch := strings.Split(raw, ":")[0]
	if mm := regexp.MustCompile(`^channel[:_](.+)$`).FindStringSubmatch(raw); mm != nil {
		ch = mm[1]
	}
	if ch != raw && known(ch) {
		return ch
	}
	return "other"
}

var settingsSections = []struct{ id, icon, label string }{
	{"general", "settings", "nav_general"},
	{"activity", "activity", "nav_activity"},
	{"referrals", "gift", "nav_referrals"},
	{"billing", "credit-card", "nav_billing"},
	{"rules", "square-pen", "nav_user_rules"},
	{"channels", "network", "nav_channels"},
}

// settingsNav is the account settings' sidebar.
func (a *app) settingsNav(c *ui.Context) {
	ui.Column(c).Grow(1).Padding(8, 6).Gap(2).BorderWidth(1, 0, 0, 0).BorderColor(a.pal.borderSubtle).Children(func() {
		a.navRow(c, "arrow-left", m("nav_back"), false, 34, func() { a.goBackFromSettings() })
		ui.Box(c).Height(10)
		for _, s := range settingsSections {
			a.navRow(c, s.icon, m(s.label), a.route.sub == s.id, 38, func() { a.go_(route{kind: routeSettings, sub: s.id}) })
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

// accountRow is the signed-in user, whose menu holds the plan, the
// settings and signing out, as the sidebar's user menu.
func (a *app) accountRow(c *ui.Context) {
	p := a.pal
	name, av := m("x_not_signed_in"), ""
	if a.me != nil && a.me.Profile != nil {
		name, av = a.me.Profile.name(), a.me.Profile.AvatarURL
	}
	ui.Row(c).Shrink(0).Gap(4).AlignItems(ui.Center).Padding(6).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		who := ui.ButtonBase(c.Key("account")).Row().Gap(8).AlignItems(ui.Center).Justify(ui.Start).Grow(1).MinWidth(0).Padding(6).Radius(5).Label(m("x_account")).FocusRing(false).Children(func() {
			a.avatar(c, av, name, 22, 11)
			ui.Text(c, name).FontSize(12).TextColor(p.textSecond).SingleLine().Grow(1)
			chev := "chevron-down"
			if a.userMenu {
				chev = "chevron-up"
			}
			ui.Icon(c, icon(chev)).Size(12, 12).TextColor(p.textTertiary)
		})
		switch {
		case a.route.area() == areaSettings:
			who.Background(p.active)
		case who.Hovered():
			who.Background(p.hover)
		}
		if who.Clicked() {
			a.userMenu = !a.userMenu
		}
		plan, bal := m("sidebar_free_plan"), ""
		if subs, _, _ := loadJSON[billingSubscriptions](a, "subscriptions", "/api/billing/subscriptions"); subs != nil {
			for _, s := range subs.Subscriptions.Items {
				if s.Status == "active" {
					plan = s.ProductName
					break
				}
			}
		}
		if cr, _, _ := loadJSON[billingCredits](a, "credits", "/api/billing/credits"); cr != nil {
			bal = usd(cr.NetUSD)
		}
		items := []menuItem{
			{icon: "credit-card", label: plan, hint: bal, divider: true, run: func() { a.go_(route{kind: routeSettings, sub: "billing"}) }},
			{icon: "gift", label: m("nav_referrals"), run: func() { a.go_(route{kind: routeSettings, sub: "referrals"}) }},
		}
		if a.route.area() == areaSettings {
			items = append(items, menuItem{icon: "folder-kanban", label: m("nav_spaces"), run: a.goBackFromSettings})
		} else {
			items = append(items, menuItem{icon: "settings", label: m("nav_settings"), run: func() { a.go_(route{kind: routeSettings, sub: "general"}) }})
		}
		items = append(items,
			menuItem{icon: "chart-column", label: m("sidebar_trending"), run: func() { a.openURL("https://cohub.live/trending") }},
			menuItem{icon: "message-square-text", label: m("sidebar_feedback"), run: func() { a.feedback = feedbackForm{open: true} }},
			menuItem{icon: "log-out", label: m("sidebar_sign_out"), run: a.signOut},
		)
		a.menuWidth(c, who, &a.userMenu, sidebarWidth-12, items)
		if a.iconButton(c.Key("help"), "circle-help", m("sidebar_help"), false).Clicked() {
			a.helpOpen = true
		}
	})
}

// collapsedRail is the sidebar folded to 52 DIPs: the mark, expanding,
// search and the areas, and the account at the bottom.
func (a *app) collapsedRail(c *ui.Context) {
	p := a.pal
	bar := c.TitleBar()
	ar := a.route.area()
	ui.Column(c.Key("rail")).Width(52).Shrink(0).AlignItems(ui.Center).Gap(4).Padding(4+bar.Height, 8, 8, 8).Background(p.sidebarBg).
		BorderWidth(0, 1, 0, 0).BorderColor(p.borderSubtle.Alpha(0.7)).DragWindow().Children(func() {
		home := ui.ButtonBase(c.Key("home")).Size(32, 32).Radius(7).Background(p.brand).Center().Label(m("sidebar_home")).FocusRing(false).Children(func() {
			ui.Text(c, "C").FontSize(11).FontWeight(700).TextColor(p.brandFg)
		})
		if home.Clicked() {
			a.go_(route{kind: routeSessions})
		}
		rail := func(name, label string, on bool, fn func()) {
			b := ui.ButtonBase(c.Key(label)).Size(32, 32).Radius(6).Center().Label(label).Tooltip(label).FocusRing(false).Children(func() {
				col := p.textTertiary
				if on {
					col = p.text
				}
				ui.Icon(c, icon(name)).Size(16, 16).TextColor(col)
			})
			switch {
			case on:
				b.Background(p.active)
			case b.Hovered():
				b.Background(p.hover)
			}
			if b.Clicked() {
				fn()
			}
		}
		rail("panel-left-open", m("sidebar_expand"), false, func() { a.sidebarOpen = true })
		rail("search", m("sidebar_search_everywhere"), false, func() { a.palette.open(paletteAll) })
		ui.Box(c).Height(8)
		rail("message-square", m("sidebar_chats"), ar == areaChats, func() { a.go_(route{kind: routeSessions}) })
		rail("layers", m("nav_spaces"), ar == areaSpaces || ar == areaSpace, func() { a.go_(route{kind: routeSpaces}) })
		ui.Spacer(c)
		if a.me != nil && a.me.Profile != nil {
			a.avatar(c, a.me.Profile.AvatarURL, a.me.Profile.name(), 26, 13)
		}
	})
}

// sourcePicker is ChatsSourcePicker.svelte: the list-filter action, with
// the source's name once it is not Web App, and its menu of sources.
func (a *app) sourcePicker(c *ui.Context) {
	p := a.pal
	current := sourceName(a.chats.source)
	if a.chats.archived {
		current = m("chats_archived")
	}
	ic := "list-filter"
	if a.chats.archived {
		ic = "archive"
	}
	open := a.menus["source"]
	plain := !a.chats.archived && a.chats.source == "web"
	size, glyph := float32(28), float32(16)
	if a.compact {
		size, glyph = 40, 20
	}
	b := ui.ButtonBase(c.Key("source")).Row().Gap(6).AlignItems(ui.Center).Height(size).Radius(size / 5).Label(m("chats_source_current", "source", current)).FocusRing(false).Children(func() {
		col := p.textTertiary
		if open {
			col = p.textSecond
		}
		ui.Icon(c, icon(ic)).Size(glyph, glyph).TextColor(col)
		if !plain {
			ui.Text(c, current).FontSize(12).TextColor(col).SingleLine().MaxWidth(120)
		}
	})
	if plain {
		b.Width(size).Center()
	} else {
		b.Padding(0, 8)
	}
	if open || b.Hovered() {
		b.Background(p.hover)
	}
	if b.Clicked() {
		open = !open
	}
	optH, optF := float32(28), float32(12)
	if a.compact {
		optH, optF = 40, 13
	}
	option := func(key, label, ic string, chosen bool, f chatsFilter) {
		row := ui.Row(c.Key(key)).Height(optH).Gap(8).AlignItems(ui.Center).Padding(0, 8).Radius(6).Cursor(ui.CursorPointer).Children(func() {
			if ic != "" {
				ui.Icon(c, icon(ic)).Size(14, 14).TextColor(p.textTertiary)
			}
			col := p.textSecond
			if chosen {
				col = p.text
			}
			ui.Text(c, label).FontSize(optF).TextColor(col).Grow(1).MinWidth(0).SingleLine()
			chk := ui.Icon(c, icon("check")).Size(14, 14).TextColor(p.brand)
			if !chosen {
				chk.Opacity(0)
			}
		})
		if row.Hovered() {
			row.Background(p.hover)
		}
		if row.Clicked() {
			open = false
			a.setChatsFilter(f)
		}
	}
	sep := func(k string) { ui.Box(c.Key(k)).Height(1).Background(p.borderSubtle).Margin(4, 0) }
	a.adaptivePopover(c, b, &open, 220, func() {
		option("all", sourceName(""), "", a.chats == chatsFilter{}, chatsFilter{})
		for _, g := range chatSourceGroups {
			sep("sep-" + g.id)
			if g.id != "other" {
				ui.Text(c, m("chats_source_group_"+g.id)).FontSize(11).TextColor(p.placeholder).Padding(4, 8, 2, 8)
			}
			for _, s := range g.sources {
				f := chatsFilter{source: s}
				option(s, sourceName(s), "", a.chats == f, f)
			}
		}
		sep("sep-archived")
		option("archived", m("chats_archived"), "archive", a.chats.archived, chatsFilter{archived: true})
	})
	a.menus["source"] = open
}
