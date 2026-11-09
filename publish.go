package main

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Publishing a file, a directory or a port as an app at a public address,
// as AppPublishDialog: /username/space/w/app, who sees it, what it may do
// and whether it shows the Cohub bar.

type publishForm struct {
	space, targetType, targetRef string
	slug, username, spaceSlug    string
	visibility                   string // public, space
	scopes                       map[string]bool
	hideBar                      bool
	busy                         bool
	err                          string
	publishedURL                 string
}

var appScopes = []string{"space.view", "session.view", "file.view", "file.edit", "taskrun.view", "session.prompt.readonly", "session.prompt.fullaccess", "command.execute"}

var (
	slugBad  = regexp.MustCompile(`[^a-z0-9_-]+`)
	slugEnds = regexp.MustCompile(`^[^a-z0-9]+|[^a-z0-9]+$`)
)

func publicSlug(s string) string {
	s = slugBad.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	s = slugEnds.ReplaceAllString(s, "")
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

func (a *app) openPublish(spaceID, targetType, ref string) {
	base := strings.TrimSuffix(path.Base(strings.TrimSuffix(ref, "/")), path.Ext(ref))
	if targetType == "port" {
		base = "port-" + ref
	}
	slug := publicSlug(base)
	if slug == "" {
		slug = "app"
	}
	a.publish = &publishForm{space: spaceID, targetType: targetType, targetRef: ref, slug: slug, visibility: "public",
		scopes: map[string]bool{appScopes[0]: true}}
}

func (a *app) publishDialog(c *ui.Context) {
	f := a.publish
	if f == nil {
		return
	}
	p := a.pal
	sp := a.space(f.space)
	owner, spaceSlug := "", ""
	if sp != nil {
		spaceSlug = sp.Slug
		if sp.OwnerProfile != nil {
			owner = sp.OwnerProfile.Username
		}
	}
	ent, _, entOK := loadJSON[struct {
		Enabled bool `json:"enabled"`
	}](a, "feature:work.publish.hide_cohub_bar", "/api/billing/features/work.publish.hide_cohub_bar")
	canHide := ent != nil && ent.Enabled
	open := true
	user := firstNonEmpty(owner, f.username)
	sslug := firstNonEmpty(spaceSlug, f.spaceSlug)
	a.dialog(c, &open, m("publish_app"), 560, func() {
		ui.Column(c.Key("publish")).Padding(16).Gap(16).Children(func() {
			label := func(s string) {
				ui.Text(c, strings.ToUpper(s)).FontSize(10).FontWeight(600).LetterSpacing(0.8).TextColor(p.textTertiary)
			}
			box := func(k string, body func()) {
				ui.Column(c.Key(k)).Gap(6).Padding(12).Radius(8).Border(1, p.borderSubtle).Background(p.bgSurface).Children(body)
			}
			if f.publishedURL != "" {
				ui.Row(c.Key("ok")).Gap(10).AlignItems(ui.Center).Padding(12).Radius(8).Border(1, p.success.Alpha(0.3)).Background(p.success.Alpha(0.08)).Children(func() {
					ui.Box(c).Size(28, 28).Radius(14).Background(p.success.Alpha(0.15)).Center().Children(func() {
						ui.Icon(c, icon("check")).Size(16, 16).TextColor(p.success)
					})
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, "Published").FontSize(13).FontWeight(500)
						ui.Text(c, f.publishedURL).FontSize(12).Font(fontMono).TextColor(p.textTertiary).SingleLine().Selectable()
					})
				})
				ui.Row(c.Key("done")).Gap(8).Children(func() {
					ui.Spacer(c)
					if a.button(c, "copy", "Copy", false).Clicked() {
						c.WriteClipboard(f.publishedURL)
						a.notify(m("copied"))
					}
					if a.button(c, "external-link", "Open", false).Clicked() {
						c.OpenURL(f.publishedURL)
					}
					if a.button(c, "", "Done", true).Clicked() {
						a.publish = nil
					}
				})
				return
			}
			box("address", func() {
				label("Public URL")
				ui.Text(c, "/"+firstNonEmpty(user, "username")+"/"+firstNonEmpty(sslug, "space")+"/w/"+firstNonEmpty(f.slug, "app")).
					FontSize(12).Font(fontMono).TextColor(p.textSecond).Padding(8, 10).Radius(6).Background(p.bgInput).SingleLine()
				ui.Row(c).Gap(10).Children(func() {
					field := func(k, name, have string, v *string) {
						ui.Column(c.Key(k)).Grow(1).Basis(0).Gap(4).Children(func() {
							ui.Text(c, name).FontSize(11).TextColor(p.textTertiary)
							if have != "" {
								ui.Text(c, have).FontSize(12).Font(fontMono).TextColor(p.textSecond).Height(30).SingleLine()
								return
							}
							in := a.field(c, k, v, "Required").Font(fontMono)
							if in.Changed() {
								*v = publicSlug(*v)
							}
						})
					}
					field("username", "Username", owner, &f.username)
					field("space-slug", m("app_publish_space_slug"), spaceSlug, &f.spaceSlug)
					field("slug", m("app_publish_app_slug"), "", &f.slug)
				})
			})
			box("source", func() {
				label("Source")
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					a.tag(c, f.targetType)
					ui.Text(c, f.targetRef).FontSize(12).Font(fontMono).TextColor(p.textSecond).SingleLine()
				})
			})
			ui.Column(c.Key("access")).Gap(6).Children(func() {
				label("Access")
				for _, v := range []struct{ value, title, copy string }{
					{"public", m("anyone_with_link"), m("app_publish_fully_public")},
					{"space", m("app_publish_space_access"), m("app_publish_space_access_detail")},
				} {
					v := v
					r := ui.ButtonBase(c.Key(v.value)).Row().Gap(10).AlignItems(ui.Start).Padding(8, 10).Radius(8).Border(1, p.borderSubtle).FocusRing(false).Children(func() {
						ring := ui.Box(c).Size(14, 14).Radius(7).Border(1, p.border).Margin(2, 0, 0, 0).Center()
						if f.visibility == v.value {
							ring.Border(4, p.brand)
						}
						ui.Column(c).Gap(2).Children(func() {
							ui.Text(c, v.title).FontSize(13).FontWeight(500)
							ui.Text(c, v.copy).FontSize(12).TextColor(p.textTertiary)
						})
					})
					if r.Clicked() {
						f.visibility = v.value
					}
				}
			})
			ui.Column(c.Key("scopes")).Gap(6).Children(func() {
				label(m("app_publish_app_can"))
				ui.Text(c, m("app_scope_group_access")).FontSize(12).FontWeight(500).TextColor(p.textSecond)
				ui.Grid(c).Columns(2).Gap(6).Children(func() {
					for _, s := range appScopes {
						s := s
						on := f.scopes[s]
						key := "app_scope_" + strings.NewReplacer(".", "_").Replace(s)
						if ui.Checkbox(c.Key(s), &on, m(key)).Tooltip(m(key + "_hint")).Changed() {
							f.scopes[s] = on
						}
					}
				})
			})
			ui.Column(c.Key("presentation")).Gap(6).Children(func() {
				label("Presentation")
				ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
					on := f.hideBar && canHide
					cb := ui.CheckboxBase(c.Key("hide"), &on).Label(m("app_publish_hide_cohub_bar"))
					if !canHide {
						cb.Disabled(true).Opacity(0.5)
					}
					if cb.Changed() {
						f.hideBar = on
					}
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Text(c, m("app_publish_hide_cohub_bar")).FontSize(13)
						ui.Text(c, m("app_publish_hide_cohub_bar_detail")).FontSize(12).TextColor(p.textTertiary)
					})
					if !canHide && entOK {
						a.tag(c, "Pro / Max")
					}
				})
				if !canHide && entOK {
					ui.Text(c, m("app_publish_included_pro_max")).FontSize(12).TextColor(p.textTertiary)
				}
			})
			if f.err != "" {
				ui.Text(c, f.err).FontSize(12).TextColor(p.errorC).Padding(8, 10).Radius(6).Background(p.errorC.Alpha(0.08))
			}
			ui.Row(c.Key("foot")).Gap(8).Children(func() {
				ui.Spacer(c)
				if a.button(c, "", m("common_cancel"), false).Clicked() {
					a.publish = nil
				}
				ready := user != "" && sslug != "" && f.slug != "" && !f.busy
				b := a.button(c, "rocket", "Publish", true)
				if !ready {
					b.Opacity(0.5)
				}
				if b.Clicked() && ready {
					a.submitPublish(f, owner == "", spaceSlug == "", canHide)
				}
			})
		})
	})
	if !open {
		a.publish = nil
	}
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}

func (a *app) submitPublish(f *publishForm, setUser, setSpaceSlug, canHide bool) {
	f.busy, f.err = true, ""
	a.spawn(func() {
		ctx := context.Background()
		err := func() error {
			if setUser {
				pr := a.me.Profile
				name := "User"
				var avatar any
				if pr != nil {
					name = firstNonEmpty(pr.DisplayName, name)
					if pr.AvatarURL != "" {
						avatar = pr.AvatarURL
					}
				}
				if err := a.api.do(ctx, "PATCH", "/api/me/profile", map[string]any{"displayName": name, "avatarUrl": avatar, "username": f.username}, nil); err != nil {
					return err
				}
			}
			if setSpaceSlug {
				if err := a.api.do(ctx, "PATCH", sp(f.space), map[string]any{"slug": f.spaceSlug}, nil); err != nil {
					return err
				}
			}
			var scopes []string
			for _, s := range appScopes {
				if f.scopes[s] {
					scopes = append(scopes, s)
				}
			}
			in := map[string]any{"spaceId": f.space, "slug": f.slug, "status": "published", "visibility": f.visibility,
				"targetType": f.targetType, "targetRef": f.targetRef, "appScopes": scopes}
			if f.hideBar && canHide {
				in["meta"] = map[string]any{"presentation": map[string]any{"hideCohubBar": true}}
			}
			var out struct {
				App struct {
					ID   string `json:"id"`
					Slug string `json:"slug"`
				} `json:"app"`
			}
			err := a.api.do(ctx, "POST", "/api/apps", in, &out)
			if err != nil && isStatus(err, http.StatusConflict) {
				// The slug is the Space's app already: a new version of it.
				var list struct {
					Apps []struct {
						ID, Slug, Status string
					} `json:"apps"`
				}
				if lerr := a.api.do(ctx, "GET", "/api/apps/space/"+url.PathEscape(f.space), nil, &list); lerr != nil {
					return err
				}
				for _, ap := range list.Apps {
					if ap.Slug == f.slug {
						delete(in, "spaceId")
						delete(in, "slug")
						in["status"] = ap.Status
						if uerr := a.api.do(ctx, "PATCH", "/api/apps/"+url.PathEscape(ap.ID), in, nil); uerr != nil {
							return uerr
						}
						out.App.Slug = ap.Slug
						return a.api.do(ctx, "POST", "/api/apps/"+url.PathEscape(ap.ID)+"/versions", nil, nil)
					}
				}
				return err
			}
			return err
		}()
		a.update(func() {
			f.busy = false
			if err != nil {
				f.err = err.Error()
				return
			}
			f.publishedURL = "https://cohub.live/" + firstNonEmpty(a.spaceOwner(f.space), f.username) + "/" + firstNonEmpty(a.spaceSlugOf(f.space), f.spaceSlug) + "/w/" + f.slug
			a.forgetPrefix("apps:" + f.space)
		})
		a.refresh()
	})
}

func (a *app) spaceOwner(id string) string {
	if s := a.space(id); s != nil && s.OwnerProfile != nil {
		return s.OwnerProfile.Username
	}
	return ""
}

func (a *app) spaceSlugOf(id string) string {
	if s := a.space(id); s != nil {
		return s.Slug
	}
	return ""
}
