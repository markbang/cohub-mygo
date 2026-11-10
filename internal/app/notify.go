package app

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Turn notifications, as TurnNotificationStack and turn-notifications:
// when a chat the user is not looking at finishes a turn, a card stacks
// in the window's corner, and the system shows one too while the window
// is in the background.

type turnNotice struct {
	SpaceID     string  `json:"spaceId"`
	SessionID   string  `json:"sessionId"`
	TurnID      string  `json:"turnId"`
	Status      string  `json:"status"`
	UserPreview *string `json:"userPreview"`
	DurationMs  *int64  `json:"durationMs"`
	StepCount   *int    `json:"stepCount"`
	at          time.Time
}

func noticeStatus(s string) string {
	switch s {
	case "completed", "failed", "interrupted", "cancelled", "merged":
		return m("notif_status_" + s)
	}
	return m("notif_status_finished")
}

func noticeDuration(ms *int64) string {
	if ms == nil || *ms < 0 {
		return ""
	}
	d := *ms
	switch {
	case d < 1000:
		return m("notif_dur_lt_1s")
	case d < 60_000:
		return m("notif_dur_s", "n", math.Round(float64(d)/100)/10)
	}
	mi, s := d/60_000, int64(math.Round(float64(d%60_000)/1000))
	if s > 0 {
		return m("notif_dur_m_s", "m", mi, "s", s)
	}
	return m("notif_dur_m", "n", mi)
}

func (n *turnNotice) meta() string {
	parts := []string{noticeStatus(n.Status)}
	if d := noticeDuration(n.DurationMs); d != "" {
		parts = append(parts, d)
	}
	if n.StepCount != nil && *n.StepCount > 0 {
		k := "notif_steps_many"
		if *n.StepCount == 1 {
			k = "notif_steps_one"
		}
		parts = append(parts, m(k, "n", *n.StepCount))
	}
	return strings.Join(parts, " · ")
}

// turnNotified takes a session.turn.notify event.
func (a *app) turnNotified(payload json.RawMessage) {
	var n turnNotice
	if json.Unmarshal(payload, &n) != nil || n.SessionID == "" {
		return
	}
	n.at = time.Now()
	a.update(func() {
		focused := a.win == nil || a.win.IsFocused()
		if focused && a.route.sessionID() == n.SessionID {
			return // the user is looking at it
		}
		for _, x := range a.notices {
			if x.TurnID == n.TurnID {
				return
			}
		}
		a.notices = append(a.notices, &n)
		if len(a.notices) > 3 {
			a.notices = a.notices[len(a.notices)-3:]
		}
		if !focused && a.win != nil {
			title := m("notif_turn_completed")
			if s := a.session(n.SessionID); s != nil {
				title = chatTitle(s)
			}
			body := n.meta()
			if n.UserPreview != nil && *n.UserPreview != "" {
				body = a.previewOf(*n.UserPreview) + "\n" + body
			}
			nt := mygo.NewNotification(mygo.NotificationOptions{Title: title, Body: body})
			sid, space := n.SessionID, n.SpaceID
			nt.OnClick(func() {
				a.win.Focus()
				a.update(func() { a.go_(route{kind: routeSpaceSession, space: space, session: sid}) })
			})
			go nt.Show()
		}
	})
}

// noticesView is the stack of finished turns in the window's corner.
func (a *app) noticesView(c *ui.Context) {
	var live []*turnNotice
	for _, n := range a.notices {
		if time.Since(n.at) < 12*time.Second {
			live = append(live, n)
		}
	}
	a.notices = live
	if len(live) == 0 {
		return
	}
	c.After(time.Second)
	p := a.pal
	ui.Column(c.Key("notices")).Attach(ui.AnchorTopRight, ui.AnchorTopRight).Top(52).Right(16).Width(340).Gap(8).Children(func() {
		for _, n := range live {
			n := n
			card := ui.Row(c.Key(n.TurnID)).Gap(10).AlignItems(ui.Start).Padding(10, 12).Radius(10).Background(p.bgElevated).
				Border(1, p.borderSubtle).Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, 0.12)).Cursor(ui.CursorPointer)
			card.Children(func() {
				name, av := a.spaceName(n.SpaceID)
				a.avatarSeed(c, av, name, n.SpaceID, 20, 6)
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					title := m("notif_turn_completed")
					if s := a.session(n.SessionID); s != nil {
						title = chatTitle(s)
					}
					ui.Text(c, title).FontSize(13).FontWeight(500).SingleLine()
					if n.UserPreview != nil && *n.UserPreview != "" {
						ui.Text(c, a.previewOf(*n.UserPreview)).FontSize(12).TextColor(p.textTertiary).SingleLine()
					}
					col := p.textTertiary
					if n.Status == "failed" {
						col = p.errorC
					}
					ui.Text(c, fmt.Sprint(name, " · ", n.meta())).FontSize(11).TextColor(col).SingleLine()
				})
				x := ui.ButtonBase(c.Key("x")).Size(20, 20).Center().Radius(4).Label(m("notif_dismiss")).FocusRing(false).Children(func() {
					ui.Icon(c, icon("x")).Size(12, 12).TextColor(p.textTertiary)
				})
				if x.Clicked() {
					n.at = time.Time{}
				}
			})
			if card.Clicked() {
				n.at = time.Time{}
				a.go_(route{kind: routeSpaceSession, space: n.SpaceID, session: n.SessionID})
			}
		}
	})
}
