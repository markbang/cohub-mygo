package main

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Approvals are what a local harness (Codex, Claude Code) asks before it
// runs a command or changes files, as SessionApprovals.svelte: a card
// above the composer per request, with the decisions it allows.

type approval struct {
	ID        string    `json:"id"`
	TurnID    string    `json:"turnId"`
	Kind      string    `json:"kind"` // command, file_change
	Command   string    `json:"command"`
	Cwd       string    `json:"cwd"`
	Reason    string    `json:"reason"`
	Paths     []string  `json:"paths"`
	Decisions []string  `json:"decisions"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type approvalList struct {
	Approvals []approval `json:"approvals"`
	CanDecide bool       `json:"canDecide"`
}

// forgetPrefix drops every loaded page whose key starts with a prefix.
func (a *app) forgetPrefix(prefixes ...string) {
	for k := range a.pages.json {
		for _, p := range prefixes {
			if strings.HasPrefix(k, p) {
				a.forget(k)
			}
		}
	}
	for k := range a.loading {
		for _, p := range prefixes {
			if strings.HasPrefix(k, p) {
				delete(a.loading, k)
			}
		}
	}
}

func (a *app) approvals(c *ui.Context, s *Session, tl *sessionTurns) {
	p := a.pal
	key := "approvals:" + s.ID
	list, _, _ := loadJSON[approvalList](a, key, "/api/spaces/"+url.PathEscape(s.SpaceID)+"/sessions/"+url.PathEscape(s.ID)+"/approvals")
	if list == nil {
		return
	}
	harness := a.composerHarness()
	for _, ap := range list.Approvals {
		if time.Now().After(ap.ExpiresAt) {
			continue
		}
		ap := ap
		ui.Column(c.Key("approval-"+ap.ID)).Shrink(0).Padding(10, 12).Background(p.bgElevated).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(768).Margin(0, ui.Auto).Gap(8).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ic, k := "square-terminal", "runtime_approval_request_command"
					if ap.Kind != "command" {
						ic, k = "file-diff", "runtime_approval_request_file_change"
					}
					ui.Icon(c, icon(ic)).Size(16, 16).TextColor(p.textTertiary)
					ui.Text(c, m(k, "harness", harnessName(harness))).FontSize(14).FontWeight(500)
				})
				if ap.Command != "" {
					ui.Text(c, ap.Command).FontSize(12).Font(fontMono).TextColor(p.textSecond).Padding(6, 10).Radius(6).Background(p.hover).MaxLines(6)
				}
				for i, path := range ap.Paths {
					if i == 6 {
						break
					}
					ui.Text(c.Key(path), path).FontSize(12).Font(fontMono).TextColor(p.textSecond).SingleLine()
				}
				var why []string
				for _, v := range []string{ap.Reason, ap.Cwd} {
					if v != "" {
						why = append(why, v)
					}
				}
				if len(why) > 0 {
					ui.Text(c, strings.Join(why, " · ")).FontSize(12).TextColor(p.textTertiary)
				}
				if !list.CanDecide {
					ui.Text(c, m("runtime_approval_waiting")).FontSize(12).TextColor(p.textTertiary)
					return
				}
				ui.Row(c).Gap(8).Wrap().Children(func() {
					for _, d := range ap.Decisions {
						d := d
						label := map[string]string{"accept": m("runtime_approval_accept"), "accept_for_session": m("runtime_approval_accept_for_session"), "decline": m("runtime_approval_decline")}[d]
						b := ui.ButtonBase(c.Key(d)).Height(36).Padding(0, 12).Radius(6).Center().FocusRing(false).Children(func() {
							col := p.text
							switch d {
							case "accept":
								col = p.brandFg
							case "decline":
								col = p.errorC
							}
							ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(col)
						})
						switch d {
						case "accept":
							b.Background(p.brand)
						case "decline":
							b.Border(1, p.errorC.Alpha(0.3))
						default:
							b.Border(1, p.borderSubtle).Background(p.bgPrimary)
						}
						if b.Clicked() {
							a.act(func(ctx context.Context) error {
								return a.api.do(ctx, "POST", "/api/spaces/"+url.PathEscape(s.SpaceID)+"/sessions/"+url.PathEscape(s.ID)+"/approvals/"+url.PathEscape(ap.ID), map[string]any{"decision": d}, nil)
							}, key)
						}
					}
				})
			})
		})
	}
}
