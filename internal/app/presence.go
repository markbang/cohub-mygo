package app

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"

	"cohub-mygo/internal/cohub"
)

// Who else is in the Space, as SpacePresenceStack: the presence snapshot
// when the Space opens, then space.presence.updated events.

type presenceSnapshot struct {
	SpaceID string         `json:"spaceId"`
	Users   []presenceUser `json:"users"`
}

type presencePanel struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

type presenceUser struct {
	UserID  string         `json:"userId"`
	Profile *cohub.Profile `json:"profile"`
	Meta    *struct {
		Panels []presencePanel `json:"panels"`
	} `json:"meta"`
	Metas []struct {
		Panels []presencePanel `json:"panels"`
	} `json:"metas"`
}

// where is what the user is at, as the stack's locationLabel.
func (u presenceUser) where() (string, int) {
	var panels []presencePanel
	if u.Meta != nil {
		panels = append(panels, u.Meta.Panels...)
	}
	for _, m := range u.Metas {
		panels = append(panels, m.Panels...)
	}
	if len(panels) == 0 {
		return "in this space", 0
	}
	pr := map[string]int{"session": 0, "file": 1, "checkpoint": 2, "task": 3, "app": 4, "work": 4, "cronjob": 5}
	best := panels[0]
	for _, x := range panels[1:] {
		a, ok := pr[x.Kind]
		if !ok {
			a = 9
		}
		b, ok := pr[best.Kind]
		if !ok {
			b = 9
		}
		if a < b {
			best = x
		}
	}
	if l := strings.TrimSpace(best.Label); l != "" {
		return l, len(panels)
	}
	return map[string]string{"session": "in a chat", "file": "in a file", "checkpoint": "reviewing a save", "task": "on a task",
		"app": "in an app", "work": "in an app", "cronjob": "checking a cronjob"}[best.Kind], len(panels)
}

func (rt *realtime) presenceUpdated(payload json.RawMessage) {
	var snap presenceSnapshot
	if json.Unmarshal(payload, &snap) != nil || snap.SpaceID == "" {
		return
	}
	rt.a.update(func() { rt.a.pages.json["presence:"+snap.SpaceID] = &snap })
}

// presenceStack is the header's stack of the others in the Space.
func (a *app) presenceStack(c *ui.Context, spaceID string) {
	if spaceID == "" {
		return
	}
	p := a.pal
	snap, _, _ := loadJSON[presenceSnapshot](a, "presence:"+spaceID, "/api/spaces/"+url.PathEscape(spaceID)+"/presence")
	if snap == nil {
		return
	}
	var users []presenceUser
	for _, u := range snap.Users {
		if a.me != nil && u.UserID == a.me.UUID || u.Profile == nil {
			continue
		}
		users = append(users, u)
	}
	if len(users) == 0 {
		return
	}
	shown := users
	if len(shown) > 3 {
		shown = shown[:3]
	}
	people := "people"
	if len(users) == 1 {
		people = "person"
	}
	label := fmt.Sprintf("%d %s online", len(users), people)
	open := a.menus["presence"]
	stack := ui.ButtonBase(c.Key("presence")).Row().Gap(4).AlignItems(ui.Center).Height(24).Padding(0, 6, 0, 2).Radius(999).Label(label).FocusRing(false).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Padding(0, 0, 0, 2).Children(func() {
			for i, u := range shown {
				av := a.avatarSeed(c, u.Profile.AvatarURL, u.Profile.Name(), u.UserID, 18, 9).Border(1, p.bgElevated)
				if i > 0 {
					av.Margin(0, 0, 0, -3)
				}
			}
			if len(users) > 3 {
				ui.Text(c, fmt.Sprintf("+%d", len(users)-3)).FontSize(10).TextColor(p.textTertiary).Margin(0, 0, 0, 4)
			}
		})
		if len(users) > 3 {
			ui.Text(c, fmt.Sprint(len(users))).FontSize(11).TextColor(p.textTertiary)
		}
	})
	if stack.Hovered() || open {
		stack.Background(p.hover)
	}
	if stack.Clicked() {
		open = !open
	}
	ui.PopoverBase(c, stack, &open, func(panel ui.Element) {
		panel.AttachTo(stack, ui.AnchorBottomRight, ui.AnchorTopRight).Top(8).Width(280).Radius(14).Border(1, p.borderSubtle).
			Background(p.bgElevated).Shadow(0, 18, 32, 0, ui.RGBA(0, 0, 0, 0.12)).Clip().Children(func() {
			ui.Row(c.Key("head")).Padding(8, 10, 7, 10).Gap(12).AlignItems(ui.Baseline).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
				ui.Text(c, "ONLINE").FontSize(11).FontWeight(650).LetterSpacing(0.2).TextColor(p.textSecond).Grow(1)
				ui.Text(c, fmt.Sprintf("%d %s", len(users), people)).FontSize(11).TextColor(p.textTertiary)
			})
			ui.Scroll(c.Key("list")).MaxHeight(280).Padding(4).Children(func() {
				for _, u := range users {
					u := u
					ui.Row(c.Key(u.UserID)).Height(44).Gap(10).Padding(0, 8).AlignItems(ui.Center).Children(func() {
						a.avatarSeed(c, u.Profile.AvatarURL, u.Profile.Name(), u.UserID, 32, 16)
						ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
							ui.Text(c, u.Profile.Name()).FontSize(13).FontWeight(500).SingleLine()
							where, n := u.where()
							if n > 1 {
								where += fmt.Sprintf(" · %d panels", n)
							}
							ui.Text(c, where).FontSize(12).TextColor(p.textTertiary).SingleLine()
						})
					})
				}
			})
			ui.Row(c.Key("foot")).Padding(8, 10).Gap(12).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
				ui.Text(c, m("presence_live_messages")).FontSize(12).TextColor(p.textSecond).Grow(1)
				on := a.prefs.danmaku()
				if ui.Switch(c.Key("live"), &on).Label(m("presence_toggle_live")).Changed() {
					a.prefs.Danmaku = &on
					a.prefs.save()
					if !on {
						a.danmaku.items = nil
					}
				}
			})
		})
	})
	a.menus["presence"] = open
}
