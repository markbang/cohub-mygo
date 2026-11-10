package app

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"cohub-mygo/internal/cohub"
)

// turnNav is the turn rail's navigator, TurnNavigator.svelte: it opens
// beside the mark the pointer is on, and closes 180ms after the pointer
// leaves the rail and it, as the web's.
type turnNav struct {
	open     bool
	turn     string // the mark it is beside
	scrolled string // the turn its list last scrolled to
	query    string
	leftAt   time.Time
}

const turnNavGrace = 180 * time.Millisecond

// hold keeps the navigator open while the pointer is on the rail or on it.
func (n *turnNav) hold(c *ui.Context, hovered bool) {
	if !n.open {
		return
	}
	now := c.Now()
	switch {
	case hovered:
		n.leftAt = time.Time{}
	case n.leftAt.IsZero():
		n.leftAt = now
		c.After(turnNavGrace)
	case now.Sub(n.leftAt) >= turnNavGrace:
		*n = turnNav{}
	default:
		c.After(turnNavGrace - now.Sub(n.leftAt))
	}
}

// show opens the navigator beside the mark of turn, with a new search.
func (n *turnNav) show(turn string) {
	if !n.open {
		*n = turnNav{open: true}
	}
	n.turn, n.leftAt = turn, time.Time{}
}

// turnNavigator lists the turns beside the rail, at at, the hovered one
// marked: a click jumps to a turn.
func (a *app) turnNavigator(c *ui.Context, turns []*cohub.Turn, at, railH float32) {
	p := a.pal
	nav := &a.turnNav
	maxH := max(min(680, a.height*0.72, railH-16), 120)
	panel := ui.Column(c.Key("turn-nav")).Absolute().Right(32).Width(min(420, a.width-72)).MaxHeight(maxH).
		Radius(8).Border(1, p.borderSubtle).Background(p.bgPrimary).Shadow(0, 20, 25, -5, p.bgPrimary.Alpha(0.2)).Clip().
		Label(m("navigator_turns_aria"))
	// Its height is the last frame's: the first frame guesses, and asks for
	// the next.
	h := panel.Bounds().H
	if h == 0 {
		h = min(maxH, 160)
		c.Invalidate()
	}
	panel.Top(min(max(at-h*0.34, 8), max(railH-h-8, 8)))
	authors := multiAuthor(turns)
	panel.Children(func() {
		ui.Row(c.Key("search")).Shrink(0).Padding(8).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle.Alpha(0.7)).Children(func() {
			field := ui.Row(c).Grow(1).Height(32).Gap(8).AlignItems(ui.Center).Padding(0, 8).Radius(6).Border(1, p.borderSubtle).Background(p.bgInput)
			var focused bool
			field.Children(func() {
				ui.Icon(c, icon("search")).Size(14, 14).TextColor(p.placeholder)
				in := ui.TextInputBase(c.Key("q"), &nav.query).Placeholder(m("navigator_search")).PlaceholderColor(p.placeholder).
					FontSize(12).Grow(1).Label(m("navigator_search"))
				in.OnShortcut(0, ui.KeyEscape, func() { a.turnNav = turnNav{} })
				focused = in.Focused()
			})
			if focused {
				field.BorderColor(p.borderStrong)
			}
		})
		shown := filterTurns(turns, nav.query, authors)
		if len(shown) == 0 {
			ui.Column(c.Key("none")).Height(112).Center().Children(func() {
				ui.Text(c, m("navigator_no_turns")).FontSize(12).TextColor(p.placeholder)
			})
			return
		}
		ui.Scroll(c.Key("turns")).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Padding(8).Gap(2).Children(func() {
				for _, t := range shown {
					a.turnNavItem(c, t, authors)
				}
			})
		})
	})
}

// turnNavItem is a turn in the navigator: its number, its message in three
// lines at most, and when it started.
func (a *app) turnNavItem(c *ui.Context, t *cohub.Turn, authors bool) {
	p := a.pal
	nav := &a.turnNav
	on := t.ID == nav.turn
	text := turnNavText(t)
	b := ui.ButtonBase(c.Key(t.ID)).Row().Gap(4).AlignItems(ui.Start).Justify(ui.Start).Padding(6, 8).Radius(6).FocusRing(false).Label("#" + strconv.Itoa(t.Sequence))
	hover := b.Hovered()
	num, body, meta := p.placeholder, p.textSecond, p.placeholder
	switch {
	case on:
		num, body, meta = p.brand, p.text, p.textTertiary
		b.Background(p.active)
	case hover:
		num, body, meta = p.textTertiary, p.text, p.textTertiary
		b.Background(p.hover)
	}
	b.Children(func() {
		ui.Text(c, "#"+strconv.Itoa(t.Sequence)).Font(fontMono).FontSize(11).LineHeight(1.625).TextColor(num).Width(26).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			if text == "" {
				ui.Text(c, m("navigator_empty_msg")).FontSize(12).LineHeight(1.625).TextColor(p.placeholder)
			} else {
				ui.Text(c, text).FontSize(12).LineHeight(1.625).TextColor(body).MaxLines(3)
			}
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				if !t.StartedAt.IsZero() {
					ui.Text(c, clock(t.StartedAt, a.now())).FontSize(10).TextColor(meta).Shrink(0).
						Tooltip(t.StartedAt.Local().Format("2006-01-02 15:04:05"))
				}
				if name := t.AuthorProfile.Name(); authors && name != "" {
					if !t.StartedAt.IsZero() {
						ui.Text(c, "·").FontSize(10).TextColor(meta).Opacity(0.7)
					}
					ui.Text(c, name).FontSize(10).TextColor(meta).SingleLine().MinWidth(0)
				}
			})
		})
	})
	if on && nav.scrolled != t.ID {
		nav.scrolled = t.ID
		b.ScrollIntoView()
	}
	if b.Clicked() {
		a.jumpToTurn(t.ID)
	}
}

// jumpToTurn scrolls the timeline to a turn's start and closes the
// navigator.
func (a *app) jumpToTurn(id string) {
	a.turnNav = turnNav{}
	tl := a.timelineOf(a.route.sessionID())
	for i, t := range tl.turns {
		if t.ID == id {
			a.timeline.ScrollTo(i, ui.Start)
			return
		}
	}
}

// turnNavText is a turn's message on one line, cut at 180 characters.
func turnNavText(t *cohub.Turn) string {
	s := []rune(oneLine(t.Prompt()))
	if len(s) <= 180 {
		return string(s)
	}
	return strings.TrimRight(string(s[:179]), " ") + "…"
}

// multiAuthor tells whether more than one person wrote the turns, when the
// navigator names who wrote each.
func multiAuthor(turns []*cohub.Turn) bool {
	first := ""
	for _, t := range turns {
		u := strings.TrimSpace(t.UserUUID)
		switch {
		case u == "":
		case first == "":
			first = u
		case u != first:
			return true
		}
	}
	return false
}

// filterTurns keeps the turns a search finds by number, message or author.
func filterTurns(turns []*cohub.Turn, query string, authors bool) []*cohub.Turn {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return turns
	}
	var out []*cohub.Turn
	for _, t := range turns {
		seq := strconv.Itoa(t.Sequence)
		if strings.Contains("#"+seq, q) || strings.Contains(strings.ToLower(turnNavText(t)), q) ||
			authors && strings.Contains(strings.ToLower(t.AuthorProfile.Name()), q) {
			out = append(out, t)
		}
	}
	return out
}

// timelinePill floats over the compact timeline's bottom, as the web's:
// to the latest when scrolled up, and the turn list.
func (a *app) timelinePill(c *ui.Context, turns []*cohub.Turn) {
	down := len(turns) > 0 && !a.timeline.AtEnd()
	list := len(turns) > 1
	if !down && !list {
		return
	}
	p := a.pal
	ui.Row(c.Key("pill")).Attach(ui.AnchorBottom, ui.AnchorBottom).Bottom(12).Gap(2).Padding(4).Radius(18).
		Background(p.bgPrimary.Alpha(0.95)).Border(1, p.borderSubtle.Alpha(0.8)).Shadow(0, 4, 18, 0, ui.RGBA(0, 0, 0, 0.16)).Children(func() {
		btn := func(key, ic, label string) bool {
			b := ui.ButtonBase(c.Key(key)).Size(28, 28).Radius(14).Center().Label(label).FocusRing(false).Children(func() {
				ui.Icon(c, icon(ic)).Size(16, 16).TextColor(p.textSecond)
			})
			if b.Pressed() || b.Hovered() {
				b.Background(p.hover)
			}
			return b.Clicked()
		}
		if down && btn("to-end", "arrow-down", m("x_to_latest")) {
			a.timeline.ScrollToEnd()
		}
		if list && btn("turns", "list-tree", m("chat_open_turn_list")) {
			a.turnSheetOpen = true
		}
	})
	if a.turnSheetOpen {
		a.turnSheet(c, turns)
	}
}

// turnSheet is the compact shell's turn list, as the web's bottom sheet:
// a turn's message and how it went; a tap jumps there.
func (a *app) turnSheet(c *ui.Context, turns []*cohub.Turn) {
	p := a.pal
	first, _ := a.timeline.Visible()
	ui.Overlay(c, func() {
		ui.Box(c.Key("scrim")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(p.overlay()).
			Label(m("bottom_sheet_close")).OnClick(func() { a.turnSheetOpen = false })
		ui.Column(c.Key("sheet")).Absolute().Left(0).Right(0).Bottom(0).MaxHeight(a.height*0.72).Radius(16, 16, 0, 0).
			Background(p.bgPrimary).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Shadow(0, -12, 36, 0, ui.RGBA(0, 0, 0, 0.28)).
			Label(m("bottom_sheet_aria")).Children(func() {
			ui.Box(c).Size(36, 4).Radius(2).Background(p.borderSubtle).Margin(10, ui.Auto, 0, ui.Auto).Shrink(0)
			ui.Scroll(c.Key("turns")).Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).Padding(8, 0).Children(func() {
					for i, t := range turns {
						a.turnSheetRow(c.Key(t.ID), t, i == first)
					}
				})
			})
		})
	})
}

func (a *app) turnSheetRow(c *ui.Context, t *cohub.Turn, on bool) {
	p := a.pal
	text := turnNavText(t)
	statusCol := p.textTertiary
	switch t.Status {
	case "failed":
		statusCol = p.errorC
	case "running":
		statusCol = p.info
	case "interrupted":
		statusCol = p.warning
	}
	b := ui.ButtonBase(c).Row().Gap(12).AlignItems(ui.Start).Justify(ui.Start).Padding(12, 16).FocusRing(false).Label("#" + strconv.Itoa(t.Sequence))
	switch {
	case on:
		b.Background(p.brand.Alpha(0.1))
	case b.Pressed():
		b.Background(p.hover)
	}
	b.Children(func() {
		num := p.textTertiary
		if on {
			num = p.brand
		}
		ui.Text(c, "#"+strconv.Itoa(t.Sequence)).FontSize(11).FontWeight(500).TextColor(num).Width(40).Shrink(0).Margin(2, 0, 0, 0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			if text == "" {
				ui.Text(c, m("navigator_empty_msg")).FontSize(13).LineHeight(1.625).TextColor(p.placeholder)
			} else {
				ui.Text(c, text).FontSize(13).LineHeight(1.625).TextColor(p.text).MaxLines(3)
			}
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				if !t.StartedAt.IsZero() {
					ui.Text(c, clock(t.StartedAt, a.now())+" ·").FontSize(11).TextColor(p.placeholder).Shrink(0)
				}
				status := t.Status
				if t.Model != "" {
					status += " · " + t.Model
				}
				ui.Text(c, status).FontSize(11).TextColor(statusCol).SingleLine().MinWidth(0)
			})
		})
	})
	id := t.ID
	if b.Clicked() {
		a.turnSheetOpen = false
		a.jumpToTurn(id)
	}
}
