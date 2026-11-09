package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

type paletteKind int

const (
	paletteAll    paletteKind = iota // ⌘K: chats, Spaces, labels and commands
	paletteSpaces                    // the Space picker of a new chat
)

type paletteState struct {
	shown bool
	kind  paletteKind
	query string
	tab   int
	sel   int
	list  ui.ListState
}

func (s *paletteState) open(k paletteKind) {
	*s = paletteState{shown: true, kind: k}
}

func (s *paletteState) toggle(k paletteKind) {
	if s.shown {
		s.shown = false
		return
	}
	s.open(k)
}

func (s *paletteState) tabs() []string {
	if s.kind == paletteSpaces {
		return []string{m("spaces_section_recent"), m("sidebar_all"), m("command_mine"), m("command_pinned"), m("spaces_archived")}
	}
	return []string{m("sidebar_all"), "Space", m("sidebar_chats"), m("sidebar_labels"), m("command_lens_command")}
}

// paletteItem is a row of the palette.
type paletteItem struct {
	key      string
	command  bool // a tile with a plus, as New Space
	title    string
	sub      string
	owner    *Profile
	avatar   string
	time     string
	pinned   bool
	choose   func()
	isAction bool
}

// paletteItems are the rows of the tab, filtered by the query.
func (a *app) paletteItems() []paletteItem {
	s := &a.palette
	q := strings.ToLower(strings.TrimSpace(s.query))
	match := func(parts ...string) bool {
		if q == "" {
			return true
		}
		return strings.Contains(strings.ToLower(strings.Join(parts, " ")), q)
	}
	now := a.now()
	spaceItem := func(sp *Space) paletteItem {
		desc := sp.Description
		if desc == "" {
			desc = "Space"
		}
		return paletteItem{
			key: "space:" + sp.ID, title: sp.Name, sub: oneLine(desc), owner: sp.OwnerProfile, avatar: sp.avatar(),
			time: dateTime(sp.activity(), now), pinned: sp.IsPinned,
			choose: func() { a.openSpace(sp.ID) },
		}
	}
	newSpace := paletteItem{key: "new-space", command: true, title: "New Space", sub: "Create a new space",
		choose: func() { a.go_(route{kind: routeNewSpace}) }}
	var items []paletteItem
	if s.kind == paletteSpaces {
		if q == "" || match("new space") {
			items = append(items, newSpace)
		}
		nav := []int{0, 1, 2, 3, 4}[s.tab]
		for _, sp := range a.filteredSpaces(nav, "") {
			if match(sp.Name, sp.Description) {
				items = append(items, spaceItem(sp))
			}
		}
		return items
	}
	runCommand := paletteItem{key: "run", command: true, title: "Run Command", sub: "Execute bash in the current space"}
	if s.tab == 0 || s.tab == 4 {
		if match("run command bash") {
			items = append(items, runCommand)
		}
	}
	if s.tab == 0 || s.tab == 1 {
		for i, sp := range a.filteredSpaces(0, "") {
			if match(sp.Name, sp.Description) {
				items = append(items, spaceItem(sp))
			}
			if i == 0 && s.tab == 0 && match("new space") {
				items = append(items, newSpace)
			}
		}
	}
	if s.tab == 0 || s.tab == 2 {
		for _, se := range a.sessions {
			if !match(se.Title, se.LatestMessageText) {
				continue
			}
			name, av := a.spaceName(se.SpaceID)
			sess := se
			items = append(items, paletteItem{
				key: "session:" + se.ID, title: se.displayTitle(), sub: name, avatar: av, owner: se.UserProfile,
				time: dateTime(se.when(), now), choose: func() { a.openSession(sess) },
			})
		}
	}
	if s.tab == 4 {
		for _, cmd := range []paletteItem{
			{key: "cmd-new-chat", command: true, title: m("sidebar_new_chat"), sub: "⌘O", choose: a.newChat},
			{key: "cmd-settings", command: true, title: m("nav_settings"), sub: "⌘,", choose: func() { a.go_(route{kind: routeSettings, sub: "general"}) }},
			{key: "cmd-sidebar", command: true, title: m("x_toggle_sidebar"), sub: "⌘B", choose: func() { a.sidebarOpen = !a.sidebarOpen }},
		} {
			if match(cmd.title) {
				items = append(items, cmd)
			}
		}
	}
	return items
}

// paletteView is the command palette, as CommandPalette.svelte: a search
// field, tabs, the results and the keys that move among them.
func (a *app) paletteView(c *ui.Context) {
	s := &a.palette
	if !s.shown {
		return
	}
	p := a.pal
	items := a.paletteItems()
	if s.sel >= len(items) {
		s.sel = max(0, len(items)-1)
	}
	choose := func(i int) {
		if i < 0 || i >= len(items) || items[i].choose == nil {
			return
		}
		s.shown = false
		items[i].choose()
	}
	ui.DialogBase(c, &s.shown, func(backdrop, panel ui.Element) {
		backdrop.Background(p.bgPrimary.Alpha(0.56)).Justify(ui.Start).Padding(90, 0, 0, 0)
		w, _ := c.Size()
		panel.Width(min(720, w-32)).MaxHeight(640).Radius(12).Background(p.popBg).Border(1, p.border).
			Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25)).Clip()
		ui.Row(c).Height(50).Shrink(0).Gap(10).AlignItems(ui.Center).Padding(0, 16).Background(p.popHead).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
			ui.Icon(c, icon("search")).Size(16, 16).TextColor(p.textTertiary)
			ph := m("command_placeholder")
			if s.kind == paletteSpaces {
				ph = m("command_placeholder_space")
			}
			in := ui.TextInputBase(c.Key("query"), &s.query).Placeholder(ph).PlaceholderColor(p.placeholder).FontSize(15).Grow(1).AutoFocus().Label(m("list_search"))
			if in.Changed() {
				s.sel = 0
			}
			in.OnTextKey(func(k ui.TextKey) bool {
				switch {
				case k.Key == ui.KeyDown || (k.Key == ui.KeyN && k.Mods == ui.Ctrl):
					s.sel = min(s.sel+1, len(items)-1)
				case k.Key == ui.KeyUp || (k.Key == ui.KeyP && k.Mods == ui.Ctrl):
					s.sel = max(s.sel-1, 0)
				case k.Key == ui.KeyEnter:
					choose(s.sel)
				default:
					return false
				}
				return true
			})
			ui.Text(c, "⌘K").FontSize(10).TextColor(p.textTertiary).Padding(2, 5).Radius(4).Border(1, p.border)
		})
		ui.Row(c).Height(36).Shrink(0).Gap(2).AlignItems(ui.Center).Padding(0, 10).Children(func() {
			for i, t := range s.tabs() {
				on := s.tab == i
				b := ui.ButtonBase(c.Key("tab-"+t)).Height(26).Padding(0, 7).Radius(5).Center().FocusRing(false).Children(func() {
					col := p.textSecond
					if on {
						col = p.text
						if s.kind == paletteAll {
							col = p.brand
						}
					}
					ui.Text(c, t).FontSize(13).TextColor(col)
				})
				if on {
					b.Background(p.hoverStrong)
					if s.kind == paletteAll {
						b.Background(p.brandMuted)
					}
				}
				if b.Clicked() {
					s.tab, s.sel = i, 0
				}
			}
			ui.Spacer(c)
			if s.kind == paletteSpaces {
				m := ui.ButtonBase(c.Key("manage")).Row().Gap(4).AlignItems(ui.Center).Height(24).Padding(0, 6).Radius(5).FocusRing(false).Children(func() {
					ui.Icon(c, icon("sliders-horizontal")).Size(12, 12).TextColor(p.textTertiary)
					ui.Text(c, m("spaces_manage")).FontSize(11).TextColor(p.textTertiary)
				})
				if m.Clicked() {
					s.shown = false
					a.go_(route{kind: routeSpaces})
				}
			}
		})
		ui.List(c.Key("results"), &s.list, len(items), func(i int) {
			a.paletteRow(c, items[i], i == s.sel, func() { choose(i) })
		}).Grow(1).MinHeight(0).Padding(0, 10, 6, 10).Gap(0).Children(func() {
			if len(items) == 0 {
				ui.Text(c, m("x_no_results")).FontSize(13).TextColor(p.textTertiary).Padding(24).Margin(0, ui.Auto)
			}
		})
		ui.Row(c).Height(32).Shrink(0).Gap(6).AlignItems(ui.Center).Padding(0, 12).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
			hint := m("command_type_default")
			if s.kind == paletteSpaces {
				hint = "Space"
			}
			ui.Text(c, hint+m("x_type_to_filter")).FontSize(10.5).Font(fontMono).TextColor(p.textTertiary).Grow(1)
			ui.Text(c, m("x_palette_keys")).FontSize(10.5).Font(fontMono).TextColor(p.textTertiary)
		})
	})
}

func (a *app) paletteRow(c *ui.Context, it paletteItem, sel bool, choose func()) {
	p := a.pal
	row := ui.Row(c.Key(it.key)).Gap(12).AlignItems(ui.Center).Height(52).Padding(0, 6).Radius(6).Children(func() {
		if it.command {
			ui.Box(c).Size(36, 36).Radius(8).Background(p.bgPrimary).Border(1, p.brandBorder).Center().Shrink(0).Children(func() {
				ui.Icon(c, icon("plus")).Size(16, 16).TextColor(p.brand)
			})
		} else {
			a.avatar(c, it.avatar, it.title, 36, 8)
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, it.title).FontSize(14).FontWeight(500).SingleLine()
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				if it.owner != nil {
					a.avatar(c, it.owner.AvatarURL, it.owner.name(), 14, 7)
					ui.Text(c, it.owner.name()).FontSize(12).TextColor(p.textSecond).Shrink(0)
					ui.Text(c, "·").FontSize(12).TextColor(p.placeholder)
				}
				ui.Text(c, it.sub).FontSize(12).TextColor(p.textSecond).SingleLine().MinWidth(0)
			})
		})
		if it.time != "" {
			ui.Text(c, it.time).FontSize(11).TextColor(p.textTertiary).Shrink(0).AlignSelf(ui.Start).Margin(8, 0, 0, 0)
		}
		ui.Box(c).Width(36).Center().Children(func() {
			switch {
			case it.command && sel:
				ui.Icon(c, icon("corner-down-left")).Size(12, 12).TextColor(p.brand)
			case it.pinned && a.palette.kind == paletteSpaces:
				ui.Icon(c, icon("pin")).Size(14, 14).TextColor(p.brand.Alpha(0.8))
			}
		})
	})
	switch {
	case sel:
		row.Background(p.brand.Alpha(0.06).Over(p.hover))
	case row.Hovered():
		row.Background(p.hover)
	}
	row.Cursor(ui.CursorPointer).OnClick(choose)
}
