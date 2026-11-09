package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

const (
	chatMaxWidth = 896 // --chat-content-max-width: 56rem
	msgInset     = 8   // --chat-msg-inset
)

// sessionView is a chat: its header, the turns and the composer, as
// routes/(app)/sessions/[sessionId] and its Space twin.
func (a *app) sessionView(c *ui.Context) {
	p := a.pal
	id := a.route.sessionID()
	s := a.session(id)
	tl := a.timelineOf(id)
	a.header(c, func() {
		a.headerLead(c)
		title := m("sidebar_chats")
		if s != nil {
			title = s.displayTitle()
			if a.route.area() == areaChats && !a.compact {
				name, av := a.spaceName(s.SpaceID)
				crumb := ui.Row(c.Key("crumb")).Gap(6).AlignItems(ui.Center).Shrink(0).MaxWidth(220).Cursor(ui.CursorPointer).Children(func() {
					a.avatarSeed(c, av, name, s.SpaceID, 18, 4)
					ui.Text(c, name).FontSize(13).SingleLine().TextColor(p.text)
				})
				crumb.OnClick(func() { a.openSpace(s.SpaceID) })
				ui.Text(c, "/").FontSize(13).TextColor(p.placeholder)
			}
		}
		if a.compact && s != nil {
			name, av := a.spaceName(s.SpaceID)
			sp := a.avatarSeed(c, av, name, s.SpaceID, 20, 6)
			sp.Cursor(ui.CursorPointer).OnClick(func() { a.openSpace(s.SpaceID) })
		}
		ui.Text(c, title).FontSize(13).SingleLine().Grow(1).MinWidth(0).TextColor(p.textSecond)
		if a.compact {
			a.iconButton(c.Key("share"), "share-2", m("referral_share"), false)
			a.moreMenu(c, s)
			if a.iconButton(c.Key("panel"), "panel-right-open", m("side_panel_show"), false).Clicked() {
				a.compactPanel = true
			}
			return
		}
		share := ui.ButtonBase(c.Key("share")).Row().Gap(6).AlignItems(ui.Center).Height(28).Padding(0, 8).Radius(6).Label(m("referral_share")).FocusRing(false).Children(func() {
			ui.Icon(c, icon("share-2")).Size(14, 14).TextColor(p.textTertiary)
			ui.Text(c, "Share").FontSize(13).TextColor(p.textTertiary)
		})
		if share.Hovered() {
			share.Background(p.hover)
		}
		a.moreMenu(c, s)
		ic, lb := "panel-right-open", m("side_panel_show")
		if a.panelOpen {
			ic, lb = "panel-right-close", m("side_panel_hide")
		}
		if a.iconButton(c.Key("panel"), ic, lb, false).Clicked() {
			a.panelOpen = !a.panelOpen
		}
	})

	turns := tl.turns
	a.timeline.Key = func(i int) any { return turns[i].ID }
	a.timeline.FollowEnd = true
	a.timeline.FollowAnimation = &ui.Smooth
	pad := float32(24)
	if a.compact {
		pad = 16
	}
	ui.Box(c.Key("timeline-box")).Grow(1).MinHeight(0).Children(func() {
		ui.List(c.Key("timeline-"+id), &a.timeline, len(turns), func(i int) {
			ui.Column(c).FillWidth().MaxWidth(chatMaxWidth).Margin(0, ui.Auto).Gap(10).Children(func() {
				a.turnView(c, id, turns[i], tl)
			})
		}).Fill().Padding(28, pad, 32, pad).Gap(28).Scrollbars(a.compact).Children(func() {
			switch {
			case !tl.loaded && a.api != nil:
				ui.Column(c).Grow(1).Center().Children(func() { a.spinner(c, 18, p.textTertiary) })
			case tl.err != "":
				ui.Text(c, tl.err).FontSize(12).TextColor(p.errorC).Margin(ui.Auto)
			case len(turns) == 0:
				ui.Text(c, m("x_no_messages")).FontSize(13).TextColor(p.textTertiary).Margin(ui.Auto)
			}
		})
		if !a.compact && len(turns) > 0 {
			a.turnRail(c, turns)
		}
		if len(turns) > 0 && !a.timeline.AtEnd() {
			down := ui.ButtonBase(c.Key("to-end")).Size(32, 32).Radius(16).Center().Background(p.bgPrimary).Border(1, p.borderSubtle).
				Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, 0.08)).Label(m("x_to_latest")).FocusRing(false).Children(func() {
				ui.Icon(c, icon("arrow-down")).Size(16, 16).TextColor(p.textSecond)
			})
			down.Attach(ui.AnchorBottom, ui.AnchorBottom).Bottom(12)
			if down.Clicked() {
				a.timeline.ScrollToEnd()
			}
		}
	})
	a.composer(c, s != nil && s.running() || lastRunning(tl))
}

// turnRail is TurnRail.svelte: a hairline down the timeline's right with a
// mark for each turn, the one in view in the brand's color.
func (a *app) turnRail(c *ui.Context, turns []*Turn) {
	p := a.pal
	first, last := a.timeline.Visible()
	n := len(turns)
	line := p.borderSubtle.Alpha(0.65)
	thumb := p.placeholder.Alpha(0.35)
	ui.Box(c.Key("rail")).Attach(ui.AnchorTopRight, ui.AnchorTopRight).Top(4).Right(4).Width(28).HeightPercent(97).PassThrough().
		Draw(func(pt *ui.Painter, r ui.Rect) {
			pt.Fill(ui.Rect{X: r.X + r.W - 14, Y: r.Y, W: 1, H: r.H}, line, 0)
			if n > 1 && last >= first {
				top := r.H * float32(first) / float32(n)
				h := max(r.H*float32(last-first+1)/float32(n), 24)
				pt.Fill(ui.Rect{X: r.X + r.W - 15, Y: r.Y + min(top, r.H-h), W: 3, H: h}, thumb, 1.5)
			}
		}).Children(func() {
		for i, t := range turns {
			cur := i == first
			col := p.placeholder.Alpha(0.85)
			switch t.Status {
			case "failed":
				col = p.errorC
			case "running":
				col = p.success
			}
			w, h := float32(6), float32(10)
			if cur {
				w, h = 12, 20
			}
			top := float32(0)
			if n > 1 {
				top = float32(i) / float32(n-1) * 94
			}
			mark := ui.ButtonBase(c.Key(t.ID)).Absolute().TopPercent(top).Right(2).Size(24, h+4).Center().
				Label(fmt.Sprintf("Turn %d", t.Sequence)).FocusRing(false).Children(func() {
				ui.Box(c).Size(w, h).Radius(w/2).Background(col).Border(1, p.chatBg.Over(p.bgPrimary).Alpha(0.9))
			})
			idx := i
			if mark.Clicked() {
				a.timeline.ScrollTo(idx, ui.Start)
			}
		}
	})
}

// headerLead is what starts a header: the sidebar's toggle when it is
// closed, the drawer's in the compact shell.
func (a *app) headerLead(c *ui.Context) {
	if a.compact {
		if a.iconButton(c.Key("menu"), "menu", m("x_menu"), false).Clicked() {
			a.compactNav = true
		}
		return
	}
}

// turnView is one exchange: the user's bubble, the agent's work, its reply.
func (a *app) turnView(c *ui.Context, sessionID string, t *Turn, tl *sessionTurns) {
	p := a.pal
	if prompt := strings.TrimSpace(t.prompt()); prompt != "" {
		ui.Column(c.Key("user")).AlignSelf(ui.End).MaxWidthPercent(100).Gap(4).Children(func() {
			ui.Text(c, prompt).Selectable().FontSize(14).LineHeight(1.65).TextColor(p.text).
				Padding(6, 8).Radius(12, 12, 6, 12).Background(p.userBubble)
			a.metaRow(c.Key("user-meta"), func() {
				a.metaCopy(c, prompt)
				if pr := t.AuthorProfile; pr != nil {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						a.avatar(c, pr.AvatarURL, pr.name(), 14, 7)
						ui.Text(c, pr.name()).FontSize(11).TextColor(p.metaText())
					})
				}
			}, t.StartedAt)
		})
	}

	running := t.running()
	a.processView(c, sessionID, t, tl, running)

	// The final message: its thinking, then the reply, as Markdown.
	reply := t.reply()
	if running && tl.live != "" && (tl.liveTurn == "" || tl.liveTurn == t.ID) {
		reply = tl.live
	}
	if !running {
		for _, b := range t.AssistantContent {
			if b.Type == "thinking" && strings.TrimSpace(b.Thinking) != "" {
				a.thinking(c, "think:"+t.ID, b.Thinking, false)
			}
		}
	}
	if reply != "" {
		ui.Column(c.Key("reply")).Padding(0, msgInset).Children(func() {
			if running {
				markdown.Stream(c, reply, &a.md)
			} else {
				markdown.View(c, reply, &a.md)
			}
		})
	}
	if t.ErrorMessage != "" {
		ui.Column(c.Key("error")).Gap(4).Padding(8, 12).Radius(8).Border(1, p.errorC.Alpha(0.3)).Background(p.errorC.Alpha(0.08)).Children(func() {
			ui.Text(c, m("x_error")).FontSize(12).FontWeight(500).TextColor(p.errorC)
			ui.Text(c, t.ErrorMessage).FontSize(12).TextColor(p.errorC)
		})
	}
	if !running && reply != "" {
		a.replyMeta(c.Key("reply-meta"), t)
	}
}

// metaRow is the line under a message: its actions, then the time on
// the right, in the quiet color of --text-placeholder at half.
func (a *app) metaRow(c *ui.Context, fn func(), at time.Time) {
	ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(0, msgInset).Height(22).Children(func() {
		fn()
		ui.Spacer(c).MinWidth(24)
		if !at.IsZero() {
			ui.Text(c, clock(at, a.now())).FontSize(11).TextColor(a.pal.metaText()).Shrink(0)
		}
	})
}

func (a *app) metaCopy(c *ui.Context, text string) {
	p := a.pal
	b := ui.ButtonBase(c.Key("copy")).Size(22, 22).Center().Radius(4).Label(m("copy")).FocusRing(false).Children(func() {
		ui.Icon(c, icon("copy")).Size(14, 14).TextColor(p.metaText())
	})
	if b.Hovered() {
		b.Background(p.hover)
	}
	if b.Clicked() {
		c.WriteClipboard(text)
	}
}

// thinking is ThinkingBlocks: one line that opens into the whole thought,
// or all of it while it streams.
func (a *app) thinking(c *ui.Context, key, text string, streaming bool) {
	p := a.pal
	text = strings.TrimSpace(text)
	if streaming || a.foldOpen[key] {
		ui.Column(c.Key(key)).Gap(4).Padding(0, msgInset).Children(func() {
			ui.Text(c, text).FontSize(13).LineHeight(1.375).TextColor(p.disabled).Selectable()
			if !streaming {
				b := ui.ButtonBase(c.Key("collapse")).FocusRing(false).AlignSelf(ui.Start).Children(func() {
					ui.Text(c, m("common_collapse")).FontSize(11).TextColor(p.placeholder)
				})
				if b.Clicked() {
					a.foldOpen[key] = false
				}
			}
		})
		return
	}
	b := ui.ButtonBase(c.Key(key)).Padding(0, msgInset).Justify(ui.Start).FocusRing(false).Children(func() {
		ui.Text(c, oneLine(text)).FontSize(13).LineHeight(1.375).TextColor(p.disabled).SingleLine()
	})
	if b.Clicked() {
		a.foldOpen[key] = true
	}
}

// replyMeta is the line under a reply: copy, fork, the harness, the model,
// the final message's tokens and time, and when it finished.
func (a *app) replyMeta(c *ui.Context, t *Turn) {
	p := a.pal
	at := t.StartedAt
	if t.CompletedAt != nil {
		at = *t.CompletedAt
	}
	a.metaRow(c, func() {
		a.metaCopy(c, t.reply())
		if h := t.Meta.Harness; h != "" && h != "cohub" {
			ui.Row(c).Gap(3).AlignItems(ui.Center).Padding(1, 5).Radius(3).Background(p.hoverStrong).Children(func() {
				a.harnessIcon(c, h, 10)
				ui.Text(c, harnessName(h)).FontSize(10).FontWeight(500).TextColor(p.textTertiary)
			})
		} else {
			ui.Box(c.Key("fork")).Size(22, 22).Center().Children(func() {
				ui.Icon(c, icon("git-fork")).Size(14, 14).TextColor(p.metaText())
			})
		}
		if t.Meta.RuntimeArchiveStatus == "ready" {
			ui.Icon(c, icon("archive")).Size(12, 12).TextColor(p.textTertiary)
		}
		ui.Text(c, t.Model).FontSize(11).TextColor(p.metaText()).SingleLine().MinWidth(0)
		if lv := levelName(t.Meta.RequestedThinkingLevel, t.ThinkingLevel); lv != "" {
			ui.Text(c, lv).FontSize(11).TextColor(p.metaText()).Shrink(0)
		}
		if u := usageLabel(t.FinalUsage); u != "" {
			ui.Text(c, u).FontSize(11).TextColor(p.metaText()).Shrink(0)
		}
		if d := t.Meta.FinalMessageDurationMs; d > 0 {
			ui.Text(c, duration(d)).FontSize(11).TextColor(p.metaText()).Shrink(0)
		}
	}, at)
}

// processView is the agent's work in a turn: a line that sums it up and
// opens into its steps, each run of tool calls folded into one line.
func (a *app) processView(c *ui.Context, sessionID string, t *Turn, tl *sessionTurns, running bool) {
	p := a.pal
	sum := t.IntermediateSummary
	steps, tools := 0, 0
	if sum != nil {
		steps, tools = sum.MessageCount, sum.ToolCallCount
	}
	var live []Block
	if running && (tl.liveTurn == "" || tl.liveTurn == t.ID) {
		live = tl.liveSteps
	}
	if steps == 0 && !running {
		return
	}
	open := tl.expanded[t.ID]
	if steps > 0 || !running {
		parts := []string{}
		if steps > 0 {
			parts = append(parts, plural("process_steps", steps))
		}
		if tools > 0 {
			parts = append(parts, plural("process_tools", tools))
		}
		if sum != nil {
			if u := usageLabel(sum.Usage); u != "" {
				parts = append(parts, u)
			}
			if cost := sum.Usage.cost(); cost > 0 {
				parts = append(parts, fmt.Sprintf("US$%.3f", cost))
			}
			if sum.DurationMs > 0 {
				parts = append(parts, duration(sum.DurationMs))
			}
		}
		label := strings.Join(parts, " · ")
		_, loading := tl.steps[t.ID]
		loading = open && !loading
		if open {
			label = m("common_collapse")
		}
		head := ui.ButtonBase(c.Key("process")).Row().Gap(8).AlignItems(ui.Center).Justify(ui.Start).Padding(8, msgInset).Radius(6).FocusRing(false).Children(func() {
			switch {
			case loading:
				a.spinner(c, 14, p.textTertiary)
			case open:
				ui.Icon(c, icon("chevron-down")).Size(14, 14).TextColor(p.textTertiary)
			default:
				ui.Icon(c, icon("chevron-right")).Size(14, 14).TextColor(p.textTertiary)
			}
			ui.Text(c, label).FontSize(13).TextColor(p.textTertiary).SingleLine()
		})
		if head.Hovered() {
			head.Background(p.hover.Alpha(0.5))
		}
		if head.Clicked() {
			tl.expanded[t.ID] = !open
			if !open {
				a.loadSteps(sessionID, t)
			}
		}
	}
	if open {
		if msgs := tl.steps[t.ID]; msgs != nil {
			a.stepEntries(c, t, msgs)
		}
	}
	if running {
		if len(live) == 0 {
			ui.Row(c.Key("working")).Gap(8).AlignItems(ui.Center).Padding(4, msgInset).Children(func() {
				a.statusDot(c, p.brand, true)
				ui.Text(c, m("x_running_dot")+elapsed(t.StartedAt, a.now())).FontSize(13).TextColor(p.textTertiary)
				c.After(time.Second)
			})
			return
		}
		done := map[string]bool{}
		for _, b := range live {
			if b.Type == "tool_result" || b.Type == "toolResult" {
				done[b.ToolUse] = true
			}
		}
		ui.Column(c.Key("live")).Gap(8).Children(func() {
			for i, b := range live {
				if done[b.ID] && b.Meta == nil {
					b.Meta = &blockMeta{ToolStatus: "done"}
				}
				switch b.Type {
				case "thinking":
					if strings.TrimSpace(b.Thinking) != "" {
						a.thinking(c, fmt.Sprint("live-think", i), b.Thinking, true)
					}
				case "tool_use", "toolCall":
					a.toolRow(c.Key(i), b, true)
				}
			}
		})
	}
}

func (a *app) loadSteps(sessionID string, t *Turn) {
	if a.api == nil || t.IntermediateIndex == nil || t.IntermediateIndex.MessagesObjectKey == "" {
		a.timelineOf(sessionID).steps[t.ID] = []*StepMessage{}
		return
	}
	key := t.IntermediateIndex.MessagesObjectKey
	a.once("steps:"+t.ID, func(ctx context.Context) {
		msgs, err := a.api.steps(ctx, sessionID, t.ID, key)
		a.update(func() {
			if err != nil {
				msgs = []*StepMessage{}
				a.reload("steps:" + t.ID)
			}
			a.timelineOf(sessionID).steps[t.ID] = msgs
		})
	})
}

// stepEntries shows the steps of a turn as buildProcessEntries groups them:
// a step with text on its own, the steps between in runs.
func (a *app) stepEntries(c *ui.Context, t *Turn, msgs []*StepMessage) {
	var run []*StepMessage
	flush := func(key int) {
		if len(run) > 0 {
			a.stepRun(c.Key(fmt.Sprint("run", key)), t, run)
			run = nil
		}
	}
	ui.Column(c.Key("steps")).Gap(12).Children(func() {
		for i, m := range msgs {
			if m.hasText() || m.StopReason == "error" || m.Meta.MessageKind == "compacted" {
				flush(i)
				a.stepMessage(c.Key(fmt.Sprint("step", i)), m)
				continue
			}
			run = append(run, m)
		}
		flush(len(msgs))
	})
}

func toolBlocks(msgs ...*StepMessage) []Block {
	var out []Block
	for _, m := range msgs {
		for _, b := range m.Content {
			if b.Type == "tool_use" || b.Type == "toolCall" {
				out = append(out, b)
			}
		}
	}
	return out
}

// stepRun is steps without text, folded into one line: "3 步 · 5 个工具 bash ×4 · read".
func (a *app) stepRun(c *ui.Context, t *Turn, msgs []*StepMessage) {
	tools := toolBlocks(msgs...)
	open := a.foldOpen[msgs[0].ID]
	parts := []string{plural("process_steps", len(msgs))}
	if len(tools) > 0 {
		parts = append(parts, plural("process_tools", len(tools)))
	}
	var names []string
	counts := map[string]int{}
	failed := 0
	for _, b := range tools {
		if counts[b.Name] == 0 {
			names = append(names, b.Name)
		}
		counts[b.Name]++
		if b.Meta != nil && (b.Meta.ToolStatus == "error" || b.Meta.ToolStatus == "failed") {
			failed++
		}
	}
	for i, n := range names {
		if counts[n] > 1 {
			names[i] = fmt.Sprintf("%s ×%d", n, counts[n])
		}
	}
	ui.Column(c).Gap(8).Children(func() { a.stepRunBody(c, msgs, tools, parts, names, failed, open) })
}

func (a *app) stepRunBody(c *ui.Context, msgs []*StepMessage, tools []Block, parts, names []string, failed int, open bool) {
	p := a.pal
	row := ui.ButtonBase(c.Key("head")).Row().Gap(8).AlignItems(ui.Center).Justify(ui.Start).MinHeight(28).Padding(4, msgInset).Radius(6).FocusRing(false).Children(func() {
		col := p.success.Alpha(0.75)
		if failed > 0 {
			col = p.errorC
		}
		a.statusDot(c, col, false)
		ui.Text(c, strings.Join(parts, " · ")).FontSize(13).TextColor(p.textTertiary).Shrink(0)
		ui.Text(c, strings.Join(names, " · ")).FontSize(13).Font(fontMono).TextColor(p.placeholder).SingleLine().Grow(1).MinWidth(0)
		if failed > 0 {
			ui.Text(c, m("tool_failed_count", "count", failed)).FontSize(11).TextColor(p.errorC)
		}
		name := "chevron-right"
		if open {
			name = "chevron-down"
		}
		ui.Icon(c, icon(name)).Size(14, 14).TextColor(p.textTertiary)
	})
	if row.Hovered() {
		row.Background(p.hover.Alpha(0.5))
	}
	if row.Clicked() {
		a.foldOpen[msgs[0].ID] = !open
	}
	if open {
		ui.Column(c.Key("body")).Gap(8).MarginStart(msgInset + 3).PaddingStart(11).BorderStartWidth(1).BorderColor(p.borderSubtle.Alpha(0.7)).Children(func() {
			for i, m := range msgs {
				ui.Column(c.Key(i)).Gap(6).Children(func() { a.stepBody(c, m) })
			}
		})
	}
}

// stepMessage is a step with text: its thinking, text, tools and meta.
func (a *app) stepMessage(c *ui.Context, m *StepMessage) {
	ui.Column(c).Gap(8).Children(func() {
		a.stepBody(c, m)
		a.metaRow(c.Key("meta"), func() {
			p := a.pal
			a.metaCopy(c, m.Text)
			ui.Text(c, m.Model).FontSize(11).TextColor(p.metaText()).SingleLine().MinWidth(0)
			if u := usageLabel(m.Usage); u != "" {
				ui.Text(c, u).FontSize(11).TextColor(p.metaText()).Shrink(0)
			}
			if m.DurationMs > 0 {
				ui.Text(c, duration(m.DurationMs)).FontSize(11).TextColor(p.metaText()).Shrink(0)
			}
		}, m.CreatedAt)
	})
}

func (a *app) stepBody(c *ui.Context, m *StepMessage) {
	p := a.pal
	for i, b := range m.Content {
		switch b.Type {
		case "thinking":
			if strings.TrimSpace(b.Thinking) != "" {
				a.thinking(c, fmt.Sprint(m.ID, ":think", i), b.Thinking, false)
			}
		case "text":
			if strings.TrimSpace(b.Text) != "" {
				ui.Column(c.Key(i)).Padding(0, msgInset).Children(func() {
					ui.Text(c, strings.TrimSpace(b.Text)).FontSize(14).LineHeight(1.7).TextColor(p.text).Selectable()
				})
			}
		case "tool_use", "toolCall":
			a.toolRow(c.Key(i), b, false)
		}
	}
	if m.Error != "" {
		ui.Text(c, m.Error).FontSize(12).TextColor(p.errorC).Padding(0, msgInset)
	}
}

// toolRow is one tool call: its name, what it ran on, and how it went.
func (a *app) toolRow(c *ui.Context, b Block, live bool) {
	p := a.pal
	status := "done"
	if b.Meta != nil && b.Meta.ToolStatus != "" {
		status = b.Meta.ToolStatus
	}
	if live && (b.Meta == nil || b.Meta.ToolStatus == "") {
		status = "running"
	}
	summary := ""
	if b.Meta != nil {
		summary = b.Meta.Summary
	}
	if summary == "" {
		summary = toolInputSummary(b)
	}
	if b.Name == "bash" && summary != "" && !strings.HasPrefix(summary, "$") {
		summary = "$ " + summary
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).MinHeight(28).Padding(4, msgInset).Radius(6).Children(func() {
		col, label := p.success.Alpha(0.75), m("tool_status_done")
		switch status {
		case "running", "pending":
			col, label = p.brand, m("session_activity_running")
		case "error", "failed":
			col, label = p.errorC, m("tool_status_failed")
		}
		a.statusDot(c, col, status == "running")
		ui.Text(c, b.Name).FontSize(13).Font(fontMono).TextColor(p.textSecond).Shrink(0).MinWidth(56)
		ui.Text(c, oneLine(summary)).FontSize(13).Font(fontMono).TextColor(p.textTertiary).SingleLine().Grow(1).MinWidth(0)
		ui.Text(c, label).FontSize(11).TextColor(p.placeholder).Shrink(0)
		ui.Icon(c, icon("chevron-right")).Size(14, 14).TextColor(p.textTertiary)
	})
}

func toolInputSummary(b Block) string {
	raw := b.Input
	if len(raw) == 0 {
		raw = b.Args
	}
	var in map[string]any
	if json.Unmarshal(raw, &in) != nil {
		return ""
	}
	for _, k := range []string{"command", "path", "url", "query", "pattern"} {
		if s, ok := in[k].(string); ok {
			return s
		}
	}
	return ""
}

// statusDot is StatusGlyph: a dot of the tone, breathing while active.
func (a *app) statusDot(c *ui.Context, col ui.Color, active bool) {
	d := a.dot(c, col, 6)
	if active {
		ph := d.Loop("breathe", 1400*time.Millisecond, ui.EaseInOut)
		if ph > 0.5 {
			ph = 1 - ph
		}
		d.Opacity(0.45 + 1.1*ph)
	}
}

// moreMenu is the header's ⋯: label, stats, quote and archive.
func (a *app) moreMenu(c *ui.Context, s *Session) {
	b := a.iconButton(c.Key("more"), "ellipsis", m("turnrail_more"), a.menus["more"])
	if b.Clicked() {
		a.menus["more"] = !a.menus["more"]
	}
	open := a.menus["more"]
	a.menu(c, b, &open, []menuItem{
		{icon: "tag", label: m("inline_label_as")},
		{icon: "gauge", label: m("stats_menu")},
		{icon: "text-quote", label: m("space_header_insert_reference")},
		{icon: "archive", label: m("session_archive"), run: func() { a.archiveSession(s) }},
	})
	a.menus["more"] = open
}

func (a *app) archiveSession(s *Session) {
	if s == nil || a.api == nil {
		return
	}
	id := s.ID
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if a.api.do(ctx, "PUT", "/api/sessions/"+id+"/archive", nil, nil) == nil {
			a.refresh()
		}
	})
}

// composer is the message box under a chat, as SessionComposer.svelte.
func (a *app) composer(c *ui.Context, running bool) {
	p := a.pal
	pad := float32(24)
	if a.compact {
		pad = 16
	}
	ui.Column(c).Shrink(0).Padding(0, pad, 16, pad).Children(func() {
		if a.sendErr != "" {
			ui.Text(c, a.sendErr).FontSize(12).TextColor(p.errorC).FillWidth().MaxWidth(832).Margin(0, ui.Auto, 6, ui.Auto)
		}
		card := ui.Column(c.Key("composer")).FillWidth().MaxWidth(832).Margin(0, ui.Auto).Radius(16).
			Background(p.bgInput).Border(1, p.brandBorder).Padding(14, 12, 12, 12).Gap(12)
		if files := card.DroppedFiles(); files != nil {
			a.addAttachmentPaths(files)
		}
		if card.FileDragOver() {
			card.Border(2, p.brand)
		}
		card.Children(func() {
			a.attachmentStrip(c)
			in := ui.TextAreaBase(c.Key("draft"), &a.draft).Lines(2, 10).Placeholder(m("composer_placeholder")).Label(m("x_message")).PlaceholderColor(p.placeholder).
				FontSize(14).LineHeight(1.55).Padding(0, 9)
			if !a.compact {
				in.AutoFocus()
			}
			in.OnTextKey(func(k ui.TextKey) bool {
				if k.Key == ui.KeyEnter && k.Mods == 0 && !a.compact {
					a.draft = k.Text()
					a.send()
					k.SetText(a.draft)
					return true
				}
				if k.Key == ui.KeyV && (k.Mods == ui.Cmd || k.Mods == ui.Ctrl) && a.pasteImage() {
					return true
				}
				return false
			})
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				if a.iconButton(c.Key("attach"), "plus", m("feedback_add_files"), false).Clicked() {
					a.pickAttachments()
				}
				if h := a.composerHarness(); h != "" && h != "cohub" {
					a.pill(c.Key("harness"), harnessName(h), "", func() { a.harnessIcon(c, h, 14) })
				}
				a.modelPicker(c)
				ui.Spacer(c)
				listening := a.voice != nil && a.voice.active
				label := m("composer_start_voice")
				if listening {
					label = m("composer_stop_voice")
				}
				mic := a.iconButton(c.Key("mic"), "mic", label, listening)
				if listening {
					mic.Background(p.errorC.Alpha(0.12))
				}
				if mic.Clicked() {
					a.toggleVoice()
				}
				empty := strings.TrimSpace(a.draft) == "" && len(a.attachments) == 0
				send := ui.ButtonBase(c.Key("send")).Size(36, 36).Radius(18).Center().FocusRing(false).Children(func() {
					switch {
					case running:
						ui.Box(c).Size(12, 12).Radius(2).Background(p.bgPrimary)
					case empty || a.sending:
						ui.Icon(c, icon("arrow-up")).Size(16, 16).TextColor(p.placeholder.Alpha(0.6))
					default:
						ui.Icon(c, icon("arrow-up")).Size(16, 16).TextColor(p.bgPrimary)
					}
				})
				switch {
				case running:
					send.Background(p.text).Label(m("board_session_stop"))
				case empty || a.sending:
					send.Background(p.hover).Label(m("feedback_send"))
				default:
					send.Background(p.text).Label(m("feedback_send"))
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

func (a *app) composerHarness() string {
	if tl := a.timelines[a.route.sessionID()]; tl != nil {
		for i := len(tl.turns) - 1; i >= 0; i-- {
			if h := tl.turns[i].Meta.Harness; h != "" {
				return h
			}
		}
	}
	return ""
}

// composerModel is the model the next message goes to: the one chosen in
// the picker, else the chat's last, else the first of the catalog.
func (a *app) composerModel() modelChoice {
	key := a.route.sessionID()
	if key == "" {
		key = "space:" + a.currentSpace()
	}
	if m, ok := a.modelChoices[key]; ok {
		return m
	}
	if tl := a.timelines[a.route.sessionID()]; tl != nil {
		for i := len(tl.turns) - 1; i >= 0; i-- {
			t := tl.turns[i]
			if t.Model == "" {
				continue
			}
			lv := t.Meta.RequestedThinkingLevel
			if lv == "" {
				lv = t.ThinkingLevel
			}
			return modelChoice{provider: t.Provider, id: t.Model, level: lv}
		}
	}
	if len(a.models) > 0 {
		return modelChoice{provider: a.models[0].Provider, id: a.models[0].ID}
	}
	return modelChoice{}
}

// modelPicker is the composer's model chip, which opens the model
// selector, as ModelSelector.svelte.
func (a *app) modelPicker(c *ui.Context) {
	p := a.pal
	cur := a.composerModel()
	label := cur.id
	if label == "" {
		label = m("model_default")
	}
	b := a.pill(c.Key("model"), label, levelName(cur.level, ""), nil).Label(m("model_choose"))
	if b.Clicked() {
		a.menus["model"] = true
		a.pages.json["model-query"] = ""
	}
	open := a.menus["model"]
	a.dialog(c, &open, m("model_selector_title"), 540, func() {
		ui.Row(c.Key("tabs")).Padding(8, 12).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle.Alpha(0.7)).Children(func() {
			ui.Row(c).Padding(2).Radius(6).Background(p.hover.Alpha(0.7)).Children(func() {
				for i, l := range []string{m("model_selector_tab_chat"), m("model_selector_tab_generation")} {
					t := ui.Box(c.Key(l)).Padding(6, 12).Radius(4).Children(func() {
						col := p.textTertiary
						if i == 0 {
							col = p.text
						}
						ui.Text(c, l).FontSize(12).FontWeight(500).TextColor(col)
					})
					if i == 0 {
						t.Background(p.bgSurface).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.06))
					}
				}
			})
		})
		q, _ := a.pages.json["model-query"].(string)
		ui.Row(c.Key("search")).Padding(8, 12).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle.Alpha(0.7)).Children(func() {
			ui.TextInputBase(c.Key("q"), &q).Placeholder(m("model_selector_search_placeholder")).PlaceholderColor(p.placeholder).
				FontSize(13).Padding(8, 12).Radius(6).Background(p.bgInput).Border(1, p.borderSubtle).Grow(1).AutoFocus()
		})
		a.pages.json["model-query"] = q
		ui.Scroll(c.Key("models")).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Padding(4, 0).Children(func() {
				n := 0
				models := append([]*Model(nil), a.models...)
				sort.SliceStable(models, func(i, j int) bool { return models[i].ID == cur.id && models[j].ID != cur.id })
				for _, mod := range models {
					if q != "" && !strings.Contains(strings.ToLower(mod.ID), strings.ToLower(q)) {
						continue
					}
					n++
					on := mod.ID == cur.id
					row := ui.Row(c.Key(mod.ID)).Gap(12).AlignItems(ui.Start).Padding(8, 16).Children(func() {
						if on {
							ui.Box(c).Absolute().Left(0).Top(14).Size(2, 16).Radius(1).Background(p.brand)
						}
						ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Text(c, mod.ID).FontSize(13).FontWeight(500).SingleLine()
								if mod.vision() {
									ui.Icon(c, icon("image")).Size(14, 14).TextColor(p.textTertiary)
								}
								a.dot(c, p.success, 6)
							})
							if cost := mod.costLabel(); cost != "" {
								ui.Text(c, cost).FontSize(11).TextColor(p.textTertiary.Alpha(0.75)).SingleLine()
							}
						})
						ui.Column(c).AlignItems(ui.End).Gap(2).Shrink(0).Children(func() {
							ui.Text(c, mod.Provider).FontSize(11).TextColor(p.textTertiary.Alpha(0.7))
							if lv := mod.levels(); len(lv) > 1 {
								level := mod.Model.DefaultLevel
								if level == "" {
									level = "high"
								}
								if on && cur.level != "" {
									level = cur.level
								}
								if !contains(lv, level) {
									level = lv[len(lv)-1]
								}
								ui.Row(c).Gap(3).AlignItems(ui.Center).Padding(1, 4).Children(func() {
									ui.Icon(c, icon("brain")).Size(11, 11).TextColor(p.textTertiary)
									ui.Text(c, levelName(level, "")).FontSize(10).TextColor(p.textTertiary)
									ui.Icon(c, icon("chevron-down")).Size(10, 10).TextColor(p.textTertiary)
								})
							}
						})
					})
					switch {
					case on:
						row.Background(p.hover)
					case row.Hovered():
						row.Background(p.hover)
					}
					row.Cursor(ui.CursorPointer).OnClick(func() {
						lv := cur.level
						if !contains(mod.levels(), lv) {
							lv = ""
						}
						a.chooseModel(modelChoice{provider: mod.Provider, id: mod.ID, level: lv})
						a.menus["model"] = false
					})
				}
				if n == 0 {
					ui.Text(c, m("model_selector_no_matching")).FontSize(13).TextColor(p.textTertiary).Padding(32).Margin(0, ui.Auto)
				}
			})
		})
	})
	if a.menus["model"] {
		a.menus["model"] = open
	}
}

// dialog is Dialog.svelte: a panel over the dimmed window, with a small
// uppercase title and a close button.
func (a *app) dialog(c *ui.Context, open *bool, title string, maxWidth float32, fn func()) {
	p := a.pal
	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.5)).Padding(16)
		_, h := c.Size()
		panel.FillWidth().MaxWidth(maxWidth).MaxHeight(min(h*0.85, 720)).Radius(12).Background(p.bgPrimary).Border(1, p.borderSubtle).
			Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25)).Clip()
		if title != "" {
			ui.Row(c.Key("dialog-head")).Height(36).Shrink(0).AlignItems(ui.Center).Padding(0, 6, 0, 12).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
				ui.Text(c, strings.ToUpper(title)).FontSize(10).FontWeight(500).LetterSpacing(0.8).TextColor(p.textTertiary).Grow(1)
				x := ui.ButtonBase(c.Key("close")).Size(24, 24).Center().Radius(5).Label(m("common_close")).FocusRing(false).Children(func() {
					ui.Icon(c, icon("x")).Size(14, 14).TextColor(p.textTertiary)
				})
				if x.Hovered() {
					x.Background(p.hover)
				}
				if x.Clicked() {
					*open = false
				}
			})
		}
		fn()
	})
}

func (a *app) chooseModel(m modelChoice) {
	key := a.route.sessionID()
	if key == "" {
		key = "space:" + a.currentSpace()
	}
	a.modelChoices[key] = m
}

func (a *app) model(id string) *Model {
	for _, m := range a.models {
		if m.ID == id {
			return m
		}
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// harnessIcon is HarnessIcon: Pi's mark, or a spark for the others.
func (a *app) harnessIcon(c *ui.Context, h string, size float32) {
	col := a.pal.textTertiary
	if h == "pi" {
		ui.Box(c).Size(size, size).Shrink(0).Draw(func(pt *ui.Painter, r ui.Rect) {
			u := r.W / 8
			for _, sq := range [][2]float32{{1, 1}, {2, 1}, {3, 1}, {4, 1}, {5, 1}, {6, 1}, {2, 2}, {2, 3}, {2, 4}, {2, 5}, {2, 6}, {5, 2}, {5, 3}, {5, 4}, {5, 5}, {6, 6}, {1, 6}} {
				pt.Fill(ui.Rect{X: r.X + sq[0]*u, Y: r.Y + sq[1]*u, W: u, H: u}, col, 0)
			}
		})
		return
	}
	ui.Icon(c, icon("sparkles")).Size(size, size).TextColor(col)
}

func harnessName(h string) string {
	switch h {
	case "pi":
		return "Pi"
	case "claude-code":
		return "Claude Code"
	case "codex":
		return "Codex"
	}
	if h == "" {
		return ""
	}
	return strings.ToUpper(h[:1]) + h[1:]
}

// levelName shortens a thinking level as the meta bar does.
func levelName(requested, effective string) string {
	lv := requested
	if lv == "" {
		lv = effective
	}
	switch lv {
	case "", "off":
		return ""
	case "xhigh":
		return "xHigh"
	}
	return strings.ToUpper(lv[:1]) + lv[1:]
}

// usageLabel is formatUsageBreakdown: "↑38.8k (缓存 36.0k) ↓1.6k".
func usageLabel(u *Usage) string {
	if u == nil {
		return ""
	}
	in := u.Input + u.CacheRead + u.CacheWrite
	var parts []string
	if in > 0 {
		s := "↑" + kilo(in)
		if u.CacheRead > 0 {
			s += " (" + m("chat_cached", "count", kilo(u.CacheRead)) + ")"
		}
		parts = append(parts, s)
	}
	if u.Output > 0 {
		parts = append(parts, "↓"+kilo(u.Output))
	}
	return strings.Join(parts, " ")
}
