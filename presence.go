package main

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/egoist/mygo/ui"
)

// Who else is in the Space, as SpacePresenceStack: the presence snapshot
// when the Space opens, then space.presence.updated events.

type presenceSnapshot struct {
	SpaceID string `json:"spaceId"`
	Users   []struct {
		UserID  string   `json:"userId"`
		Profile *Profile `json:"profile"`
	} `json:"users"`
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
	var users []*Profile
	for _, u := range snap.Users {
		if a.me != nil && u.UserID == a.me.UUID || u.Profile == nil {
			continue
		}
		users = append(users, u.Profile)
	}
	if len(users) == 0 {
		return
	}
	shown := users
	if len(shown) > 3 {
		shown = shown[:3]
	}
	label := fmt.Sprintf("%d online", len(users))
	ui.Row(c.Key("presence")).AlignItems(ui.Center).Padding(0, 4).Tooltip(label).Children(func() {
		for i, u := range shown {
			av := a.avatar(c, u.AvatarURL, u.name(), 18, 9).Border(2, p.bgPrimary)
			if i > 0 {
				av.Margin(0, 0, 0, -6)
			}
		}
		if len(users) > 3 {
			ui.Text(c, fmt.Sprintf("+%d", len(users)-3)).FontSize(10).TextColor(p.textTertiary).Margin(0, 0, 0, 4)
		}
	})
}
