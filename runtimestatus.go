package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// runtimeStatus is the header's "Local" button of a Space whose runtime
// is a computer of the user's, and its popover, as SpaceRuntimeStatus:
// the harnesses and their models, the connection, the workspace bridge,
// the file watcher, the displays, and what to run when it is offline.

type runtimeDetail struct {
	Kind      string  `json:"kind"`
	Online    bool    `json:"online"`
	RuntimeID *string `json:"runtimeId"`
	Client    *struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"client"`
	FileWatcher *struct {
		Backend    string    `json:"backend"`
		State      string    `json:"state"`
		ObservedAt time.Time `json:"observedAt"`
	} `json:"fileWatcher"`
	Displays []struct {
		ID     string `json:"id"`
		Kind   string `json:"kind"`
		Name   string `json:"name"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"displays"`
	Workspace *struct {
		Online     bool       `json:"online"`
		ObservedAt *time.Time `json:"observedAt"`
	} `json:"workspace"`
	ObservedAt   *time.Time `json:"observedAt"`
	Capabilities *struct {
		Harnesses []string `json:"harnesses"`
		Models    []struct {
			Harness string `json:"harness"`
		} `json:"models"`
	} `json:"capabilities"`
}

const runtimeFresh = time.Minute

func stale(t time.Time, now time.Time) bool { return t.IsZero() || now.Sub(t) > runtimeFresh }

// tone is how the runtime is: online, attention, offline or unknown.
func (r *runtimeDetail) tone(now time.Time) string {
	if r.ObservedAt != nil && stale(*r.ObservedAt, now) {
		return "unknown"
	}
	if !r.Online {
		return "offline"
	}
	if w := r.Workspace; w != nil && (!w.Online || w.ObservedAt == nil || stale(*w.ObservedAt, now)) {
		return "attention"
	}
	if f := r.FileWatcher; f != nil && !stale(f.ObservedAt, now) && f.State != "running" {
		return "attention"
	}
	return "online"
}

func (a *app) runtimeStatus(c *ui.Context, spaceID string) {
	if spaceID == "" || a.api == nil {
		return
	}
	key := "runtime-detail:" + spaceID
	r, _, _ := loadJSON[runtimeDetail](a, key, sp(spaceID)+"/runtime")
	if r == nil || r.Kind != "local" {
		return
	}
	p := a.pal
	now := time.Now()
	tone := r.tone(now)
	label := map[string]string{"online": m("runtime_ready"), "attention": m("runtime_degraded"), "unknown": m("runtime_watcher_unknown"), "offline": m("runtime_offline")}[tone]
	dot := func(size float32) {
		d := ui.Box(c).Size(size, size).Radius(size / 2)
		switch tone {
		case "online":
			d.Background(p.success)
		case "attention":
			d.Background(p.warning)
		case "offline":
			d.Border(1, p.placeholder)
		default:
			d.Background(p.placeholder)
		}
	}
	btn := ui.ButtonBase(c.Key("runtime")).Row().Gap(6).AlignItems(ui.Center).Height(28).Padding(0, 8).Radius(6).Label(m("runtime_title") + " · " + label).FocusRing(false).Children(func() {
		ui.Icon(c, icon("monitor")).Size(16, 16).TextColor(p.textTertiary)
		if a.width >= 1024 {
			ui.Text(c, m("runtime_local")).FontSize(13).FontWeight(500).TextColor(p.textSecond)
		}
		dot(6)
		if (tone == "attention" || tone == "offline") && a.width >= 1024 {
			col := p.textSecond
			if tone == "attention" {
				col = p.warning
			}
			ui.Text(c, label).FontSize(13).TextColor(col)
		}
	})
	if btn.Hovered() {
		btn.Background(p.hover)
	}
	open := a.menus["runtime"]
	if btn.Clicked() {
		open = !open
		if open {
			a.forget(key)
		}
	}
	ui.PopoverBase(c, btn, &open, func(panel ui.Element) {
		panel.AttachTo(btn, ui.AnchorBottomRight, ui.AnchorTopRight).Top(8).Width(344).Radius(10).Border(1, p.borderSubtle).
			Background(p.bgElevated).Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, 0.12)).Clip().Children(func() {
			ui.Row(c.Key("head")).Padding(14, 12, 10, 16).Gap(12).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					ui.Text(c, m("runtime_title")).FontSize(14).FontWeight(600)
					ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
						dot(6)
						col := p.textTertiary
						if tone == "attention" {
							col = p.warning
						}
						ui.Text(c, label).FontSize(12).TextColor(col)
					})
				})
				if a.iconButton(c.Key("close"), "x", m("common_close"), false).Clicked() {
					open = false
				}
			})
			ui.Scroll(c.Key("body")).MaxHeight(a.height-160).Padding(0, 16, 12, 16).Children(func() {
				row := func(k, dt string, dd func()) {
					ui.Row(c.Key(k)).Padding(8, 0).Gap(12).AlignItems(ui.Start).Children(func() {
						ui.Text(c, dt).FontSize(13).TextColor(p.textTertiary).Width(110).Shrink(0)
						ui.Column(c).Grow(1).AlignItems(ui.End).Gap(4).Children(dd)
					})
				}
				meta := func(s string) { ui.Text(c, s).FontSize(12).TextColor(p.textTertiary) }
				row("harnesses", m("runtime_harnesses"), func() {
					if r.Capabilities == nil || len(r.Capabilities.Harnesses) == 0 {
						ui.Text(c, "—").FontSize(13).TextColor(p.textSecond)
						return
					}
					for _, h := range r.Capabilities.Harnesses {
						n := 0
						for _, md := range r.Capabilities.Models {
							if md.Harness == h {
								n++
							}
						}
						ui.Row(c.Key(h)).Gap(6).AlignItems(ui.Center).Children(func() {
							a.harnessIcon(c, h, 14)
							ui.Text(c, harnessName(h)).FontSize(13).TextColor(p.textSecond)
							meta(m("runtime_model_count", "count", n))
						})
					}
				})
				row("connection", m("runtime_connection"), func() {
					s := m("runtime_offline")
					switch {
					case tone == "unknown":
						s = m("runtime_watcher_unknown")
					case r.Online:
						s = m("runtime_online")
					}
					ui.Text(c, s).FontSize(13).TextColor(p.textSecond)
					if r.Online {
						client := m("runtime_client_outdated")
						if r.Client != nil {
							name := r.Client.Name
							switch name {
							case "cli":
								name = "cohub"
							case "android":
								name = "Android"
							}
							client = name + " " + r.Client.Version
						}
						meta(client)
					}
				})
				row("bridge", m("runtime_workspace_bridge"), func() {
					s := m("runtime_watcher_unknown")
					if r.Workspace != nil {
						s = m("runtime_offline")
						if r.Workspace.Online && tone != "unknown" {
							s = m("runtime_online")
						}
					}
					ui.Text(c, s).FontSize(13).TextColor(p.textSecond)
				})
				row("watcher", m("runtime_file_watcher"), func() {
					f := r.FileWatcher
					if f != nil && stale(f.ObservedAt, now) {
						f = nil
					}
					s := m("runtime_watcher_unknown")
					if f != nil {
						s = map[string]string{"running": m("runtime_watcher_running"), "degraded": m("runtime_watcher_degraded"), "unavailable": m("runtime_watcher_unavailable")}[f.State]
					}
					ui.Text(c, s).FontSize(13).TextColor(p.textSecond)
					if f != nil {
						meta(f.Backend + " · " + timeAgo(f.ObservedAt, now))
					}
				})
				if len(r.Displays) > 0 {
					ui.Column(c.Key("displays")).Padding(10, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
						ui.Text(c, m("runtime_displays")).FontSize(12).FontWeight(500).TextColor(p.textTertiary)
						for _, d := range r.Displays {
							name := d.Name
							if d.Kind == "agent" {
								name = m("runtime_display_agent")
							}
							if name == "" {
								name = d.ID
							}
							d := d
							ui.Row(c.Key(d.ID)).Height(36).Gap(8).AlignItems(ui.Center).Children(func() {
								ui.Row(c).Grow(1).Gap(8).AlignItems(ui.Baseline).Children(func() {
									ui.Text(c, name).FontSize(13).TextColor(p.textSecond).SingleLine()
									meta(itoa(d.Width) + "×" + itoa(d.Height))
								})
								if a.can(spaceID, "sandbox.view") {
									b := ui.ButtonBase(c.Key("open")).Height(28).Padding(0, 10).Radius(6).Background(p.bgInput).Border(1, p.borderSubtle).Center().FocusRing(false).Children(func() {
										ui.Text(c, m("runtime_display_open")).FontSize(12).TextColor(p.textSecond)
									})
									if b.Clicked() {
										open = false
										a.openDisplay(spaceID, d.ID)
									}
								}
							})
						}
					})
				}
				if tone != "online" {
					hint := m("runtime_disconnected_hint")
					switch tone {
					case "attention":
						hint = m("runtime_degraded_hint")
					case "unknown":
						hint = m("runtime_unknown_hint")
					}
					ui.Text(c, hint).FontSize(13).LineHeight(1.6).TextColor(p.textSecond).Padding(8, 0)
				}
				if r.RuntimeID != nil {
					ui.Column(c.Key("diag")).Margin(8, 0, 0, 0).Padding(10, 0, 0, 0).Gap(6).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
						sum := ui.ButtonBase(c.Key("summary")).AlignSelf(ui.Start).Row().Gap(4).AlignItems(ui.Center).FocusRing(false).Children(func() {
							glyph := "▸"
							if a.runtimeDiag {
								glyph = "▾"
							}
							ui.Text(c, glyph+" "+m("runtime_diagnostics")).FontSize(12).TextColor(p.textTertiary)
						})
						if sum.Clicked() {
							a.runtimeDiag = !a.runtimeDiag
						}
						if a.runtimeDiag {
							ui.Text(c, m("runtime_id")).FontSize(12).TextColor(p.textTertiary)
							ui.Text(c, *r.RuntimeID).FontSize(12).Font(fontMono).TextColor(p.textSecond).Selectable()
							ui.Text(c, "cohub runtime logs --follow").FontSize(12).Font(fontMono).TextColor(p.textSecond).Selectable()
						}
					})
				}
				if tone == "offline" && a.can(spaceID, "sandbox.manage") {
					ui.Column(c.Key("cmd")).Padding(10, 0, 0, 0).Gap(6).Children(func() {
						ui.Text(c, m("runtime_offline_hint")).FontSize(12).TextColor(p.textTertiary)
						ui.Text(c, "cohub runtime up --space "+spaceID).FontSize(12).Font(fontMono).TextColor(p.textSecond).Selectable()
					})
				}
			})
			ui.Row(c.Key("foot")).Padding(6, 10, 6, 16).Gap(8).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
				at := ""
				if r.ObservedAt != nil {
					at = m("runtime_last_updated", "time", timeAgo(*r.ObservedAt, now))
				}
				ui.Text(c, at).FontSize(12).TextColor(p.textTertiary).Grow(1)
				if a.iconButton(c.Key("refresh"), "refresh-cw", m("runtime_refresh"), false).Clicked() {
					a.reload(key)
				}
				if a.can(spaceID, "sandbox.manage") && a.iconButton(c.Key("manage"), "settings-2", m("runtime_manage"), false).Clicked() {
					open = false
					a.go_(route{kind: routeSpaceSettings, space: spaceID, sub: "runtime"})
				}
			})
		})
	})
	a.menus["runtime"] = open
}
