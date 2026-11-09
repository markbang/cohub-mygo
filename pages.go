package main

import (
	"github.com/egoist/mygo/ui"
)

// pageData holds what the pages beyond chats load, by key.
type pageData struct {
	json map[string]any
}

func (d *pageData) init() { d.json = map[string]any{} }

func (a *app) stubPage(c *ui.Context, title string) {
	p := a.pal
	a.header(c, func() {
		a.headerLead(c)
		ui.Text(c, title).FontSize(13).Grow(1)
	})
	ui.Column(c).Grow(1).Center().Children(func() {
		ui.Text(c, title).FontSize(13).TextColor(p.textTertiary)
	})
}

