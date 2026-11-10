package app

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"cohub-mygo/internal/cohub"
	"cohub-mygo/internal/i18n"
)

// The Space's wallet, as SpaceWallet and SpaceFundPanel: its balance,
// public funding, funding from the user's balance or online checkout, and
// what went in and out.

type walletFunding struct {
	PublicFunding bool `json:"publicFunding"`
	Checkout      bool `json:"checkout"`
	Tiers         []struct {
		ProductKey string  `json:"productKey"`
		AmountUSD  float64 `json:"amountUsd"`
		PriceUSD   float64 `json:"priceUsd"`
	} `json:"tiers"`
	Transfer *struct {
		AllowanceUSD float64 `json:"allowanceUsd"`
		MinUSD       float64 `json:"minUsd"`
		MaxUSD       float64 `json:"maxUsd"`
	} `json:"transfer"`
}

type walletActivity struct {
	Items []struct {
		ID          string    `json:"id"`
		Kind        string    `json:"kind"`
		AmountUSD   float64   `json:"amountUsd"`
		ActorUserID *string   `json:"actorUserId"`
		CreatedAt   time.Time `json:"createdAt"`
	} `json:"items"`
	HasMore bool `json:"hasMore"`
}

type walletForm struct {
	tier         string
	custom       string
	customOpen   bool
	preferOnline bool
	busy         bool
	notice       string
	page         int
}

func (a *app) walletPage(c *ui.Context) {
	p := a.pal
	id := a.route.space
	w, err, ok := loadJSON[map[string]any](a, "wallet:"+id, sp(id)+"/wallet")
	fund, _, _ := loadJSON[walletFunding](a, "wallet-funding:"+id, sp(id)+"/wallet/funding")
	f := a.walletForms[id]
	if f == nil {
		f = &walletForm{page: 1}
		a.walletForms[id] = f
	}
	name, _ := a.spaceName(id)
	a.header(c, func() {
		if a.iconButton(c.Key("back"), "arrow-left", m("space_back"), false).Clicked() {
			a.goBack()
		}
		ui.Text(c, name).FontSize(13).Grow(1)
	})
	ui.Scroll(c.Key("wallet")).Grow(1).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(768).Margin(0, ui.Auto).Padding(24, 32).Gap(8).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, m("wallet_title")).FontSize(18).FontWeight(600).Grow(1)
				if a.iconButton(c.Key("refresh"), "refresh-cw", m("wallet_refresh"), false).Clicked() {
					a.forgetPrefix("wallet:"+id, "wallet-funding:"+id, "wallet-act:"+id)
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
			ui.Text(c, usd(math.Round(bal*1e4)/1e4)).FontSize(24).Font(fontMono).Margin(8, 0, 24, 0)
			link := "https://cohub.live/spaces/" + id + "/fund"
			ui.Row(c).AlignItems(ui.Center).Padding(0, 0, 10, 0).Children(func() {
				ui.Text(c, m("wallet_fund")).FontSize(14).FontWeight(600).Grow(1)
				cp := ui.ButtonBase(c.Key("copy-link")).Row().Gap(6).AlignItems(ui.Center).Height(28).Padding(0, 8).Radius(5).FocusRing(false).Children(func() {
					copied := a.copiedValue == link && time.Since(a.copiedAt) < 2*time.Second
					ic, label := "link", m("wallet_fund_link_copy")
					if copied {
						ic, label = "check", m("wallet_fund_link_copied")
					}
					ui.Icon(c, icon(ic)).Size(13, 13).TextColor(p.textSecond)
					ui.Text(c, label).FontSize(12).TextColor(p.textSecond)
				})
				if cp.Hovered() {
					cp.Background(p.hover)
				}
				if cp.Clicked() {
					c.WriteClipboard(link)
					a.copiedAt, a.copiedValue = time.Now(), link
				}
			})
			if a.can(id, "space.commerce.manage") && fund != nil {
				on := fund.PublicFunding
				a.settingsRow(c, m("wallet_public_funding"), m("wallet_public_funding_hint"), func() {
					if ui.Switch(c.Key("fund"), &on).Changed() {
						fund.PublicFunding = on
						a.act(func(ctx context.Context) error {
							return a.api.Do(ctx, "PATCH", sp(id)+"/profile", map[string]any{"fundingEnabled": on}, nil)
						}, "wallet-funding:"+id)
					}
				})
			}
			ui.Text(c, m("wallet_fund_note")).FontSize(12).TextColor(p.textTertiary).Margin(8, 0)
			if fund == nil {
				a.spinner(c, 14, p.textTertiary)
			} else {
				a.fundPanel(c, id, name, fund, f)
			}
			a.walletActivities(c, id, f)
		})
	})
}

func (a *app) fundPanel(c *ui.Context, id, name string, fund *walletFunding, f *walletForm) {
	p := a.pal
	if len(fund.Tiers) == 0 && fund.Transfer == nil {
		ui.Text(c, m("wallet_recharge_unavailable")).FontSize(12).TextColor(p.textTertiary)
		return
	}
	if f.tier == "" && !f.customOpen && len(fund.Tiers) > 0 {
		f.tier = fund.Tiers[0].ProductKey
	}
	covers := func(v float64) bool {
		return fund.Transfer != nil && v >= fund.Transfer.MinUSD && v <= fund.Transfer.MaxUSD && v <= fund.Transfer.AllowanceUSD
	}
	var amount float64
	var tierKey string
	for _, t := range fund.Tiers {
		if t.ProductKey == f.tier && !f.customOpen {
			amount, tierKey = t.AmountUSD, t.ProductKey
		}
	}
	customErr := ""
	if f.customOpen {
		v, err := strconv.ParseFloat(strings.TrimSpace(f.custom), 64)
		switch {
		case strings.TrimSpace(f.custom) == "":
		case err != nil || v != math.Trunc(v) || fund.Transfer == nil || v < fund.Transfer.MinUSD || v > fund.Transfer.MaxUSD:
			lo, hi := 1.0, 5000.0
			if fund.Transfer != nil {
				lo, hi = fund.Transfer.MinUSD, fund.Transfer.MaxUSD
			}
			customErr = m("wallet_fund_invalid_amount", "min", formatPrice(lo), "max", formatPrice(hi))
		case v > fund.Transfer.AllowanceUSD:
			customErr = m("wallet_fund_insufficient", "amount", formatMoney(fund.Transfer.AllowanceUSD))
		default:
			amount = v
		}
	}
	method := ""
	switch {
	case tierKey != "" && fund.Checkout && (f.preferOnline || !covers(amount)):
		method = "checkout"
	case amount > 0 && covers(amount):
		method = "balance"
	}
	ui.Row(c.Key("amounts")).Gap(8).Children(func() {
		for _, t := range fund.Tiers {
			t := t
			on := f.tier == t.ProductKey && !f.customOpen
			card := ui.ButtonBase(c.Key(t.ProductKey)).Column().AlignItems(ui.Start).Grow(1).Basis(0).Gap(4).Padding(12).Radius(6).Border(1, p.borderSubtle).FocusRing(false).Children(func() {
				ui.Text(c, formatPrice(t.AmountUSD)).FontSize(13).FontWeight(600).Font(fontMono)
				via := m("wallet_fund_via_checkout")
				if covers(t.AmountUSD) {
					via = m("wallet_fund_via_balance")
				}
				ui.Text(c, via).FontSize(10).TextColor(p.textTertiary)
			})
			if on {
				card.Background(p.brandBg).Border(1, p.brandBorder)
			} else if card.Hovered() {
				card.Background(p.hover)
			}
			if card.Clicked() {
				f.tier, f.customOpen, f.preferOnline, f.notice = t.ProductKey, false, false, ""
			}
		}
	})
	if fund.Transfer != nil {
		if !f.customOpen {
			ob := ui.ButtonBase(c.Key("other")).AlignSelf(ui.Start).Height(24).Padding(0, 4).Radius(4).FocusRing(false).Children(func() {
				ui.Text(c, m("wallet_fund_other")).FontSize(11).TextColor(p.textTertiary)
			})
			if ob.Clicked() {
				f.customOpen, f.notice = true, ""
			}
		} else {
			ui.Column(c.Key("custom")).Gap(4).Children(func() {
				a.field(c, "custom", &f.custom, "US$").Width(200).Font(fontMono)
				hint := m("wallet_fund_other_hint", "min", formatPrice(fund.Transfer.MinUSD), "max", formatPrice(fund.Transfer.MaxUSD))
				col := p.textTertiary
				if customErr != "" {
					hint, col = customErr, p.errorC
				}
				ui.Text(c, hint).FontSize(11).TextColor(col)
			})
		}
	}
	label := m("wallet_fund_pay_balance", "amount", formatPrice(amount))
	if method == "checkout" {
		label = m("wallet_fund_pay_checkout", "amount", formatPrice(amount))
	}
	pay := ui.ButtonBase(c.Key("pay")).Row().Gap(6).Height(40).Radius(6).Background(p.brand).Center().FocusRing(false).Margin(6, 0, 0, 0).Children(func() {
		if f.busy {
			a.spinner(c, 14, p.brandFg)
		}
		ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(p.brandFg)
	})
	if method == "" || f.busy {
		pay.Opacity(0.5)
	} else if pay.Clicked() {
		f.busy, f.notice = true, ""
		if method == "balance" {
			v := amount
			a.act(func(ctx context.Context) error {
				var out struct {
					State string `json:"state"`
				}
				err := a.api.Do(ctx, "POST", sp(id)+"/wallet/transfers", map[string]any{"id": cohub.NewID(), "amountUsd": v}, &out)
				a.update(func() {
					f.busy = false
					switch {
					case err != nil:
						f.notice = err.Error()
					case out.State == "completed":
						a.notify(m("wallet_fund_done", "space", name, "amount", formatPrice(v)))
					default:
						f.notice = m("wallet_fund_processing")
					}
				})
				return nil
			}, "wallet:"+id, "wallet-funding:"+id, "wallet-act:"+id+":1", "credits")
		} else {
			key := tierKey
			a.act(func(ctx context.Context) error {
				var out struct {
					Checkout struct {
						CheckoutURL    *string `json:"checkoutUrl"`
						CheckoutUsable bool    `json:"checkoutUsable"`
					} `json:"checkout"`
				}
				err := a.api.Do(ctx, "POST", sp(id)+"/wallet/orders", map[string]any{"id": cohub.NewID(), "productKey": key, "returnPath": "/spaces/" + id + "/wallet"}, &out)
				a.update(func() {
					f.busy = false
					if err != nil || out.Checkout.CheckoutURL == nil || !out.Checkout.CheckoutUsable {
						f.notice = m("wallet_checkout_failed")
						return
					}
					c.OpenURL(*out.Checkout.CheckoutURL)
					f.notice = m("wallet_payment_pending")
				})
				return nil
			})
		}
	}
	ui.Row(c.Key("alt")).Margin(8, 0, 0, 0).Children(func() {
		if fund.Transfer != nil {
			ui.Text(c, m("wallet_fund_allowance", "amount", formatMoney(fund.Transfer.AllowanceUSD))+" · "+m("wallet_fund_allowance_note")).FontSize(11).TextColor(p.textTertiary).Grow(1)
		}
		if tierKey != "" && fund.Checkout && covers(amount) {
			alt := m("wallet_fund_checkout_instead")
			if f.preferOnline {
				alt = m("wallet_fund_balance_instead")
			}
			ab := ui.Text(c.Key("switch"), alt).FontSize(11).TextColor(p.textSecond).Cursor(ui.CursorPointer)
			if ab.Clicked() {
				f.preferOnline = !f.preferOnline
			}
		}
	})
	if f.notice != "" {
		ui.Text(c.Key("notice"), f.notice).FontSize(12).TextColor(p.textSecond).Padding(10).Radius(6).Background(p.hover)
	}
}

func (a *app) walletActivities(c *ui.Context, id string, f *walletForm) {
	p := a.pal
	act, aerr, _ := loadJSON[walletActivity](a, fmt.Sprintf("wallet-act:%s:%d", id, f.page), sp(id)+"/wallet/activities?page="+url.QueryEscape(strconv.Itoa(f.page)))
	ui.Text(c.Key("act-h"), m("billing_activity_heading")).FontSize(14).FontWeight(600).Margin(32, 0, 12, 0)
	switch {
	case act == nil && aerr == nil:
		ui.Text(c.Key("act-loading"), m("common_loading")).FontSize(12).TextColor(p.textTertiary)
		return
	case aerr != nil:
		ui.Text(c.Key("act-err"), m("billing_activity_load_failed")).FontSize(12).TextColor(p.errorC)
		return
	case len(act.Items) == 0:
		ui.Text(c.Key("act-none"), m("billing_no_activity")).FontSize(12).TextColor(p.textTertiary)
		return
	}
	kinds := map[string]string{"recharge": "wallet_activity_recharge", "usage": "wallet_activity_usage", "refund": "wallet_activity_refund",
		"reversal": "wallet_activity_reversal", "revoke": "wallet_activity_revoke", "adjust": "wallet_activity_adjust",
		"transfer_in": "wallet_transfer_out", "transfer_returned": "wallet_transfer_returned"}
	ui.Column(c.Key("act")).Children(func() {
		for i, it := range act.Items {
			row := ui.Row(c.Key(it.ID)).Gap(12).AlignItems(ui.Start).Padding(12, 0)
			if i > 0 {
				row.BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle)
			}
			row.Children(func() {
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					label := it.Kind
					if k, ok := kinds[it.Kind]; ok {
						label = m(k)
					}
					ui.Text(c, label).FontSize(13)
					ui.Text(c, i18n.LongDate(it.CreatedAt)).FontSize(11).TextColor(p.textTertiary)
				})
				col := p.textSecond
				if it.AmountUSD > 0 {
					col = p.success
				}
				ui.Text(c, strconv.FormatFloat(it.AmountUSD, 'f', -1, 64)).FontSize(13).Font(fontMono).TextColor(col)
			})
		}
	})
	if f.page > 1 || act.HasMore {
		ui.Row(c.Key("pages")).Gap(8).Children(func() {
			ui.Spacer(c)
			if a.iconButton(c.Key("prev"), "chevron-left", m("billing_prev_page"), false).Clicked() && f.page > 1 {
				f.page--
			}
			if a.iconButton(c.Key("next"), "chevron-right", m("billing_next_page"), false).Clicked() && act.HasMore {
				f.page++
			}
		})
	}
}
