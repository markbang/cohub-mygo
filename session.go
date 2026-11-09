package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

const chatMaxWidth = 896 // --chat-content-max-width: 56rem

// sessionView is a chat: its header, the turns and the composer.
func (a *app) sessionView(c *ui.Context) {
	p := a.pal
	s := a.session(a.selSession)
	tl := a.timelines[a.selSession]
	a.header(c, func() {
		if !a.sidebarOpen {
			if a.iconButton(c.Key("open-sidebar"), "panel-left", "展开侧栏 (⌘B)", false).Clicked() {
				a.sidebarOpen = true
			}
		}
		if a.compact {
			if a.iconButton(c.Key("back"), "chevron-left", "返回", false).Clicked() {
				a.selSession = ""
			}
		}
		title := "对话"
		if s != nil {
			title = s.displayTitle()
			name, av := "", ""
			if sp := a.space(s.SpaceID); sp != nil {
				name, av = sp.Name, sp.avatar()
			} else if s.Space != nil {
				name, av = s.Space.Name, s.Space.avatarURL()
			}
			if !a.compact {
				crumb := ui.Row(c.Key("crumb")).Gap(6).AlignItems(ui.Center).Shrink(0).MaxWidth(200).Cursor(ui.CursorPointer).Children(func() {
					a.avatar(c, av, name, 18, 4)
					ui.Text(c, name).FontSize(13).Font(fontMono).SingleLine().TextColor(p.text)
				})
				crumb.OnClick(func() { a.openSpace(s.SpaceID) })
				ui.Text(c, "/").FontSize(13).TextColor(p.placeholder)
			}
		}
		ui.Text(c, title).FontSize(13).Font(fontMono).SingleLine().Grow(1).MinWidth(0)
		if a.compact {
			return
		}
		ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(0, 6).Children(func() {
			ui.Icon(c, icon("monitor")).Size(14, 14).TextColor(p.textTertiary)
			status, col := "云端", p.placeholder
			if sp := a.space(a.spaceOf(s)); sp != nil && sp.SandboxStatus == "running" {
				col = p.success
			}
			ui.Text(c, status).FontSize(12).TextColor(p.textTertiary)
			a.dot(c, col, 6)
		})
		share := ui.ButtonBase(c.Key("share")).Row().Gap(5).AlignItems(ui.Center).Height(26).Padding(0, 8).Radius(6).FocusRing(false).Children(func() {
			ui.Icon(c, icon("share-2")).Size(13, 13).TextColor(p.textTertiary)
			ui.Text(c, "Share").FontSize(12).TextColor(p.textTertiary)
		})
		if share.Hovered() {
			share.Background(p.hover)
		}
		a.iconButton(c.Key("more"), "ellipsis", "更多", false)
		if a.iconButton(c.Key("panel"), "panel-right", "侧边面板", a.panelOpen).Clicked() {
			a.panelOpen = !a.panelOpen
		}
	})

	turns := []*Turn(nil)
	if tl != nil {
		turns = tl.turns
	}
	a.timeline.Key = func(i int) any { return turns[i].ID }
	a.timeline.FollowEnd = true
	a.timeline.FollowAnimation = &ui.Smooth
	ui.List(c.Key("timeline-"+a.selSession), &a.timeline, len(turns), func(i int) {
		ui.Column(c).FillWidth().MaxWidth(chatMaxWidth).Margin(0, ui.Auto).Gap(10).Children(func() {
			a.turnView(c, turns[i], tl)
		})
	}).Grow(1).Padding(20, 24, 28, 24).Gap(26).Children(func() {
		switch {
		case tl == nil || !tl.loaded:
			ui.Column(c).Grow(1).Center().Children(func() { a.spinner(c, 18, p.textTertiary) })
		case tl.err != "":
			ui.Text(c, tl.err).FontSize(12).TextColor(p.errorC).Margin(ui.Auto)
		case len(turns) == 0:
			ui.Text(c, "还没有消息").FontSize(13).TextColor(p.textTertiary).Margin(ui.Auto)
		}
	})
	a.composer(c, s != nil && s.running() || lastRunning(tl))
}

func (a *app) spaceOf(s *Session) string {
	if s == nil {
		return a.selSpace
	}
	return s.SpaceID
}

func lastRunning(tl *sessionTurns) bool {
	return tl != nil && len(tl.turns) > 0 && tl.turns[len(tl.turns)-1].running()
}

// turnView is one exchange: the user's bubble, the agent's work, its reply.
func (a *app) turnView(c *ui.Context, t *Turn, tl *sessionTurns) {
	p := a.pal
	now := a.now()
	// The user's message, on the right.
	if prompt := strings.TrimSpace(t.prompt()); prompt != "" {
		ui.Column(c).AlignItems(ui.End).Gap(4).Children(func() {
			ui.Text(c, prompt).Selectable().FontSize(14).LineHeight(1.6).TextColor(p.text).
				Padding(8, 14).Radius(12, 12, 6, 12).Background(p.userBubble).MaxWidthPercent(85).MaxWidth(832)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(0, 4).Opacity(0.85).Children(func() {
				ui.Icon(c, icon("copy")).Size(12, 12).TextColor(p.placeholder)
				if t.AuthorProfile != nil {
					a.avatar(c, t.AuthorProfile.AvatarURL, t.AuthorProfile.name(), 14, 7)
					ui.Text(c, t.AuthorProfile.name()).FontSize(11).Font(fontMono).TextColor(p.placeholder)
				}
				ui.Spacer(c).MinWidth(40)
				ui.Text(c, relTime(t.StartedAt, now)).FontSize(11).Font(fontMono).TextColor(p.placeholder)
			})
		})
	}

	running := t.running()
	steps, tools := 0, 0
	if t.IntermediateSummary != nil {
		steps, tools = t.IntermediateSummary.MessageCount, t.IntermediateSummary.ToolCallCount
	}
	reply := t.reply()
	if running && tl != nil && tl.live != "" {
		reply = tl.live
		steps = max(steps, tl.liveSteps)
	}
	// The agent's work: steps and tools, or that it is working.
	if running || steps > 0 {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Height(28).Padding(0, 2).Children(func() {
			col := p.success
			if running {
				col = p.brand
			}
			a.dot(c, col, 6)
			label := fmt.Sprintf("%d 步 · %d 个工具", steps, tools)
			if running {
				label = "运行中 · " + elapsed(t.StartedAt, now)
				if steps > 0 {
					label += fmt.Sprintf(" · %d 步", steps)
				}
				c.After(1e9)
			}
			ui.Text(c, label).FontSize(12).Font(fontMono).TextColor(p.textTertiary).Grow(1)
			if running {
				a.spinner(c, 12, p.textTertiary)
			} else {
				ui.Icon(c, icon("chevron-right")).Size(13, 13).TextColor(p.placeholder)
			}
		})
	}

	// The reply, as Markdown.
	if reply != "" {
		if running {
			markdown.Stream(c, reply, &a.md)
		} else {
			markdown.View(c, reply, &a.md)
		}
	}
	if t.ErrorMessage != "" {
		ui.Text(c, t.ErrorMessage).FontSize(12).TextColor(p.errorC).Padding(8, 12).Radius(8).Background(p.errorC.Alpha(0.08))
	}
	if !running {
		a.replyMeta(c, t)
	}
}

// replyMeta is the line under a reply: copy, the harness, the model and
// its tokens, and when it finished.
func (a *app) replyMeta(c *ui.Context, t *Turn) {
	p := a.pal
	ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(0, 2).Children(func() {
		cp := ui.ButtonBase(c.Key("copy")).Size(20, 20).Center().Radius(4).Label("复制").FocusRing(false).Children(func() {
			ui.Icon(c, icon("copy")).Size(12, 12).TextColor(p.placeholder)
		})
		if cp.Hovered() {
			cp.Background(p.hover)
		}
		if cp.Clicked() {
			c.WriteClipboard(t.reply())
		}
		if h := t.Meta.Harness; h != "" {
			ui.Row(c).Gap(3).AlignItems(ui.Center).Padding(1, 5).Radius(4).Background(p.hoverStrong).Children(func() {
				ui.Icon(c, icon("sparkles")).Size(10, 10).TextColor(p.textTertiary)
				ui.Text(c, strings.ToUpper(h[:1])+h[1:]).FontSize(10).FontWeight(500).TextColor(p.textTertiary)
			})
		}
		meta := t.Model
		if lv := t.ThinkingLevel; lv != "" && lv != "off" {
			meta += " " + strings.ToUpper(lv[:1]) + lv[1:]
		}
		if u := t.TotalUsage; u != nil {
			in := u.Input + u.CacheRead + u.CacheWrite
			meta += fmt.Sprintf(" ↑%s", kilo(in))
			if u.CacheRead > 0 {
				meta += fmt.Sprintf(" (缓存 %s)", kilo(u.CacheRead))
			}
			meta += " ↓" + kilo(u.Output)
		}
		ui.Text(c, meta).FontSize(11).Font(fontMono).TextColor(p.placeholder).SingleLine().Grow(1).MinWidth(0)
		if t.CompletedAt != nil {
			ui.Text(c, relTime(*t.CompletedAt, a.now())).FontSize(11).Font(fontMono).TextColor(p.placeholder)
		}
	})
}

// composer is the message box under a chat, as SessionComposer.svelte.
func (a *app) composer(c *ui.Context, running bool) {
	p := a.pal
	pad := float32(24)
	if a.compact {
		pad = 10
	}
	ui.Column(c).Shrink(0).Padding(0, pad, pad*0.67, pad).Children(func() {
		if a.sendErr != "" {
			ui.Text(c, a.sendErr).FontSize(12).TextColor(p.errorC).FillWidth().MaxWidth(chatMaxWidth-64).Margin(0, ui.Auto, 6, ui.Auto)
		}
		card := ui.Column(c.Key("composer")).FillWidth().MaxWidth(chatMaxWidth-64).Margin(0, ui.Auto).Radius(16).
			Background(p.bgInput).Border(1, p.brandBorder).Padding(14, 12, 10, 12).Gap(10).
			Shadow(0, 4, 16, 0, p.brand.Alpha(0.05))
		card.Children(func() {
			in := ui.TextAreaBase(c.Key("draft"), &a.draft).Lines(2, 10).Placeholder("发送消息…").Label("消息").PlaceholderColor(p.placeholder).
				FontSize(14).LineHeight(1.55).Padding(0, 10).AutoFocus()
			in.OnTextKey(func(k ui.TextKey) bool {
				if k.Key == ui.KeyEnter && k.Mods == 0 {
					a.draft = k.Text()
					a.send()
					k.SetText(a.draft)
					return true
				}
				return false
			})
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				a.iconButton(c.Key("attach"), "plus", "添加附件", false)
				a.pill(c.Key("harness"), a.harnessLabel(), func() {
					ui.Icon(c, icon("sparkles")).Size(12, 12).TextColor(p.textTertiary)
				})
				a.pill(c.Key("model"), a.modelLabel(), nil)
				ui.Spacer(c)
				a.iconButton(c.Key("mic"), "mic", "语音输入", false)
				empty := strings.TrimSpace(a.draft) == ""
				send := ui.ButtonBase(c.Key("send")).Size(36, 36).Radius(18).Center().FocusRing(false).Children(func() {
					name, col := "arrow-up", p.bgPrimary
					switch {
					case running:
						name = "square"
					case empty || a.sending:
						col = p.placeholder
					}
					ic := ui.Icon(c, icon(name)).Size(16, 16).TextColor(col)
					if running {
						ic.Size(13, 13)
					}
				})
				switch {
				case running:
					send.Background(p.text).Label("停止")
				case empty || a.sending:
					send.Background(p.hoverStrong).Label("发送")
				default:
					send.Background(p.text).Label("发送")
				}
				if send.Clicked() {
					if running {
						a.abort()
					} else {
						a.send()
					}
				}
			})
		})
	})
}

func (a *app) harnessLabel() string {
	if tl := a.timelines[a.selSession]; tl != nil {
		for i := len(tl.turns) - 1; i >= 0; i-- {
			if h := tl.turns[i].Meta.Harness; h != "" {
				return strings.ToUpper(h[:1]) + h[1:]
			}
		}
	}
	return "Pi"
}

func (a *app) modelLabel() string {
	if tl := a.timelines[a.selSession]; tl != nil && len(tl.turns) > 0 {
		t := tl.turns[len(tl.turns)-1]
		if t.Model != "" {
			label := t.Model
			if lv := t.ThinkingLevel; lv != "" && lv != "off" {
				label += " · " + strings.ToUpper(lv[:1]) + lv[1:]
			}
			return label
		}
	}
	return "默认模型"
}

// newChatView is a Space with no chat open: its card, and the composer
// that starts a chat in it.
func (a *app) newChatView(c *ui.Context) {
	p := a.pal
	sp := a.space(a.selSpace)
	a.header(c, func() {
		if !a.sidebarOpen {
			if a.iconButton(c.Key("open-sidebar"), "panel-left", "展开侧栏 (⌘B)", false).Clicked() {
				a.sidebarOpen = true
			}
		}
		if a.compact {
			if a.iconButton(c.Key("back"), "chevron-left", "返回", false).Clicked() {
				a.selSpace = ""
			}
		}
		ui.Text(c, "新建对话").FontSize(13).Font(fontMono).Grow(1)
		if !a.compact {
			if a.iconButton(c.Key("panel"), "panel-right", "侧边面板", a.panelOpen).Clicked() {
				a.panelOpen = !a.panelOpen
			}
		}
	})
	ui.Scroll(c.Key("hero")).Grow(1).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(chatMaxWidth).Margin(0, ui.Auto).Padding(64, 24, 24, 24).Gap(18).Children(func() {
			if sp == nil {
				a.spinner(c, 18, p.textTertiary)
				return
			}
			ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
				a.avatar(c, sp.avatar(), sp.Name, 48, 10)
				ui.Column(c).Gap(4).MinWidth(0).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, sp.Name).FontSize(34).FontWeight(600).Font(fontMono).SingleLine()
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
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Created by").FontSize(14).TextColor(p.textSecond)
					a.avatar(c, o.AvatarURL, o.name(), 18, 9)
					ui.Text(c, o.name()).FontSize(14).FontWeight(500)
				})
			}
		})
	})
	a.composer(c, false)
}
