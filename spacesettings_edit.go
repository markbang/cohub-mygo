package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The Space settings' sections, editable as the web's settings page:
// the profile (picture, slug, description), access and members with
// invite links, environment variables, channels and the sandbox.

// spaceEdit holds the settings' drafts across frames.
type spaceEdit struct {
	desc, slug, envName, envValue string
	editingSlug                   bool
	reveal                        map[string]bool
	bindChannel                   string
	busy                          string
}

func (a *app) spaceEditFor(id string, d *spaceDetail) *spaceEdit {
	if e := a.spaceEdits[id]; e != nil {
		return e
	}
	e := &spaceEdit{desc: d.Description, slug: d.Slug, reveal: map[string]bool{}}
	a.spaceEdits[id] = e
	return e
}

func (a *app) spaceProfile(c *ui.Context, d *spaceDetail) {
	p := a.pal
	e := a.spaceEditFor(d.ID, d)
	edit := a.can(d.ID, "space.edit")
	key := "space:" + d.ID
	a.settingsHeading(c, m("space_section_profile"), m("space_profile_desc"), nil)
	ui.Row(c.Key("id")).Gap(16).AlignItems(ui.Start).Padding(24, 0).Children(func() {
		av := ""
		if d.PublicProfile != nil {
			av = d.PublicProfile.AvatarURL
		}
		ui.Column(c).Width(64).AlignItems(ui.Center).Gap(6).Children(func() {
			a.avatarSeed(c, av, d.Name, d.ID, 56, 28)
			if edit {
				ch := ui.ButtonBase(c.Key("avatar")).Row().Gap(4).AlignItems(ui.Center).Label(m("space_change_avatar")).FocusRing(false).Children(func() {
					if e.busy == "avatar" {
						a.spinner(c, 12, p.textTertiary)
					} else {
						ui.Icon(c, icon("upload")).Size(12, 12).TextColor(p.textTertiary)
					}
					ui.Text(c, m("space_change")).FontSize(11).TextColor(p.textTertiary)
				})
				if ch.Clicked() {
					a.pickAvatar("space_avatar", d.ID, func(ctx context.Context, u string) error {
						return a.api.do(ctx, "PATCH", sp(d.ID)+"/profile", map[string]any{"avatarUrl": u}, nil)
					}, key)
				}
			}
		})
		ui.Column(c).Grow(1).Gap(6).Padding(2, 0).Children(func() {
			ui.Text(c, d.Name).FontSize(15).FontWeight(500)
			ui.Row(c.Key("sid")).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, strings.ToUpper(m("space_id"))).FontSize(11).LetterSpacing(0.6).TextColor(p.textTertiary)
				a.copyChip(c, "sid", d.ID[:8]+"…"+d.ID[len(d.ID)-4:], d.ID)
			})
			owner := "username"
			if d.OwnerProfile != nil && d.OwnerProfile.Username != "" {
				owner = d.OwnerProfile.Username
			}
			ui.Row(c.Key("url")).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, strings.ToUpper(m("space_url"))).FontSize(11).LetterSpacing(0.6).TextColor(p.textTertiary)
				if e.editingSlug {
					ui.Text(c, "/"+owner+"/").FontSize(11).Font(fontMono).TextColor(p.textTertiary)
					in := ui.TextInputBase(c.Key("slug"), &e.slug).FontSize(11).Font(fontMono).Height(22).Padding(0, 6).Radius(4).Border(1, p.brand).Width(160).AutoFocus().Label(m("space_slug"))
					save := func() {
						e.editingSlug = false
						slug := strings.TrimSpace(e.slug)
						a.act(func(ctx context.Context) error {
							var v any = slug
							if slug == "" {
								v = nil
							}
							return a.api.do(ctx, "PATCH", sp(d.ID), map[string]any{"slug": v}, nil)
						}, key)
					}
					if in.Submitted() {
						save()
					}
					in.OnShortcut(0, ui.KeyEscape, func() { e.editingSlug, e.slug = false, d.Slug })
					if a.iconButton(c.Key("slug-ok"), "check", m("space_save_slug"), false).Clicked() {
						save()
					}
					return
				}
				link := "https://cohub.live/" + owner + "/" + d.Slug
				a.copyChip(c, "url", "/"+owner+"/"+d.Slug, link)
				if edit {
					pb := ui.ButtonBase(c.Key("edit-slug")).Size(18, 18).Center().Radius(4).Label(m("space_edit_slug")).FocusRing(false).Children(func() {
						ui.Icon(c, icon("pencil")).Size(12, 12).TextColor(p.textTertiary)
					})
					if pb.Clicked() {
						e.editingSlug, e.slug = true, d.Slug
					}
				}
			})
		})
	})
	ui.Column(c.Key("desc")).Gap(8).Padding(24, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Text(c, m("space_description")).FontSize(13).FontWeight(500)
		in := ui.TextAreaBase(c.Key("area"), &e.desc).Lines(3, 8).Placeholder(m("space_what_for")).PlaceholderColor(p.placeholder).FontSize(13).
			Padding(8, 12).Radius(6).Border(1, p.borderSubtle).Background(p.bgInput).Label(m("space_description"))
		if in.Focused() {
			in.Border(1, p.brand.Alpha(0.5))
		}
		dirty := strings.TrimSpace(e.desc) != strings.TrimSpace(d.Description)
		save := ui.ButtonBase(c.Key("save")).Row().Gap(6).AlignItems(ui.Center).Height(32).Padding(0, 12).Radius(5).Background(p.brand).AlignSelf(ui.Start).FocusRing(false).Children(func() {
			if e.busy == "desc" {
				a.spinner(c, 12, p.brandFg)
			}
			ui.Text(c, m("space_save")).FontSize(12).FontWeight(500).TextColor(p.brandFg)
		})
		if !dirty || !edit {
			save.Opacity(0.5)
		} else if save.Clicked() {
			desc := strings.TrimSpace(e.desc)
			e.busy = "desc"
			a.act(func(ctx context.Context) error {
				defer a.update(func() { e.busy = "" })
				return a.api.do(ctx, "PATCH", sp(d.ID)+"/profile", map[string]any{"description": desc}, nil)
			}, key)
		}
	})
	ui.Row(c.Key("sticker")).Gap(16).AlignItems(ui.Center).Padding(16, 0).BorderWidth(1, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, m("space_like_sticker")).FontSize(13)
			ui.Text(c, m("space_like_sticker_hint")).FontSize(12).TextColor(p.textTertiary)
		})
		ui.Box(c).Size(36, 36).Radius(6).Border(1, p.borderSubtle).Center().Children(func() {
			ui.Icon(c, icon("heart")).Size(16, 16).TextColor(p.errorC)
		})
		if edit && a.button(c, "upload", m("space_upload"), false).Clicked() {
			a.pickAvatar("space_like_sticker", d.ID, func(ctx context.Context, u string) error {
				return a.api.do(ctx, "PATCH", sp(d.ID)+"/profile", map[string]any{"likeSticker": map[string]any{"url": u}}, nil)
			}, key)
		}
	})
}

// pickAvatar asks for a picture, puts it on public storage for the
// purpose, and hands its URL to apply.
func (a *app) pickAvatar(purpose, spaceID string, apply func(ctx context.Context, url string) error, reload ...string) {
	if a.win == nil || a.api == nil {
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Filters: []mygo.FileFilter{{Name: "Images", MIMETypes: []string{"image/*"}}}})
		if err != nil || len(paths) == 0 {
			return
		}
		b, err := os.ReadFile(paths[0])
		if err != nil {
			return
		}
		a.update(func() {
			at := &attachment{id: newID(), name: filepath.Base(paths[0]), data: b, mime: http.DetectContentType(b)}
			a.act(func(ctx context.Context) error {
				u, err := a.api.uploadAsset(ctx, purpose, spaceID, at)
				if err != nil {
					return err
				}
				return apply(ctx, u)
			}, reload...)
		})
	}()
}

// uploadAsset is uploadAttachment for any purpose of public storage.
func (cl *client) uploadAsset(ctx context.Context, purpose, spaceID string, at *attachment) (string, error) {
	if purpose == "chat_attachment" {
		return cl.uploadAttachment(ctx, spaceID, "", at, nil)
	}
	saved := at.mime
	if i := strings.Index(saved, ";"); i >= 0 {
		at.mime = saved[:i]
	}
	in := map[string]any{
		"purpose": purpose, "uploadProtocol": "presigned_put_v1",
		"file": map[string]any{"size": len(at.data), "mimeType": at.mime, "filename": at.name},
	}
	if spaceID != "" {
		in["spaceId"] = spaceID
	}
	var plan struct {
		Asset struct {
			PublicURL     string            `json:"publicUrl"`
			UploadURL     string            `json:"uploadUrl"`
			UploadHeaders map[string]string `json:"uploadHeaders"`
		} `json:"asset"`
	}
	if err := cl.do(ctx, "POST", "/api/public-assets/uploads", in, &plan); err != nil {
		return "", err
	}
	req, _ := http.NewRequestWithContext(ctx, "PUT", plan.Asset.UploadURL, strings.NewReader(string(at.data)))
	req.ContentLength = int64(len(at.data))
	for k, v := range plan.Asset.UploadHeaders {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	res.Body.Close()
	if res.StatusCode/100 != 2 {
		return "", fmt.Errorf("upload failed: HTTP %d", res.StatusCode)
	}
	return plan.Asset.PublicURL, nil
}

var roleValues = []string{"", "guest", "builder", "host"}

func roleNames(values []string) []string {
	var out []string
	for _, v := range values {
		v := v
		out = append(out, roleLabel(&v))
	}
	return out
}

func roleOf(name string, values []string) *string {
	for _, v := range values {
		v := v
		if roleLabel(&v) == name && v != "" {
			return &v
		}
	}
	return nil
}

type invitation struct {
	Token     string     `json:"token"`
	Role      string     `json:"role"`
	ExpiresAt *time.Time `json:"expiresAt"`
	UseCount  int        `json:"useCount"`
	MaxUses   *int       `json:"maxUses"`
}

func (a *app) spaceAccess(c *ui.Context, d *spaceDetail) {
	p := a.pal
	manage := a.can(d.ID, "member.manage")
	acc, _, _ := loadJSON[spaceAccess](a, "access:"+d.ID, sp(d.ID)+"/access")
	mem, _, _ := loadJSON[spaceMembers](a, "members:"+d.ID, sp(d.ID)+"/members")
	ui.Column(c.Key("access")).Padding(40, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		a.settingsHeading(c, m("space_section_access"), m("space_access_desc"), func() {
			if manage && a.button(c, "link", m("space_invite"), true).Clicked() {
				a.act(func(ctx context.Context) error {
					var out invitation
					err := a.api.do(ctx, "POST", sp(d.ID)+"/invitations", map[string]any{"role": "builder"}, &out)
					if err == nil {
						a.update(func() { a.toast, a.toastAt = m("space_invite_created"), time.Now() })
					}
					return err
				}, "invites:"+d.ID)
			}
		})
		ui.Box(c).Height(16)
		policy := func(field, cur string, values []string) {
			name := roleLabel(&cur)
			before := name
			sel := ui.Select(c.Key(field), &name, roleNames(values)).Width(120).Height(32)
			if !manage {
				sel.Disabled(true)
			}
			if name != before {
				role := roleOf(name, values)
				a.act(func(ctx context.Context) error {
					return a.api.do(ctx, "PATCH", sp(d.ID)+"/access", map[string]any{field: role}, nil)
				}, "access:"+d.ID)
				if acc != nil {
					if field == "signed_in_user" {
						acc.SignedIn = role
					} else {
						acc.Anonymous = role
					}
				}
			}
		}
		signed, anon := "", ""
		if acc != nil && acc.SignedIn != nil {
			signed = *acc.SignedIn
		}
		if acc != nil && acc.Anonymous != nil {
			anon = *acc.Anonymous
		}
		a.settingsRow(c, m("space_signed_in_users"), m("space_access_any_signed_in"), func() { policy("signed_in_user", signed, roleValues[:3]) })
		a.settingsRow(c, m("space_access_anon"), m("space_access_no_account"), func() { policy("anonymous_user", anon, roleValues[:2]) })
		guest := m("space_guest_page_default")
		a.settingsRow(c, m("space_guest_page"), m("space_guest_page_hint"), func() { a.selectBox(c, "guest", &guest, []string{guest}) })
		count := 0
		if mem != nil {
			count = len(mem.Items)
		}
		ui.Row(c.Key("members-h")).Gap(4).Padding(16, 0, 8, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
			ui.Text(c, m("space_members_caption")).FontSize(13).FontWeight(500)
			ui.Text(c, fmt.Sprintf("· %d", count)).FontSize(13).TextColor(p.textTertiary)
		})
		if mem != nil {
			for _, it := range mem.Items {
				it := it
				ui.Row(c.Key(it.UserID)).Gap(10).AlignItems(ui.Center).Height(44).Children(func() {
					a.avatar(c, it.Profile.AvatarURL, it.Profile.name(), 28, 14)
					ui.Column(c).Grow(1).Children(func() {
						ui.Text(c, it.Profile.name()).FontSize(13)
						if it.Profile != nil && it.Profile.Username != "" {
							ui.Text(c, "@"+it.Profile.Username).FontSize(11).TextColor(p.textTertiary)
						}
					})
					self := a.me != nil && a.me.UUID == it.UserID
					if !manage || self {
						role := it.Role
						ui.Text(c, roleLabel(&role)).FontSize(12).TextColor(p.textTertiary)
						return
					}
					name := roleLabel(&it.Role)
					before := name
					ui.Select(c.Key("role"), &name, roleNames(roleValues[1:])).Width(112).Height(30)
					if name != before {
						if r := roleOf(name, roleValues[1:]); r != nil {
							a.act(func(ctx context.Context) error {
								return a.api.do(ctx, "PUT", sp(d.ID)+"/members", map[string]any{"userId": it.UserID, "role": *r}, nil)
							}, "members:"+d.ID)
						}
					}
					rm := ui.ButtonBase(c.Key("rm")).Size(28, 28).Center().Radius(5).Label(m("space_remove_member")).FocusRing(false).Children(func() {
						ui.Icon(c, icon("x")).Size(14, 14).TextColor(p.textTertiary)
					})
					if rm.Hovered() {
						rm.Background(p.errorC.Alpha(0.08))
					}
					if rm.Clicked() {
						a.confirm(m("space_remove_member"), m("space_remove_member_confirm"), m("space_remove_member"), func() {
							a.act(func(ctx context.Context) error {
								return a.api.do(ctx, "DELETE", sp(d.ID)+"/members", map[string]any{"userId": it.UserID}, nil)
							}, "members:"+d.ID)
						})
					}
				})
			}
		}
		if manage {
			a.invitations(c, d)
		}
	})
}

func (a *app) invitations(c *ui.Context, d *spaceDetail) {
	p := a.pal
	inv, _, _ := loadJSON[struct {
		Items []invitation `json:"items"`
	}](a, "invites:"+d.ID, sp(d.ID)+"/invitations?status=active&limit=20")
	ui.Column(c.Key("invites")).Gap(6).Padding(16, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Text(c, m("space_invite_links")).FontSize(13).FontWeight(500)
		if inv == nil || len(inv.Items) == 0 {
			ui.Text(c, m("space_no_invite_links")).FontSize(12).TextColor(p.textTertiary)
			return
		}
		owner := ""
		if d.OwnerProfile != nil {
			owner = d.OwnerProfile.Username
		}
		for _, it := range inv.Items {
			it := it
			base := "/spaces/" + d.ID
			if owner != "" && d.Slug != "" {
				base = "/" + url.PathEscape(owner) + "/" + url.PathEscape(d.Slug)
			}
			link := "https://cohub.live" + base + "/join/" + url.PathEscape(it.Token)
			ui.Row(c.Key(it.Token)).Gap(10).AlignItems(ui.Center).Padding(8, 10).Radius(6).Border(1, p.borderSubtle).Children(func() {
				role := it.Role
				ui.Text(c, roleLabel(&role)).FontSize(12).Width(64)
				ui.Text(c, link).FontSize(11).Font(fontMono).TextColor(p.textTertiary).SingleLine().Grow(1).MinWidth(0)
				if it.ExpiresAt != nil {
					ui.Text(c, shortDateTime(it.ExpiresAt.Local())).FontSize(11).TextColor(p.placeholder)
				}
				cp := ui.ButtonBase(c.Key("copy")).Size(26, 26).Center().Radius(5).Label(m("space_copy_invite")).FocusRing(false).Children(func() {
					ic := "copy"
					if a.copiedValue == link && time.Since(a.copiedAt) < 2*time.Second {
						ic = "check"
					}
					ui.Icon(c, icon(ic)).Size(13, 13).TextColor(p.textTertiary)
				})
				if cp.Clicked() {
					c.WriteClipboard(link)
					a.copiedAt, a.copiedValue = time.Now(), link
				}
				rv := ui.ButtonBase(c.Key("revoke")).Size(26, 26).Center().Radius(5).Label(m("space_revoke_invite")).FocusRing(false).Children(func() {
					ui.Icon(c, icon("trash-2")).Size(13, 13).TextColor(p.textTertiary)
				})
				if rv.Clicked() {
					a.act(func(ctx context.Context) error {
						return a.api.do(ctx, "DELETE", sp(d.ID)+"/invitations/"+url.PathEscape(it.Token), nil, nil)
					}, "invites:"+d.ID)
				}
			})
		}
	})
}

func (a *app) spaceEnv(c *ui.Context, d *spaceDetail) {
	p := a.pal
	e := a.spaceEditFor(d.ID, d)
	edit := a.can(d.ID, "space.edit")
	key := "space:" + d.ID
	a.settingsHeading(c, m("space_section_env"), m("space_env_desc"), nil)
	ui.Row(c.Key("vars-h")).Padding(24, 0, 8, 0).Children(func() {
		ui.Text(c, m("space_variables")).FontSize(13).FontWeight(500).Grow(1)
		ui.Text(c, m("space_available_to_process")).FontSize(11).TextColor(p.textTertiary)
	})
	ui.Column(c.Key("vars")).Radius(6).Border(1, p.borderSubtle).Children(func() {
		if len(d.Meta.ExtraEnv) == 0 {
			ui.Text(c, m("space_no_vars")).FontSize(12).TextColor(p.textTertiary).Padding(16)
		}
		for i, v := range d.Meta.ExtraEnv {
			v := v
			row := ui.Row(c.Key(v.Name)).Gap(12).AlignItems(ui.Center).Padding(8, 12)
			if i > 0 {
				row.BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle)
			}
			row.Children(func() {
				ui.Text(c, v.Name).FontSize(12).Font(fontMono).Width(200).SingleLine()
				shown := strings.Repeat("•", 12)
				if e.reveal[v.Name] {
					shown = v.Value
				}
				ui.Text(c, shown).FontSize(12).Font(fontMono).TextColor(p.textTertiary).Grow(1).MinWidth(0).SingleLine()
				ic := "eye"
				if e.reveal[v.Name] {
					ic = "eye-off"
				}
				if a.iconButton(c.Key("reveal"), ic, m("x_reveal"), false).Clicked() {
					e.reveal[v.Name] = !e.reveal[v.Name]
				}
				if edit && a.iconButton(c.Key("rm"), "trash-2", m("space_remove_variable"), false).Clicked() {
					a.act(func(ctx context.Context) error {
						return a.api.do(ctx, "DELETE", sp(d.ID)+"/env/"+url.PathEscape(v.Name), nil, nil)
					}, key)
				}
			})
		}
		if edit {
			ui.Row(c.Key("add")).Gap(8).AlignItems(ui.Center).Padding(8, 12).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
				a.field(c, "env-name", &e.envName, "NAME").Width(200).Font(fontMono)
				a.field(c, "env-value", &e.envValue, "value").Grow(1).Font(fontMono)
				add := a.button(c, "plus", m("space_add"), true)
				name := strings.TrimSpace(e.envName)
				if add.Clicked() && name != "" {
					value := e.envValue
					e.envName, e.envValue = "", ""
					a.act(func(ctx context.Context) error {
						return a.api.do(ctx, "POST", sp(d.ID)+"/env", map[string]any{"name": name, "value": value}, nil)
					}, key)
				}
			})
		}
	})
	ui.Row(c.Key("mounted-h")).Padding(28, 0, 8, 0).Children(func() {
		ui.Text(c, m("space_mounted_spaces")).FontSize(13).FontWeight(500).Grow(1)
		ui.Text(c, m("space_mounted_readonly")).FontSize(11).TextColor(p.textTertiary)
	})
	mods, _, _ := loadJSON[struct {
		Items []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			MountSlug string `json:"mountSlug"`
			Enabled   bool   `json:"enabled"`
		} `json:"items"`
	}](a, "mods:"+d.ID, sp(d.ID)+"/mods")
	ui.Column(c.Key("mounted")).Radius(6).Border(1, p.borderSubtle).Children(func() {
		if mods == nil || len(mods.Items) == 0 {
			ui.Text(c, m("space_no_mounted")).FontSize(12).TextColor(p.textTertiary).Padding(16)
			return
		}
		for i, md := range mods.Items {
			md := md
			row := ui.Row(c.Key(md.ID)).Gap(10).AlignItems(ui.Center).Padding(8, 12)
			if i > 0 {
				row.BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle)
			}
			row.Children(func() {
				ui.Icon(c, icon("package")).Size(14, 14).TextColor(p.textTertiary)
				ui.Text(c, md.Name).FontSize(13).Grow(1)
				ui.Text(c, "/mods/"+md.MountSlug).FontSize(11).Font(fontMono).TextColor(p.textTertiary)
				on := md.Enabled
				if ui.Switch(c.Key("on"), &on).Changed() && edit {
					a.act(func(ctx context.Context) error {
						return a.api.do(ctx, "PATCH", sp(d.ID)+"/mods/"+url.PathEscape(md.ID), map[string]any{"enabled": on}, nil)
					}, "mods:"+d.ID)
				}
				if edit && a.iconButton(c.Key("rm"), "trash-2", m("space_remove_mod"), false).Clicked() {
					a.confirm(m("space_remove_mod"), m("space_mod_restart_confirm"), m("space_remove_mod"), func() {
						a.act(func(ctx context.Context) error {
							return a.api.do(ctx, "DELETE", sp(d.ID)+"/mods/"+url.PathEscape(md.ID), nil, nil)
						}, "mods:"+d.ID)
					})
				}
			})
		}
	})
}

type spaceBinding struct {
	ChannelID string `json:"channelId"`
	Channel   *struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Provider string `json:"provider"`
	} `json:"channel"`
}

func (a *app) spaceChannels(c *ui.Context, d *spaceDetail) {
	p := a.pal
	e := a.spaceEditFor(d.ID, d)
	manage := a.can(d.ID, "channel.manage")
	bound, _, _ := loadJSON[[]spaceBinding](a, "space-channels:"+d.ID, sp(d.ID)+"/channels")
	all, _, _ := loadJSON[[]channel](a, "channels", "/api/channels")
	a.settingsHeading(c, m("space_section_channels"), m("space_channels_desc"), nil)
	ui.Column(c.Key("bound")).Margin(24, 0, 0, 0).Radius(6).Border(1, p.borderSubtle).Children(func() {
		n := 0
		if bound != nil {
			for _, b := range *bound {
				b := b
				n++
				name, provider := b.ChannelID, ""
				if b.Channel != nil {
					name, provider = b.Channel.Name, b.Channel.Provider
				}
				ui.Row(c.Key(b.ChannelID)).Gap(10).AlignItems(ui.Center).Padding(8, 12).Children(func() {
					a.dot(c, providerColor(provider, p), 8)
					ui.Text(c, name).FontSize(13).Grow(1)
					ui.Text(c, provider).FontSize(11).TextColor(p.textTertiary)
					if manage {
						ub := ui.ButtonBase(c.Key("unbind")).Height(28).Padding(0, 8).Radius(5).Center().FocusRing(false).Children(func() {
							ui.Text(c, m("space_unbind")).FontSize(11).TextColor(p.textTertiary)
						})
						if ub.Hovered() {
							ub.Background(p.errorC.Alpha(0.08))
						}
						if ub.Clicked() {
							a.act(func(ctx context.Context) error {
								return a.api.do(ctx, "DELETE", sp(d.ID)+"/channels/"+url.PathEscape(b.ChannelID), nil, nil)
							}, "space-channels:"+d.ID, "channels")
						}
					}
				})
			}
		}
		if n == 0 {
			ui.Text(c, m("space_no_channels")).FontSize(12).TextColor(p.textTertiary).Padding(16)
		}
	})
	if !manage || all == nil {
		return
	}
	var free []string
	ids := map[string]string{}
	for _, ch := range *all {
		if ch.BoundSpace == nil {
			free = append(free, ch.Name)
			ids[ch.Name] = ch.ID
		}
	}
	if len(free) == 0 {
		return
	}
	ui.Row(c.Key("bind")).Gap(8).AlignItems(ui.Center).Margin(12, 0, 0, 0).Children(func() {
		ui.Text(c, m("space_channel_to_bind")).FontSize(12).TextColor(p.textSecond)
		if e.bindChannel == "" {
			e.bindChannel = free[0]
		}
		ui.Select(c.Key("pick"), &e.bindChannel, free).Width(200).Height(32)
		if a.button(c, "link", m("space_bind"), true).Clicked() {
			id := ids[e.bindChannel]
			a.act(func(ctx context.Context) error {
				return a.api.do(ctx, "POST", sp(d.ID)+"/channels/"+url.PathEscape(id), map[string]any{"config": nil}, nil)
			}, "space-channels:"+d.ID, "channels")
		}
	})
}

type sandboxConfig struct {
	Config struct {
		Sandbox struct {
			Provider    string `json:"provider"`
			Spec        string `json:"spec"`
			Image       string `json:"image"`
			AppliedSpec string `json:"appliedSpec"`
			AllowedSpec string `json:"allowedSpec"`
			AutoDestroy struct {
				Mode       string `json:"mode"`
				TTLSeconds int    `json:"ttlSeconds"`
			} `json:"autoDestroy"`
			Preferences struct {
				Images         []string `json:"images"`
				PendingRestart bool     `json:"pendingRestart"`
			} `json:"preferences"`
			Specs map[string]struct {
				ID          string `json:"id"`
				Rank        int    `json:"rank"`
				Label       string `json:"label"`
				Description string `json:"description"`
				Resources   struct {
					Limits struct {
						CPU    string `json:"cpu"`
						Memory string `json:"memory"`
					} `json:"limits"`
				} `json:"resources"`
			} `json:"specs"`
		} `json:"sandbox"`
	} `json:"config"`
}

func (a *app) spaceSandbox(c *ui.Context, d *spaceDetail) {
	p := a.pal
	manage := a.can(d.ID, "sandbox.manage")
	cfg, _, _ := loadJSON[sandboxConfig](a, "sandbox-config:"+d.ID, sp(d.ID)+"/config")
	a.settingsHeading(c, m("space_section_sandbox"), m("space_sandbox_desc"), nil)
	ui.Box(c).Height(16)
	a.settingsRow(c, m("channels_col_status"), "", func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			col := p.placeholder
			if d.SandboxStatus == "running" || d.SandboxStatus == "ready" {
				col = p.success
			}
			a.dot(c, col, 7)
			ui.Text(c, d.SandboxStatus).FontSize(12).Font(fontMono)
			if manage && a.button(c, "refresh-cw", m("x_restart"), false).Clicked() {
				a.confirm(m("x_restart"), m("space_mount_changes_restart"), m("x_restart"), func() {
					a.act(func(ctx context.Context) error {
						return a.api.do(ctx, "POST", sp(d.ID)+"/sandbox/recreate", nil, nil)
					}, "space:"+d.ID)
				})
			}
		})
	})
	if cfg == nil {
		return
	}
	sb := cfg.Config.Sandbox
	type specRow struct {
		id, label, desc string
		rank            int
	}
	var specs []specRow
	for id, s := range sb.Specs {
		desc := s.Description
		if s.Resources.Limits.CPU != "" {
			desc += " · " + s.Resources.Limits.CPU + " CPU · " + s.Resources.Limits.Memory
		}
		specs = append(specs, specRow{id, s.Label, desc, s.Rank})
	}
	for i := range specs {
		for j := i + 1; j < len(specs); j++ {
			if specs[j].rank < specs[i].rank {
				specs[i], specs[j] = specs[j], specs[i]
			}
		}
	}
	allowed := 1 << 30
	if s, ok := sb.Specs[sb.AllowedSpec]; ok {
		allowed = s.Rank
	}
	ui.Column(c.Key("specs")).Gap(8).Padding(16, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, m("space_compute_spec")).FontSize(13).Grow(1)
			if sb.Spec != sb.AppliedSpec && sb.AppliedSpec != "" {
				ui.Text(c, m("space_restart_to_apply")).FontSize(11).TextColor(p.warning)
			}
		})
		ui.Row(c).Gap(8).Wrap().Children(func() {
			for _, s := range specs {
				s := s
				on := sb.Spec == s.id
				ok := manage && s.rank <= allowed
				card := ui.ButtonBase(c.Key(s.id)).Column().Gap(2).Width(180).Padding(10, 12).Radius(8).Border(1, p.borderSubtle).FocusRing(false).Children(func() {
					ui.Text(c, s.label).FontSize(13).FontWeight(500)
					ui.Text(c, s.desc).FontSize(11).TextColor(p.textTertiary).MaxLines(2)
				})
				if on {
					card.Border(1, p.brand).Background(p.brandMuted)
				}
				if !ok {
					card.Opacity(0.5)
				} else if card.Clicked() && !on {
					a.act(func(ctx context.Context) error {
						return a.api.do(ctx, "PATCH", sp(d.ID)+"/config", map[string]any{"sandbox": map[string]any{"spec": s.id}}, nil)
					}, "sandbox-config:"+d.ID)
				}
			}
		})
	})
	img := sb.Image
	if img == "" {
		img = "default"
	}
	a.settingsRow(c, m("sandbox_image"), "", func() {
		if !manage || len(sb.Preferences.Images) < 2 {
			ui.Text(c, img).FontSize(12).Font(fontMono)
			return
		}
		before := img
		ui.Select(c.Key("image"), &img, sb.Preferences.Images).Width(160).Height(32)
		if img != before {
			a.act(func(ctx context.Context) error {
				return a.api.do(ctx, "PATCH", sp(d.ID)+"/config", map[string]any{"sandbox": map[string]any{"image": img}}, nil)
			}, "sandbox-config:"+d.ID)
		}
	})
	a.settingsRow(c, m("x_provider"), "", func() { ui.Text(c, sb.Provider).FontSize(12).Font(fontMono) })
	ttl := "—"
	if sb.AutoDestroy.TTLSeconds > 0 {
		ttl = fmt.Sprintf("%s · %dh", sb.AutoDestroy.Mode, sb.AutoDestroy.TTLSeconds/3600)
	}
	a.settingsRow(c, m("x_auto_destroy"), "", func() { ui.Text(c, ttl).FontSize(12).Font(fontMono) })
}
