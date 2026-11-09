package main

import (
	"github.com/egoist/mygo/ui"
)

// spacesView is the Space directory: a row per Space, newest activity first.
func (a *app) spacesView(c *ui.Context) {
	p := a.pal
	a.header(c, func() {
		if !a.sidebarOpen && !a.compact {
			if a.iconButton(c.Key("open-sidebar"), "panel-left", "展开侧栏 (⌘B)", false).Clicked() {
				a.sidebarOpen = true
			}
		}
		if a.status != "" && a.me == nil {
			a.spinner(c, 13, p.textTertiary)
			ui.Text(c, "连接中…").FontSize(13).FontWeight(500).Grow(1)
		} else {
			ui.Text(c, sidebarSpaceNav[a.spacesNav].label).FontSize(13).FontWeight(500).Grow(1)
		}
		a.iconButton(c.Key("search"), "search", "搜索 Space", false)
		add := ui.ButtonBase(c.Key("new-space")).Size(28, 28).Center().Radius(6).Label("新建 Space").FocusRing(false).Children(func() {
			ui.Icon(c, icon("plus")).Size(16, 16).TextColor(p.brand)
		})
		if add.Hovered() {
			add.Background(p.brandMuted)
		}
	})
	if a.compact {
		ui.ScrollHorizontal(c.Key("space-nav")).Shrink(0).Scrollbars(false).Padding(8, 10).Children(func() {
			ui.Row(c).Gap(4).Children(func() {
				for i, it := range sidebarSpaceNav {
					on := a.spacesNav == i
					b := ui.ButtonBase(c.Key(it.label)).Height(28).Padding(0, 10).Radius(6).Center().FocusRing(false).Children(func() {
						col := p.textTertiary
						if on {
							col = p.brand
						}
						ui.Text(c, it.label).FontSize(13).TextColor(col)
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
	list := a.filteredSpaces()
	now := a.now()
	a.spaceList.Key = func(i int) any { return list[i].ID }
	ui.List(c.Key("spaces"), &a.spaceList, len(list), func(i int) {
		sp := list[i]
		row := ui.Row(c.Key(sp.ID)).Gap(10).AlignItems(ui.Center).Padding(8, 10).Radius(6).Children(func() {
			a.avatar(c, sp.avatar(), sp.Name, 36, 8)
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, sp.Name).FontSize(14).Font(fontMono).SingleLine()
				desc, col := sp.Description, p.textTertiary
				if desc == "" {
					desc = "暂无描述"
				}
				ui.Text(c, oneLine(desc)).FontSize(12).Font(fontMono).TextColor(col).SingleLine()
			})
			if sp.IsPinned {
				ui.Icon(c, icon("pin")).Size(12, 12).TextColor(p.textTertiary)
			}
			ui.Text(c, relTime(sp.activity(), now)).FontSize(11).Font(fontMono).TextColor(p.textTertiary).Shrink(0)
		})
		a.pressable(row, false)
		row.OnClick(func() { a.openSpace(sp.ID) })
	}).Grow(1).Padding(6, 6).Gap(2).Children(func() {
		if len(list) == 0 && a.me != nil {
			ui.Text(c, "没有 Space").FontSize(13).TextColor(p.textTertiary).Padding(20)
		}
	})
}
