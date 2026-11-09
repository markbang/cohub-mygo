package main

import (
	"context"
	"fmt"

	"github.com/egoist/mygo/ui"
)

// spacesView is the Space directory: a row per Space, newest activity first.
func (a *app) spacesView(c *ui.Context) {
	p := a.pal
	a.header(c, func() {
		if !a.compact {
			a.headerLead(c)
		}
		if a.status != "" && a.me == nil {
			a.spinner(c, 13, p.textTertiary)
			ui.Text(c, m("sync_connecting")).FontSize(13).FontWeight(500).Grow(1)
		} else if a.compact {
			a.logo(c, 30)
			ui.Text(c, "Space").FontSize(17).FontWeight(600).Grow(1).MarginStart(6)
		} else {
			ui.Spacer(c)
		}
		a.iconButton(c.Key("search"), "search", m("command_placeholder_space"), false).OnClick(func() { a.palette.open(paletteSpaces) })
		add := ui.ButtonBase(c.Key("new-space")).Size(28, 28).Center().Radius(6).Label(m("spaces_new")).FocusRing(false).Children(func() {
			ui.Icon(c, icon("plus")).Size(16, 16).TextColor(p.brand)
		})
		if add.Hovered() {
			add.Background(p.brandMuted)
		}
	})
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
	ui.List(c.Key("spaces"), &a.spaceList, len(list), func(i int) {
		sp := list[i]
		row := ui.Row(c.Key(sp.ID)).Gap(10).AlignItems(ui.Center).Padding(8, 6).Radius(6).Children(func() {
			a.avatar(c, sp.avatar(), sp.Name, size, 8)
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				fs := float32(14)
				if a.compact {
					fs = 16
				}
				ui.Text(c, sp.Name).FontSize(fs).SingleLine()
				desc := sp.Description
				if desc == "" {
					desc = m("spaces_row_no_description")
				}
				ui.Text(c, oneLine(desc)).FontSize(fs - 2).TextColor(p.textTertiary).SingleLine()
			})
			ui.Row(c).Gap(6).AlignItems(ui.Center).AlignSelf(ui.Start).Margin(4, 0, 0, 0).Shrink(0).Children(func() {
				if sp.IsPinned {
					ui.Icon(c, icon("pin")).Size(12, 12).TextColor(p.textTertiary)
				}
				ui.Text(c, relTime(sp.activity(), now)).FontSize(11).TextColor(p.textTertiary)
			})
		})
		a.pressable(row, false)
		row.OnClick(func() { a.openSpace(sp.ID) })
	}).Grow(1).Padding(6, 6).Gap(0).Children(func() {
		if len(list) == 0 && a.me != nil {
			ui.Text(c, m("x_no_spaces")).FontSize(13).TextColor(p.textTertiary).Padding(20)
		}
	})
}

// spaceHome is a Space with no chat open: its card, and the composer
// that starts a chat in it, as routes/(app)/spaces/[id].
func (a *app) spaceHome(c *ui.Context) {
	p := a.pal
	id := a.route.space
	sp := a.space(id)
	a.once("usage7:"+id, func(ctx context.Context) {
		u, err := a.api.spaceUsage(ctx, id, 7)
		if err == nil {
			a.update(func() { a.usage7d[id] = u })
		}
	})
	a.header(c, func() {
		a.headerLead(c)
		ui.Text(c, m("sidebar_new_chat")).FontSize(13).TextColor(p.textSecond).Grow(1)
		if !a.compact {
			if a.iconButton(c.Key("panel"), "panel-right", m("x_side_panel"), false).Clicked() {
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
				a.avatar(c, sp.avatar(), sp.Name, 48, 8)
				ui.Column(c).Gap(2).MinWidth(0).Children(func() {
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Text(c, sp.Name).FontSize(34).FontWeight(600).LetterSpacing(-0.5).SingleLine()
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
			if o := sp.OwnerProfile; o != nil {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Created by").FontSize(14).TextColor(p.textSecond)
					a.avatar(c, o.AvatarURL, o.name(), 18, 9)
					ui.Text(c, o.name()).FontSize(14).FontWeight(500)
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
