package main

import (
	"context"
	"net/url"
	"strings"
	"time"

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
	run   *runCommand // the Run Command mode
}

// runCommand is a bash command run in the Space and its output.
type runCommand struct {
	command, status, err, output, taskID string
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
	seed     string
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
			key: "space:" + sp.ID, title: sp.Name, sub: oneLine(desc), owner: sp.OwnerProfile, avatar: sp.avatar(), seed: sp.ID,
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
	runCommand := paletteItem{key: "run", command: true, title: "Run Command", sub: "Execute bash in the current space",
		choose: func() { a.palette.shown, a.palette.run = true, &runCommand{status: "idle"} }}
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
				key: "session:" + se.ID, title: se.displayTitle(), sub: name, avatar: av, seed: se.SpaceID, owner: se.UserProfile,
				time: dateTime(se.when(), now), choose: func() { a.openSession(sess) },
			})
		}
	}
	if s.tab == 4 {
		for _, cmd := range []paletteItem{
			{key: "cmd-new-chat", command: true, title: m("sidebar_new_chat"), sub: "⌘O", choose: a.newChat},
			{key: "cmd-settings", command: true, title: m("nav_settings"), sub: "⌘,", choose: func() { a.go_(route{kind: routeSettings, sub: "general"}) }},
			{key: "cmd-sidebar", command: true, title: m("x_toggle_sidebar"), sub: "⌘B", choose: func() { a.sidebarOpen = !a.sidebarOpen }},
			{key: "cmd-manage", command: true, title: "Manage Spaces", sub: "Browse, organize, and archive spaces", choose: func() { a.go_(route{kind: routeSpaces}) }},
			{key: "cmd-changelog", command: true, title: "Changelog", sub: "What's new in Cohub", choose: func() { a.openURL("https://cohub.live/changelog") }},
			{key: "cmd-general", command: true, title: m("nav_general"), sub: m("nav_settings"), choose: func() { a.go_(route{kind: routeSettings, sub: "general"}) }},
			{key: "cmd-activity", command: true, title: m("nav_activity"), sub: m("nav_settings"), choose: func() { a.go_(route{kind: routeSettings, sub: "activity"}) }},
			{key: "cmd-billing", command: true, title: m("nav_billing"), sub: m("nav_settings"), choose: func() { a.go_(route{kind: routeSettings, sub: "billing"}) }},
			{key: "cmd-referrals", command: true, title: m("nav_referrals"), sub: m("nav_settings"), choose: func() { a.go_(route{kind: routeSettings, sub: "referrals"}) }},
			{key: "cmd-rules", command: true, title: m("nav_user_rules"), sub: m("nav_settings"), choose: func() { a.go_(route{kind: routeSettings, sub: "rules"}) }},
			{key: "cmd-channels", command: true, title: m("nav_channels"), sub: m("nav_settings"), choose: func() { a.go_(route{kind: routeSettings, sub: "channels"}) }},
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
		if s.run != nil {
			s.shown = true
		}
	}
	ui.DialogBase(c, &s.shown, func(backdrop, panel ui.Element) {
		backdrop.Background(p.bgPrimary.Alpha(0.56)).Justify(ui.Start).Padding(90, 0, 0, 0)
		w, _ := c.Size()
		panel.Width(min(720, w-32)).MaxHeight(640).Radius(12).Background(p.popBg).Border(1, p.border).
			Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25)).Clip()
		if s.run != nil {
			a.runCommandView(c, s.run)
			return
		}
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
			name := it.title
			if strings.HasPrefix(it.key, "session:") {
				name = it.sub
			}
			a.avatarSeed(c, it.avatar, name, it.seed, 36, 8)
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

// runCommandView is the palette's Run Command mode: a bash command run in
// the Space, as a task, and what it printed.
func (a *app) runCommandView(c *ui.Context, r *runCommand) {
	p := a.pal
	spaceID := a.currentSpace()
	ui.Row(c.Key("run-head")).Height(50).Shrink(0).Gap(10).AlignItems(ui.Center).Padding(0, 16).Background(p.popHead).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Icon(c, icon("square-terminal")).Size(16, 16).TextColor(p.textTertiary)
		in := ui.TextInputBase(c.Key("command"), &r.command).Placeholder(m("command_run_placeholder")).PlaceholderColor(p.placeholder).
			FontSize(15).Font(fontMono).Grow(1).AutoFocus().Label(m("command_run_title"))
		in.OnShortcut(0, ui.KeyEscape, func() { a.palette.run = nil })
		if in.Submitted() {
			a.submitRun(spaceID, r)
		}
		ui.Text(c, m("command_run_title")).FontSize(10).TextColor(p.textTertiary).Padding(2, 5).Radius(4).Border(1, p.border)
	})
	ui.Scroll(c.Key("run-out")).Grow(1).MinHeight(120).MaxHeight(480).Children(func() {
		ui.Column(c).Padding(12, 16).Gap(8).Children(func() {
			status := m("command_run_hint")
			col := p.textTertiary
			switch {
			case spaceID == "":
				status, col = m("command_run_needs_space"), p.warning
			case r.status == "queued":
				status = m("command_queued")
			case r.status == "running":
				status = m("command_running")
			case r.status == "done":
				status, col = m("command_done", "id", r.taskID[:min(8, len(r.taskID))]), p.success
			case r.status == "failed":
				status, col = r.err, p.errorC
			}
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				if r.status == "queued" || r.status == "running" {
					a.spinner(c, 12, p.textTertiary)
				}
				ui.Text(c, status).FontSize(12).TextColor(col)
			})
			if r.output != "" {
				ui.Text(c, r.output).FontSize(12).Font(fontMono).LineHeight(1.5).TextColor(p.textSecond).Padding(10, 12).Radius(6).Background(p.bgCode).Selectable()
			}
		})
	})
	ui.Row(c.Key("run-foot")).Height(32).Shrink(0).AlignItems(ui.Center).Padding(0, 12).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Text(c, m("command_press_run")+" · esc").FontSize(10.5).Font(fontMono).TextColor(p.textTertiary)
	})
}

func (a *app) submitRun(spaceID string, r *runCommand) {
	cmd := strings.TrimSpace(r.command)
	if spaceID == "" {
		r.status, r.err = "failed", m("command_open_space")
		return
	}
	if cmd == "" || r.status == "queued" || r.status == "running" {
		return
	}
	r.status, r.err, r.output = "queued", "", ""
	a.act(func(ctx context.Context) error {
		var out struct {
			TaskRunID string `json:"taskRunId"`
		}
		if err := a.api.do(ctx, "POST", sp(spaceID)+"/commands", map[string]any{"command": cmd}, &out); err != nil {
			a.update(func() { r.status, r.err = "failed", err.Error() })
			return nil
		}
		a.update(func() { r.status, r.taskID = "running", out.TaskRunID })
		for i := 0; i < 600; i++ {
			time.Sleep(time.Second)
			var t struct {
				Run struct {
					Status       string  `json:"status"`
					ErrorMessage *string `json:"errorMessage"`
					Result       *struct {
						Output  *string          `json:"output"`
						Content []map[string]any `json:"content"`
					} `json:"result"`
				} `json:"run"`
				Progress *struct {
					Content []map[string]any `json:"content"`
				} `json:"progress"`
			}
			pctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			err := a.api.do(pctx, "GET", "/api/tasks/"+url.PathEscape(out.TaskRunID), nil, &t)
			cancel()
			if err != nil {
				continue
			}
			text := ""
			if t.Progress != nil {
				text = toolOutput(t.Progress.Content)
			}
			switch t.Run.Status {
			case "completed":
				if res := t.Run.Result; res != nil {
					if res.Output != nil {
						text = *res.Output
					} else {
						text = toolOutput(res.Content)
					}
				}
				a.update(func() { r.status, r.output = "done", text })
				return nil
			case "failed":
				msg := m("command_failed")
				if t.Run.ErrorMessage != nil {
					msg = *t.Run.ErrorMessage
				}
				a.update(func() { r.status, r.err, r.output = "failed", msg, text })
				return nil
			}
			a.update(func() { r.output = text })
		}
		return nil
	})
}

// toolOutput is what tool results printed, or the text blocks.
func toolOutput(blocks []map[string]any) string {
	var out []string
	for _, b := range blocks {
		switch b["type"] {
		case "tool_result":
			switch c := b["content"].(type) {
			case string:
				out = append(out, c)
			case []any:
				for _, x := range c {
					if m, ok := x.(map[string]any); ok {
						if t, ok := m["text"].(string); ok {
							out = append(out, t)
						}
					}
				}
			}
		case "text":
			if t, ok := b["text"].(string); ok {
				out = append(out, t)
			}
		}
	}
	return strings.Join(out, "\n")
}
