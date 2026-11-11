package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"cohub-mygo/internal/cohub"
)

// findState is the chat's find bar: its query, and the turns it found,
// which the bar walks a turn at a time.
type findState struct {
	open  bool
	query string
	match int
	key   string
	turns []int
}

// findBar shows the bar above the timeline while it is open, and scrolls
// to the turn the query moves to.
func (a *app) findBar(c *ui.Context, turns []*cohub.Turn) {
	if !a.find.open {
		return
	}
	matches := a.findIn(turns)
	before := a.find.match
	ui.FindBar(c, &a.find.open, &a.find.query, len(matches), &a.find.match)
	if !a.find.open {
		a.focusDraft = true
		return
	}
	if a.find.match != before && len(matches) > 0 {
		a.timeline.ScrollTo(matches[a.find.match], ui.Start)
	}
}

// findIn counts the loaded turns holding the query, and starts over when
// the query, the chat or its turns change.
func (a *app) findIn(turns []*cohub.Turn) []int {
	last := 0
	if len(turns) > 0 {
		last = len(turns[len(turns)-1].Reply())
	}
	key := fmt.Sprint(a.route.sessionID(), "\x00", len(turns), "\x00", last, "\x00", a.find.query)
	if key == a.find.key {
		return a.find.turns
	}
	a.find.key, a.find.match, a.find.turns = key, 0, nil
	q := strings.ToLower(strings.TrimSpace(a.find.query))
	if q == "" {
		return nil
	}
	for i, t := range turns {
		text := t.Reply()
		for _, b := range t.UserContent {
			text += "\n" + b.Text
		}
		if strings.Contains(strings.ToLower(text), q) {
			a.find.turns = append(a.find.turns, i)
		}
	}
	return a.find.turns
}
