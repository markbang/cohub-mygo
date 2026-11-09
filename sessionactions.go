package main

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// What the chat header does: rename the chat in place, share it, label
// it, show its statistics, quote it in the composer and archive it, as
// SpaceWorkspaceHeader, SessionShareDialog, ResourceLabelPicker and
// SessionStatsDetails.

// field is the app's text box: a hairline border that turns brand while
// it has the focus.
func (a *app) field(c *ui.Context, key string, v *string, placeholder string) ui.Element {
	p := a.pal
	in := ui.TextInputBase(c.Key(key), v).Placeholder(placeholder).PlaceholderColor(p.placeholder).FontSize(13).
		Height(34).Padding(0, 10).Radius(6).Border(1, p.borderSubtle).Background(p.bgInput).Label(placeholder)
	if in.Focused() {
		in.Border(1, p.brand)
	}
	return in
}

// act runs a write against the API off the main thread, then reloads
// what the keys name; failures land in the status line.
func (a *app) act(fn func(ctx context.Context) error, reload ...string) {
	if a.api == nil {
		a.sendErr = m("x_demo_no_send")
		return
	}
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := fn(ctx)
		a.update(func() {
			if err != nil {
				a.toast, a.toastAt, a.toastOK = err.Error(), time.Now(), false
				return
			}
			for _, k := range reload {
				a.forget(k)
			}
		})
	})
}

// forget drops a loaded page so the next frame loads it again.
func (a *app) forget(key string) {
	delete(a.pages.json, key)
	a.reload(key)
}

// sessionTitle is the header's title, which a click turns into a text box.
func (a *app) sessionTitle(c *ui.Context, s *Session, title string) {
	p := a.pal
	if s != nil && a.renaming == s.ID {
		in := ui.TextInputBase(c.Key("rename"), &a.renameText).FontSize(13).Padding(2, 4).Radius(4).
			Background(p.hoverStrong).Grow(1).MinWidth(0).MaxWidth(480).AutoFocus().Label(m("space_header_session_name_ph"))
		save := func() {
			id, t := s.ID, strings.TrimSpace(a.renameText)
			a.renaming = ""
			if t == s.Title {
				return
			}
			old := s.Title
			s.Title = t
			a.act(func(ctx context.Context) error {
				var title any = t
				if t == "" {
					title = nil
				}
				err := a.api.do(ctx, "PATCH", "/api/sessions/"+url.PathEscape(id), map[string]any{"title": title}, nil)
				if err != nil {
					a.update(func() { s.Title = old })
				}
				return err
			})
		}
		if in.Submitted() {
			save()
		}
		in.OnShortcut(0, ui.KeyEscape, func() { a.renaming = "" })
		ok := ui.ButtonBase(c.Key("rename-ok")).Size(22, 22).Center().Radius(4).Label(m("common_save")).FocusRing(false).Children(func() {
			ui.Icon(c, icon("check")).Size(14, 14).TextColor(p.success)
		})
		if ok.Clicked() {
			save()
		}
		no := ui.ButtonBase(c.Key("rename-no")).Size(22, 22).Center().Radius(4).Label(m("common_cancel")).FocusRing(false).Children(func() {
			ui.Icon(c, icon("x")).Size(14, 14).TextColor(p.textTertiary)
		})
		if no.Clicked() {
			a.renaming = ""
		}
		ui.Spacer(c)
		return
	}
	t := ui.Text(c.Key("title"), title).FontSize(13).SingleLine().Grow(1).MinWidth(0).TextColor(p.textSecond)
	if s != nil {
		t.Cursor(ui.CursorText).Tooltip("Click to rename")
		if t.Hovered() {
			t.TextColor(p.text)
		}
		if t.Clicked() {
			a.renaming, a.renameText = s.ID, s.Title
		}
	}
}

// shareButton is the header's Share, Shared once anyone with the link
// can read the chat.
func (a *app) shareButton(c *ui.Context, s *Session) {
	p := a.pal
	public := s != nil && a.sessionPublic(s.ID)
	ic, label, col := "share-2", "Share", p.textTertiary
	if public {
		ic, label, col = "globe", "Shared", p.success
	}
	b := ui.ButtonBase(c.Key("share")).Row().Gap(6).AlignItems(ui.Center).Height(28).Padding(0, 8).Radius(6).Label(m("referral_share")).FocusRing(false).Children(func() {
		ui.Icon(c, icon(ic)).Size(14, 14).TextColor(col)
		if !a.compact {
			ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(col)
		}
	})
	if a.compact {
		b.Size(40, 40).Padding(0).Center()
	}
	if b.Hovered() {
		b.Background(p.hover)
	}
	if b.Clicked() && s != nil {
		a.shareOpen = true
	}
	if a.shareOpen && s != nil {
		a.shareDialog(c, s)
	}
}

type accessPolicy struct {
	SignedIn  *string `json:"signed_in_user"`
	Anonymous *string `json:"anonymous_user"`
}

func (a *app) sessionPublic(id string) bool {
	pol, _, _ := loadJSON[accessPolicy](a, "session-access:"+id, "/api/sessions/"+url.PathEscape(id)+"/access")
	return pol != nil && (pol.SignedIn != nil || pol.Anonymous != nil)
}

func (a *app) shareDialog(c *ui.Context, s *Session) {
	p := a.pal
	link := "https://cohub.live/spaces/" + s.SpaceID + "/sessions/" + s.ID
	key := "session-access:" + s.ID
	public := a.sessionPublic(s.ID)
	a.dialog(c, &a.shareOpen, m("session_share_title"), 400, func() {
		ui.Column(c.Key("share")).Padding(16).Gap(18).Children(func() {
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, "LINK").FontSize(10).FontWeight(600).LetterSpacing(0.8).TextColor(p.textTertiary)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(4, 4, 4, 10).Radius(8).Border(1, p.borderSubtle).Background(p.bgInput).Children(func() {
					ui.Text(c, link).FontSize(12).Font(fontMono).TextColor(p.textSecond).SingleLine().Grow(1).MinWidth(0)
					label := "Copy"
					if time.Since(a.copiedAt) < 2*time.Second {
						label = "Copied"
					}
					b := a.button(c, "copy", label, false)
					if b.Clicked() {
						c.WriteClipboard(link)
						a.copiedAt = time.Now()
					}
				})
			})
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, "ACCESS").FontSize(10).FontWeight(600).LetterSpacing(0.8).TextColor(p.textTertiary)
				row := ui.Row(c.Key("access")).Gap(10).AlignItems(ui.Center).Padding(10).Radius(8).Border(1, p.borderSubtle).Cursor(ui.CursorPointer)
				row.Children(func() {
					ic, col := "lock", p.textTertiary
					if public {
						ic, col = "globe", p.success
					}
					ui.Box(c).Size(32, 32).Radius(8).Center().Background(col.Alpha(0.1)).Children(func() {
						ui.Icon(c, icon(ic)).Size(16, 16).TextColor(col)
					})
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Text(c, m("anyone_with_link")).FontSize(13).FontWeight(500)
						desc := "Private"
						if public {
							desc = "Public · view only"
						}
						ui.Text(c, desc).FontSize(11).TextColor(p.textTertiary)
					})
					on := public
					ui.Switch(c.Key("public"), &on)
				})
				if row.Clicked() && a.can(s.SpaceID, "session.access.manage") {
					next := !public
					a.pages.json[key] = &accessPolicy{Anonymous: map[bool]*string{true: ptr("guest"), false: nil}[next]}
					a.act(func(ctx context.Context) error {
						if next {
							return a.api.do(ctx, "PATCH", "/api/sessions/"+url.PathEscape(s.ID)+"/access", map[string]any{"anonymous_user": "guest"}, nil)
						}
						return a.api.do(ctx, "DELETE", "/api/sessions/"+url.PathEscape(s.ID)+"/access", nil, nil)
					}, key)
				}
			})
		})
	})
}

func ptr[T any](v T) *T { return &v }

// moreMenu is the header's ⋯: label, stats, quote and archive.
func (a *app) moreMenu(c *ui.Context, s *Session) {
	b := a.iconButton(c.Key("more"), "ellipsis", m("space_header_more_actions"), a.menus["more"])
	if b.Clicked() {
		a.menus["more"] = !a.menus["more"]
	}
	open := a.menus["more"]
	items := []menuItem{}
	if s != nil {
		items = append(items,
			menuItem{icon: "list-tree", label: m("inline_label_as"), run: func() { a.labelTarget = &labelTarget{space: s.SpaceID, kind: "session", ref: s.ID} }},
			menuItem{icon: "gauge", label: m("stats_menu"), run: func() { a.statsFor = s.ID }},
			menuItem{icon: "text-cursor-input", label: m("space_header_insert_reference"), run: func() { a.insertReference("/sessions/" + s.ID + ".jsonl") }},
		)
		archived := s.ArchivedAt != nil
		label, ic := m("session_archive"), "archive"
		switch {
		case archived:
			label, ic = m("session_unarchive"), "archive-restore"
		case s.running():
			label = m("session_archive_running")
		}
		items = append(items, menuItem{icon: ic, label: label, run: func() {
			if !archived && s.running() {
				return
			}
			a.setSessionArchived(s, !archived)
		}})
	}
	a.menuWidth(c, b, &open, 176, items)
	a.menus["more"] = open
	if a.labelTarget != nil {
		a.labelPicker(c, b)
	}
	if s != nil && a.statsFor == s.ID {
		a.statsDialog(c, b, s)
	}
}

// insertReference puts a path in the composer as the web does: ` path `.
func (a *app) insertReference(path string) {
	a.draft += " `" + path + "` "
}

func (a *app) setSessionArchived(s *Session, archived bool) {
	method := "PUT"
	if !archived {
		method = "DELETE"
	}
	now := time.Now()
	old := s.ArchivedAt
	if archived {
		s.ArchivedAt = &now
	} else {
		s.ArchivedAt = nil
	}
	id := s.ID
	a.act(func(ctx context.Context) error {
		err := a.api.do(ctx, method, "/api/sessions/"+url.PathEscape(id)+"/archive", nil, nil)
		if err != nil {
			a.update(func() { s.ArchivedAt = old })
			return err
		}
		a.refresh()
		return nil
	})
}

func (a *app) archiveSession(s *Session) {
	if s != nil {
		a.setSessionArchived(s, true)
	}
}

// labelTarget is what the label picker labels.
type labelTarget struct {
	space, kind, ref string
	selected         map[string]bool
	loaded           bool
	newName          string
	creating         bool
	saving           bool
}

type resourceLabels struct {
	Labels      []*Label `json:"labels"`
	Assignments []struct {
		LabelID string `json:"labelId"`
	} `json:"assignments"`
}

// labelRow is a label of the picker's tree, with its ref ("parent/child").
type labelRow struct {
	label *Label
	ref   string
	depth int
}

// flatLabels flattens a Space's label tree in order.
func flatLabels(list []*Label) (out []labelRow) {
	var walk func(ls []*Label, prefix string, depth int)
	walk = func(ls []*Label, prefix string, depth int) {
		for _, l := range ls {
			ref := l.Name
			if prefix != "" {
				ref = prefix + "/" + l.Name
			}
			out = append(out, labelRow{l, ref, depth})
			walk(l.Children, ref, depth+1)
		}
	}
	walk(list, "", 0)
	return out
}

// labelPicker is ResourceLabelPicker: the user's labels with checks, a
// new label, and Apply.
func (a *app) labelPicker(c *ui.Context, anchor ui.Element) {
	p := a.pal
	t := a.labelTarget
	key := "res-labels:" + t.space + ":" + t.kind + ":" + t.ref
	res, err, _ := loadJSON[resourceLabels](a, key, "/api/spaces/"+url.PathEscape(t.space)+"/resources/"+t.kind+"/labels?resourceRef="+url.QueryEscape(t.ref))
	var labels []labelRow
	assigned := map[string]bool{}
	if res != nil {
		labels = flatLabels(res.Labels)
		for _, as := range res.Assignments {
			assigned[as.LabelID] = true
		}
		if !t.loaded {
			t.selected, t.loaded = map[string]bool{}, true
			for _, l := range labels {
				if assigned[l.label.ID] && l.label.Source == "user" {
					t.selected[l.ref] = true
				}
			}
		}
	}
	open := true
	ui.PopoverBase(c, anchor, &open, func(panel ui.Element) {
		panel.Width(360).Margin(6, 0, 0, 0).Radius(10).Background(p.bgPrimary).Border(1, p.borderSubtle).
			Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, 0.15)).Clip()
		ui.Column(c.Key("labels")).Padding(12).Gap(10).Children(func() {
			ui.Row(c).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Gap(2).Grow(1).Children(func() {
					ui.Text(c, m("inline_label_as")).FontSize(13).FontWeight(500)
					ui.Text(c, m("label_choose_hint")).FontSize(11).TextColor(p.textTertiary)
				})
				if a.iconButton(c.Key("close"), "x", m("label_dismiss_picker"), false).Clicked() {
					open = false
				}
			})
			ui.Scroll(c.Key("list")).MaxHeight(300).Children(func() {
				ui.Column(c).Gap(1).Children(func() {
					switch {
					case res == nil && err == nil:
						a.spinner(c, 14, p.textTertiary)
					case err != nil:
						ui.Text(c, err.Error()).FontSize(12).TextColor(p.errorC)
					case len(labels) == 0:
						ui.Column(c).Padding(12).Gap(4).AlignItems(ui.Center).Children(func() {
							ui.Text(c, m("sidebar_no_labels")).FontSize(12).FontWeight(500).TextColor(p.textSecond)
							ui.Text(c, m("label_create_hint")).FontSize(11).TextColor(p.textTertiary)
						})
					}
					for _, l := range labels {
						ref := l.ref
						user := l.label.Source == "user"
						on := t.selected[ref] || (!user && assigned[l.label.ID])
						row := ui.Row(c.Key(ref)).Height(30).Gap(8).AlignItems(ui.Center).Padding(0, 8, 0, 8+float32(l.depth)*16).Radius(6).Children(func() {
							box := ui.Box(c).Size(14, 14).Radius(4).Border(1, p.borderStrong).Center()
							if on {
								col := p.brand
								if !user {
									col = p.placeholder
								}
								box.Background(col).Border(1, col).Children(func() {
									ui.Icon(c, icon("check")).Size(10, 10).TextColor(p.brandFg)
								})
							}
							name := a.labelName(l.label)
							if l.label.SystemKey == "session-archived" || l.label.Name == "Archived" && !user {
								name = m("spaces_archived")
							}
							if strings.HasPrefix(l.label.SystemKey, "session-user:") && l.depth > 0 {
								if a.me != nil && a.me.Profile != nil && a.me.UUID == l.label.Name {
									a.avatar(c, a.me.Profile.AvatarURL, name, 14, 7)
								}
							}
							t := ui.Text(c, name).FontSize(12).Grow(1).SingleLine()
							if !user {
								t.TextColor(p.textTertiary)
							}
						})
						if user {
							row.Cursor(ui.CursorPointer)
							if row.Hovered() {
								row.Background(p.hover)
							}
							if row.Clicked() {
								t.selected[ref] = !on
							}
						}
					}
				})
			})
			if t.creating {
				ui.Row(c.Key("new")).Gap(6).Children(func() {
					in := a.field(c, "new-name", &t.newName, m("sidebar_new_label")).Grow(1).AutoFocus()
					add := a.button(c, "plus", m("composer_create"), true)
					if (in.Submitted() || add.Clicked()) && strings.TrimSpace(t.newName) != "" {
						name := strings.TrimSpace(t.newName)
						t.newName, t.creating = "", false
						t.selected[name] = true
						a.act(func(ctx context.Context) error {
							return a.api.do(ctx, "POST", "/api/spaces/"+url.PathEscape(t.space)+"/labels", map[string]any{"labelRef": name}, nil)
						}, key, "labels:"+t.space)
					}
				})
			}
			ui.Row(c.Key("foot")).Gap(8).AlignItems(ui.Center).Children(func() {
				nb := ui.ButtonBase(c.Key("new-label")).Row().Gap(4).AlignItems(ui.Center).Height(28).Padding(0, 6).Radius(6).FocusRing(false).Children(func() {
					ui.Icon(c, icon("plus")).Size(14, 14).TextColor(p.textTertiary)
					ui.Text(c, m("sidebar_new_label")).FontSize(12).TextColor(p.textTertiary)
				})
				if nb.Clicked() {
					t.creating = !t.creating
				}
				ui.Spacer(c)
				n := 0
				for _, v := range t.selected {
					if v {
						n++
					}
				}
				ui.Text(c, m("label_selected_count", "count", n)).FontSize(11).TextColor(p.textTertiary)
				apply := ui.ButtonBase(c.Key("apply")).Row().Gap(4).AlignItems(ui.Center).Height(28).Padding(0, 10).Radius(6).Background(p.brand).FocusRing(false).Children(func() {
					ui.Icon(c, icon("check")).Size(13, 13).TextColor(p.brandFg)
					ui.Text(c, m("common_apply")).FontSize(12).FontWeight(500).TextColor(p.brandFg)
				})
				if apply.Clicked() && !t.saving {
					refs := []string{}
					for ref, v := range t.selected {
						if v {
							refs = append(refs, ref)
						}
					}
					sort.Strings(refs)
					t.saving = true
					a.act(func(ctx context.Context) error {
						err := a.api.do(ctx, "PUT", "/api/spaces/"+url.PathEscape(t.space)+"/resources/"+t.kind+"/labels?resourceRef="+url.QueryEscape(t.ref), map[string]any{"labelRefs": refs}, nil)
						a.update(func() {
							t.saving = false
							if err == nil && a.labelTarget == t {
								a.labelTarget = nil
							}
						})
						return err
					}, key, "labels:"+t.space)
				}
			})
		})
	})
	if !open {
		a.labelTarget = nil
	}
}

type sessionStats struct {
	Stats struct {
		Own execStats `json:"own"`
	} `json:"stats"`
}

type execStats struct {
	Turns             int      `json:"turns"`
	Generations       int      `json:"generations"`
	Calls             *int     `json:"calls"`
	ToolCalls         *int     `json:"toolCalls"`
	ElapsedMs         *float64 `json:"elapsedMs"`
	QueueMs           *float64 `json:"queueMs"`
	ModelMs           *float64 `json:"modelMs"`
	ToolMs            *float64 `json:"toolMs"`
	TTFTMs            float64  `json:"ttftMs"`
	TTFTSamples       int      `json:"ttftSamples"`
	OutputMs          float64  `json:"outputMs"`
	TimedOutputTokens float64  `json:"timedOutputTokens"`
	UnknownUsageCalls int      `json:"unknownUsageCalls"`
	Usage             *struct {
		Input       int64 `json:"input"`
		Output      int64 `json:"output"`
		CacheRead   int64 `json:"cacheRead"`
		CacheWrite  int64 `json:"cacheWrite"`
		TotalTokens int64 `json:"totalTokens"`
	} `json:"usage"`
	ModelCostUSD *float64 `json:"modelCostUsd"`
}

// statsDialog is the chat's statistics: the headline numbers, where the
// time went, what the tokens were.
func (a *app) statsDialog(c *ui.Context, anchor ui.Element, s *Session) {
	p := a.pal
	st, err, _ := loadJSON[sessionStats](a, "stats:"+s.ID, "/api/sessions/"+url.PathEscape(s.ID)+"/stats")
	open := true
	ui.PopoverBase(c, anchor, &open, func(panel ui.Element) {
		panel.Width(360).Margin(6, 0, 0, 0).Radius(8).Background(p.bgPrimary).Border(1, p.borderSubtle).
			Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, 0.15)).Clip()
		ui.Row(c.Key("stats-head")).Gap(12).AlignItems(ui.Center).Padding(4, 4, 4, 12).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
			ui.Text(c, m("stats_session")).FontSize(12).FontWeight(500).Grow(1)
			if a.iconButton(c.Key("close"), "x", m("common_close"), false).Clicked() {
				open = false
			}
		})
		ui.Column(c.Key("stats")).Padding(12).Gap(16).Children(func() {
			if st == nil {
				if err != nil {
					ui.Text(c, m("stats_unavailable")).FontSize(12).TextColor(p.textTertiary)
				} else {
					ui.Text(c, m("stats_loading")).FontSize(12).TextColor(p.textTertiary)
				}
				return
			}
			x := st.Stats.Own
			type kv struct{ v, k string }
			hero := []kv{{fmt.Sprint(x.Turns), m("stats_turns")}}
			if x.ElapsedMs != nil {
				hero = append(hero, kv{duration(int64(*x.ElapsedMs)), m("stats_elapsed")})
			}
			if x.Usage != nil {
				hero = append(hero, kv{kilo(x.Usage.TotalTokens), m("stats_tokens")})
			}
			ui.Row(c.Key("hero")).Gap(12).Children(func() {
				for _, h := range hero {
					ui.Column(c.Key(h.k)).Grow(1).Basis(0).Gap(6).Children(func() {
						ui.Text(c, h.v).FontSize(19).Font(fontMono)
						ui.Text(c, h.k).FontSize(10).TextColor(p.placeholder)
					})
				}
			})
			bar := func(key, title, total string, segs []struct {
				label string
				v     float64
				shown string
				col   ui.Color
			}) {
				sum := 0.0
				for _, s := range segs {
					sum += s.v
				}
				if sum <= 0 {
					return
				}
				ui.Column(c.Key(key)).Gap(6).Children(func() {
					ui.Row(c).Children(func() {
						ui.Text(c, title).FontSize(10).TextColor(p.placeholder).Grow(1)
						ui.Text(c, total).FontSize(10).TextColor(p.placeholder)
					})
					ui.Row(c).Height(6).Radius(3).Clip().Background(p.borderSubtle).Children(func() {
						for i, s := range segs {
							if s.v > 0 {
								ui.Box(c.Key(i)).Grow(float32(s.v / sum)).Basis(0).Background(s.col)
							}
						}
					})
					ui.Row(c).Gap(12).Wrap().Children(func() {
						for i, s := range segs {
							if s.v <= 0 {
								continue
							}
							ui.Row(c.Key(i)).Gap(4).AlignItems(ui.Center).Children(func() {
								ui.Box(c).Size(6, 6).Radius(3).Background(s.col)
								ui.Text(c, s.label+" "+s.shown).FontSize(10).TextColor(p.placeholder)
							})
						}
					})
				})
			}
			type seg = struct {
				label string
				v     float64
				shown string
				col   ui.Color
			}
			f := func(v *float64) float64 {
				if v == nil {
					return 0
				}
				return *v
			}
			el := ""
			if x.ElapsedMs != nil {
				el = duration(int64(*x.ElapsedMs))
			}
			bar("timing", m("stats_timing"), el, []seg{
				{m("stats_queue"), f(x.QueueMs), duration(int64(f(x.QueueMs))), p.placeholder.Alpha(0.5)},
				{m("stats_model_time"), f(x.ModelMs), duration(int64(f(x.ModelMs))), p.brand},
				{m("stats_tool_time"), f(x.ToolMs), duration(int64(f(x.ToolMs))), p.placeholder},
			})
			if u := x.Usage; u != nil {
				in := u.Input + u.CacheRead + u.CacheWrite
				bar("usage", m("stats_usage"), groupThousands(fmt.Sprint(u.TotalTokens)), []seg{
					{m("stats_input"), float64(in), kilo(in), p.placeholder},
					{m("stats_output"), float64(u.Output), kilo(u.Output), p.brand},
				})
			}
			ui.Column(c.Key("rows")).Gap(6).Padding(12, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
				row := func(k, v string) {
					ui.Row(c.Key(k)).Children(func() {
						ui.Text(c, k).FontSize(12).TextColor(p.textTertiary).Grow(1)
						ui.Text(c, v).FontSize(12).Font(fontMono).TextColor(p.textSecond)
					})
				}
				if x.Calls != nil {
					row(m("stats_calls"), fmt.Sprint(*x.Calls))
				}
				if x.ToolCalls != nil {
					row(m("stats_tools"), fmt.Sprint(*x.ToolCalls))
				}
				if x.TTFTSamples > 0 {
					row(m("stats_ttft"), duration(int64(x.TTFTMs/float64(x.TTFTSamples))))
				}
				if x.OutputMs > 0 {
					row(m("stats_tps"), fmt.Sprintf("%.1f tok/s", x.TimedOutputTokens/x.OutputMs*1000))
				}
				if u := x.Usage; u != nil && x.UnknownUsageCalls == 0 {
					if in := u.Input + u.CacheRead + u.CacheWrite; in > 0 {
						row(m("stats_cache"), fmt.Sprintf("%.0f%%", float64(u.CacheRead)/float64(in)*100))
					}
				}
				if x.ModelCostUSD != nil {
					row(m("stats_model_cost"), formatMoney(*x.ModelCostUSD))
				}
			})
		})
	})
	if !open {
		a.statsFor = ""
	}
}
