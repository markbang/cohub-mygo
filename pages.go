package main

import (
	"github.com/egoist/mygo/ui"
)

// pageData holds what the pages beyond chats load, by key.
type pageData struct {
	json map[string]any
}

func (d *pageData) init() { d.json = map[string]any{} }

// pageHeader is a page's title block, as PageHeader.svelte.
func (a *app) pageTitle(c *ui.Context, title, sub string) {
	p := a.pal
	ui.Column(c).Gap(6).Padding(0, 0, 20, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Text(c, title).FontSize(18).FontWeight(600)
		if sub != "" {
			ui.Text(c, sub).FontSize(13).TextColor(p.textSecond)
		}
	})
}

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

func (a *app) spaceSettings(c *ui.Context)  { a.stubPage(c, "Space 设置") }
func (a *app) walletPage(c *ui.Context)     { a.stubPage(c, "Space 钱包") }
func (a *app) filesPage(c *ui.Context)      { a.stubPage(c, "文件") }
func (a *app) checkpointPage(c *ui.Context) { a.stubPage(c, "存档") }
func (a *app) taskPage(c *ui.Context)       { a.stubPage(c, "任务") }
func (a *app) cronPage(c *ui.Context)       { a.stubPage(c, "定时任务") }

func (a *app) panelContent(c *ui.Context) {
	p := a.pal
	ui.Column(c).Grow(1).Padding(16).Children(func() {
		msg := []string{"本对话中 Agent 生成的媒体和改动的文件会显示在这里。", "Space 的文件会显示在这里。", "生成的图片和视频会显示在这里。", "发布的应用会显示在这里。"}[a.panelTab]
		if a.route.sessionID() == "" && a.panelTab == 0 {
			msg = "开始对话后，这里会显示它生成的媒体和改动的文件。"
		}
		ui.Text(c, msg).FontSize(12).LineHeight(1.6).TextColor(p.textTertiary)
	})
}
