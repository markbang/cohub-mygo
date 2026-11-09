package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"
)

// A published App of the Space, as the app manager: its state, views,
// where it comes from, and opening, publishing, disabling and deleting it.

type appStats struct {
	Summary struct {
		TotalViews int `json:"totalViews"`
		Views24h   int `json:"views24h"`
		Views7d    int `json:"views7d"`
		Views30d   int `json:"views30d"`
		TotalLikes int `json:"totalLikes"`
	} `json:"summary"`
	Daily []struct {
		Date  string `json:"date"`
		Views int    `json:"views"`
	} `json:"daily"`
}

// appURL is the App's public address: /username/space/w/app.
func (a *app) appURL(spaceID, slug string) string {
	d, _, _ := loadJSON[spaceDetail](a, "space:"+spaceID, sp(spaceID))
	if d == nil || d.OwnerProfile == nil || d.OwnerProfile.Username == "" || d.Slug == "" {
		return ""
	}
	return "https://cohub.live/" + url.PathEscape(d.OwnerProfile.Username) + "/" + url.PathEscape(d.Slug) + "/w/" + url.PathEscape(slug)
}

func (a *app) appView(c *ui.Context, spaceID, id string) {
	p := a.pal
	list, err, ok := loadJSON[struct {
		Apps []*appRec `json:"apps"`
	}](a, "apps:"+spaceID, "/api/apps/space/"+url.PathEscape(spaceID))
	var ap *appRec
	if list != nil {
		for _, x := range list.Apps {
			if x.ID == id {
				ap = x
			}
		}
	}
	if !ok || err != nil || ap == nil {
		a.loadingBlock(c, err, 200)
		return
	}
	manage := a.can(spaceID, "app.manage")
	public := a.appURL(spaceID, ap.Slug)
	reload := "apps:" + spaceID
	ui.Scroll(c.Key("app-" + id)).Grow(1).Children(func() {
		ui.Column(c).Padding(20).Gap(20).Children(func() {
			ui.Column(c.Key("head")).Gap(10).Padding(0, 0, 18, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle.Alpha(0.7)).Children(func() {
				title := ap.Meta.Title
				if title == "" {
					title = ap.Slug
				}
				ui.Text(c, title).FontSize(24).FontWeight(600).LetterSpacing(-0.3)
				ui.Row(c).Gap(10).AlignItems(ui.Center).Wrap().Children(func() {
					badge := func(label string, col ui.Color) {
						ui.Text(c.Key(label), label).FontSize(10).FontWeight(500).TextColor(col).Padding(2, 7).Radius(99).Background(col.Alpha(0.12))
					}
					if ap.Status == "published" {
						badge(m("resource_app_published"), p.success)
					} else {
						badge(m("resource_app_disabled"), p.textTertiary)
					}
					vis := map[string]string{"public": m("resource_app_public")}[ap.Visibility]
					if vis == "" {
						vis = ap.Visibility
					}
					badge(vis, p.info)
					ui.Text(c, "/"+ap.Slug).FontSize(11).Font(fontMono).TextColor(p.textTertiary)
					a.copyChip(c, "app-id", ap.ID[:8], ap.ID)
				})
				ui.Row(c.Key("actions")).Gap(8).Wrap().Children(func() {
					btn := func(key, ic, label string, brand, danger bool) bool {
						b := ui.ButtonBase(c.Key(key)).Row().Gap(6).AlignItems(ui.Center).Height(32).Padding(0, 12).Radius(5).FocusRing(false).Children(func() {
							col := p.textSecond
							switch {
							case brand:
								col = p.brandFg
							case danger:
								col = p.errorC
							}
							ui.Icon(c, icon(ic)).Size(14, 14).TextColor(col)
							ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(col)
						})
						switch {
						case brand:
							b.Background(p.brand)
						default:
							b.Background(p.bgElevated)
							if b.Hovered() {
								b.Background(p.hover)
							}
						}
						return b.Clicked()
					}
					if ap.Status == "published" && public != "" && btn("open", "external-link", m("resource_app_open"), true, false) {
						c.OpenURL(public)
					}
					if !manage {
						return
					}
					if ap.Status == "published" {
						if btn("disable", "power", m("resource_app_disable"), false, false) {
							a.act(func(ctx context.Context) error {
								return a.api.do(ctx, "PATCH", "/api/apps/"+url.PathEscape(ap.ID), map[string]any{"status": "disabled"}, nil)
							}, reload)
						}
					} else if btn("publish", "rocket", m("resource_app_publish"), false, false) {
						a.act(func(ctx context.Context) error {
							return a.api.do(ctx, "PATCH", "/api/apps/"+url.PathEscape(ap.ID), map[string]any{"status": "published"}, nil)
						}, reload)
					}
					if btn("delete", "trash-2", m("common_delete"), false, true) {
						a.confirm(m("resource_app_delete_app_title"), m("resource_app_delete_app_message", "title", title), m("common_delete"), func() {
							a.act(func(ctx context.Context) error {
								err := a.api.do(ctx, "DELETE", "/api/apps/"+url.PathEscape(ap.ID), nil, nil)
								if err == nil {
									a.update(func() { a.closeWindow() })
								}
								return err
							}, reload)
						})
					}
				})
			})
			a.appStatsView(c, ap.ID)
			ui.Column(c.Key("target")).Gap(8).Children(func() {
				ui.Row(c).AlignItems(ui.End).Children(func() {
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Text(c, strings.ToUpper(m("resource_app_target"))).FontSize(10).FontWeight(500).LetterSpacing(1.6).TextColor(p.placeholder)
						ui.Text(c, fmt.Sprintf("v%d", ap.LatestVersion)).FontSize(11).Font(fontMono).TextColor(p.placeholder)
					})
					if manage && ap.Status == "published" {
						ub := ui.ButtonBase(c.Key("update")).Row().Gap(6).AlignItems(ui.Center).Height(30).Padding(0, 10).Radius(5).Background(p.text).FocusRing(false).Children(func() {
							ui.Icon(c, icon("rocket")).Size(13, 13).TextColor(p.bgPrimary)
							ui.Text(c, m("resource_app_update_version")).FontSize(12).FontWeight(500).TextColor(p.bgPrimary)
						})
						if ub.Clicked() {
							a.act(func(ctx context.Context) error {
								err := a.api.do(ctx, "POST", "/api/apps/"+url.PathEscape(ap.ID)+"/versions", nil, nil)
								if err == nil {
									a.update(func() { a.notify(m("resource_app_published_version", "version", ap.LatestVersion+1)) })
								}
								return err
							}, reload)
						}
					}
				})
				ui.Row(c).Radius(8).Background(p.bgElevated.Alpha(0.4)).Border(1, p.borderSubtle.Alpha(0.6)).Clip().Children(func() {
					ui.Box(c).Width(3).Background(p.brand)
					ui.Column(c).Padding(14, 18).Gap(6).Grow(1).Children(func() {
						ui.Text(c, ap.TargetRef).FontSize(13).Font(fontMono)
						kind := map[string]string{"file": "resource_app_target_file", "directory": "resource_app_target_directory", "port": "resource_app_target_port"}[ap.TargetType]
						if kind != "" {
							ui.Text(c, m(kind)).FontSize(12).TextColor(p.textTertiary)
						}
					})
				})
				ui.Column(c).Gap(4).Padding(10, 12).Radius(7).Background(p.bgElevated.Alpha(0.3)).Children(func() {
					ui.Text(c, strings.ToUpper(m("app_view_app_permissions"))).FontSize(10).FontWeight(500).LetterSpacing(0.6).TextColor(p.placeholder)
					scopes := "—"
					if len(ap.AppScopes) > 0 {
						scopes = strings.Join(ap.AppScopes, ", ")
					}
					ui.Text(c, scopes).FontSize(12).Font(fontMono)
				})
			})
			if public != "" {
				ui.Row(c.Key("url")).Gap(8).AlignItems(ui.Center).Padding(8, 12).Radius(6).Background(p.bgElevated.Alpha(0.3)).Children(func() {
					l := ui.Text(c, strings.TrimPrefix(public, "https://")).FontSize(12).Font(fontMono).TextColor(p.textSecond).SingleLine().Grow(1).MinWidth(0).Cursor(ui.CursorPointer)
					if l.Clicked() {
						c.OpenURL(public)
					}
					a.copyChip(c, "public", "", public)
				})
			}
		})
	})
}

func (a *app) appStatsView(c *ui.Context, id string) {
	p := a.pal
	st, err, _ := loadJSON[appStats](a, "app-stats:"+id, "/api/apps/"+url.PathEscape(id)+"/stats")
	ui.Column(c.Key("stats")).Gap(10).Children(func() {
		ui.Text(c, strings.ToUpper(m("app_stats_views"))).FontSize(10).FontWeight(500).LetterSpacing(1.6).TextColor(p.placeholder)
		switch {
		case st == nil && err == nil:
			ui.Text(c, m("app_stats_loading")).FontSize(12).TextColor(p.textTertiary)
			return
		case err != nil:
			ui.Text(c, m("app_stats_unavailable")).FontSize(12).TextColor(p.textTertiary)
			return
		}
		ui.Row(c).Gap(12).Children(func() {
			for _, kv := range []struct {
				k string
				v int
			}{{m("app_stats_total"), st.Summary.TotalViews}, {m("app_stats_last_24h"), st.Summary.Views24h}, {m("app_stats_last_7d"), st.Summary.Views7d}, {m("app_stats_likes"), st.Summary.TotalLikes}} {
				ui.Column(c.Key(kv.k)).Grow(1).Basis(0).Gap(4).Children(func() {
					ui.Text(c, fmt.Sprint(kv.v)).FontSize(19).Font(fontMono)
					ui.Text(c, kv.k).FontSize(10).TextColor(p.placeholder)
				})
			}
		})
		mx := 1
		for _, d := range st.Daily {
			mx = max(mx, d.Views)
		}
		ui.Column(c.Key("chart")).Gap(4).Children(func() {
			ui.Text(c, m("app_stats_activity_30d")).FontSize(10).TextColor(p.placeholder)
			ui.Row(c).Height(48).Gap(2).AlignItems(ui.End).Children(func() {
				for _, d := range st.Daily {
					h := float32(d.Views) / float32(mx) * 48
					bar := ui.Box(c.Key(d.Date)).Grow(1).Basis(0).Height(max(h, 2)).Radius(2).Tooltip(m("app_stats_day_title", "date", d.Date, "count", d.Views))
					if d.Views > 0 {
						bar.Background(p.brand.Alpha(0.75))
					} else {
						bar.Background(p.borderSubtle)
					}
				}
			})
		})
	})
}

type installedApps struct {
	Format  string           `json:"format"`
	Version int              `json:"version"`
	Apps    []map[string]any `json:"apps"`
}

// appsPanel is AppsPanel.svelte: the Space's published Apps, then the
// Apps installed in it (.cohub/apps.json), which can be turned off or
// removed.
func (a *app) appsPanel(c *ui.Context, spaceID string) {
	p := a.pal
	apps, aerr, ok := loadJSON[struct {
		Apps []*appRec `json:"apps"`
	}](a, "apps:"+spaceID, "/api/apps/space/"+url.PathEscape(spaceID))
	ui.Scroll(c.Key("apps")).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Padding(4, 6, 12, 6).Children(func() {
			count := ""
			if apps != nil {
				count = fmt.Sprint(len(apps.Apps))
			}
			a.panelSection(c, "published", m("apps_panel_published"), count, func() {
				switch {
				case !ok:
					a.spinner(c, 14, p.textTertiary)
				case aerr != nil:
					ui.Text(c, m("apps_panel_load_failed")).FontSize(12).TextColor(p.errorC).Padding(6)
				case len(apps.Apps) == 0:
					ui.Text(c, m("apps_panel_published_empty")).FontSize(12).TextColor(p.textTertiary).Padding(6)
				}
				if apps == nil {
					return
				}
				for _, ap := range apps.Apps {
					ap := ap
					title := ap.title()
					on := a.route.kind == routeSpaceApp && a.route.sub == ap.ID
					row := ui.Row(c.Key(ap.ID)).Gap(10).AlignItems(ui.Center).Padding(5, 6).Radius(6).Cursor(ui.CursorPointer).Children(func() {
						a.avatarSeed(c, "", title, ap.ID, 28, 8)
						ui.Column(c).Grow(1).MinWidth(0).Children(func() {
							t := ui.Text(c, title).FontSize(12).SingleLine()
							if ap.Status != "published" {
								t.TextColor(p.textTertiary)
							}
							ui.Text(c, ap.Slug).FontSize(10).TextColor(p.placeholder).SingleLine()
						})
					})
					a.pressable(row, on)
					if row.Clicked() {
						a.go_(route{kind: routeSpaceApp, space: spaceID, sub: ap.ID})
					}
				}
			})
			a.installedApps(c, spaceID)
		})
	})
}

func (a *app) installedApps(c *ui.Context, spaceID string) {
	p := a.pal
	key := "file:" + spaceID + ":.cohub/apps.json"
	f, ferr, ok := loadJSON[fsFile](a, key, sp(spaceID)+"/fs/file?path="+url.QueryEscape(".cohub/apps.json"))
	var doc installedApps
	if f != nil && f.Kind == "text" {
		json.Unmarshal([]byte(f.Content), &doc)
	}
	edit := a.can(spaceID, "file.edit")
	a.panelSectionMeta(c, "installed", m("apps_panel_installed"), fmt.Sprint(len(doc.Apps)), func() {
		mb := ui.ButtonBase(c.Key("market")).Row().Gap(4).AlignItems(ui.Center).Height(24).Padding(0, 6).Radius(5).FocusRing(false).Children(func() {
			ui.Icon(c, icon("package-open")).Size(14, 14).TextColor(p.textTertiary)
			ui.Text(c, m("apps_panel_marketplace")).FontSize(11).TextColor(p.textTertiary)
		})
		if mb.Hovered() {
			mb.Background(p.hover)
		}
		if mb.Clicked() {
			c.OpenURL("https://cohub.live/spaces/" + spaceID + "?window=app%3Amarketplace")
		}
	}, func() {
		switch {
		case !ok:
			a.spinner(c, 14, p.textTertiary)
			return
		case ferr != nil || len(doc.Apps) == 0:
			ui.Text(c, m("apps_panel_installed_empty")).FontSize(12).TextColor(p.textTertiary).Padding(6)
			return
		}
		save := func(next []map[string]any) {
			doc.Apps = next
			b, _ := json.MarshalIndent(doc, "", "  ")
			a.act(func(ctx context.Context) error {
				return a.api.do(ctx, "PUT", sp(spaceID)+"/fs/file", map[string]any{"path": ".cohub/apps.json", "content": string(b) + "\n", "encoding": "utf-8"}, nil)
			}, key)
		}
		for i, ap := range doc.Apps {
			i := i
			snap, _ := ap["snapshot"].(map[string]any)
			name, _ := snap["name"].(string)
			iconURL, _ := snap["icon"].(string)
			ref, _ := ap["ref"].(string)
			link, _ := ap["url"].(string)
			enabled, _ := ap["enabled"].(bool)
			id, _ := ap["id"].(string)
			row := ui.Row(c.Key(id)).Gap(10).AlignItems(ui.Center).Padding(5, 6).Radius(6).Cursor(ui.CursorPointer)
			hover := row.Hovered()
			row.Children(func() {
				a.avatarSeed(c, iconURL, name, ref, 28, 8)
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					t := ui.Text(c, name).FontSize(12).SingleLine()
					if !enabled {
						t.TextColor(p.textTertiary)
					}
					ui.Text(c, ref).FontSize(10).TextColor(p.placeholder).SingleLine()
				})
				on := enabled
				sw := ui.Switch(c.Key("on"), &on)
				if !edit {
					sw.Disabled(true)
				} else if sw.Changed() {
					next := append([]map[string]any(nil), doc.Apps...)
					cp := map[string]any{}
					for k, v := range next[i] {
						cp[k] = v
					}
					cp["enabled"] = on
					next[i] = cp
					save(next)
				}
				if edit && hover {
					if a.iconButton(c.Key("rm"), "trash-2", m("apps_panel_uninstall"), false).Clicked() {
						a.confirm(m("apps_panel_uninstall"), m("apps_panel_uninstall_confirm", "name", name), m("apps_panel_uninstall"), func() {
							next := append(append([]map[string]any(nil), doc.Apps[:i]...), doc.Apps[i+1:]...)
							save(next)
						})
					}
				}
			})
			if hover {
				row.Background(p.hover)
			}
			if row.Clicked() && link != "" {
				c.OpenURL(link)
			}
		}
	})
}
