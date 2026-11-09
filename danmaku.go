package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Danmaku floats what others write in the Space's other chats across the
// workspace, as SpaceDanmakuLayer: a pill of the author and the text,
// flying right to left in one of ten lanes, which opens the message.

type danmakuItem struct {
	id, text, sessionID, author, avatar, seed string
	sequence                                  int
	lane                                      int
	at                                        time.Time
	dur                                       time.Duration
	width                                     float32
}

type danmakuState struct {
	items     []*danmakuItem
	laneFree  [10]time.Time
	lastShown map[string]time.Time
	seen      map[string]bool
}

const (
	danmakuLanes    = 10
	danmakuDur      = 9 * time.Second
	danmakuMax      = 24
	danmakuGap      = 180 * time.Millisecond
	danmakuThrottle = 1500 * time.Millisecond
)

var bareURL = regexp.MustCompile(`^https?://\S+$`)

// danmakuText is a turn's text to float, as extractDanmakuText.
func danmakuText(t *Turn) string {
	for _, b := range t.UserContent {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			return strings.TrimSpace(b.Text)
		}
	}
	for _, b := range t.UserContent {
		switch b.Type {
		case "image":
			return "Sent an image"
		case "shell_command":
			return "Sent a command"
		}
	}
	s := strings.TrimSpace(t.UserText)
	if bareURL.MatchString(s) {
		return "Sent a link"
	}
	return s
}

// danmakuTurn takes a session.turn.created of the Space open.
func (a *app) danmakuTurn(spaceID, sessionID string, payload json.RawMessage) {
	var p struct {
		Turn *Turn `json:"turn"`
	}
	if json.Unmarshal(payload, &p) != nil || p.Turn == nil || !a.prefs.danmaku() {
		return
	}
	t := p.Turn
	if spaceID != a.currentSpace() || sessionID == a.route.sessionID() || t.UserUUID == "" || a.me == nil || t.UserUUID == a.me.UUID {
		return
	}
	d := &a.danmaku
	if d.seen == nil {
		d.seen, d.lastShown = map[string]bool{}, map[string]time.Time{}
	}
	if d.seen[t.ID] {
		return
	}
	d.seen[t.ID] = true
	text := strings.Join(strings.Fields(danmakuText(t)), " ")
	if text == "" {
		return
	}
	if r := []rune(text); len(r) > 120 {
		text = string(r[:120]) + "…"
	}
	now := time.Now()
	if now.Sub(d.lastShown[t.UserUUID]) < danmakuThrottle || len(d.items) >= danmakuMax {
		return
	}
	lane := 0
	for i := 1; i < danmakuLanes; i++ {
		if d.laneFree[i].Before(d.laneFree[lane]) {
			lane = i
		}
	}
	if d.laneFree[lane].After(now) {
		return
	}
	w := a.width
	if w == 0 {
		w = 1200
	}
	itemW := float32(min(len([]rune(text)), 120)*9 + 64)
	enter := time.Duration(float32(danmakuDur) * itemW / (w + itemW))
	d.laneFree[lane] = now.Add(enter + danmakuGap)
	d.lastShown[t.UserUUID] = now
	name, avatar := "", ""
	if pr := t.AuthorProfile; pr != nil {
		name, avatar = pr.name(), pr.AvatarURL
	}
	if name == "" {
		name = t.UserUUID[:min(8, len(t.UserUUID))]
	}
	d.items = append(d.items, &danmakuItem{id: t.ID, text: text, sessionID: sessionID, author: name, avatar: avatar, seed: t.UserUUID,
		sequence: t.Sequence, lane: lane, dur: danmakuDur})
}

// danmakuLayer draws the pills flying over the workspace.
func (a *app) danmakuLayer(c *ui.Context) {
	d := &a.danmaku
	if len(d.items) == 0 {
		return
	}
	p := a.pal
	now := c.Now()
	live := d.items[:0]
	for _, it := range d.items {
		if it.at.IsZero() {
			it.at = now
		}
		if now.Sub(it.at) < it.dur+200*time.Millisecond {
			live = append(live, it)
		}
	}
	d.items = live
	if len(live) == 0 {
		return
	}
	layer := ui.Box(c.Key("danmaku")).Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).FillWidth().HeightPercent(100).PassThrough().Clip()
	if b := layer.LayoutBounds(); b.W > 0 {
		a.danmakuWidth = b.W
	}
	layer.Children(func() {
		w := a.danmakuWidth
		if w == 0 {
			w = a.width
		}
		for _, it := range live {
			it := it
			k := float32(now.Sub(it.at)) / float32(it.dur)
			x := w - k*(w+max(it.width, 200))
			pill := ui.ButtonBase(c.Key(it.id)).Absolute().Left(x).Top(44+float32(it.lane)*42+12).Row().Gap(6).AlignItems(ui.Center).
				MaxWidth(min(420, w*0.6)).Padding(3, 12, 3, 3).Radius(999).Border(1, p.borderSubtle).Background(p.bgElevated.Alpha(0.92)).
				Shadow(0, 6, 18, 0, ui.RGBA(0, 0, 0, 0.1)).Label("Open " + it.author + "'s message in its chat").FocusRing(false).Children(func() {
				a.avatarSeed(c, it.avatar, it.author, it.seed, 20, 10)
				ui.Text(c, it.author).FontSize(14).FontWeight(550).TextColor(p.textTertiary).SingleLine().MaxWidth(110).Shrink(0)
				ui.Text(c, "·").FontSize(14).TextColor(p.placeholder)
				ui.Text(c, it.text).FontSize(14).TextColor(p.textSecond).SingleLine()
			})
			if b := pill.LayoutBounds(); b.W > 0 {
				it.width = b.W
			}
			if pill.Hovered() {
				// Hover holds the pill still, as the web pauses it.
				if !a.danmakuFrame.IsZero() {
					it.at = it.at.Add(now.Sub(a.danmakuFrame))
				}
				pill.Border(1, p.brand.Alpha(0.4))
			}
			if pill.Clicked() {
				a.go_(route{kind: routeSpaceSession, space: a.currentSpace(), session: it.sessionID})
			}
		}
		c.AnimationFrame()
	})
	a.danmakuFrame = now
}
