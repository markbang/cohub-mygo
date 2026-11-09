package main

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"
)

// invitePage is an invitation to a Space, as InvitePage.svelte: who
// invites, as what role, until when, and accepting it.

type inviteDetail struct {
	Token            string  `json:"token"`
	SpaceID          string  `json:"spaceId"`
	SpaceName        string  `json:"spaceName"`
	SpaceAvatarURL   *string `json:"spaceAvatarUrl"`
	SpaceDescription *string `json:"spaceDescription"`
	Role             string  `json:"role"`
	ViewerRole       *string `json:"viewerRole"`
	ExpiresInSeconds *int    `json:"expiresInSeconds"`
	Inviter          struct {
		UserUUID    string  `json:"userUuid"`
		DisplayName string  `json:"displayName"`
		AvatarURL   *string `json:"avatarUrl"`
	} `json:"inviter"`
}

var roleRank = map[string]int{"guest": 1, "builder": 2, "host": 3}

func inviteRole(r string) string { return roleLabel(&r) }

// isStatus is whether err is the API's answer with that HTTP status.
func isStatus(err error, code int) bool {
	return err != nil && strings.Contains(err.Error(), fmt.Sprintf(": %d ", code))
}

func inviteExpiry(s *int) string {
	if s == nil {
		return m("invite_never_expires")
	}
	n := float64(*s)
	switch {
	case n < 60:
		return m("space_invite_expires_lt1m")
	case n < 3600:
		return m("space_invite_expires_m", "n", int(math.Ceil(n/60)))
	case n < 86400:
		return m("space_invite_expires_h", "n", int(math.Ceil(n/3600)))
	}
	return m("space_invite_expires_d", "n", int(math.Ceil(n/86400)))
}

func (a *app) invitePage(c *ui.Context) {
	p := a.pal
	token := a.route.sub
	key := "invite:" + token
	d, err, ok := loadJSON[inviteDetail](a, key, "/api/invite/"+url.PathEscape(token))
	ui.Column(c.Key("invite")).Grow(1).FillWidth().Background(p.bgPrimary).Center().Padding(40, 16).Children(func() {
		ui.Column(c).Width(420).MaxWidth(9999).Children(func() {
			ui.Box(c).Size(36, 36).Radius(7).Background(p.brand).Center().Children(func() {
				ui.Text(c, "C").FontSize(12).FontWeight(700).TextColor(p.brandFg)
			})
			ui.Box(c).Height(32)
			switch {
			case !ok:
				ui.Row(c).Height(208).Center().Gap(8).Children(func() {
					a.spinner(c, 16, p.textTertiary)
					ui.Text(c, m("invite_loading")).FontSize(12).TextColor(p.textTertiary)
				})
			case err != nil && (isStatus(err, 404) || isStatus(err, 410)):
				ui.Column(c).Padding(32, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Gap(8).Children(func() {
					ui.Text(c, m("invite_unavailable_title")).FontSize(18).FontWeight(600)
					ui.Text(c, m("invite_unavailable_body")).FontSize(13).LineHeight(1.5).TextColor(p.textTertiary)
				})
			case err != nil || d == nil:
				ui.Column(c).Padding(32, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Gap(8).Children(func() {
					ui.Text(c, m("invite_error_title")).FontSize(18).FontWeight(600)
					if err != nil {
						ui.Text(c, err.Error()).FontSize(13).TextColor(p.textTertiary)
					}
					if a.button(c, "", m("common_retry"), false).Clicked() {
						a.reload(key)
					}
				})
			default:
				a.inviteDetail(c, d, key)
			}
		})
	})
}

func (a *app) inviteDetail(c *ui.Context, d *inviteDetail, key string) {
	p := a.pal
	viewer := ""
	if d.ViewerRole != nil {
		viewer = *d.ViewerRole
	}
	member := viewer != "" && roleRank[viewer] >= roleRank[d.Role]
	ui.Column(c).Padding(28, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
			a.avatarSeed(c, deref(d.SpaceAvatarURL), d.SpaceName, d.SpaceID, 48, 10)
			ui.Column(c).Grow(1).Gap(6).Children(func() {
				ui.Text(c, m("invite_title", "space", d.SpaceName)).FontSize(22).FontWeight(600).LineHeight(1.2)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					a.avatarSeed(c, deref(d.Inviter.AvatarURL), d.Inviter.DisplayName, d.Inviter.UserUUID, 16, 8)
					ui.Text(c, m("invite_invited_by", "name", d.Inviter.DisplayName)).FontSize(12).TextColor(p.textTertiary).SingleLine()
				})
			})
		})
		if s := deref(d.SpaceDescription); s != "" {
			ui.Box(c).Height(16)
			ui.Text(c, s).FontSize(13).LineHeight(1.55).TextColor(p.textSecond).MaxLines(3)
		}
		ui.Box(c).Height(24)
		ui.Column(c).Gap(4).Padding(16, 0).BorderWidth(1, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
			line := m("invite_join_as", "role", inviteRole(d.Role))
			if member {
				line = m("invite_member")
			} else if viewer != "" {
				line = m("invite_role_upgrade", "current", inviteRole(viewer), "role", inviteRole(d.Role))
			}
			ui.Text(c, line).FontSize(13).TextColor(p.textSecond)
			if !member {
				ui.Text(c, inviteExpiry(d.ExpiresInSeconds)).FontSize(12).TextColor(p.textTertiary)
			}
		})
		if a.inviteErr != "" {
			ui.Box(c).Height(20)
			ui.Text(c, a.inviteErr).FontSize(12).TextColor(p.errorC)
		}
		ui.Box(c).Height(24)
		label := m("invite_accept")
		if member {
			label = m("invite_open_space")
		} else if a.inviteBusy {
			label = m("invite_joining")
		}
		b := ui.ButtonBase(c.Key("accept")).Row().Gap(8).AlignItems(ui.Center).Height(40).Padding(0, 20).Radius(8).Background(p.brand).FocusRing(false).Children(func() {
			if a.inviteBusy {
				a.spinner(c, 16, p.brandFg)
			}
			ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(p.brandFg)
			if !a.inviteBusy {
				ui.Icon(c, icon("arrow-right")).Size(16, 16).TextColor(p.brandFg)
			}
		})
		if b.Clicked() && !a.inviteBusy {
			if member {
				a.go_(route{kind: routeSpace, space: d.SpaceID})
				return
			}
			a.inviteBusy, a.inviteErr = true, ""
			a.spawn(func() {
				var out struct {
					SpaceID string `json:"spaceId"`
				}
				err := a.api.do(context.Background(), "POST", "/api/invite/"+url.PathEscape(d.Token)+"/accept", nil, &out)
				a.update(func() {
					a.inviteBusy = false
					if err != nil {
						a.inviteErr = m("invite_failed_join") + " " + err.Error()
						return
					}
					a.forget(key)
					id := out.SpaceID
					if id == "" {
						id = d.SpaceID
					}
					a.go_(route{kind: routeSpace, space: id})
				})
				a.refresh()
			})
		}
	})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
