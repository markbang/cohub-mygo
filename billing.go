package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type billingCredits struct {
	NetUSD float64 `json:"netUsd"`
	Groups []struct {
		Key                string  `json:"key"`
		RemainingAmountUSD float64 `json:"remainingAmountUsd"`
		Grants             []struct {
			ID                     string     `json:"id"`
			BenefitName            string     `json:"benefitName"`
			GrantKind              string     `json:"grantKind"`
			Status                 string     `json:"status"`
			AvailableNow           bool       `json:"availableNow"`
			RemainingAmountUSD     float64    `json:"remainingAmountUsd"`
			OriginalAmountUSD      *float64   `json:"originalAmountUsd"`
			ConsumedAmountUSD      float64    `json:"consumedAmountUsd"`
			UsageConsumedAmountUSD float64    `json:"usageConsumedAmountUsd"`
			ConsumedPercent        *float64   `json:"consumedPercent"`
			ExpiresAt              *time.Time `json:"expiresAt"`
		} `json:"grants"`
	} `json:"groups"`
}

type billingSubscriptions struct {
	Subscriptions struct {
		Items []struct {
			ProductName       string    `json:"productName"`
			ProductKey        string    `json:"productKey"`
			Status            string    `json:"status"`
			CancelAtPeriodEnd bool      `json:"cancelAtPeriodEnd"`
			CurrentPeriodEnd  time.Time `json:"currentPeriodEnd"`
		} `json:"items"`
	} `json:"subscriptions"`
}

// usd is formatUsdAmount: US$ and as many decimals as the amount has.
func usd(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	whole, frac, _ := strings.Cut(s, ".")
	neg := strings.HasPrefix(whole, "-")
	whole = strings.TrimPrefix(whole, "-")
	var b strings.Builder
	for i, ch := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	out := "US$" + b.String()
	if frac != "" {
		out += "." + frac
	}
	if neg {
		out = "-" + out
	}
	return out
}

func zhDate(t time.Time) string { return t.Local().Format("2006年1月2日 15:04") }

// billingPage is BillingSettings.svelte's balance tab: the plan, the
// balance, and the balance by when it expires.
func (a *app) billingPage(c *ui.Context) {
	p := a.pal
	credits, cerr, _ := loadJSON[billingCredits](a, "credits", "/api/billing/credits")
	subs, _, _ := loadJSON[billingSubscriptions](a, "subscriptions", "/api/billing/subscriptions")
	tab, _ := a.pages.json["billing-tab"].(int)
	a.settingsScroll(c, 1024, func() {
		a.settingsTitle(c, m("page_title_billing"), m("billing_description"), nil)
		ui.Row(c.Key("cards")).Gap(12).Padding(20, 0).Children(func() {
			ui.Column(c.Key("plan")).Grow(2).Basis(0).Gap(6).Padding(12).Radius(6).Border(1, p.borderSubtle).Children(func() {
				ui.Text(c, m("billing_subscription")).FontSize(11).LetterSpacing(0.6).TextColor(p.textTertiary)
				name, line := m("billing_no_subscription"), ""
				if subs != nil {
					for _, s := range subs.Subscriptions.Items {
						if s.Status != "active" {
							continue
						}
						name = s.ProductName
						if s.CancelAtPeriodEnd {
							line = m("billing_status_active") + " · " + m("billing_auto_renew_canceled_ends", "date", zhDate(s.CurrentPeriodEnd))
						} else {
							line = m("billing_status_active") + " · " + m("billing_renews", "date", zhDate(s.CurrentPeriodEnd))
						}
						break
					}
				}
				ui.Text(c, name).FontSize(14).FontWeight(600).SingleLine()
				if line != "" {
					ui.Text(c, line).FontSize(11).TextColor(p.textTertiary).SingleLine()
				}
			})
			ui.Column(c.Key("balance")).Grow(1).Basis(0).Gap(6).Padding(12).Radius(6).Border(1, p.borderSubtle).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icon("wallet")).Size(14, 14).TextColor(p.textTertiary)
					ui.Text(c, m("billing_balance")).FontSize(11).LetterSpacing(0.6).TextColor(p.textTertiary)
				})
				v := m("billing_loading")
				col := p.textTertiary
				if credits != nil {
					v, col = usd(credits.NetUSD), p.text
					if credits.NetUSD < 0 {
						col = p.errorC
					}
				}
				ui.Text(c, v).FontSize(18).FontWeight(600).Font(fontMono).TextColor(col)
			})
		})
		ui.Row(c.Key("tabs")).Gap(4).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
			for i, l := range []string{m("billing_tab_balance"), m("billing_tab_plans"), m("billing_tab_redeem")} {
				on := tab == i
				b := ui.ButtonBase(c.Key(l)).Height(34).Padding(0, 8).Center().FocusRing(false).Children(func() {
					col := p.textTertiary
					if on {
						col = p.text
					}
					ui.Text(c, l).FontSize(12).FontWeight(500).TextColor(col)
				})
				if on {
					b.BorderWidth(0, 0, 2, 0).BorderColor(p.brand)
				}
				if b.Clicked() {
					a.pages.json["billing-tab"] = i
				}
			}
		})
		if tab != 0 {
			ui.Text(c.Key("todo"), m("billing_no_plans")).FontSize(12).TextColor(p.textTertiary).Padding(20, 0)
			return
		}
		ui.Text(c.Key("h"), m("billing_balance_heading")).FontSize(13).FontWeight(500).Margin(20, 0, 12, 0)
		ui.Text(c.Key("h2"), m("billing_balance_by_expiration")).FontSize(13).FontWeight(500)
		if credits == nil {
			a.loadingBlock(c, cerr, 120)
			return
		}
		ui.Column(c.Key("groups")).Margin(12, 0, 0, 0).Radius(6).Border(1, p.borderSubtle).Children(func() {
			for gi, g := range credits.Groups {
				col := ui.Column(c.Key(g.Key)).Padding(12).Gap(8)
				if gi > 0 {
					col.BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle)
				}
				col.Children(func() {
					ui.Row(c).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Grow(1).Children(func() {
							ui.Text(c, expiryGroup(g.Key)).FontSize(12).FontWeight(500)
							n := len(g.Grants)
							key := "billing_grants_many"
							if n == 1 {
								key = "billing_grants_one"
							}
							ui.Text(c, m(key, "count", n)).FontSize(11).TextColor(p.textTertiary)
						})
						ui.Text(c, usd(g.RemainingAmountUSD)).FontSize(13).Font(fontMono)
					})
					for _, gr := range g.Grants {
						ui.Column(c.Key(gr.ID)).Padding(8, 10).Radius(5).Background(p.hover.Alpha(0.6)).Gap(6).Children(func() {
							ui.Row(c).Gap(8).AlignItems(ui.Start).Children(func() {
								ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
									ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
										name := gr.BenefitName
										if name == "" {
											name = gr.GrantKind
										}
										ui.Text(c, name).FontSize(11).TextColor(p.textSecond).SingleLine()
										st := strings.ToUpper(gr.Status[:1]) + gr.Status[1:]
										tc := p.textTertiary
										if !gr.AvailableNow {
											tc = p.placeholder
										}
										ui.Text(c, st).FontSize(10).TextColor(tc).Padding(1, 5).Radius(3).Border(1, p.borderSubtle)
									})
									ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
										ui.Icon(c, icon("clock")).Size(12, 12).TextColor(p.textTertiary)
										exp := m("billing_no_expiration")
										if gr.ExpiresAt != nil {
											exp = zhDate(*gr.ExpiresAt)
										}
										ui.Text(c, exp).FontSize(11).TextColor(p.textTertiary)
									})
								})
								vc := p.textSecond
								if gr.RemainingAmountUSD <= 0 {
									vc = p.placeholder
								}
								ui.Text(c, usd(gr.RemainingAmountUSD)).FontSize(11).Font(fontMono).TextColor(vc)
							})
							pct := 0.0
							if gr.ConsumedPercent != nil {
								pct = *gr.ConsumedPercent
							}
							ui.Row(c).Children(func() {
								used := m("billing_used", "amount", usd(gr.ConsumedAmountUSD))
								if gr.OriginalAmountUSD != nil {
									used += " " + m("billing_of", "amount", usd(*gr.OriginalAmountUSD))
								}
								ui.Text(c, used).FontSize(10).TextColor(p.textTertiary).Grow(1)
								ui.Text(c, fmt.Sprintf("%g%%", pct)).FontSize(10).Font(fontMono).TextColor(p.textTertiary)
							})
							ui.Box(c).Height(6).Radius(3).Background(p.hoverStrong).Clip().Children(func() {
								ui.Box(c).Height(6).WidthPercent(float32(min(pct, 100))).Radius(3).Background(p.brand.Alpha(0.7))
							})
							if gr.UsageConsumedAmountUSD > 0 {
								ui.Text(c, m("billing_usage", "amount", usd(gr.UsageConsumedAmountUSD))).FontSize(10).Font(fontMono).TextColor(p.placeholder)
							}
						})
					}
				})
			}
		})
	})
}

func expiryGroup(key string) string {
	switch key {
	case "expired":
		return m("billing_expired")
	case "lt_7d":
		return m("billing_expires_within_7d")
	case "lt_30d":
		return m("billing_expires_within_30d")
	case "gte_30d", "gt_30d":
		return m("billing_expires_after_30d")
	}
	return m("billing_no_expiration")
}
