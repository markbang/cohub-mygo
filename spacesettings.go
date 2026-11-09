package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"
)

type spaceDetail struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Slug          string         `json:"slug"`
	Description   string         `json:"description"`
	PublicProfile *publicProfile `json:"publicProfile"`
	OwnerProfile  *Profile       `json:"ownerProfile"`
	SandboxStatus string         `json:"sandboxStatus"`
	Meta          struct {
		Config struct {
			Sandbox struct {
				Spec        string `json:"spec"`
				Provider    string `json:"provider"`
				Image       string `json:"image"`
				AutoDestroy struct {
					Mode       string `json:"mode"`
					TTLSeconds int    `json:"ttlSeconds"`
				} `json:"autoDestroy"`
			} `json:"sandbox"`
		} `json:"config"`
		ExtraEnv []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"extraEnv"`
	} `json:"meta"`
}

var spaceSettingsSections = []struct{ id, icon, label string }{
	{"profile", "globe", "space_section_profile"},
	{"access", "users", "space_section_access"},
	{"environment", "terminal", "space_section_env"},
	{"channels", "network", "space_section_channels"},
	{"sandbox", "settings", "space_section_sandbox"},
}

// spaceSettings is /spaces/:id/settings: a header, the sections' nav and
// the sections, as the web's Space settings page.
func (a *app) spaceSettings(c *ui.Context) {
	p := a.pal
	id := a.route.space
	d, err, ok := loadJSON[spaceDetail](a, "space:"+id, sp(id))
	a.header(c, func() {
		if a.iconButton(c.Key("back"), "arrow-left", m("space_back"), false).Clicked() {
			a.go_(route{kind: routeSpace, space: id})
		}
		ui.Text(c, m("space_title")).FontSize(13).FontWeight(600).Grow(1)
		for _, it := range []struct{ icon, label string }{{"wallet", m("wallet_title")}, {"package", m("space_section_commerce")}} {
			b := ui.ButtonBase(c.Key(it.label)).Row().Gap(6).AlignItems(ui.Center).Height(28).Padding(0, 8).Radius(6).FocusRing(false).Children(func() {
				ui.Icon(c, icon(it.icon)).Size(14, 14).TextColor(p.textTertiary)
				ui.Text(c, it.label).FontSize(12).TextColor(p.textTertiary)
			})
			if b.Hovered() {
				b.Background(p.hover)
			}
			if it.icon == "wallet" && b.Clicked() {
				a.go_(route{kind: routeSpaceWallet, space: id})
			}
		}
	})
	ui.Row(c.Key("body")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
		if !a.compact {
			ui.Column(c.Key("nav")).Width(176).Shrink(0).Gap(2).Padding(12, 8).BorderWidth(0, 1, 0, 0).BorderColor(p.borderSubtle).Children(func() {
				for _, s := range spaceSettingsSections {
					on := a.route.sub == s.id
					row := ui.Row(c.Key(s.id)).Gap(10).AlignItems(ui.Center).Height(36).Padding(0, 8).Radius(5).Children(func() {
						col := p.textTertiary
						if on {
							col = p.text
						}
						ui.Icon(c, icon(s.icon)).Size(15, 15).TextColor(col)
						t := ui.Text(c, m(s.label)).FontSize(13).TextColor(col)
						if on {
							t.FontWeight(500)
						}
					})
					a.pressable(row, on)
					row.OnClick(func() { a.route.sub = s.id })
				}
			})
		}
		ui.Scroll(c.Key("main-"+a.route.sub)).Grow(1).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(720).Margin(0, ui.Auto).Padding(28, 24, 40, 24).Children(func() {
				if !ok || d == nil {
					a.loadingBlock(c, err, 200)
					return
				}
				switch a.route.sub {
				case "access":
					a.spaceAccess(c, d)
				case "environment":
					a.spaceEnv(c, d)
				case "channels":
					a.spaceChannels(c, d)
				case "sandbox":
					a.spaceSandbox(c, d)
				default:
					a.spaceProfile(c, d)
					ui.Box(c).Height(40)
					a.spaceAccess(c, d)
				}
			})
		})
	})
}

func (a *app) settingsHeading(c *ui.Context, title, desc string, action func()) {
	p := a.pal
	ui.Row(c.Key("h-"+title)).Gap(12).AlignItems(ui.Start).Children(func() {
		ui.Column(c).Grow(1).Gap(4).Children(func() {
			ui.Text(c, title).FontSize(18).FontWeight(600).LetterSpacing(-0.2)
			ui.Text(c, desc).FontSize(13).TextColor(p.textTertiary)
		})
		if action != nil {
			action()
		}
	})
}

// settingsRow is SettingsRow.svelte: a label and hint, a control on the right.
func (a *app) settingsRow(c *ui.Context, label, hint string, control func()) {
	p := a.pal
	ui.Row(c.Key("row-"+label)).Gap(24).AlignItems(ui.Center).Padding(16, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, label).FontSize(13)
			if hint != "" {
				ui.Text(c, hint).FontSize(12).LineHeight(1.5).TextColor(p.textTertiary)
			}
		})
		if control != nil {
			control()
		}
	})
}

// selectBox is the settings' select, as SETTINGS_SELECT.
func (a *app) selectBox(c *ui.Context, key string, value *string, options []string) {
	ui.Select(c.Key(key), value, options).Width(112).Height(32)
}

func (a *app) spaceProfile(c *ui.Context, d *spaceDetail) {
	p := a.pal
	a.settingsHeading(c, m("space_section_profile"), m("space_profile_desc"), nil)
	ui.Row(c.Key("id")).Gap(16).AlignItems(ui.Start).Padding(24, 0).Children(func() {
		av := ""
		if d.PublicProfile != nil {
			av = d.PublicProfile.AvatarURL
		}
		ui.Column(c).Width(64).AlignItems(ui.Center).Gap(6).Children(func() {
			a.avatar(c, av, d.Name, 56, 28)
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icon("upload")).Size(12, 12).TextColor(p.textTertiary)
				ui.Text(c, m("space_change")).FontSize(11).TextColor(p.textTertiary)
			})
		})
		ui.Column(c).Grow(1).Gap(6).Padding(2, 0).Children(func() {
			ui.Text(c, d.Name).FontSize(15).FontWeight(500)
			kv := func(k, v string, edit bool) {
				ui.Row(c.Key(k)).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, strings.ToUpper(k)).FontSize(11).LetterSpacing(0.6).TextColor(p.textTertiary)
					ui.Text(c, v).FontSize(11).Font(fontMono).TextColor(p.textTertiary)
					ui.Icon(c, icon("copy")).Size(12, 12).TextColor(p.textTertiary)
					if edit {
						ui.Icon(c, icon("pencil")).Size(12, 12).TextColor(p.textTertiary)
					}
				})
			}
			kv(m("space_id"), d.ID[:8]+"…"+d.ID[len(d.ID)-4:], false)
			owner := "username"
			if d.OwnerProfile != nil && d.OwnerProfile.Username != "" {
				owner = d.OwnerProfile.Username
			}
			kv(m("space_url"), "/"+owner+"/"+d.Slug, true)
		})
	})
	ui.Column(c.Key("desc")).Gap(8).Padding(24, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Text(c, m("space_description")).FontSize(13).FontWeight(500)
		key := "space-desc:" + d.ID
		draft, ok := a.pages.json[key].(string)
		if !ok {
			draft = d.Description
		}
		ui.TextArea(c.Key("area"), &draft).Lines(3, 8).Placeholder(m("space_what_for")).FontSize(13)
		a.pages.json[key] = draft
		save := ui.ButtonBase(c.Key("save")).Height(32).Padding(0, 12).Radius(5).Background(p.brand).Center().AlignSelf(ui.Start).FocusRing(false).Children(func() {
			ui.Text(c, m("space_save")).FontSize(12).FontWeight(500).TextColor(p.brandFg)
		})
		if strings.TrimSpace(draft) == strings.TrimSpace(d.Description) {
			save.Opacity(0.5)
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
		a.button(c, "upload", m("space_upload"), false)
	})
}

type spaceAccess struct {
	SignedIn  *string `json:"signed_in_user"`
	Anonymous *string `json:"anonymous_user"`
}

type spaceMembers struct {
	Items []struct {
		UserID  string   `json:"userId"`
		Role    string   `json:"role"`
		Profile *Profile `json:"profile"`
	} `json:"items"`
}

func roleLabel(r *string) string {
	if r == nil || *r == "" {
		return m("space_role_no_access")
	}
	switch *r {
	case "guest":
		return m("space_role_guest")
	case "builder":
		return m("space_role_builder")
	case "host":
		return m("space_role_host")
	}
	return *r
}

func (a *app) spaceAccess(c *ui.Context, d *spaceDetail) {
	p := a.pal
	acc, _, _ := loadJSON[spaceAccess](a, "access:"+d.ID, sp(d.ID)+"/access")
	mem, _, _ := loadJSON[spaceMembers](a, "members:"+d.ID, sp(d.ID)+"/members")
	ui.Column(c.Key("access")).Padding(40, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		a.settingsHeading(c, m("space_section_access"), m("space_access_desc"), func() {
			a.button(c, "link", m("space_invite"), true)
		})
		ui.Box(c).Height(16)
		signed, anon := m("space_role_no_access"), m("space_role_no_access")
		if acc != nil {
			signed, anon = roleLabel(acc.SignedIn), roleLabel(acc.Anonymous)
		}
		opts := []string{m("space_role_no_access"), m("space_role_guest"), m("space_role_builder")}
		a.settingsRow(c, m("space_signed_in_users"), m("space_access_any_signed_in"), func() { a.selectBox(c, "signed", &signed, opts) })
		a.settingsRow(c, m("space_access_anon"), m("space_access_no_account"), func() { a.selectBox(c, "anon", &anon, opts[:2]) })
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
				ui.Row(c.Key(it.UserID)).Gap(10).AlignItems(ui.Center).Height(44).Children(func() {
					a.avatar(c, it.Profile.AvatarURL, it.Profile.name(), 28, 14)
					ui.Text(c, it.Profile.name()).FontSize(13).Grow(1)
					role := it.Role
					ui.Text(c, roleLabel(&role)).FontSize(12).TextColor(p.textTertiary)
				})
			}
		}
	})
}

func (a *app) spaceEnv(c *ui.Context, d *spaceDetail) {
	p := a.pal
	a.settingsHeading(c, m("space_section_env"), m("space_env_desc"), nil)
	ui.Row(c.Key("vars-h")).Padding(24, 0, 8, 0).Children(func() {
		ui.Text(c, m("space_variables")).FontSize(13).FontWeight(500).Grow(1)
		ui.Text(c, m("space_available_to_process")).FontSize(11).TextColor(p.textTertiary)
	})
	ui.Column(c.Key("vars")).Radius(6).Border(1, p.borderSubtle).Children(func() {
		if len(d.Meta.ExtraEnv) == 0 {
			ui.Text(c, m("space_no_vars")).FontSize(12).TextColor(p.textTertiary).Padding(16)
		}
		for i, e := range d.Meta.ExtraEnv {
			row := ui.Row(c.Key(e.Name)).Gap(12).AlignItems(ui.Center).Padding(10, 12)
			if i > 0 {
				row.BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle)
			}
			row.Children(func() {
				ui.Text(c, e.Name).FontSize(12).Font(fontMono).Width(200).SingleLine()
				ui.Text(c, strings.Repeat("•", 12)).FontSize(12).Font(fontMono).TextColor(p.textTertiary).Grow(1)
				ui.Icon(c, icon("eye")).Size(14, 14).TextColor(p.textTertiary)
				ui.Icon(c, icon("trash-2")).Size(14, 14).TextColor(p.textTertiary)
			})
		}
	})
	ui.Row(c.Key("mounted-h")).Padding(28, 0, 8, 0).Children(func() {
		ui.Text(c, m("space_mounted_spaces")).FontSize(13).FontWeight(500).Grow(1)
		ui.Text(c, m("space_mounted_readonly")).FontSize(11).TextColor(p.textTertiary)
	})
	ui.Text(c.Key("mounted"), m("space_no_mounted")).FontSize(12).TextColor(p.textTertiary).Padding(16).Radius(6).Border(1, p.borderSubtle)
}

func (a *app) spaceChannels(c *ui.Context, d *spaceDetail) {
	p := a.pal
	list, _, _ := loadJSON[[]channel](a, "channels", "/api/channels")
	a.settingsHeading(c, m("space_section_channels"), m("space_channels_desc"), nil)
	ui.Column(c.Key("bound")).Margin(24, 0, 0, 0).Radius(6).Border(1, p.borderSubtle).Children(func() {
		n := 0
		if list != nil {
			for _, ch := range *list {
				if ch.BoundSpace == nil || ch.BoundSpace.ID != d.ID {
					continue
				}
				n++
				ui.Row(c.Key(ch.ID)).Gap(10).AlignItems(ui.Center).Padding(10, 12).Children(func() {
					a.dot(c, providerColor(ch.Provider, p), 8)
					ui.Text(c, ch.Name).FontSize(13).Grow(1)
					ui.Text(c, m("space_unbind")).FontSize(12).TextColor(p.textTertiary)
				})
			}
		}
		if n == 0 {
			ui.Text(c, m("space_no_channels")).FontSize(12).TextColor(p.textTertiary).Padding(16)
		}
	})
}

func (a *app) spaceSandbox(c *ui.Context, d *spaceDetail) {
	p := a.pal
	a.settingsHeading(c, m("space_section_sandbox"), m("space_sandbox_desc"), nil)
	ui.Box(c).Height(16)
	sb := d.Meta.Config.Sandbox
	a.settingsRow(c, "状态", "", func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			col := p.placeholder
			if d.SandboxStatus == "running" || d.SandboxStatus == "ready" {
				col = p.success
			}
			a.dot(c, col, 7)
			ui.Text(c, d.SandboxStatus).FontSize(12).Font(fontMono)
		})
	})
	a.settingsRow(c, "规格", "", func() { ui.Text(c, sb.Spec).FontSize(12).Font(fontMono) })
	a.settingsRow(c, "提供方", "", func() { ui.Text(c, sb.Provider).FontSize(12).Font(fontMono) })
	img := sb.Image
	if img == "" {
		img = "default"
	}
	a.settingsRow(c, "镜像", "", func() { ui.Text(c, img).FontSize(12).Font(fontMono) })
	ttl := "—"
	if sb.AutoDestroy.TTLSeconds > 0 {
		ttl = fmt.Sprintf("%s · %dh", sb.AutoDestroy.Mode, sb.AutoDestroy.TTLSeconds/3600)
	}
	a.settingsRow(c, "自动销毁", "", func() { ui.Text(c, ttl).FontSize(12).Font(fontMono) })
}

type walletSummary struct {
	BalanceUSD float64 `json:"balanceUsd"`
	Balance    float64 `json:"balance"`
}

// walletPage is /spaces/:id/wallet: the Space's shared balance.
func (a *app) walletPage(c *ui.Context) {
	p := a.pal
	id := a.route.space
	w, err, ok := loadJSON[map[string]any](a, "wallet:"+id, sp(id)+"/wallet")
	name, _ := a.spaceName(id)
	a.header(c, func() {
		if a.iconButton(c.Key("back"), "arrow-left", m("space_back"), false).Clicked() {
			a.goBack()
		}
		ui.Text(c, name).FontSize(13).Grow(1)
	})
	ui.Scroll(c.Key("wallet")).Grow(1).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(720).Margin(0, ui.Auto).Padding(28, 24).Gap(8).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, m("wallet_title")).FontSize(18).FontWeight(600).Grow(1)
				if a.iconButton(c.Key("refresh"), "refresh-cw", m("rules_refresh"), false).Clicked() {
					a.reload("wallet:" + id)
				}
			})
			ui.Text(c, m("wallet_description")).FontSize(12).TextColor(p.textTertiary)
			if !ok || err != nil {
				a.loadingBlock(c, err, 120)
				return
			}
			bal := 0.0
			for _, k := range []string{"balanceUsd", "availableUsd", "netUsd"} {
				if v, ok := (*w)[k].(float64); ok {
					bal = v
					break
				}
			}
			ui.Text(c, usd(bal)).FontSize(24).FontWeight(600).Font(fontMono).Margin(8, 0, 24, 0)
			ui.Row(c).AlignItems(ui.Center).Padding(0, 0, 10, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
				ui.Text(c, m("wallet_fund")).FontSize(14).FontWeight(600).Grow(1)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icon("link")).Size(13, 13).TextColor(p.textTertiary)
					ui.Text(c, m("wallet_fund_link_copy")).FontSize(12).TextColor(p.textTertiary)
				})
			})
			var off bool
			a.settingsRow(c, m("wallet_public_funding"), m("wallet_public_funding_hint"), func() { ui.Switch(c.Key("fund"), &off) })
			ui.Text(c, m("wallet_fund_note")).FontSize(12).TextColor(p.textTertiary).Margin(8, 0)
			ui.Row(c.Key("amounts")).Gap(8).Children(func() {
				for i, v := range []string{"US$10", "US$50", "US$200", "US$1,000"} {
					on := i == 0
					card := ui.Column(c.Key(v)).Grow(1).Basis(0).Gap(4).Padding(12).Radius(6).Border(1, p.borderSubtle).Children(func() {
						ui.Text(c, v).FontSize(13).FontWeight(600).Font(fontMono)
						ui.Text(c, m("wallet_fund_via_balance")).FontSize(10).TextColor(p.textTertiary)
					})
					if on {
						card.Background(p.brandBg).BorderColor(p.brandBorder)
					}
				}
			})
			ui.Text(c, m("wallet_fund_other")).FontSize(11).TextColor(p.textTertiary).Margin(10, 0)
			credits, _, _ := loadJSON[billingCredits](a, "credits", "/api/billing/credits")
			ui.ButtonBase(c.Key("pay")).Height(40).Radius(6).Background(p.brand).Center().FocusRing(false).Children(func() {
				ui.Text(c, m("wallet_fund_pay_balance", "amount", "US$10")).FontSize(13).FontWeight(500).TextColor(p.brandFg)
			})
			ui.Row(c).Margin(8, 0, 0, 0).Children(func() {
				if credits != nil {
					ui.Text(c, m("wallet_fund_allowance", "amount", formatCost(credits.NetUSD))+" · "+m("wallet_fund_allowance_note")).FontSize(11).TextColor(p.textTertiary).Grow(1)
				}
				ui.Text(c, m("wallet_fund_checkout_instead")).FontSize(11).TextColor(p.textTertiary)
			})
			ui.Text(c, m("billing_activity_heading")).FontSize(14).FontWeight(600).Margin(32, 0, 12, 0)
			ui.Text(c, m("billing_no_activity")).FontSize(12).TextColor(p.textTertiary)
		})
	})
}

var _ = url.PathEscape
