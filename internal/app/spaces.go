package app

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"cohub-mygo/internal/cohub"
)

// spacesView is the Space directory: a row per Space, newest activity first.
func (a *app) spacesView(c *ui.Context) {
	p := a.pal
	actions := func() {
		a.iconButton(c.Key("search"), "search", m("list_search"), false).OnClick(func() { a.palette.open(paletteSpaces) })
		size, glyph := float32(28), float32(16)
		if a.compact {
			size, glyph = 40, 20
		}
		add := ui.ButtonBase(c.Key("new-space")).Size(size, size).Center().Radius(size / 5).Label(m("spaces_new")).FocusRing(false).Children(func() {
			ui.Icon(c, icon("plus")).Size(glyph, glyph).TextColor(p.brand)
		})
		if add.Hovered() {
			add.Background(p.brandMuted)
		}
		if add.Clicked() {
			a.go_(route{kind: routeNewSpace})
		}
	}
	if len(a.selectedSpaces) > 0 {
		a.header(c, func() {
			if a.iconButton(c.Key("cancel"), "x", m("common_cancel"), false).Clicked() {
				a.selectedSpaces = map[string]bool{}
			}
			ui.Text(c, m("label_selected_count", "count", len(a.selectedSpaces))).FontSize(13).FontWeight(600).Grow(1)
			ids := []string{}
			allPinned := true
			for id := range a.selectedSpaces {
				ids = append(ids, id)
				if sp := a.space(id); sp == nil || !sp.IsPinned {
					allPinned = false
				}
			}
			pin, pinL := "pin", m("command_pin")
			if allPinned {
				pin, pinL = "pin-off", m("command_unpin")
			}
			if a.iconButton(c.Key("pin"), pin, pinL, false).Clicked() {
				for _, id := range ids {
					a.setSpacePinned(id, !allPinned)
				}
				a.selectedSpaces = map[string]bool{}
			}
			arch, archL := "archive", m("spaces_archive")
			if a.spacesNav == 4 {
				arch, archL = "archive-restore", m("spaces_unarchive")
			}
			if a.iconButton(c.Key("archive"), arch, archL, false).Clicked() {
				for _, id := range ids {
					a.setSpaceArchived(id, a.spacesNav != 4)
				}
				a.selectedSpaces = map[string]bool{}
			}
		})
	} else if a.compact {
		a.compactListHeader(c, c.TitleBar(), m("spaces_title"), actions)
	} else {
		a.header(c, func() {
			a.headerLead(c)
			if a.status != "" && a.me == nil {
				a.spinner(c, 13, p.textTertiary)
				ui.Text(c, m("sync_connecting")).FontSize(13).FontWeight(500).Grow(1)
			} else {
				ui.Text(c, m("spaces_title")).FontSize(13).FontWeight(600).Grow(1).SingleLine()
			}
			actions()
		})
	}
	if a.compact {
		ui.ScrollHorizontal(c.Key("space-nav")).Shrink(0).Scrollbars(false).Padding(8, 12).Children(func() {
			ui.Row(c).Gap(4).Children(func() {
				for i, it := range sidebarSpaceNav {
					on := a.spacesNav == i
					b := ui.ButtonBase(c.Key(it.icon)).Height(28).Padding(0, 8).Radius(6).Center().FocusRing(false).Children(func() {
						col := p.textSecond
						if on {
							col = p.brand
						}
						ui.Text(c, m(it.label)).FontSize(13).TextColor(col)
					})
					if on {
						b.Background(p.brandMuted)
					}
					if b.Clicked() {
						a.spacesNav = i
					}
				}
			})
		})
	}
	list := a.filteredSpaces(a.spacesNav, "")
	now := a.now()
	a.spaceList.Key = func(i int) any { return list[i].ID }
	size := float32(36)
	if a.compact {
		size = 40
	}
	// The open row menu is drawn after the list, so the list's rows do not
	// clip it.
	var menuAt ui.Element
	var menuSpace *cohub.Space
	ui.List(c.Key("spaces"), &a.spaceList, len(list), func(i int) {
		sp := list[i]
		rowH := float32(52)
		if a.compact {
			rowH = 60
		}
		row := ui.Row(c.Key(sp.ID)).Gap(10).AlignItems(ui.Center).Height(rowH).Padding(0, 6).Radius(6)
		row.Children(func() {
			selecting := len(a.selectedSpaces) > 0
			on := a.selectedSpaces[sp.ID]
			lead := ui.ButtonBase(c.Key("lead")).Size(size, size).Label(m("spaces_selected_aria", "name", sp.Name)).FocusRing(false).Children(func() {
				a.avatarSeed(c, sp.Avatar(), sp.Name, sp.ID, size, 8)
				if on || selecting || row.Hovered() {
					mark := ui.Box(c.Key("mark")).Absolute().Bottom(-4).Right(-4).Size(18, 18).Radius(9).Border(2, p.bgPrimary).Center()
					if on {
						mark.Background(p.brand).Children(func() { ui.Icon(c, icon("check")).Size(10, 10).TextColor(p.brandFg) })
					} else {
						mark.Background(p.bgElevated)
					}
				}
			})
			if lead.Clicked() {
				if on {
					delete(a.selectedSpaces, sp.ID)
				} else {
					a.selectedSpaces[sp.ID] = true
				}
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				fs, sub, meta := float32(14), float32(12), float32(11)
				if a.compact {
					fs, sub, meta = 16, 14, 12
				}
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, sp.Name).FontSize(fs).FontWeight(500).LineHeight((fs + 4) / fs).SingleLine().Grow(1).MinWidth(0)
					if sp.IsPinned && a.spacesNav != 3 {
						ui.Icon(c, icon("pin")).Size(12, 12).TextColor(p.placeholder)
					}
					ui.Text(c, relTime(sp.LastActivityAt, now)).FontSize(meta).TextColor(p.placeholder).Shrink(0)
				})
				desc, quiet := spaceSubtitle(sp)
				col := p.textTertiary
				if quiet {
					col = p.placeholder
				}
				ui.Text(c, oneLine(desc)).FontSize(sub).LineHeight((sub + 4) / sub).TextColor(col).SingleLine()
			})
			if !a.compact {
				key := "space-row:" + sp.ID
				open := a.menus[key]
				more := ui.ButtonBase(c.Key("more")).Size(28, 28).Center().Radius(6).Label(m("inline_more_actions")).FocusRing(false).Children(func() {
					col := p.textTertiary
					if open {
						col = p.text
					}
					ui.Icon(c, icon("ellipsis")).Size(16, 16).TextColor(col)
				})
				if !open && !row.Hovered() {
					more.Opacity(0)
				}
				if open || more.Hovered() {
					more.Background(p.active)
				}
				if more.Clicked() {
					a.menus[key] = !open
				}
				if t := a.labelTarget; open || t != nil && t.kind == "space" && t.ref == sp.ID {
					menuAt, menuSpace = more, sp
					more.Opacity(1)
				}
			}
		})
		a.pressable(row, false)
		row.OnClick(func() { a.openSpace(sp.ID) })
	}).Grow(1).Padding(6, 6).Gap(0).Children(func() {
		if len(list) == 0 && a.me != nil {
			ui.Text(c, m("x_no_spaces")).FontSize(13).TextColor(p.textTertiary).Padding(20)
		}
	})
	if menuSpace != nil {
		a.spaceRowMenu(c, menuAt, menuSpace)
	}
}

// spaceRowMenu is SpaceRowMenu.svelte.
func (a *app) spaceRowMenu(c *ui.Context, anchor ui.Element, sp *cohub.Space) {
	key := "space-row:" + sp.ID
	open := a.menus[key]
	pin, arch := "pin", "archive"
	pinL, archL := m("command_pin"), m("spaces_archive")
	if sp.IsPinned {
		pin, pinL = "pin-off", m("command_unpin")
	}
	if sp.IsArchived {
		arch, archL = "archive-restore", m("spaces_unarchive")
	}
	items := []menuItem{
		{icon: pin, label: pinL, run: func() { a.setSpacePinned(sp.ID, !sp.IsPinned) }},
		{icon: arch, label: archL, run: func() { a.setSpaceArchived(sp.ID, !sp.IsArchived) }},
		{icon: "tag", label: m("spaces_label_add") + "…", run: func() { a.labelTarget = &labelTarget{kind: "space", ref: sp.ID} }},
		{icon: "wallet", label: m("wallet_title"), run: func() { a.go_(route{kind: routeSpaceWallet, space: sp.ID}) }},
	}
	if sp.Relation != "public" {
		items = append(items, menuItem{icon: "settings", label: m("sidebar_space_settings"), run: func() { a.go_(route{kind: routeSpaceSettings, space: sp.ID}) }})
	}
	items[len(items)-1].divider = true
	items = append(items, menuItem{icon: "circle-check", label: m("spaces_select"), run: func() { a.selectedSpaces[sp.ID] = true }})
	a.menuWidth(c, anchor, &open, 200, items)
	a.menus[key] = open
	if t := a.labelTarget; t != nil && t.kind == "space" && t.ref == sp.ID {
		a.labelPicker(c, anchor)
	}
}

// spaceHome is a Space with no chat open: its card, and the composer
// that starts a chat in it, as routes/(app)/spaces/[id].
func (a *app) spaceHome(c *ui.Context) {
	p := a.pal
	id := a.route.space
	sp := a.space(id)
	a.once("usage7:"+id, func(ctx context.Context) {
		u, err := a.api.SpaceUsage(ctx, id, 7)
		if err == nil {
			a.update(func() { a.usage7d[id] = u })
		}
	})
	a.header(c, func() {
		a.headerLead(c)
		ui.Text(c, m("sidebar_new_chat")).FontSize(13).TextColor(p.textSecond).Grow(1)
		if !a.compact {
			a.runtimeStatus(c, id)
		}
		a.presenceStack(c, id)
		if !a.compact {
			ic, lb := "panel-right-open", m("side_panel_show")
			if a.panelOpen {
				ic, lb = "panel-right-close", m("side_panel_hide")
			}
			if a.iconButton(c.Key("panel"), ic, lb, false).Clicked() {
				a.panelOpen = !a.panelOpen
			}
		}
	})
	ui.Scroll(c.Key("hero")).Grow(1).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(chatMaxWidth).Margin(0, ui.Auto).Padding(64, 24, 24, 24).Gap(20).Children(func() {
			if sp == nil {
				a.spinner(c, 18, p.textTertiary)
				return
			}
			ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
				a.avatarSeed(c, sp.Avatar(), sp.Name, sp.ID, 48, 8)
				ui.Column(c).Gap(2).MinWidth(0).Children(func() {
					ui.Row(c).Gap(10).AlignItems(ui.Center).Wrap().Children(func() {
						ui.Text(c, sp.Name).FontSize(34).FontWeight(600).LetterSpacing(-0.5).LineHeight(1.15).Shrink(1).MinWidth(0)
						col := p.placeholder
						if sp.SandboxStatus == "running" || sp.SandboxStatus == "ready" {
							col = p.success
						}
						a.dot(c, col, 8)
					})
					ui.Text(c, "Created "+sp.CreatedAt.Local().Format("Jan 2, 03:04 PM")).FontSize(11).Font(fontMono).TextColor(p.textTertiary)
				})
			})
			if sp.Description != "" {
				ui.Text(c, sp.Description).FontSize(14).LineHeight(1.7).TextColor(p.textSecond)
			}
			mem, _, _ := loadJSON[spaceMembers](a, "members:"+id, "/api/spaces/"+url.PathEscape(id)+"/members")
			var others []*cohub.Profile
			if mem != nil {
				type m2 struct {
					role, id string
					p        *cohub.Profile
				}
				var list []m2
				for _, it := range mem.Items {
					if it.Profile != nil && (sp.OwnerProfile == nil || it.UserID != sp.OwnerProfile.UserUUID) {
						list = append(list, m2{it.Role, it.UserID, it.Profile})
					}
				}
				sort.SliceStable(list, func(i, j int) bool {
					if list[i].role != list[j].role {
						return list[i].role < list[j].role
					}
					return list[i].id < list[j].id
				})
				for _, x := range list {
					others = append(others, x.p)
				}
			}
			if o := sp.OwnerProfile; o != nil || len(others) > 0 {
				ui.Row(c.Key("people")).Gap(4).AlignItems(ui.Center).Wrap().Children(func() {
					person := func(pr *cohub.Profile) {
						ui.Row(c.Key(pr.UserUUID+pr.Name())).Gap(4).AlignItems(ui.Center).Margin(0, 4, 0, 0).Children(func() {
							a.avatar(c, pr.AvatarURL, pr.Name(), 18, 9)
							ui.Text(c, pr.Name()).FontSize(14).FontWeight(500)
						})
					}
					if o != nil {
						ui.Text(c, "Created by").FontSize(14).TextColor(p.textSecond)
						person(o)
					}
					if len(others) > 0 {
						lead := "with"
						if o == nil {
							lead = "Members include"
						}
						ui.Text(c, lead).FontSize(14).TextColor(p.textSecond).Margin(0, 4, 0, 0)
						for _, pr := range others {
							person(pr)
						}
					}
					ui.Text(c, "·").FontSize(14).TextColor(p.textTertiary)
				})
			}
			if u := a.usage7d[id]; u != nil && u.Totals.RequestCount > 0 {
				ui.Text(c, m("x_space_usage", "tokens", compactCount(u.Totals.TotalTokens), "requests", u.Totals.RequestCount, "cost", fmt.Sprintf("US$%.2f", u.Totals.CostTotal))).FontSize(14).LineHeight(1.7).TextColor(p.textSecond)
			}
		})
	})
	a.composer(c, false)
}

// compactCount is the usage line's count: 228.0M, 6.1K.
func compactCount(n int64) string {
	switch {
	case n >= 1e9:
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	case n >= 1e6:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.1fK", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

// spaceSubtitle is the row's second line: the description, else the
// owner's handle for a Space someone else owns (SpacesListPane subtitleOf).
func spaceSubtitle(sp *cohub.Space) (string, bool) {
	if d := strings.TrimSpace(sp.Description); d != "" {
		return d, false
	}
	if o := sp.OwnerProfile; sp.Relation != "" && sp.Relation != "owner" && o != nil {
		if h := strings.TrimSpace(o.Username); h != "" {
			return "@" + h, true
		}
		return strings.TrimSpace(o.DisplayName), true
	}
	return m("spaces_row_no_description"), true
}
