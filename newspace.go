package main

import (
	"context"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type newSpaceForm struct {
	name, slug, desc string
	source           int // 0 blank, 1 git, 2 save
	gitURL           string
	creating         bool
	err              string
}

// newSpacePage is /spaces/new: the form that creates a Space.
func (a *app) newSpacePage(c *ui.Context) {
	p := a.pal
	f := &a.newSpace
	bar := c.TitleBar()
	ui.Row(c.Key("head")).Height(headerHeight).Shrink(0).Gap(12).AlignItems(ui.Center).Padding(0, 16+bar.Right, 0, 16).
		Background(p.bgPrimary).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).DragWindow().Children(func() {
		back := ui.ButtonBase(c.Key("back")).Size(20, 20).Center().Label(m("x_back")).FocusRing(false).Children(func() {
			ui.Icon(c, icon("arrow-left")).Size(16, 16).TextColor(p.textTertiary)
		})
		if back.Clicked() {
			a.goBack()
		}
		ui.Box(c).Size(1, 16).Background(p.borderSubtle)
		ui.Text(c, m("space_new_title")).FontSize(11).FontWeight(500).TextColor(p.textSecond)
	})
	label := func(s string) {
		ui.Text(c, strings.ToUpper(s)).FontSize(10).FontWeight(500).LetterSpacing(0.8).TextColor(p.textTertiary).Margin(0, 0, 6, 0)
	}
	input := func(key string, v *string, ph string, mono bool) {
		in := ui.TextInputBase(c.Key(key), v).Placeholder(ph).PlaceholderColor(p.placeholder).FontSize(13).Padding(6, 12).Radius(5).
			Background(p.bgInput).Border(1, p.borderSubtle)
		if mono {
			in.Font(fontMono)
		}
	}
	card := func(key string, fn func()) {
		ui.Column(c.Key(key)).Gap(12).Padding(16).Radius(6).Border(1, p.borderSubtle).Background(p.bgSurface).Children(fn)
	}
	ui.Scroll(c.Key("form")).Grow(1).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(672).Padding(16).Gap(12).Children(func() {
			card("space", func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "SPACE").FontSize(10).FontWeight(500).LetterSpacing(0.8).TextColor(p.placeholder)
					ui.Text(c, "Create a new space. Sandbox provisioning and content bootstrap will run independently.").FontSize(13).TextColor(p.textTertiary)
				})
				ui.Column(c).Children(func() { label("Name"); input("name", &f.name, "my-product-space", true) })
				ui.Column(c).Children(func() {
					label("Slug")
					input("slug", &f.slug, "optional-url-name", true)
					ui.Text(c, "Optional. Lowercase letters, numbers, hyphens, or underscores.").FontSize(11).TextColor(p.placeholder).Margin(4, 0, 0, 0)
				})
				ui.Column(c).Children(func() {
					label("Description")
					ui.TextAreaBase(c.Key("desc"), &f.desc).Lines(3, 6).Placeholder("Optional description").PlaceholderColor(p.placeholder).
						FontSize(13).Padding(8, 12).Radius(5).Background(p.bgInput).Border(1, p.borderSubtle)
				})
				ui.Column(c).Children(func() {
					label("Bootstrap source")
					ui.Row(c).Gap(8).Children(func() {
						for i, l := range []string{"Blank", m("space_new_git_repo"), "Save"} {
							on := f.source == i
							b := ui.ButtonBase(c.Key(l)).Row().Gap(8).AlignItems(ui.Center).Justify(ui.Start).Grow(1).Basis(0).Height(36).Padding(0, 10).Radius(5).
								Background(p.bgInput).Border(1, p.borderSubtle).FocusRing(false).Children(func() {
								ring := ui.Box(c).Size(14, 14).Radius(7).Border(1, p.border).Center()
								if on {
									ring.BorderColor(p.brand).Children(func() { ui.Box(c).Size(8, 8).Radius(4).Background(p.brand) })
								}
								ui.Text(c, l).FontSize(12).TextColor(p.textSecond)
							})
							if b.Clicked() {
								f.source = i
							}
						}
					})
					if f.source == 1 {
						ui.Box(c).Height(8)
						input("git", &f.gitURL, "https://github.com/owner/repo", true)
					}
				})
			})
			card("mods", func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icon("package")).Size(13, 13).TextColor(p.placeholder)
					ui.Text(c, "MOUNTED SPACES").FontSize(10).FontWeight(500).LetterSpacing(0.8).TextColor(p.placeholder)
				})
				ui.Text(c, m("space_new_mounted_readonly", "path", "/mods/<slug>")).FontSize(12).TextColor(p.textTertiary)
				var modID, modName, modSlug string
				ui.Row(c).Gap(8).Children(func() {
					for _, x := range []struct {
						k, ph string
						v     *string
						mono  bool
					}{{"mod-id", "Mod Space UUID", &modID, true}, {"mod-name", "Display name", &modName, false}, {"mod-slug", "Mount slug", &modSlug, true}} {
						ui.Column(c.Key(x.k)).Grow(1).Basis(0).Children(func() { input(x.k, x.v, x.ph, x.mono) })
					}
					ui.Row(c.Key("add")).Gap(4).AlignItems(ui.Center).Height(32).Padding(0, 12).Radius(5).Background(p.brand.Alpha(0.5)).Children(func() {
						ui.Icon(c, icon("plus")).Size(12, 12).TextColor(p.brandFg)
						ui.Text(c, "Add").FontSize(12).TextColor(p.brandFg)
					})
				})
				ui.Column(c).Gap(8).Padding(10, 12).Radius(5).Background(p.bgInput).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Grow(1).Gap(2).Children(func() {
							ui.Text(c, "Cohub Base").FontSize(12).FontWeight(500)
							ui.Text(c, "/mods/cohub_base").FontSize(10).Font(fontMono).TextColor(p.textTertiary)
						})
						ui.Text(c, "ENABLED").FontSize(10).FontWeight(500).TextColor(p.success).Padding(1, 6).Radius(3).Background(p.success.Alpha(0.15))
						ui.Text(c, "Disable").FontSize(11).TextColor(p.textTertiary)
						ui.Text(c, "Remove").FontSize(11).TextColor(p.textTertiary)
					})
					slug := "cohub_base"
					ui.Row(c).Children(func() { input("base-slug", &slug, "", true) })
				})
			})
			card("env", func() {
				ui.Text(c, m("space_new_env_vars")).FontSize(10).FontWeight(500).LetterSpacing(0.8).TextColor(p.placeholder)
				ui.Text(c, "Optional env vars injected into the space environment.").FontSize(13).TextColor(p.textTertiary)
				ui.Text(c, m("space_new_no_env")).FontSize(13).TextColor(p.placeholder)
				ui.Row(c.Key("add-env")).Gap(6).AlignItems(ui.Center).AlignSelf(ui.Start).Height(32).Padding(0, 10).Radius(5).Border(1, p.borderSubtle).Background(p.bgInput).Children(func() {
					ui.Icon(c, icon("plus")).Size(13, 13).TextColor(p.textSecond)
					ui.Text(c, "Add env var").FontSize(12).TextColor(p.textSecond)
				})
			})
			if f.err != "" {
				ui.Text(c, f.err).FontSize(12).Font(fontMono).TextColor(p.errorC)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				cancel := ui.ButtonBase(c.Key("cancel")).Height(32).Padding(0, 12).Radius(5).Border(1, p.borderSubtle).Center().FocusRing(false).Children(func() {
					ui.Text(c, m("common_cancel")).FontSize(12).TextColor(p.textSecond)
				})
				if cancel.Clicked() {
					a.goBack()
				}
				label := m("space_new_create")
				if f.creating {
					label = m("space_new_creating")
				}
				create := ui.ButtonBase(c.Key("create")).Height(32).Padding(0, 14).Radius(5).Background(p.brand).Center().FocusRing(false).Children(func() {
					ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(p.brandFg)
				})
				if strings.TrimSpace(f.name) == "" {
					create.Opacity(0.5)
				} else if create.Clicked() && !f.creating {
					a.createSpace()
				}
			})
		})
	})
}

func (a *app) createSpace() {
	f := &a.newSpace
	if a.api == nil {
		f.err = "demo"
		return
	}
	f.creating, f.err = true, ""
	in := map[string]any{"name": strings.TrimSpace(f.name), "source": "web"}
	if s := strings.TrimSpace(f.slug); s != "" {
		in["slug"] = s
	}
	if d := strings.TrimSpace(f.desc); d != "" {
		in["description"] = d
	}
	switch f.source {
	case 1:
		in["bootstrapSource"] = map[string]any{"type": "git", "url": strings.TrimSpace(f.gitURL)}
	default:
		in["bootstrapSource"] = map[string]any{"type": "blank"}
	}
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		var out struct {
			Space *Space `json:"space"`
			ID    string `json:"id"`
		}
		err := a.api.do(ctx, "POST", "/api/spaces", in, &out)
		a.update(func() {
			f.creating = false
			if err != nil {
				f.err = err.Error()
				return
			}
			id := out.ID
			if out.Space != nil {
				id = out.Space.ID
			}
			a.newSpace = newSpaceForm{}
			if id != "" {
				a.openSpace(id)
			}
		})
		a.refresh()
	})
}
