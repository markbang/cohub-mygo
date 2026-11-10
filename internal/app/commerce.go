package app

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"cohub-mygo/internal/cohub"
	"cohub-mygo/internal/i18n"
)

// The Space's commerce, as /spaces/:id/settings/commerce: set it up, then
// benefits (features or credits), products (one-time, priced in US$),
// which benefits each product grants, and the orders.

type commerceProduct struct {
	Key         string  `json:"key"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
	Visibility  string  `json:"visibility"`
	Pricing     struct {
		AmountUSD float64 `json:"amountUsd"`
	} `json:"pricing"`
}

type commerceBenefit struct {
	Key         string  `json:"key"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
	Type        string  `json:"type"`
	Config      struct {
		Amount        float64 `json:"amount"`
		ExpiresInDays *int    `json:"expiresInDays"`
	} `json:"config"`
}

type commerceForm struct {
	kind               string // product, benefit, bind
	editing            string // the key edited, "" for a new one
	name, desc, amount string
	benefitType        string
	product, benefit   string
	err                string
	busy               bool
}

func (a *app) commercePage(c *ui.Context) {
	p := a.pal
	id := a.route.space
	base := sp(id) + "/commerce"
	products, perr, pok := loadJSON[struct {
		Products []commerceProduct `json:"products"`
	}](a, "commerce-products:"+id, base+"/products")
	benefits, _, _ := loadJSON[struct {
		Benefits []commerceBenefit `json:"benefits"`
	}](a, "commerce-benefits:"+id, base+"/benefits")
	bindings, _, _ := loadJSON[struct {
		Bindings []struct {
			ProductKey string `json:"productKey"`
			BenefitKey string `json:"benefitKey"`
		} `json:"bindings"`
	}](a, "commerce-bindings:"+id, base+"/product-benefits")
	orders, _, _ := loadJSON[struct {
		Orders []struct {
			ID                  string         `json:"id"`
			ProductNameSnapshot string         `json:"productNameSnapshot"`
			Status              string         `json:"status"`
			AmountSnapshot      float64        `json:"amountSnapshot"`
			PaidAmountSnapshot  float64        `json:"paidAmountSnapshot"`
			CreatedAt           time.Time      `json:"createdAt"`
			BuyerProfile        *cohub.Profile `json:"buyerProfile"`
			PaidAt              *time.Time     `json:"paidAt"`
		} `json:"orders"`
	}](a, "commerce-orders:"+id, base+"/orders?limit=20")
	reloadAll := []string{"commerce-products:" + id, "commerce-benefits:" + id, "commerce-bindings:" + id, "commerce-orders:" + id}
	name, _ := a.spaceName(id)
	a.header(c, func() {
		back := ui.Text(c.Key("back"), m("commerce_back")).FontSize(12).TextColor(p.textTertiary).Cursor(ui.CursorPointer).Padding(0, 6)
		if back.Clicked() {
			a.go_(route{kind: routeSpaceSettings, space: id, sub: "profile"})
		}
		ui.Text(c, m("commerce_header")).FontSize(13).FontWeight(500).Grow(1)
		_ = name
	})
	ui.Scroll(c.Key("commerce")).Grow(1).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(768).Margin(0, ui.Auto).Padding(28, 24).Gap(28).Children(func() {
			if !pok {
				a.loadingBlock(c, nil, 120)
				return
			}
			if perr != nil && strings.Contains(perr.Error(), "not_initialized") || perr != nil && strings.Contains(perr.Error(), "not initialized") {
				ui.Column(c.Key("setup")).Radius(10).Border(1, p.borderSubtle).Clip().Children(func() {
					ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(14, 16).Background(p.bgSurfaceHover).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
						ui.Icon(c, icon("sparkles")).Size(16, 16).TextColor(p.textTertiary)
						ui.Column(c).Gap(2).Children(func() {
							ui.Text(c, m("commerce_not_setup")).FontSize(14).FontWeight(600)
							ui.Text(c, m("commerce_setup_hint")).FontSize(12).TextColor(p.textTertiary)
						})
					})
					ui.Row(c).AlignItems(ui.Center).Padding(12, 16).Children(func() {
						ui.Text(c, m("commerce_setup_one_time")).FontSize(12).TextColor(p.textTertiary).Grow(1)
						sb := ui.ButtonBase(c.Key("setup-btn")).Row().Gap(6).AlignItems(ui.Center).Height(32).Padding(0, 12).Radius(6).Background(p.brand).FocusRing(false).Children(func() {
							ui.Icon(c, icon("sparkles")).Size(14, 14).TextColor(p.brandFg)
							ui.Text(c, m("commerce_setup")).FontSize(12).FontWeight(500).TextColor(p.brandFg)
						})
						if sb.Clicked() {
							a.act(func(ctx context.Context) error {
								err := a.api.Do(ctx, "POST", base+"/setup", nil, nil)
								if err == nil {
									a.update(func() { a.notify(m("commerce_setup_ready")) })
								}
								return err
							}, reloadAll...)
						}
					})
				})
				return
			}
			if perr != nil {
				ui.Text(c, m("commerce_load_failed")+" "+perr.Error()).FontSize(12).TextColor(p.errorC)
				return
			}
			ui.Text(c, m("commerce_tip_bind")).FontSize(12).TextColor(p.textTertiary)
			section := func(key, title, desc, add string, onAdd func(), body func()) {
				ui.Column(c.Key(key)).Gap(10).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Grow(1).Gap(2).Children(func() {
							ui.Text(c, title).FontSize(14).FontWeight(600)
							ui.Text(c, desc).FontSize(12).TextColor(p.textTertiary)
						})
						if add != "" && a.button(c, "plus", add, false).Clicked() {
							onAdd()
						}
					})
					ui.Column(c).Radius(8).Border(1, p.borderSubtle).Children(body)
				})
			}
			row := func(key string, i int, body func()) {
				r := ui.Row(c.Key(key)).Gap(10).AlignItems(ui.Center).Padding(10, 12)
				if i > 0 {
					r.BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle)
				}
				r.Children(body)
			}
			empty := func(title, hint string) {
				ui.Column(c.Key("empty")).Padding(16).Gap(4).Children(func() {
					ui.Text(c, title).FontSize(13).FontWeight(500).TextColor(p.textSecond)
					ui.Text(c, hint).FontSize(12).TextColor(p.textTertiary)
				})
			}
			section("benefits", m("commerce_benefits"), m("commerce_benefits_desc"), m("commerce_new_benefit"), func() {
				a.commerce = &commerceForm{kind: "benefit", benefitType: "feature"}
			}, func() {
				if benefits == nil || len(benefits.Benefits) == 0 {
					empty(m("commerce_no_benefits"), m("commerce_no_benefits_hint"))
					return
				}
				for i, b := range benefits.Benefits {
					b := b
					row(b.Key, i, func() {
						ui.Column(c).Grow(1).Gap(2).Children(func() {
							ui.Text(c, b.Name).FontSize(13).FontWeight(500)
							kind := m("commerce_feature")
							if b.Type == "credits" {
								kind = m("commerce_credits") + " · " + m("commerce_credits_amount", "count", b.Config.Amount)
							}
							if b.Status == "archived" {
								kind += " · " + m("commerce_archived")
							}
							ui.Text(c, kind+" · "+b.Key).FontSize(11).Font(fontMono).TextColor(p.textTertiary)
						})
						if b.Status != "archived" {
							if a.iconButton(c.Key("edit"), "pencil", m("commerce_edit"), false).Clicked() {
								desc := ""
								if b.Description != nil {
									desc = *b.Description
								}
								a.commerce = &commerceForm{kind: "benefit", editing: b.Key, name: b.Name, desc: desc, benefitType: b.Type}
							}
							if a.iconButton(c.Key("archive"), "archive", m("commerce_archive"), false).Clicked() {
								a.confirm(m("commerce_archive"), m("commerce_archive_benefit_confirm", "name", b.Name), m("commerce_archive"), func() {
									a.act(func(ctx context.Context) error {
										return a.api.Do(ctx, "PATCH", base+"/benefits/"+url.PathEscape(b.Key), map[string]any{"status": "archived"}, nil)
									}, reloadAll...)
								})
							}
						}
					})
				}
			})
			section("products", m("commerce_products"), m("commerce_products_desc"), m("commerce_new_product"), func() {
				a.commerce = &commerceForm{kind: "product"}
			}, func() {
				if products == nil || len(products.Products) == 0 {
					empty(m("commerce_no_products"), m("commerce_no_products_hint"))
					return
				}
				for i, pr := range products.Products {
					pr := pr
					row(pr.Key, i, func() {
						ui.Column(c).Grow(1).Gap(2).Children(func() {
							ui.Text(c, pr.Name).FontSize(13).FontWeight(500)
							meta := m("commerce_one_time") + " · " + pr.Status + " · " + pr.Visibility
							ui.Text(c, meta).FontSize(11).TextColor(p.textTertiary)
						})
						ui.Text(c, formatPrice(pr.Pricing.AmountUSD)).FontSize(13).Font(fontMono)
						if pr.Status != "archived" {
							if a.iconButton(c.Key("edit"), "pencil", m("commerce_edit"), false).Clicked() {
								desc := ""
								if pr.Description != nil {
									desc = *pr.Description
								}
								a.commerce = &commerceForm{kind: "product", editing: pr.Key, name: pr.Name, desc: desc, amount: fmt.Sprint(pr.Pricing.AmountUSD)}
							}
							if a.iconButton(c.Key("archive"), "archive", m("commerce_archive"), false).Clicked() {
								a.confirm(m("commerce_archive"), m("commerce_archive_product_confirm", "name", pr.Name), m("commerce_archive"), func() {
									a.act(func(ctx context.Context) error {
										return a.api.Do(ctx, "PATCH", base+"/products/"+url.PathEscape(pr.Key), map[string]any{"status": "archived"}, nil)
									}, reloadAll...)
								})
							}
						}
					})
				}
			})
			canBind := products != nil && len(products.Products) > 0 && benefits != nil && len(benefits.Benefits) > 0
			addBind := ""
			if canBind {
				addBind = m("commerce_bind_benefit")
			}
			section("bindings", m("commerce_bindings"), m("commerce_bindings_desc"), addBind, func() {
				a.commerce = &commerceForm{kind: "bind", product: products.Products[0].Key, benefit: benefits.Benefits[0].Key}
			}, func() {
				if bindings == nil || len(bindings.Bindings) == 0 {
					hint := m("commerce_no_bindings")
					if !canBind {
						hint = m("commerce_bind_hint_none")
					}
					empty(hint, "")
					return
				}
				for i, b := range bindings.Bindings {
					b := b
					row(b.ProductKey+"/"+b.BenefitKey, i, func() {
						ui.Text(c, b.ProductKey).FontSize(12).Font(fontMono)
						ui.Icon(c, icon("arrow-right")).Size(12, 12).TextColor(p.textTertiary)
						ui.Text(c, b.BenefitKey).FontSize(12).Font(fontMono).Grow(1)
						ub := ui.ButtonBase(c.Key("unbind")).Height(26).Padding(0, 8).Radius(5).Center().FocusRing(false).Children(func() {
							ui.Text(c, m("commerce_unbind")).FontSize(11).TextColor(p.textTertiary)
						})
						if ub.Clicked() {
							q := url.Values{"productKey": {b.ProductKey}, "benefitKey": {b.BenefitKey}}
							a.act(func(ctx context.Context) error {
								return a.api.Do(ctx, "DELETE", base+"/product-benefits?"+q.Encode(), nil, nil)
							}, reloadAll...)
						}
					})
				}
			})
			section("orders", m("commerce_orders"), "", "", nil, func() {
				if orders == nil || len(orders.Orders) == 0 {
					empty(m("commerce_no_orders"), "")
					return
				}
				for i, o := range orders.Orders {
					o := o
					row(o.ID, i, func() {
						if pr := o.BuyerProfile; pr != nil {
							a.avatar(c, pr.AvatarURL, pr.Name(), 20, 10)
						}
						ui.Column(c).Grow(1).Gap(2).Children(func() {
							ui.Text(c, o.ProductNameSnapshot).FontSize(13)
							ui.Text(c, o.Status+" · "+i18n.LongDate(o.CreatedAt)).FontSize(11).TextColor(p.textTertiary)
						})
						ui.Text(c, formatMoney(o.PaidAmountSnapshot)).FontSize(13).Font(fontMono)
					})
				}
			})
		})
	})
	a.commerceDialog(c, base, reloadAll)
}

func (a *app) commerceDialog(c *ui.Context, base string, reload []string) {
	f := a.commerce
	if f == nil {
		return
	}
	p := a.pal
	open := true
	title := map[string]string{"product": m("commerce_create_product"), "benefit": m("commerce_create_benefit"), "bind": m("commerce_new_binding")}[f.kind]
	if f.editing != "" {
		title = map[string]string{"product": m("commerce_edit_product"), "benefit": m("commerce_edit_benefit")}[f.kind]
	}
	a.dialog(c, &open, title, 440, func() {
		ui.Column(c.Key("form")).Padding(16).Gap(12).Children(func() {
			label := func(t string) {
				ui.Text(c, strings.ToUpper(t)).FontSize(10).FontWeight(500).LetterSpacing(0.6).TextColor(p.textTertiary)
			}
			if f.kind == "bind" {
				prods, _, _ := loadJSON[struct {
					Products []commerceProduct `json:"products"`
				}](a, "commerce-products:"+a.route.space, base+"/products")
				bens, _, _ := loadJSON[struct {
					Benefits []commerceBenefit `json:"benefits"`
				}](a, "commerce-benefits:"+a.route.space, base+"/benefits")
				var pk, bk []string
				if prods != nil {
					for _, x := range prods.Products {
						pk = append(pk, x.Key)
					}
				}
				if bens != nil {
					for _, x := range bens.Benefits {
						bk = append(bk, x.Key)
					}
				}
				label(m("commerce_product_label"))
				ui.Select(c.Key("product"), &f.product, pk).Height(32)
				label(m("commerce_benefit_label"))
				ui.Select(c.Key("benefit"), &f.benefit, bk).Height(32)
				ui.Text(c, m("commerce_bind_hint_immediate")).FontSize(11).TextColor(p.textTertiary)
			} else {
				label(m("x_name"))
				a.field(c, "name", &f.name, "")
				label(m("space_description"))
				a.field(c, "desc", &f.desc, "")
				if f.kind == "product" && f.editing == "" {
					label("US$")
					a.field(c, "amount", &f.amount, "10").Font(fontMono)
				}
				if f.kind == "benefit" && f.editing == "" {
					ui.Row(c.Key("type")).Gap(8).Children(func() {
						for _, t := range []string{"feature", "credits"} {
							t := t
							on := f.benefitType == t
							b := ui.ButtonBase(c.Key(t)).Height(28).Padding(0, 12).Radius(6).Center().Border(1, p.borderSubtle).FocusRing(false).Children(func() {
								ui.Text(c, m("commerce_"+t)).FontSize(12)
							})
							if on {
								b.Border(1, p.brand).Background(p.brandMuted)
							}
							if b.Clicked() {
								f.benefitType = t
							}
						}
					})
					if f.benefitType == "credits" {
						label(m("commerce_credits"))
						a.field(c, "credits", &f.amount, "100").Font(fontMono)
					}
				}
			}
			if f.err != "" {
				ui.Text(c, f.err).FontSize(12).TextColor(p.errorC)
			}
			ui.Row(c.Key("foot")).Gap(8).Children(func() {
				ui.Spacer(c)
				if a.button(c, "", m("commerce_cancel"), false).Clicked() {
					a.commerce = nil
				}
				if a.button(c, "check", title, true).Clicked() && !f.busy {
					a.submitCommerce(f, base, reload)
				}
			})
		})
	})
	if !open {
		a.commerce = nil
	}
}

func (a *app) submitCommerce(f *commerceForm, base string, reload []string) {
	name, desc := strings.TrimSpace(f.name), strings.TrimSpace(f.desc)
	var method, path string
	in := map[string]any{}
	switch {
	case f.kind == "bind":
		method, path = "POST", base+"/product-benefits"
		in["productKey"], in["benefitKey"] = f.product, f.benefit
	case name == "":
		f.err = m("x_name") + "?"
		return
	case f.editing != "":
		method, path = "PATCH", base+"/"+f.kind+"s/"+url.PathEscape(f.editing)
		in["name"], in["description"] = name, desc
	case f.kind == "product":
		amount, err := strconv.ParseFloat(strings.TrimSpace(f.amount), 64)
		if err != nil || amount <= 0 {
			f.err = "US$?"
			return
		}
		method, path = "POST", base+"/products"
		in["name"], in["amountUsd"], in["status"], in["visibility"] = name, amount, "active", "public"
		if desc != "" {
			in["description"] = desc
		}
	default:
		method, path = "POST", base+"/benefits"
		in["name"], in["type"] = name, f.benefitType
		if desc != "" {
			in["description"] = desc
		}
		if f.benefitType == "credits" {
			n, err := strconv.ParseFloat(strings.TrimSpace(f.amount), 64)
			if err != nil || n <= 0 {
				f.err = m("commerce_credits") + "?"
				return
			}
			in["amount"] = n
		}
	}
	f.busy, f.err = true, ""
	a.act(func(ctx context.Context) error {
		err := a.api.Do(ctx, method, path, in, nil)
		a.update(func() {
			f.busy = false
			if err != nil {
				f.err = err.Error()
				return
			}
			a.commerce = nil
		})
		return nil
	}, reload...)
}
