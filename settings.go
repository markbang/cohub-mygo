package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

// settingsPage is /settings/:section, as features/settings.
func (a *app) settingsPage(c *ui.Context) {
	switch a.route.sub {
	case "activity":
		a.activityPage(c, "")
	case "referrals":
		a.referralsPage(c)
	case "billing":
		a.billingPage(c)
	case "rules":
		a.rulesPage(c)
	case "channels":
		a.channelsPage(c)
	default:
		a.generalSettings(c)
	}
}

// settingsHeaderless gives a page without a header a strip that drags the
// window, under the title bar's buttons.
func (a *app) settingsHeaderless(c *ui.Context) {
	if a.compact {
		a.header(c, func() {
			if a.iconButton(c.Key("back"), "arrow-left", m("common_back"), false).Clicked() {
				a.goBackFromSettings()
			}
			ui.Spacer(c)
		})
		return
	}
	ui.Box(c.Key("drag")).Height(8).Shrink(0).DragWindow()
}

// settingsScroll is a settings page's column: padded, at most maxWidth wide.
func (a *app) settingsScroll(c *ui.Context, maxWidth float32, fn func()) {
	a.settingsHeaderless(c)
	ui.Scroll(c.Key("settings-" + a.route.sub)).Grow(1).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(maxWidth+48).Padding(20, 24, 28, 24).Children(fn)
	})
}

func (a *app) settingsTitle(c *ui.Context, title, sub string, actions func()) {
	p := a.pal
	ui.Row(c.Key("title")).Gap(12).AlignItems(ui.Start).Padding(0, 0, 20, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, title).FontSize(18).FontWeight(600).LetterSpacing(-0.2)
			if sub != "" {
				ui.Text(c, sub).FontSize(13).LineHeight(1.5).TextColor(p.textTertiary)
			}
		})
		if actions != nil {
			actions()
		}
	})
}

// optionCard is a choice of the settings' grids: an icon tile, a label and
// a hint, the chosen one on the brand's background with a check.
func (a *app) optionCard(c *ui.Context, iconName, label, hint string, on bool, choose func()) ui.Element {
	p := a.pal
	b := ui.ButtonBase(c.Key(label)).Row().Gap(8).AlignItems(ui.Center).Justify(ui.Start).Padding(10, 12).Radius(6).Grow(1).Basis(0).MinWidth(0).FocusRing(false).Children(func() {
		tile := ui.Box(c).Size(28, 28).Radius(5).Center().Shrink(0)
		col := p.textTertiary
		if on {
			tile.Background(p.brand.Alpha(0.15))
			col = p.brand
		} else {
			tile.Background(p.hoverStrong)
		}
		tile.Children(func() { ui.Icon(c, icon(iconName)).Size(14, 14).TextColor(col) })
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			tc := p.textTertiary
			if on {
				tc = p.text
			}
			ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(tc).SingleLine()
			ui.Text(c, hint).FontSize(10).TextColor(p.textTertiary).SingleLine()
		})
		if on {
			ui.Icon(c, icon("check")).Size(14, 14).TextColor(p.brand)
		}
	})
	switch {
	case on:
		b.Background(p.brandBg)
	case b.Hovered():
		b.Background(p.hover)
	}
	if b.Clicked() {
		choose()
	}
	return b
}

// grid lays cards out in rows of n.
func grid(c *ui.Context, n int, count int, cell func(i int)) {
	ui.Column(c).Gap(8).Children(func() {
		for r := 0; r*n < count; r++ {
			ui.Row(c.Key(r)).Gap(8).Children(func() {
				for i := r * n; i < r*n+n; i++ {
					if i < count {
						cell(i)
					} else {
						ui.Box(c.Key(i)).Grow(1).Basis(0)
					}
				}
			})
		}
	})
}

var themeOptions = []struct{ value, icon, label, hint string }{
	{"dark", "moon", "settings_theme_dark", "settings_theme_dark_description"},
	{"light", "sun", "settings_theme_light", "settings_theme_light_description"},
	{"solarized-dark", "palette", "settings_theme_solarized_dark", "settings_theme_solarized_dark_description"},
	{"solarized-light", "palette", "settings_theme_solarized_light", "settings_theme_solarized_light_description"},
	{"neta-studio", "palette", "settings_theme_neta_studio", "settings_theme_neta_studio_description"},
	{"neta-pixel", "palette", "settings_theme_neta_pixel", "settings_theme_neta_pixel_description"},
	{"neta-pixel-dark", "palette", "settings_theme_neta_pixel_dark", "settings_theme_neta_pixel_dark_description"},
	{"system", "monitor", "settings_language_system", "settings_theme_system_description"},
}

// generalSettings is GeneralSettings.svelte: the profile, the language and
// the theme.
func (a *app) generalSettings(c *ui.Context) {
	p := a.pal
	a.settingsScroll(c, 672, func() {
		a.settingsTitle(c, m("nav_general"), m("settings_general_description"), nil)
		ui.Row(c.Key("profile")).Gap(16).AlignItems(ui.Start).Padding(48, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
			name, av, user, mail, id := "", "", "", "", ""
			if a.me != nil {
				mail, id = a.me.Email, a.me.UUID
				if pr := a.me.Profile; pr != nil {
					name, av, user = pr.name(), pr.AvatarURL, pr.Username
				}
			}
			ui.Column(c).Width(64).AlignItems(ui.Center).Gap(6).Shrink(0).Children(func() {
				a.avatar(c, av, name, 56, 28)
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icon("upload")).Size(12, 12).TextColor(p.textTertiary)
					ui.Text(c, m("common_change")).FontSize(11).TextColor(p.textTertiary)
				})
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(6).Padding(2, 0).Children(func() {
				ui.Text(c, name).FontSize(15).FontWeight(500)
				ui.Text(c, "@"+user).FontSize(12).TextColor(p.textTertiary)
				kv := func(k, v string, copy bool) {
					ui.Row(c.Key(k)).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, k).FontSize(11).LetterSpacing(0.6).TextColor(p.textTertiary)
						ui.Text(c, v).FontSize(11).TextColor(p.textTertiary).Font(fontOf(copy))
						if copy {
							b := ui.ButtonBase(c.Key("copy")).Size(20, 20).Center().Radius(4).Label(m("settings_copy_uuid")).FocusRing(false).Children(func() {
								ui.Icon(c, icon("copy")).Size(12, 12).TextColor(p.textTertiary)
							})
							if b.Clicked() {
								c.WriteClipboard(id)
							}
						}
					})
				}
				if mail != "" {
					kv("EMAIL", mail, false)
				}
				if len(id) > 12 {
					kv("ID", id[:8]+"…"+id[len(id)-4:], true)
				}
			})
		})
		ui.Column(c.Key("lang")).Gap(4).Padding(48, 0, 24, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
			ui.Text(c, m("settings_language")).FontSize(13).FontWeight(500)
			ui.Text(c, m("settings_language_description")).FontSize(12).TextColor(p.textTertiary)
			ui.Box(c).Height(12)
			langs := []struct{ v, l, h string }{
				{"system", "settings_language_system", "settings_language_system_description"},
				{"en", "settings_language_english", "settings_language_english_description"},
				{"zh-CN", "settings_language_chinese", "settings_language_chinese_description"},
			}
			cur := a.prefs.Lang
			if cur == "" {
				cur = "system"
			}
			grid(c, 3, len(langs), func(i int) {
				l := langs[i]
				a.optionCard(c, "languages", m(l.l), m(l.h), cur == l.v, func() { a.setLang(l.v) })
			})
		})
		ui.Column(c.Key("theme")).Gap(4).Padding(24, 0).Children(func() {
			ui.Text(c, m("settings_theme")).FontSize(13).FontWeight(500)
			ui.Text(c, m("settings_theme_description")).FontSize(12).TextColor(p.textTertiary)
			ui.Box(c).Height(12)
			grid(c, 3, len(themeOptions), func(i int) {
				o := themeOptions[i]
				label := m(o.label)
				a.optionCard(c, o.icon, label, m(o.hint), a.theme == o.value, func() { a.setTheme(o.value) })
			})
			ui.Row(c.Key("custom")).Gap(8).AlignItems(ui.Center).Padding(10, 12).Radius(6).Border(1, p.borderSubtle).Margin(8, 0, 0, 0).Children(func() {
				on := a.customThemeOn()
				tile := ui.Box(c).Size(28, 28).Radius(5).Center()
				col := p.textTertiary
				if on {
					tile.Background(p.brand.Alpha(0.15))
					col = p.brand
				} else {
					tile.Background(p.hoverStrong)
				}
				tile.Children(func() { ui.Icon(c, icon("paintbrush")).Size(14, 14).TextColor(col) })
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, m("settings_custom_theme")).FontSize(12).FontWeight(500)
					ui.Text(c, m("settings_custom_theme_description", "path", ".cohub/theme.css")).FontSize(10).TextColor(p.textTertiary).SingleLine()
				})
				ui.Text(c, m("common_edit")).FontSize(11).TextColor(p.textTertiary).Padding(4, 8)
				v := on
				if ui.Switch(c, &v).Label(m("settings_custom_theme_toggle")).Changed() {
					a.prefs.CustomTheme = &v
					a.prefs.save()
				}
			})
		})
	})
}

func fontOf(mono bool) string {
	if mono {
		return fontMono
	}
	return fontSans
}

func (a *app) customThemeOn() bool { return a.prefs.CustomTheme == nil || *a.prefs.CustomTheme }

func (a *app) setLang(v string) {
	a.prefs.Lang = v
	a.prefs.save()
	setLocale(v)
}

// loadJSON loads a page's JSON once into pages.json[key].
func loadJSON[T any](a *app, key, path string) (*T, error, bool) {
	a.once(key, func(ctx context.Context) {
		var v T
		err := a.api.get(ctx, path, &v)
		a.update(func() {
			if err != nil {
				a.pages.json[key] = err
				return
			}
			a.pages.json[key] = &v
		})
	})
	switch v := a.pages.json[key].(type) {
	case *T:
		return v, nil, true
	case error:
		return nil, v, true
	}
	return nil, nil, false
}

func (a *app) loadingBlock(c *ui.Context, err error, height float32) {
	p := a.pal
	ui.Column(c.Key("loading")).Height(height).Center().Gap(6).Children(func() {
		if err != nil {
			ui.Text(c, err.Error()).FontSize(12).TextColor(p.errorC).Font(fontMono)
			return
		}
		a.spinner(c, 16, p.textTertiary)
	})
}

// button is the settings' small button: bordered, or the brand's.
func (a *app) button(c *ui.Context, iconName, label string, brand bool) ui.Element {
	p := a.pal
	b := ui.ButtonBase(c.Key(label)).Row().Gap(6).AlignItems(ui.Center).Height(32).Padding(0, 10).Radius(5).FocusRing(false).Shrink(0).Children(func() {
		col := p.textSecond
		if brand {
			col = p.brand
		}
		if iconName != "" {
			ui.Icon(c, icon(iconName)).Size(14, 14).TextColor(col)
		}
		t := ui.Text(c, label).FontSize(12).TextColor(col)
		if brand {
			t.FontWeight(500)
		}
	})
	if brand {
		b.Background(p.brandMuted).Border(1, p.brandBorder)
		if b.Hovered() {
			b.Background(p.brand.Alpha(0.15))
		}
	} else {
		b.Background(p.bgSurface).Border(1, p.borderSubtle)
		if b.Hovered() {
			b.Background(p.hover)
		}
	}
	return b
}

type referralDashboard struct {
	Code   string `json:"code"`
	Reward struct {
		InviterUSD float64 `json:"inviterUsd"`
		InviteeUSD float64 `json:"inviteeUsd"`
	} `json:"reward"`
	Quota struct {
		Limit, CycleDays, ClaimHours, Used, Reserved, Available int
	} `json:"quota"`
	Summary struct {
		Total     int     `json:"total"`
		Pending   int     `json:"pending"`
		EarnedUSD float64 `json:"earnedUsd"`
	} `json:"summary"`
	Items []struct {
		ID        string    `json:"id"`
		Status    string    `json:"status"`
		ClaimedAt time.Time `json:"claimedAt"`
		Profile   *Profile  `json:"profile"`
	} `json:"items"`
}

func (a *app) referralsPage(c *ui.Context) {
	p := a.pal
	d, err, ok := loadJSON[referralDashboard](a, "referrals", "/api/me/referrals")
	a.settingsScroll(c, 672, func() {
		a.settingsTitle(c, m("page_title_referrals"), m("referral_share_subtitle"), nil)
		if d == nil || !ok {
			a.loadingBlock(c, err, 176)
			return
		}
		url := "https://cohub.live/referrals/" + d.Code
		ui.Column(c.Key("body")).Gap(20).Padding(24, 0).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
				ui.Box(c).Size(32, 32).Radius(6).Background(p.brandMuted).Center().Children(func() {
					ui.Icon(c, icon("gift")).Size(16, 16).TextColor(p.brand)
				})
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, m("referral_give_get", "inviteeAmount", fmt.Sprintf("US$%.2f", d.Reward.InviteeUSD), "inviterAmount", fmt.Sprintf("US$%.2f", d.Reward.InviterUSD))).FontSize(16).FontWeight(600)
					ui.Text(c, m("referral_both_credits")).FontSize(12).TextColor(p.textTertiary)
				})
			})
			ui.Row(c).Gap(8).Children(func() {
				ui.Row(c).Grow(1).Height(36).Padding(0, 10).AlignItems(ui.Center).Radius(5).Border(1, p.borderSubtle).Background(p.bgInput).Children(func() {
					ui.Text(c, url).FontSize(12).Font(fontMono).TextColor(p.textSecond).SingleLine()
				})
				cp := ui.ButtonBase(c.Key("copy")).Row().Gap(6).AlignItems(ui.Center).Height(36).Padding(0, 12).Radius(5).Background(p.brand).FocusRing(false).Children(func() {
					ui.Icon(c, icon("copy")).Size(14, 14).TextColor(p.brandFg)
					ui.Text(c, m("referral_copy_link")).FontSize(12).FontWeight(500).TextColor(p.brandFg)
				})
				if cp.Clicked() {
					c.WriteClipboard(url)
				}
				ui.Row(c.Key("share")).Gap(6).AlignItems(ui.Center).Height(36).Padding(0, 12).Radius(5).Border(1, p.borderSubtle).Children(func() {
					ui.Icon(c, icon("share-2")).Size(14, 14).TextColor(p.textSecond)
					ui.Text(c, m("referral_share")).FontSize(12).TextColor(p.textSecond)
				})
			})
			ui.Row(c).Gap(32).Padding(16, 0).BorderWidth(1, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
				for _, s := range []struct{ v, l string }{
					{fmt.Sprintf("%d/%d", d.Quota.Used, d.Quota.Limit), m("referral_cycle_successful")},
					{fmt.Sprint(d.Summary.Pending), m("referral_cycle_pending")},
					{fmt.Sprint(d.Quota.Available), m("referral_cycle_available")},
					{fmt.Sprintf("US$%.2f", d.Summary.EarnedUSD), m("referral_credits_earned")},
				} {
					ui.Column(c.Key(s.l)).Gap(2).Children(func() {
						ui.Text(c, s.v).FontSize(17).FontWeight(600).Font(fontMono)
						ui.Text(c, s.l).FontSize(11).TextColor(p.textTertiary)
					})
				}
			})
		})
		ui.Row(c.Key("list-head")).AlignItems(ui.Center).Padding(0, 0, 12, 0).Children(func() {
			ui.Text(c, m("page_title_referrals")).FontSize(13).FontWeight(500)
			ui.Text(c, fmt.Sprintf(" · %d", d.Summary.Total)).FontSize(13).TextColor(p.textTertiary).Grow(1)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(0, 8).Children(func() {
				ui.Icon(c, icon("refresh-cw")).Size(12, 12).TextColor(p.textTertiary)
				ui.Text(c, m("referral_replace_link")).FontSize(11).TextColor(p.textTertiary)
			})
		})
		ui.Column(c.Key("list")).BorderWidth(1, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
			if len(d.Items) == 0 {
				ui.Column(c).Padding(32, 0).AlignItems(ui.Center).Gap(4).Children(func() {
					ui.Text(c, m("referral_no_referrals")).FontSize(12).TextColor(p.textTertiary)
					ui.Text(c, m("referral_share_to_start")).FontSize(11).TextColor(p.placeholder)
				})
			}
			for _, it := range d.Items {
				ui.Row(c.Key(it.ID)).Height(48).Gap(12).AlignItems(ui.Center).Children(func() {
					name := m("referral_c_user")
					av := ""
					if it.Profile != nil {
						name, av = it.Profile.name(), it.Profile.AvatarURL
					}
					a.avatar(c, av, name, 28, 14)
					ui.Column(c).Grow(1).Children(func() {
						ui.Text(c, name).FontSize(13)
						ui.Text(c, m("referral_joined", "date", it.ClaimedAt.Local().Format("2006/1/2"))).FontSize(11).TextColor(p.textTertiary)
					})
					ui.Text(c, it.Status).FontSize(11).TextColor(p.textTertiary)
				})
			}
		})
		ui.Text(c.Key("foot"), m("referral_footnote", "limit", d.Quota.Limit, "days", d.Quota.CycleDays, "hours", d.Quota.ClaimHours)).FontSize(10).LineHeight(1.6).TextColor(p.placeholder).Margin(16, 0, 0, 0)
	})
}

type userRules struct {
	Content     string    `json:"content"`
	Path        string    `json:"path"`
	PublishedAt time.Time `json:"publishedAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (a *app) rulesPage(c *ui.Context) {
	p := a.pal
	r, err, ok := loadJSON[userRules](a, "rules", "/api/me/rules")
	a.settingsScroll(c, 768, func() {
		ui.Row(c.Key("title")).Gap(12).AlignItems(ui.Start).Children(func() {
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				ui.Text(c, m("page_title_rules")).FontSize(18).FontWeight(600)
				ui.Text(c, m("rules_description")).FontSize(13).LineHeight(1.5).TextColor(p.textTertiary)
				for i, k := range []string{"rules_step1", "rules_step2", "rules_step3"} {
					ui.Row(c.Key(k)).Gap(4).Children(func() {
						ui.Text(c, fmt.Sprintf("%d.", i+1)).FontSize(13).FontWeight(500).TextColor(p.textSecond)
						ui.Text(c, m(k, "file", "AGENTS.md")).FontSize(13).TextColor(p.textTertiary)
					})
				}
			})
			if a.button(c, "refresh-cw", m("rules_refresh"), false).Clicked() {
				a.reload("rules")
			}
			a.button(c, "external-link", m("rules_open_home"), true)
		})
		ui.Row(c.Key("warn")).Gap(10).Padding(12).Radius(6).Background(p.warning.Alpha(0.12)).Margin(20, 0, 0, 0).Children(func() {
			ui.Icon(c, icon("shield-alert")).Size(16, 16).TextColor(p.warning)
			ui.Text(c, m("rules_warning")).FontSize(12).LineHeight(1.6).TextColor(p.textTertiary).Grow(1)
		})
		if r == nil || !ok {
			a.loadingBlock(c, err, 160)
			return
		}
		ui.Row(c.Key("meta")).Gap(12).Margin(24, 0, 12, 0).Children(func() {
			card := func(k, v string) {
				ui.Column(c.Key(k)).Grow(1).Basis(0).Gap(6).Padding(10, 12).Radius(6).Border(1, p.borderSubtle).Background(p.bgSurface).Children(func() {
					ui.Text(c, k).FontSize(10).TextColor(p.textTertiary)
					ui.Text(c, v).FontSize(13).Font(fontMono)
				})
			}
			path := r.Path
			if path == "" {
				path = "/configs/user/AGENTS.md"
			}
			card(m("rules_published_file"), path)
			when := r.UpdatedAt
			if when.IsZero() {
				when = r.PublishedAt
			}
			v := m("rules_not_published_yet")
			if !when.IsZero() {
				v = longDate(when)
			}
			card(m("rules_updated"), v)
		})
		ui.Column(c.Key("preview")).Radius(6).Border(1, p.borderSubtle).Clip().Children(func() {
			ui.Row(c).Padding(8, 12).Background(p.bgSurface).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
				ui.Text(c, m("rules_preview")).FontSize(11).FontWeight(500).TextColor(p.textSecond).Grow(1)
				ui.Text(c, m("rules_read_only")).FontSize(10).TextColor(p.textTertiary)
			})
			ui.Text(c, r.Content).FontSize(12).Font(fontMono).LineHeight(1.7).TextColor(p.textSecond).Padding(12, 16).Selectable()
		})
	})
}

type channel struct {
	ID         string `json:"id"`
	Provider   string `json:"provider"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	BoundSpace *struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"boundSpace"`
	Health *struct {
		State   string `json:"state"`
		Message string `json:"message"`
	} `json:"health"`
}

func (a *app) channelsPage(c *ui.Context) {
	p := a.pal
	list, err, ok := loadJSON[[]channel](a, "channels", "/api/channels")
	a.settingsScroll(c, 672, func() {
		ui.Row(c.Key("title")).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				ui.Text(c, m("page_title_channels")).FontSize(18).FontWeight(600)
				ui.Text(c, m("channels_description")).FontSize(13).TextColor(p.textTertiary)
			})
			a.button(c, "plus", m("channels_add"), true)
		})
		if list == nil || !ok {
			a.loadingBlock(c, err, 160)
			return
		}
		ui.Column(c.Key("table")).Margin(24, 0, 0, 0).Radius(6).Border(1, p.borderSubtle).Clip().Children(func() {
			cols := func(fn ...func()) {
				widths := []float32{28, -1, 72, -1, 24}
				for i, f := range fn {
					cell := ui.Row(c.Key(i)).AlignItems(ui.Center)
					if widths[i] > 0 {
						cell.Width(widths[i]).Shrink(0)
					} else {
						cell.Grow(1).Basis(0).MinWidth(0)
					}
					cell.Children(f)
				}
			}
			ui.Row(c.Key("head")).Gap(12).Padding(8, 12).Background(p.bgSurface).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
				label := func(s string) func() {
					return func() {
						ui.Text(c, strings.ToUpper(s)).FontSize(10).FontWeight(500).LetterSpacing(0.8).TextColor(p.placeholder)
					}
				}
				cols(func() {}, label(m("channels_col_channel")), label(m("channels_col_status")), label(m("channels_col_bound_space")), func() {})
			})
			for i, ch := range *list {
				row := ui.Row(c.Key(ch.ID)).Gap(12).Padding(10, 12)
				if i > 0 {
					row.BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle)
				}
				row.Children(func() {
					cols(func() {
						ui.Box(c).Size(28, 28).Radius(5).Border(1, p.borderSubtle).Background(p.bgSurface).Center().Children(func() {
							ui.Row(c).Gap(1).AlignItems(ui.Center).Children(func() {
								a.dot(c, providerColor(ch.Provider, p), 8)
								ui.Icon(c, icon("network")).Size(12, 12).TextColor(p.textTertiary)
							})
						})
					}, func() {
						ui.Column(c).MinWidth(0).Children(func() {
							ui.Text(c, ch.Name).FontSize(13).FontWeight(500).SingleLine()
							ui.Text(c, strings.ToUpper(ch.Provider)).FontSize(10).LetterSpacing(0.6).TextColor(p.textTertiary)
						})
					}, func() {
						state := "unbound"
						if ch.Health != nil && ch.BoundSpace != nil {
							state = ch.Health.State
						}
						label, col := m("channel_health_ready"), p.success
						switch state {
						case "ready":
						case "unbound":
							label, col = m("channels_not_bound"), p.textTertiary
						default:
							label, col = state, p.warning
						}
						ui.Text(c, label).FontSize(10).FontWeight(500).TextColor(col).Padding(2, 6).Radius(999).Background(col.Alpha(0.12))
					}, func() {
						if ch.BoundSpace != nil {
							ui.Text(c, ch.BoundSpace.Title).FontSize(12).Font(fontMono).TextColor(p.textSecond).SingleLine()
						} else {
							ui.Text(c, m("channels_not_bound")).FontSize(12).TextColor(p.placeholder)
						}
					}, func() {
						col := p.placeholder
						if ch.BoundSpace == nil {
							col = p.textSecond
						}
						ui.Icon(c, icon("trash-2")).Size(14, 14).TextColor(col)
					})
				})
			}
		})
	})
}

func providerColor(provider string, p *palette) ui.Color {
	switch provider {
	case "qq":
		return ui.RGB(18, 183, 245)
	case "feishu":
		return ui.RGB(51, 112, 255)
	case "wechat":
		return ui.RGB(7, 193, 96)
	case "discord":
		return ui.RGB(88, 101, 242)
	}
	return p.textTertiary
}

var _ = markdown.View
