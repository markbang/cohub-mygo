package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"cohub-mygo/internal/cohub"
	"cohub-mygo/internal/i18n"
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
			title = chatTitle(s)
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
		a.sessionTitle(c, s, title)
		if s != nil && !a.compact {
			a.runtimeStatus(c, s.SpaceID)
			a.presenceStack(c, s.SpaceID)
		}
		if a.compact {
			a.shareButton(c, s)
			a.moreMenu(c, s)
			if a.iconButton(c.Key("panel"), "panel-right-open", m("side_panel_show"), false).Clicked() {
				a.compactPanel = true
			}
			return
		}
		a.shareButton(c, s)
		a.moreMenu(c, s)
		ic, lb := "panel-right-open", m("side_panel_show")
		if a.panelOpen {
			ic, lb = "panel-right-close", m("side_panel_hide")
		}
		if a.iconButton(c.Key("panel"), ic, lb, false).Clicked() {
			a.panelOpen = !a.panelOpen
		}
	})

	turns := tl.shown()
	a.timeline.Key = func(i int) any { return turns[i].ID }
	a.timeline.FollowEnd = true
	a.timeline.FollowAnimation = &ui.Smooth
	pad := float32(24)
	if a.compact {
		pad = 16
	}
	a.findBar(c, turns)
	ui.Box(c.Key("timeline-box")).Grow(1).MinHeight(0).Children(func() {
		list := ui.List(c.Key("timeline-"+id), &a.timeline, len(turns), func(i int) {
			ui.Column(c).FillWidth().MaxWidth(chatMaxWidth).Margin(0, ui.Auto).Gap(10).Children(func() {
				a.turnView(c, id, turns[i], tl)
			})
		}).Fill().Padding(28, pad, 32, pad).Gap(28).Scrollbars(a.compact).TrackScroll(&a.timelineScroll)
		list.Children(func() {
			switch {
			case !tl.loaded && a.api != nil:
				ui.Column(c).Grow(1).Center().Children(func() { a.spinner(c, 18, p.textTertiary) })
			case tl.err != "":
				ui.Text(c, tl.err).FontSize(12).TextColor(p.errorC).Margin(ui.Auto)
			case len(turns) == 0:
				ui.Text(c, m("x_no_messages")).FontSize(13).TextColor(p.textTertiary).Margin(ui.Auto)
			}
		})
		// Scrolled up to the first turn loaded, the turns before it load.
		if first, last := a.timeline.Visible(); len(turns) > 0 && last >= first && first == 0 && tl.hasMore {
			a.loadOlderTurns(id)
		}
		if tl.loadingOlder {
			ui.Box(c.Key("older")).Attach(ui.AnchorTop, ui.AnchorTop).Top(12).Children(func() { a.spinner(c, 16, p.textTertiary) })
		}
		if !a.compact && len(turns) > 0 {
			a.turnRail(c, id, turns, tl, list.Bounds().H)
		}
		if a.compact {
			a.timelinePill(c, turns)
		} else if len(turns) > 0 && !a.timeline.AtEnd() {
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
	if s != nil {
		a.followups(c, s, tl)
		a.approvals(c, s, tl)
		a.archivedNotice(c, s)
	}
	a.composer(c, s != nil && s.Running() || lastRunning(tl))
}

// archivedNotice is the line above the composer of an archived chat, as the
// web's: sending restores it, or the button does now.
func (a *app) archivedNotice(c *ui.Context, s *cohub.Session) {
	if s.ArchivedAt == nil {
		return
	}
	p := a.pal
	pad := float32(24)
	if a.compact {
		pad = 16
	}
	ui.Row(c.Key("archived")).Shrink(0).Padding(0, pad, 4, pad).Children(func() {
		ui.Row(c).FillWidth().MaxWidth(chatMaxWidth).Margin(0, ui.Auto).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icon("archive")).Size(14, 14).TextColor(p.textTertiary)
			ui.Text(c, m("session_archived_notice")).FontSize(12).TextColor(p.textTertiary).SingleLine().Grow(1).MinWidth(0)
			b := ui.ButtonBase(c.Key("unarchive")).Padding(2, 6).Radius(4).Label(m("session_unarchive")).FocusRing(false)
			hover := b.Hovered()
			b.Children(func() {
				col := p.textSecond
				if hover {
					col = p.text
				}
				ui.Text(c, m("session_unarchive")).FontSize(12).TextColor(col)
			})
			if hover {
				b.Background(p.hover)
			}
			if b.Clicked() {
				a.setSessionArchived(s, false)
			}
		})
	})
}

// turnRail is TurnRail.svelte: a hairline down the timeline's right with a
// mark for each turn, the one in view in the brand's color, over a track
// whose thumb shows where the timeline is and drags it. Hovering a mark
// opens the turn navigator beside it; a control at the top loads the turns
// before the first loaded.
func (a *app) turnRail(c *ui.Context, sessionID string, turns []*cohub.Turn, tl *sessionTurns, listH float32) {
	p := a.pal
	first, _ := a.timeline.Visible()
	n := len(turns)
	line := p.borderSubtle.Alpha(0.65)
	nav := &a.turnNav
	rail := ui.Box(c.Key("rail")).Attach(ui.AnchorTopRight, ui.AnchorTopRight).Top(4).Right(4).Width(28).HeightPercent(97).PassThrough()
	// The rail passes the pointer through: it is hovered over its marks,
	// its track and the navigator, which are inside it.
	nav.hold(c, rail.Hovered())
	railH := rail.Bounds().H
	rail.Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.Fill(ui.Rect{X: r.X + r.W - 14, Y: r.Y, W: 1, H: r.H}, line, 0)
	}).Children(func() {
		a.railTrack(c, listH)
		older := tl.hasMore || tl.loadingOlder
		if older {
			b := ui.ButtonBase(c.Key("older")).Absolute().Top(-2).Right(2).Size(24, 24).Center().
				Label(m("turnrail_load_older")).Tooltip(m("turnrail_load_older")).FocusRing(false)
			hover := b.Hovered()
			b.Children(func() {
				if tl.loadingOlder {
					a.spinner(c, 12, p.textTertiary)
					return
				}
				ui.Column(c).Gap(2).AlignItems(ui.Center).Opacity(map[bool]float32{true: 1, false: 0.7}[hover]).Children(func() {
					for _, al := range []float32{0.7, 0.45, 0.25} {
						ui.Box(c).Size(4, 4).Radius(2).Background(p.placeholder.Alpha(al))
					}
				})
			})
			if b.Clicked() {
				a.loadOlderTurns(sessionID)
			}
		}
		// The marks keep clear of the control above them.
		from, span := float32(0), float32(94)
		if older {
			from, span = 5, 89
		}
		at := float32(-1)
		for i, t := range turns {
			cur := i == first
			col := p.placeholder.Alpha(0.85)
			switch t.Status {
			case "failed":
				col = p.errorC
			case "running":
				col = p.success
			case "interrupted":
				col = p.warning
			}
			w, h := float32(6), float32(10)
			if cur {
				w, h, col = 12, 20, p.brand
			}
			top := from
			if n > 1 {
				top = from + float32(i)/float32(n-1)*span
			}
			mark := ui.ButtonBase(c.Key(t.ID)).Absolute().TopPercent(top).Right(2).Size(24, h+4).Center().
				Label(m("turnrail_jump_turn", "n", t.Sequence)).FocusRing(false)
			if mark.Hovered() && !a.railDrag.on {
				nav.show(t.ID)
				if !cur {
					w, col = 10, col.Alpha(1)
				}
			}
			if nav.open && nav.turn == t.ID {
				at = railH*top/100 + (h+4)/2
			}
			mark.Children(func() {
				ui.Box(c).Size(w, h).Radius(w/2).Background(col).Border(1, p.chatBg.Over(p.bgPrimary).Alpha(0.9))
			})
			id := t.ID
			if mark.Clicked() {
				a.jumpToTurn(id)
			}
		}
		if nav.open && at >= 0 && railH > 0 {
			a.turnNavigator(c, turns, at, railH)
		}
	})
}

// railDrag is a drag of the rail's track: where the pointer is down the
// track, and where on the thumb it holds it.
type railDrag struct {
	on      bool
	y, grab float32
}

// railTrack is the rail's scroll track, as the web's: its thumb shows the
// timeline's place and size; a press puts the thumb's middle there, and a
// drag moves it, scrolling the timeline.
func (a *app) railTrack(c *ui.Context, listH float32) {
	p := a.pal
	sc := &a.timelineScroll
	if sc.MaxY <= 1 || listH <= 0 {
		return
	}
	track := ui.Box(c.Key("track")).Absolute().Top(0).Bottom(0).Right(4).Width(20).Cursor(ui.CursorPointer).Label(m("turnrail_scroll"))
	th := track.Bounds().H
	frac := min(max(listH/(listH+sc.MaxY), 0.06), 0.64)
	thumbH := th * frac
	thumbTop := (th - thumbH) * min(max(sc.Y/sc.MaxY, 0), 1)
	d := &a.railDrag
	_, dy, held := track.Dragged()
	_, py, _ := track.PointerPosition()
	scrollTo := func(y float32) {
		if room := th - thumbH; room > 0 {
			sc.Y = min(max((y-d.grab)/room, 0), 1) * sc.MaxY
			c.Invalidate()
		}
	}
	switch {
	case held && !d.on:
		d.on, d.y, d.grab = true, py, thumbH/2
		if py >= thumbTop && py <= thumbTop+thumbH {
			d.grab = py - thumbTop
		}
		a.turnNav = turnNav{}
		scrollTo(d.y)
	case held:
		d.y += dy
		scrollTo(d.y)
	case d.on:
		d.on = false
	}
	hover, dragging := track.Hovered(), d.on
	track.Draw(func(pt *ui.Painter, r ui.Rect) {
		w, right, col := float32(3), float32(4.5), p.placeholder.Alpha(0.35)
		switch {
		case dragging:
			w, right, col = 5, 3.5, p.brand.Alpha(0.7)
		case hover:
			w, right, col = 5, 3.5, p.textTertiary.Alpha(0.45)
		}
		pt.Fill(ui.Rect{X: r.X + r.W - right - w, Y: r.Y + thumbTop, W: w, H: thumbH}, col, w/2)
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
func (a *app) turnView(c *ui.Context, sessionID string, t *cohub.Turn, tl *sessionTurns) {
	p := a.pal
	if prompt := strings.TrimSpace(t.Prompt()); prompt != "" {
		ui.Column(c.Key("user")).AlignSelf(ui.End).MaxWidthPercent(100).Gap(4).Children(func() {
			a.userText(c, prompt).Selectable().FontSize(14).LineHeight(1.65).TextColor(p.text).
				Padding(6, 8).Radius(12, 12, 6, 12).Background(p.userBubble)
			a.metaRow(c.Key("user-meta"), func() {
				a.metaCopy(c, prompt)
				a.sentFrom(c, sessionID, t)
				if pr := t.AuthorProfile; pr != nil {
					who := ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						a.avatar(c, pr.AvatarURL, pr.Name(), 14, 7)
						ui.Text(c, pr.Name()).FontSize(11).TextColor(p.metaText())
					})
					if pr.Username != "" {
						who.Tooltip(pr.Name() + " · @" + pr.Username)
					}
				}
				if t.Status == "cancelled" {
					tip := m("chat_not_sent_to_agent")
					if pr := t.AuthorProfile; pr != nil {
						tip = m("chat_cancelled_by", "name", pr.Name())
					}
					ui.Text(c, m("chat_cancelled")).FontSize(11).TextColor(p.metaText()).Tooltip(tip)
				}
			}, t.StartedAt)
		})
	}

	running := t.Running()
	a.processView(c, sessionID, t, tl, running)

	// The final message: its thinking, then the reply, as Markdown.
	reply := t.Reply()
	if st := tl.liveOf(t); running && st != nil {
		if live := cohub.JoinText(st.Current()); live != "" {
			reply = live
		}
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
	// An interrupted turn has its line too, with what it spent.
	if !running && (reply != "" || t.Status == "interrupted") {
		a.replyMeta(c.Key("reply-meta"), t)
	}
}

// sentFrom is the chat a prompt came from, when an agent there sent it, as
// a link to it.
func (a *app) sentFrom(c *ui.Context, sessionID string, t *cohub.Turn) {
	src := t.Meta.RequestSource
	if src == nil || src.SessionID == "" || src.SessionID == sessionID {
		return
	}
	p := a.pal
	label := m("sent_from_chat")
	if s := a.session(src.SessionID); s != nil {
		label = m("sent_from", "title", chatTitle(s))
	}
	b := ui.ButtonBase(c.Key("sent-from")).Row().Gap(4).AlignItems(ui.Center).MaxWidth(192).Shrink(1).MinWidth(0).
		Label(label).Tooltip(label).FocusRing(false)
	hover := b.Hovered()
	b.Children(func() {
		col := p.placeholder.Alpha(0.7)
		if hover {
			col = p.textSecond
		}
		ui.Icon(c, icon("corner-down-left")).Size(12, 12).TextColor(p.brand.Alpha(0.6))
		ui.Text(c, label).FontSize(11).TextColor(col).SingleLine().MinWidth(0)
	})
	if b.Clicked() {
		a.go_(route{kind: routeSession, session: src.SessionID})
	}
}

// metaRow is the line under a message: its actions, then the time on
// the right, in the quiet color of --text-placeholder at half.
func (a *app) metaRow(c *ui.Context, fn func(), at time.Time) {
	ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(0, msgInset).Height(22).Children(func() {
		fn()
		ui.Spacer(c).MinWidth(24)
		if !at.IsZero() {
			ui.Text(c, clock(at, a.now())).FontSize(11).TextColor(a.pal.metaText()).Shrink(0).
				Tooltip(at.Local().Format("2006-01-02 15:04:05"))
		}
	})
}

func (a *app) metaCopy(c *ui.Context, text string) {
	p := a.pal
	b := ui.ButtonBase(c.Key("copy")).Size(22, 22).Center().Radius(4).Label(m("chat_copy_message")).Tooltip(m("chat_copy_message")).FocusRing(false).Children(func() {
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
func (a *app) replyMeta(c *ui.Context, t *cohub.Turn) {
	p := a.pal
	at := t.StartedAt
	if t.CompletedAt != nil {
		at = *t.CompletedAt
	}
	a.metaRow(c, func() {
		a.metaCopy(c, t.Reply())
		if h := t.Meta.Harness; h != "" && h != "cohub" {
			ui.Row(c).Gap(3).AlignItems(ui.Center).Padding(1, 5).Radius(3).Background(p.hoverStrong).Children(func() {
				a.harnessIcon(c, h, 10)
				ui.Text(c, harnessName(h)).FontSize(10).FontWeight(500).TextColor(p.textTertiary)
			})
		} else {
			fb := ui.ButtonBase(c.Key("fork")).Size(22, 22).Center().Radius(4).Label(m("chat_fork_here")).Tooltip(m("chat_fork_here")).FocusRing(false).Children(func() {
				if a.forking == t.ID {
					a.spinner(c, 12, p.metaText())
					return
				}
				ui.Icon(c, icon("git-fork")).Size(14, 14).TextColor(p.metaText())
			})
			if fb.Hovered() {
				fb.Background(p.hover)
			}
			if fb.Clicked() && a.forking == "" {
				a.forkTurn(a.route.sessionID(), t.ID)
			}
		}
		if t.Meta.RuntimeArchiveStatus == "ready" {
			ui.Icon(c, icon("archive")).Size(12, 12).TextColor(p.textTertiary)
		}
		model := ui.Text(c, t.Model).FontSize(11).TextColor(p.metaText()).SingleLine().MinWidth(0)
		if t.Provider != "" {
			model.Tooltip(t.Provider + "/" + t.Model)
		}
		if lv := levelName(t.Meta.RequestedThinkingLevel, t.ThinkingLevel); lv != "" {
			ui.Text(c, lv).FontSize(11).TextColor(p.metaText()).Shrink(0)
		}
		// The tokens and time open the turn's statistics, as the web's.
		if u, d := usageLabel(t.FinalUsage), t.Meta.FinalMessageDurationMs; u != "" || d > 0 {
			b := ui.ButtonBase(c.Key("turn-stats")).Row().Gap(4).AlignItems(ui.Center).Padding(1, 3).Radius(4).Shrink(0).
				Label(m("stats_turn")).FocusRing(false)
			hover := b.Hovered()
			b.Children(func() {
				col := p.metaText()
				if hover {
					col = p.textSecond
				}
				if u != "" {
					ui.Text(c, u).FontSize(11).TextColor(col)
				}
				if d > 0 {
					ui.Text(c, duration(d)).FontSize(11).TextColor(col)
				}
			})
			if hover {
				b.Background(p.hover)
			}
			if b.Clicked() {
				if a.turnStatsFor == t.ID {
					a.turnStatsFor = ""
				} else {
					a.turnStatsFor = t.ID
				}
			}
			if a.turnStatsFor == t.ID {
				a.turnStatsDialog(c, b, t)
			}
		}
	}, at)
}

// processView is the agent's work in a turn: a line that sums it up and
// opens into its steps, each run of tool calls folded into one line.
func (a *app) processView(c *ui.Context, sessionID string, t *cohub.Turn, tl *sessionTurns, running bool) {
	p := a.pal
	sum := t.IntermediateSummary
	steps, tools := 0, 0
	if sum != nil {
		steps, tools = sum.MessageCount, sum.ToolCallCount
	}
	st := tl.liveOf(t)
	var live []cohub.Block
	if running && st != nil {
		live = st.Current()
	}
	if steps == 0 && !running {
		return
	}
	open := tl.expanded[t.ID]
	if steps > 0 || !running {
		parts := []string{}
		if steps > 0 {
			parts = append(parts, i18n.Plural("process_steps", steps))
		}
		if tools > 0 {
			parts = append(parts, i18n.Plural("process_tools", tools))
		}
		if sum != nil {
			if u := usageLabel(sum.Usage); u != "" {
				parts = append(parts, u)
			}
			if cost := sum.Usage.TotalCost(); cost > 0 {
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
		if sum != nil && !open {
			if tip := usageTitle(sum.Usage, sum.DurationMs); tip != "" {
				head.Tooltip(tip)
			}
		}
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
	if running && !open && st != nil {
		// The steps done so far, as the web shows the running turn's.
		if done := st.Done(); len(done) > 0 {
			a.stepEntries(c, t, done)
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
		results := toolResults(live)
		ui.Column(c.Key("live")).Gap(8).Children(func() {
			for i, b := range live {
				if results[b.ID] != nil && b.Meta == nil {
					b.Meta = &cohub.BlockMeta{ToolStatus: "done"}
				}
				switch b.Type {
				case "thinking":
					if strings.TrimSpace(b.Thinking) != "" {
						a.thinking(c, fmt.Sprint("live-think", i), b.Thinking, true)
					}
				case "tool_use", "toolCall":
					a.toolRow(c.Key(i), b, results[b.ID], toolSource{}, true)
				}
			}
		})
	}
}

func (a *app) loadSteps(sessionID string, t *cohub.Turn) {
	if a.api == nil || t.IntermediateIndex == nil || t.IntermediateIndex.MessagesObjectKey == "" {
		a.timelineOf(sessionID).steps[t.ID] = []*cohub.StepMessage{}
		return
	}
	key := t.IntermediateIndex.MessagesObjectKey
	a.once("steps:"+t.ID, func(ctx context.Context) {
		msgs, err := a.api.Steps(ctx, sessionID, t.ID, key)
		a.update(func() {
			if err != nil {
				msgs = []*cohub.StepMessage{}
				a.reload("steps:" + t.ID)
			}
			a.timelineOf(sessionID).steps[t.ID] = msgs
		})
	})
}

// stepEntries shows the steps of a turn as buildProcessEntries groups them:
// a step with text on its own, the steps between in runs.
func (a *app) stepEntries(c *ui.Context, t *cohub.Turn, msgs []*cohub.StepMessage) {
	var run []*cohub.StepMessage
	flush := func(key int) {
		if len(run) > 0 {
			a.stepRun(c.Key(fmt.Sprint("run", key)), t, run)
			run = nil
		}
	}
	ui.Column(c.Key("steps")).Gap(12).Children(func() {
		for i, m := range msgs {
			if m.HasText() || m.StopReason == "error" || m.Meta.MessageKind == "compacted" {
				flush(i)
				a.stepMessage(c.Key(fmt.Sprint("step", i)), t, m)
				continue
			}
			run = append(run, m)
		}
		flush(len(msgs))
	})
}

func toolBlocks(msgs ...*cohub.StepMessage) []cohub.Block {
	var out []cohub.Block
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
func (a *app) stepRun(c *ui.Context, t *cohub.Turn, msgs []*cohub.StepMessage) {
	tools := toolBlocks(msgs...)
	open := a.foldOpen[msgs[0].ID]
	parts := []string{i18n.Plural("process_steps", len(msgs))}
	if len(tools) > 0 {
		parts = append(parts, i18n.Plural("process_tools", len(tools)))
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
	ui.Column(c).Gap(8).Children(func() { a.stepRunBody(c, t, msgs, tools, parts, names, failed, open) })
}

func (a *app) stepRunBody(c *ui.Context, t *cohub.Turn, msgs []*cohub.StepMessage, tools []cohub.Block, parts, names []string, failed int, open bool) {
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
				ui.Column(c.Key(i)).Gap(6).Children(func() { a.stepBody(c, t, m) })
			}
		})
	}
}

// stepMessage is a step with text: its thinking, text, tools and meta.
func (a *app) stepMessage(c *ui.Context, t *cohub.Turn, m *cohub.StepMessage) {
	ui.Column(c).Gap(8).Children(func() {
		a.stepBody(c, t, m)
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

func (a *app) stepBody(c *ui.Context, t *cohub.Turn, m *cohub.StepMessage) {
	p := a.pal
	results := toolResults(m.Content)
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
			a.toolRow(c.Key(i), b, results[b.ID], toolSource{t.ID, m.ToolCallsObjectKey}, false)
		}
	}
	if m.Error != "" {
		ui.Text(c, m.Error).FontSize(12).TextColor(p.errorC).Padding(0, msgInset)
	}
}

// toolRow is one tool call: its name, what it ran on, and how it went. A
// click opens what it got and gave back, as the web's; a call running
// shows it open. result is the call's result, when it came.
func (a *app) toolRow(c *ui.Context, b cohub.Block, result *cohub.Block, src toolSource, live bool) {
	p := a.pal
	status := "done"
	if b.Meta != nil && b.Meta.ToolStatus != "" {
		status = b.Meta.ToolStatus
	}
	if live && (b.Meta == nil || b.Meta.ToolStatus == "") {
		status = "running"
	}
	running := status == "running" || status == "pending"
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
	key := "tool:" + b.ID
	open, chosen := a.foldOpen[key]
	if !chosen {
		open = running
	}
	ui.Column(c).Children(func() {
		head := ui.ButtonBase(c.Key("head")).Row().Gap(8).AlignItems(ui.Center).Justify(ui.Start).MinHeight(28).Padding(4, msgInset).Radius(6).
			Label(b.Name + " " + summary).FocusRing(false)
		if head.Hovered() {
			head.Background(p.hover.Alpha(0.5))
		}
		head.Children(func() {
			col, label := p.success.Alpha(0.75), m("tool_status_done")
			switch {
			case running:
				col, label = p.brand, m("session_activity_running")
			case status == "error" || status == "failed":
				col, label = p.errorC, m("tool_status_failed")
			}
			a.statusDot(c, col, status == "running")
			ui.Text(c, b.Name).FontSize(13).Font(fontMono).TextColor(p.textSecond).Shrink(0).MinWidth(56)
			ui.Text(c, oneLine(summary)).FontSize(13).Font(fontMono).TextColor(p.textTertiary).SingleLine().Grow(1).MinWidth(0)
			st := ui.Text(c, label).FontSize(11).TextColor(p.placeholder).Shrink(0)
			if b.Meta != nil && b.Meta.Timing != nil && b.Meta.Timing.DurationMs > 0 {
				ms := b.Meta.Timing.DurationMs
				st.Tooltip(fmt.Sprintf("%s: %s (%s ms)", m("chat_duration"), duration(ms), groupThousands(strconv.FormatInt(ms, 10))))
			}
			chevron := "chevron-right"
			if open {
				chevron = "chevron-down"
			}
			ui.Icon(c, icon(chevron)).Size(14, 14).TextColor(p.textTertiary)
		})
		if head.Clicked() {
			a.foldOpen[key] = !open
		}
		if open {
			a.toolDetail(c, b, result, src, running)
		}
	})
}

// toolDetail is a tool call opened: its input, the main one first and the
// others by name, then its output, or that it still runs.
func (a *app) toolDetail(c *ui.Context, b cohub.Block, result *cohub.Block, src toolSource, running bool) {
	p := a.pal
	// A finished step's file leaves its results out: its tool calls file
	// has them, and the inputs whole.
	if result != nil && result.Meta != nil && result.Meta.ResultDetail == "omitted" {
		calls, err, ok := a.toolCalls(src)
		var call *cohub.ToolCall
		for _, tc := range calls {
			if tc.ID == b.ID {
				call = tc
			}
		}
		full := cohub.Block{Type: "tool_result", ToolUse: b.ID}
		switch {
		case src.key == "" || ok && call == nil:
			full.Content, _ = json.Marshal(m("tool_result_omitted"))
		case !ok:
			full.Content, _ = json.Marshal(m("tool_loading_details"))
		case err != nil:
			full.Content, full.IsError = json.RawMessage(strconv.Quote(m("tool_load_failed"))), true
		default:
			if len(call.Input) > 0 {
				b.Input = call.Input
			}
			if call.Result != nil {
				full.Content, full.IsError = call.Result.Content, call.Result.IsError
			}
		}
		result = &full
	}
	main, params := toolInputParts(b)
	out, outLabel, outCol := "", m("tool_out"), p.textSecond
	switch {
	case result != nil:
		out = strings.TrimRight(result.ResultText(), "\n")
		if result.IsError {
			outLabel, outCol = m("tool_err"), p.errorC
		}
	case running:
		out = m("tool_running_verb", "verb", m("tool_verb_executing"))
	}
	ui.Column(c.Key("detail")).Gap(8).MarginStart(msgInset+3).Padding(4, 8, 4, 21).BorderStartWidth(1).BorderColor(p.borderSubtle.Alpha(0.7)).Children(func() {
		part := func(label string, body func()) {
			ui.Row(c.Key(label)).Gap(8).AlignItems(ui.Start).Children(func() {
				ui.Text(c, label).Font(fontMono).FontSize(10).TextColor(p.placeholder).Width(28).Shrink(0).Margin(3, 0, 0, 0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(body)
			})
		}
		if main != "" || len(params) > 0 {
			part(m("tool_in"), func() {
				if main != "" {
					ui.Scroll(c.Key("main")).MaxHeight(224).Children(func() {
						ui.Text(c, main).Font(fontMono).FontSize(12).LineHeight(1.375).TextColor(p.textSecond).Selectable()
					})
				}
				for _, kv := range params {
					ui.Row(c.Key(kv.k)).Gap(8).AlignItems(ui.Start).Children(func() {
						ui.Text(c, strings.ToUpper(kv.k)).Font(fontMono).FontSize(10).TextColor(p.placeholder).MinWidth(72).Shrink(0).Margin(1, 0, 0, 0)
						ui.Text(c, kv.v).FontSize(12).TextColor(p.textSecond).Grow(1).MinWidth(0).Selectable()
					})
				}
			})
		}
		if out != "" {
			part(outLabel, func() {
				ui.Scroll(c.Key("out")).MaxHeight(320).Children(func() {
					ui.Text(c, out).Font(fontMono).FontSize(13).LineHeight(1.375).TextColor(outCol).Selectable()
				})
			})
		}
	})
}

// toolSource is where a step's tool calls are whole: its turn, and the
// file of them.
type toolSource struct{ turn, key string }

// toolCalls loads a step's tool calls file once, and reports whether it
// came.
func (a *app) toolCalls(src toolSource) ([]*cohub.ToolCall, error, bool) {
	if src.key == "" {
		return nil, nil, false
	}
	key := "toolcalls:" + src.key
	sessionID := a.route.sessionID()
	a.once(key, func(ctx context.Context) {
		calls, err := a.api.ToolCalls(ctx, sessionID, src.turn, src.key)
		a.update(func() {
			if err != nil {
				a.pages.json[key] = err
				return
			}
			a.pages.json[key] = calls
		})
	})
	switch v := a.pages.json[key].(type) {
	case []*cohub.ToolCall:
		return v, nil, true
	case error:
		return nil, v, true
	}
	return nil, nil, false
}

// toolInputParts splits a call's input into its main value, as the
// command a shell ran, and the others, by name.
func toolInputParts(b cohub.Block) (string, []kv) {
	raw := b.Input
	if len(raw) == 0 {
		raw = b.Args
	}
	var in map[string]any
	if json.Unmarshal(raw, &in) != nil {
		return "", nil
	}
	main, mainKey := "", ""
	for _, k := range []string{"command", "path", "url", "query", "pattern"} {
		if s, ok := in[k].(string); ok {
			main, mainKey = s, k
			break
		}
	}
	keys := make([]string, 0, len(in))
	for k := range in {
		if k != mainKey {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var params []kv
	for _, k := range keys {
		var v string
		switch x := in[k].(type) {
		case string:
			v = x
		case float64:
			v = strconv.FormatFloat(x, 'f', -1, 64)
			if k == "timeout" {
				v += "s"
			}
		default:
			j, _ := json.Marshal(x)
			v = string(j)
		}
		params = append(params, kv{k: k, v: v})
	}
	return main, params
}

// toolResults are the results among blocks, by the call they answer.
func toolResults(blocks []cohub.Block) map[string]*cohub.Block {
	out := map[string]*cohub.Block{}
	for i := range blocks {
		if b := &blocks[i]; (b.Type == "tool_result" || b.Type == "toolResult") && b.ToolUse != "" {
			out[b.ToolUse] = b
		}
	}
	return out
}

func toolInputSummary(b cohub.Block) string {
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
			a.viewportChips(c)
			a.attachmentStrip(c)
			in := ui.TextAreaBase(c.Key("draft"), &a.draft).Lines(2, 10).Placeholder(m("composer_placeholder")).Label(m("x_message")).PlaceholderColor(p.placeholder).
				FontSize(14).LineHeight(1.55).Padding(0, 9)
			if !a.compact && !a.find.open {
				in.AutoFocus()
			}
			if a.focusDraft {
				in.Focus()
				a.focusDraft = false
			}
			if a.draftCaret >= 0 {
				in.SetTextSelection(a.draftCaret, a.draftCaret)
				a.draftCaret = -1
			}
			menuKey := a.composerMenus(c, in)
			in.OnTextKey(func(k ui.TextKey) bool {
				if menuKey != nil && menuKey(k) {
					return true
				}
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
				a.harnessPicker(c)
				if a.harnessChoice().harness == "cohub" {
					a.modelPicker(c)
				}
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
				// While a turn runs, a message is a follow-up it sends; with
				// none, the button stops the turn.
				stop := running && empty
				send := ui.ButtonBase(c.Key("send")).Size(36, 36).Radius(18).Center().FocusRing(false).Children(func() {
					switch {
					case stop:
						ui.Box(c).Size(12, 12).Radius(2).Background(p.bgPrimary)
					case empty || a.sending:
						ui.Icon(c, icon("arrow-up")).Size(16, 16).TextColor(p.placeholder.Alpha(0.6))
					default:
						ui.Icon(c, icon("arrow-up")).Size(16, 16).TextColor(p.bgPrimary)
					}
				})
				switch {
				case stop:
					send.Background(p.text).Label(m("board_session_stop"))
				case empty || a.sending:
					send.Background(p.hover).Label(m("feedback_send"))
				default:
					send.Background(p.text).Label(m("feedback_send"))
				}
				if send.Clicked() {
					if stop {
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
func (a *app) composerModel() cohub.ModelChoice {
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
			return cohub.ModelChoice{Provider: t.Provider, ID: t.Model, Level: lv}
		}
	}
	for _, md := range a.models {
		if !md.Model.Hidden {
			return cohub.ModelChoice{Provider: md.Provider, ID: md.ID}
		}
	}
	return cohub.ModelChoice{}
}

// modelPicker is the composer's model chip, which opens the model
// selector, as ModelSelector.svelte.
func (a *app) modelPicker(c *ui.Context) {
	p := a.pal
	cur := a.composerModel()
	label := cur.ID
	if label == "" {
		label = m("model_default")
	}
	b := a.pill(c.Key("model"), label, levelName(cur.Level, ""), nil).Label(m("model_choose"))
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
				models := append([]*cohub.Model(nil), a.models...)
				sort.SliceStable(models, func(i, j int) bool { return models[i].ID == cur.ID && models[j].ID != cur.ID })
				for _, mod := range models {
					if mod.Model.Hidden && mod.ID != cur.ID || q != "" && !strings.Contains(strings.ToLower(mod.ID), strings.ToLower(q)) {
						continue
					}
					n++
					on := mod.ID == cur.ID
					row := ui.Row(c.Key(mod.ID)).Gap(12).AlignItems(ui.Start).Padding(8, 16).Children(func() {
						if on {
							ui.Box(c).Absolute().Left(0).Top(14).Size(2, 16).Radius(1).Background(p.brand)
						}
						ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Text(c, mod.ID).FontSize(13).FontWeight(500).SingleLine()
								if mod.Vision() {
									ui.Icon(c, icon("image")).Size(14, 14).TextColor(p.textTertiary)
								}
								a.dot(c, p.success, 6)
							})
							if cost := modelCostLabel(mod); cost != "" {
								ui.Text(c, cost).FontSize(11).TextColor(p.textTertiary.Alpha(0.75)).SingleLine()
							}
						})
						ui.Column(c).AlignItems(ui.End).Gap(2).Shrink(0).Children(func() {
							ui.Text(c, mod.Provider).FontSize(11).TextColor(p.textTertiary.Alpha(0.7))
							if lv := mod.Levels(); len(lv) > 1 {
								level := mod.Model.DefaultLevel
								if level == "" {
									level = "high"
								}
								if on && cur.Level != "" {
									level = cur.Level
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
						lv := cur.Level
						if !contains(mod.Levels(), lv) {
							lv = ""
						}
						a.chooseModel(cohub.ModelChoice{Provider: mod.Provider, ID: mod.ID, Level: lv})
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

func (a *app) chooseModel(m cohub.ModelChoice) {
	key := a.route.sessionID()
	if key == "" {
		key = "space:" + a.currentSpace()
	}
	a.modelChoices[key] = m
}

func (a *app) model(id string) *cohub.Model {
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
	// The web's marks: Pi's and Cursor's follow the text's color.
	if h == "claude-code" {
		h = "claude"
	}
	if _, ok := icons["harness-"+h]; !ok {
		ui.Icon(c, icon("monitor")).Size(size, size).TextColor(a.pal.textTertiary)
		return
	}
	if h == "pi" || h == "cursor" {
		ui.Icon(c, icon("harness-"+h)).Size(size, size).Shrink(0).TextColor(a.pal.textSecond)
		return
	}
	ui.Image(c, colorIcon("harness-"+h, size)).Size(size, size).Shrink(0).Radius(3)
}

// colorIcon is an SVG of its own colors, drawn at 3x for sharpness.
var colorIcons = map[string]*ui.Bitmap{}

func colorIcon(name string, size float32) *ui.Bitmap {
	k := name + "@" + strconv.Itoa(int(size))
	if b, ok := colorIcons[k]; ok {
		return b
	}
	px := int(size * 3)
	b := ui.NewBitmap(icon(name).Image(px, px, nil))
	colorIcons[k] = b
	return b
}

func harnessName(h string) string {
	switch h {
	case "pi":
		return "Pi"
	case "claude-code", "claude":
		return "Claude Code"
	case "workbuddy":
		return "WorkBuddy"
	case "cohub":
		return "Cohub"
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
func usageLabel(u *cohub.Usage) string {
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

// usageTitle is the steps line's tooltip, as the web's: the tokens in and
// out, the cache written, the total, the cost and how long they took.
func usageTitle(u *cohub.Usage, ms int64) string {
	var parts []string
	if u != nil {
		if in := u.Input + u.CacheRead + u.CacheWrite; in > 0 {
			s := kilo(in)
			if u.CacheRead > 0 {
				s += " (" + m("chat_cached", "count", kilo(u.CacheRead)) + ")"
			}
			parts = append(parts, m("chat_input_label", "value", s))
		}
		if u.Output > 0 {
			parts = append(parts, m("chat_output_label", "value", kilo(u.Output)))
		}
		if u.CacheWrite > 0 {
			parts = append(parts, m("chat_cache_write_label", "value", kilo(u.CacheWrite)))
		}
		total := u.TotalTokens
		if total == 0 {
			total = u.Input + u.Output + u.CacheRead + u.CacheWrite
		}
		if total > 0 {
			parts = append(parts, m("chat_total_label", "value", kilo(total)))
		}
		if cost := u.TotalCost(); cost > 0 {
			parts = append(parts, m("chat_cost_label", "value", fmt.Sprintf("US$%.3f", cost)))
		}
	}
	if ms > 0 {
		parts = append(parts, fmt.Sprintf("%s: %s (%s ms)", m("chat_duration"), duration(ms), groupThousands(strconv.FormatInt(ms, 10))))
	}
	return strings.Join(parts, " · ")
}

// forkTurn starts a chat that goes on from a turn, and opens it.
func (a *app) forkTurn(sessionID, turnID string) {
	if sessionID == "" {
		return
	}
	a.forking = turnID
	inSpace := a.route.area() == areaSpace
	a.act(func(ctx context.Context) error {
		var out struct {
			Session *cohub.Session `json:"session"`
		}
		err := a.api.Do(ctx, "POST", "/api/sessions/"+url.PathEscape(sessionID)+"/turns/"+url.PathEscape(turnID)+"/fork", map[string]any{}, &out)
		a.update(func() {
			a.forking = ""
			if err != nil || out.Session == nil {
				return
			}
			s := out.Session
			a.sessions = append([]*cohub.Session{s}, a.sessions...)
			if inSpace {
				a.go_(route{kind: routeSpaceSession, space: s.SpaceID, session: s.ID})
			} else {
				a.go_(route{kind: routeSession, session: s.ID})
			}
		})
		if err == nil {
			a.refresh()
		}
		return err
	})
}

// modelCostLabel is formatModelCost: "US$4/M 输入 · US$20/M 输出 · …".
func modelCostLabel(m_ *cohub.Model) string {
	c := m_.Model.Cost
	var parts []string
	for _, x := range []struct {
		v   float64
		key string
	}{{c.Input, "model_cost_input"}, {c.Output, "model_cost_output"}, {c.CacheRead, "model_cost_cache_read"}, {c.CacheWrite, "model_cost_cache_write"}} {
		if x.v == 0 {
			continue
		}
		var s string
		switch {
		case x.v < 0.01:
			s = fmt.Sprintf("US$%.4f", x.v)
		case x.v < 1:
			s = fmt.Sprintf("US$%.2f", x.v)
		case x.v == float64(int64(x.v)):
			s = fmt.Sprintf("US$%d", int64(x.v))
		default:
			s = fmt.Sprintf("US$%.2f", x.v)
		}
		parts = append(parts, s+"/M "+m(x.key))
	}
	return strings.Join(parts, " · ")
}
