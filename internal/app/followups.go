package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"cohub-mygo/internal/cohub"
)

// shown are the turns the timeline shows: the loaded ones but the
// follow-ups waiting, which the composer lists, then the messages sent that
// no loaded turn holds yet.
func (tl *sessionTurns) shown() []*cohub.Turn {
	out := make([]*cohub.Turn, 0, len(tl.turns)+len(tl.pending))
	for _, t := range tl.turns {
		if t.Queued() && (tl.stream == nil || tl.stream.Turn() != t.ID) {
			continue
		}
		out = append(out, t)
	}
	for _, t := range tl.pending {
		if !t.Queued() {
			out = append(out, t)
		}
	}
	return out
}

// queued are the follow-ups waiting for the turn running.
func (tl *sessionTurns) queued() []*cohub.Turn {
	var out []*cohub.Turn
	for _, t := range append(append([]*cohub.Turn(nil), tl.turns...), tl.pending...) {
		if t.Queued() && (tl.stream == nil || tl.stream.Turn() != t.ID) {
			out = append(out, t)
		}
	}
	return out
}

// liveOf is the stream of t while it is the turn being written.
func (tl *sessionTurns) liveOf(t *cohub.Turn) *cohub.Stream {
	if st := tl.stream; st != nil && (st.Turn() == "" || st.Turn() == t.ID) {
		return st
	}
	return nil
}

// settlePending drops the messages sent that a loaded turn now holds.
func (tl *sessionTurns) settlePending() {
	kept := tl.pending[:0]
	for _, p := range tl.pending {
		found := false
		for _, t := range tl.turns {
			if t.Meta.ClientMessageID == p.Meta.ClientMessageID {
				found = true
				break
			}
		}
		if !found {
			kept = append(kept, p)
		}
	}
	tl.pending = kept
}

// dropPending forgets a message sent that the API refused.
func (tl *sessionTurns) dropPending(clientMessageID string) {
	for i, p := range tl.pending {
		if p.Meta.ClientMessageID == clientMessageID {
			tl.pending = append(tl.pending[:i], tl.pending[i+1:]...)
			return
		}
	}
}

// pendingTurn is a message sent, shown until its turn loads: queued behind
// the turn running, or running.
func (a *app) pendingTurn(tl *sessionTurns, text, clientMessageID string, queued bool) *cohub.Turn {
	t := &cohub.Turn{
		ID: "local:" + clientMessageID, Status: "running", Intent: "followup", UserText: text,
		StartedAt: a.now(), Meta: cohub.TurnMeta{ClientMessageID: clientMessageID},
	}
	if queued {
		t.Status = "queued"
	}
	if text != "" {
		t.UserContent = []cohub.Block{{Type: "text", Text: text}}
	}
	if a.me != nil {
		t.AuthorProfile = a.me.Profile
	}
	if n := len(tl.turns); n > 0 {
		t.Sequence = tl.turns[n-1].Sequence + 1
	}
	return t
}

// followups is the queue above the composer, as the web's: the follow-ups
// waiting for the turn running, each to steer now, which interrupts the
// turn for it, or to cancel.
func (a *app) followups(c *ui.Context, s *cohub.Session, tl *sessionTurns) {
	queued := tl.queued()
	if len(queued) == 0 {
		return
	}
	p := a.pal
	pad := float32(24)
	if a.compact {
		pad = 16
	}
	ui.Column(c.Key("followups")).Shrink(0).Padding(0, pad).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(chatMaxWidth).Margin(0, ui.Auto).Padding(8, msgInset).Gap(4).
			BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle.Alpha(0.7)).Background(p.bgContent).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, m("chat_followup")).FontSize(11).FontWeight(500).TextColor(p.textSecond)
				ui.Text(c, m("chat_queued", "count", len(queued))).FontSize(11).TextColor(p.placeholder)
			})
			ui.Scroll(c.Key("list")).MaxHeight(min(192, a.height*0.28)).Children(func() {
				ui.Column(c).Gap(4).PaddingEnd(4).Children(func() {
					for _, t := range queued {
						a.followupRow(c.Key(t.ID), s, t)
					}
				})
			})
		})
	})
}

func (a *app) followupRow(c *ui.Context, s *cohub.Session, t *cohub.Turn) {
	p := a.pal
	// A message on its way has no turn to act on yet.
	busy := a.followupBusy[t.ID] || strings.HasPrefix(t.ID, "local:")
	row := ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(6, 8).Radius(8)
	if row.Hovered() {
		row.Background(p.hover.Alpha(0.6))
	}
	row.Children(func() {
		text := oneLine(t.Prompt())
		if text == "" {
			text = m("chat_followup")
		}
		ui.Text(c, text).FontSize(12).TextColor(p.textTertiary).SingleLine().Grow(1).MinWidth(0)
		action := func(key, label string, col, hoverCol ui.Color) bool {
			b := ui.ButtonBase(c.Key(key)).Padding(4, 6).Radius(4).Label(label).FocusRing(false).Disabled(busy)
			hover := b.Hovered()
			b.Children(func() {
				if hover {
					col = hoverCol
				}
				ui.Text(c, label).FontSize(12).TextColor(col).SingleLine()
			})
			if busy {
				b.Opacity(0.5)
			} else if hover {
				b.Background(p.bgSurface)
			}
			return b.Clicked()
		}
		if action("steer", m("chat_steer_now"), p.textSecond, p.text) {
			a.followupAction(s, t.ID, true)
		}
		if action("cancel", m("common_cancel"), p.placeholder, p.textSecond) {
			a.followupAction(s, t.ID, false)
		}
	})
}

// followupAction steers a follow-up, or cancels it. A follow-up that ran or
// went meanwhile (409) only reloads the chat.
func (a *app) followupAction(s *cohub.Session, turnID string, steer bool) {
	if a.api == nil {
		a.sendErr = m("x_demo_no_send")
		return
	}
	if a.followupBusy[turnID] {
		return
	}
	a.followupBusy[turnID] = true
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var err error
		if steer {
			err = a.api.SteerTurn(ctx, s.SpaceID, s.ID, turnID)
		} else {
			err = a.api.CancelTurn(ctx, s.SpaceID, s.ID, turnID)
		}
		var se *cohub.StatusError
		if errors.As(err, &se) && se.Status == http.StatusConflict {
			err = nil
		}
		a.loadTurns(s.ID)
		a.update(func() {
			delete(a.followupBusy, turnID)
			if err != nil {
				a.toast, a.toastAt, a.toastOK = err.Error(), time.Now(), false
			}
		})
	})
}
