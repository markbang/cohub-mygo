package main

import (
	"net/url"

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
	Access        *struct {
		Role        string   `json:"role"`
		Permissions []string `json:"permissions"`
	} `json:"access"`
	Meta struct {
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
		ui.Scroll(c.Key("main-" + a.route.sub)).Grow(1).Children(func() {
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
	ui.Row(c.Key("h-" + title)).Gap(12).AlignItems(ui.Start).Children(func() {
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

// can tells whether the user's access to the Space grants the permission,
// as SpaceWorkspacePage hasAccessPermission. It loads the Space's detail.
func (a *app) can(spaceID, permission string) bool {
	if spaceID == "" {
		return false
	}
	d, _, _ := loadJSON[spaceDetail](a, "space:"+spaceID, sp(spaceID))
	return d != nil && d.Access != nil && contains(d.Access.Permissions, permission)
}
