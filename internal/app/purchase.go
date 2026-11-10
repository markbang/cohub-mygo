package app

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"cohub-mygo/internal/i18n"
)

// The billing catalog and the purchase panel of the billing settings'
// plans tab: features/billing/{product.ts, PurchasePanel, ProductCard,
// CheckoutSummary}. Paying happens on the web: the app opens the plans tab
// in the browser with the product chosen there.

type catalogPricing struct {
	AmountMinor        int64   `json:"amountMinor"`
	AmountUSD          float64 `json:"amountUsd"`
	CompareAtAmountUSD float64 `json:"compareAtAmountUsd"`
	DiscountRate       float64 `json:"discountRate"`
}

type catalogOffer struct {
	EndsAt  *time.Time `json:"endsAt"`
	Pricing struct {
		AmountUSD         float64 `json:"amountUsd"`
		DiscountAmountUSD float64 `json:"discountAmountUsd"`
		PaidAmountUSD     float64 `json:"paidAmountUsd"`
	} `json:"pricing"`
}

type catalogProduct struct {
	Key         string         `json:"key"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Visibility  string         `json:"visibility"`
	Kind        string         `json:"kind"`     // plan, addon
	Interval    string         `json:"interval"` // monthly, yearly, one_time
	Pricing     catalogPricing `json:"pricing"`
	Offer       *catalogOffer  `json:"offer"`
	Display     struct {
		Description    *string  `json:"description"`
		Benefits       []string `json:"benefits"`
		CreditsAmount  float64  `json:"creditsAmount"`
		CreditBenefits []struct {
			CycleAmountUSD  float64 `json:"cycleAmountUsd"`
			PeriodAmountUSD float64 `json:"periodAmountUsd"`
		} `json:"creditBenefits"`
	} `json:"display"`
}

type billingCatalog struct {
	Catalog struct {
		Payment struct {
			Available bool    `json:"available"`
			Reason    *string `json:"reason"`
		} `json:"payment"`
		Products             []*catalogProduct `json:"products"`
		Plans                []*catalogProduct `json:"plans"`
		Addons               []*catalogProduct `json:"addons"`
		CurrentSubscriptions []struct {
			ProductKey string `json:"productKey"`
			Status     string `json:"status"`
		} `json:"currentSubscriptions"`
		HasActiveSubscription bool   `json:"hasActiveSubscription"`
		DefaultPlanProductKey string `json:"defaultPlanProductKey"`
	} `json:"catalog"`
}

type subscriptionHistory struct {
	Subscriptions struct {
		Page       int `json:"page"`
		Items      []subscriptionRow
		Pagination struct {
			HasMore bool `json:"hasMore"`
		} `json:"pagination"`
	} `json:"subscriptions"`
}

type subscriptionRow struct {
	ID                 string     `json:"id"`
	ProductName        string     `json:"productName"`
	Status             string     `json:"status"`
	AmountUSD          float64    `json:"amountUsd"`
	PaidAmountUSD      float64    `json:"paidAmountUsd"`
	CurrentPeriodStart *time.Time `json:"currentPeriodStart"`
	CurrentPeriodEnd   *time.Time `json:"currentPeriodEnd"`
	CancelAtPeriodEnd  bool       `json:"cancelAtPeriodEnd"`
}

// catalogView is product.ts catalogView: public paid plans by interval,
// packs, and the plan the user is on.
type catalogView struct {
	monthly, yearly, packs []*catalogProduct
	current                *catalogProduct
	hasPaidPlan            bool
}

func viewCatalog(cat *billingCatalog) catalogView {
	var v catalogView
	c := &cat.Catalog
	free := func(p *catalogProduct) bool { return p.Pricing.AmountUSD == 0 }
	for _, p := range c.Plans {
		if p.Visibility != "public" || p.Key == c.DefaultPlanProductKey || free(p) {
			continue
		}
		switch p.Interval {
		case "monthly":
			v.monthly = append(v.monthly, p)
		case "yearly":
			v.yearly = append(v.yearly, p)
		}
	}
	v.packs = append(v.packs, c.Addons...)
	for _, list := range [][]*catalogProduct{v.monthly, v.yearly, v.packs} {
		sort.SliceStable(list, func(i, j int) bool { return list[i].Pricing.AmountMinor < list[j].Pricing.AmountMinor })
	}
	for _, s := range c.CurrentSubscriptions {
		if s.Status != "active" && s.Status != "trialing" {
			continue
		}
		for _, p := range c.Plans {
			if p.Key == s.ProductKey {
				v.current = p
			}
		}
		v.hasPaidPlan = s.ProductKey != "" && s.ProductKey != c.DefaultPlanProductKey
		break
	}
	return v
}

func (v catalogView) plans(interval string) []*catalogProduct {
	if interval == "yearly" && len(v.yearly) > 0 {
		return v.yearly
	}
	if len(v.monthly) == 0 {
		return v.yearly
	}
	return v.monthly
}

func planTier(p *catalogProduct) string {
	s := strings.ToLower(p.Key + " " + p.Name)
	switch {
	case p.Pricing.AmountUSD == 0 || strings.Contains(s, "free"):
		return "free"
	case strings.Contains(s, "max"):
		return "max"
	case strings.Contains(s, "pro") || strings.Contains(s, "standard"):
		return "pro"
	case strings.Contains(s, "plus"):
		return "plus"
	}
	return p.Key
}

func recommendedPlan(plans []*catalogProduct) *catalogProduct {
	for _, p := range plans {
		if planTier(p) == "pro" {
			return p
		}
	}
	if len(plans) == 0 {
		return nil
	}
	return plans[min(1, len(plans)-1)]
}

func recommendedPack(packs []*catalogProduct) *catalogProduct {
	var pool []*catalogProduct
	for _, p := range packs {
		if p.Offer != nil {
			pool = append(pool, p)
		}
	}
	if len(pool) == 0 {
		pool = packs
	}
	if len(pool) == 0 {
		return nil
	}
	return pool[min(1, len(pool)-1)]
}

func (p *catalogProduct) balance() float64 {
	if b := p.Display.CreditBenefits; len(b) > 0 {
		sum := 0.0
		for _, x := range b {
			sum += x.CycleAmountUSD
		}
		return sum
	}
	if p.Display.CreditsAmount > 0 {
		return p.Display.CreditsAmount * 1e-8
	}
	return p.Pricing.AmountUSD
}

func (p *catalogProduct) periodBalance() float64 {
	if b := p.Display.CreditBenefits; len(b) > 0 {
		sum := 0.0
		for _, x := range b {
			sum += x.PeriodAmountUSD
		}
		return sum
	}
	return p.balance()
}

// price is displayPrice for a signed-in viewer: the offer's when one
// applies.
func (p *catalogProduct) price() (amount, was float64, percent int, offer bool) {
	if o := p.Offer; o != nil && o.Pricing.DiscountAmountUSD > 0 {
		return o.Pricing.PaidAmountUSD, o.Pricing.AmountUSD, int(math.Round(o.Pricing.DiscountAmountUSD / o.Pricing.AmountUSD * 100)), true
	}
	return p.Pricing.AmountUSD, 0, 0, false
}

func (p *catalogProduct) savings(monthly []*catalogProduct) int {
	if r := p.Pricing.DiscountRate; r > 0 {
		return int(math.Round(r * 100))
	}
	if ca := p.Pricing.CompareAtAmountUSD; ca > p.Pricing.AmountUSD {
		return int(math.Round((ca - p.Pricing.AmountUSD) / ca * 100))
	}
	if p.Interval != "yearly" {
		return 0
	}
	for _, mp := range monthly {
		if planTier(mp) == planTier(p) && mp.Pricing.AmountUSD > 0 {
			year := mp.Pricing.AmountUSD * 12
			return max(0, int(math.Round((year-p.Pricing.AmountUSD)/year*100)))
		}
	}
	return 0
}

// price formats are format.ts: whole dollars stay whole, else cents.
func formatPrice(v float64) string {
	if v == math.Trunc(v) {
		return "US$" + groupThousands(fmt.Sprintf("%.0f", v))
	}
	return formatMoney(v)
}

func formatMoney(v float64) string {
	s := fmt.Sprintf("%.2f", math.Abs(v))
	whole, frac, _ := strings.Cut(s, ".")
	out := "US$" + groupThousands(whole) + "." + frac
	if v < 0 {
		out = "-" + out
	}
	return out
}

func groupThousands(s string) string {
	var b strings.Builder
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	return b.String()
}

// purchaseState is the plans tab's choices, kept across frames.
type purchaseState struct {
	family   string // plans, packs
	interval string // monthly, yearly
	selected string
	codeOpen bool
	code     string
}

// plansTab is the purchase panel and the subscription history.
func (a *app) plansTab(c *ui.Context) {
	p := a.pal
	cat, cerr, _ := loadJSON[billingCatalog](a, "catalog", "/api/billing/catalog")
	s := &a.purchase
	if cat == nil {
		ui.Column(c.Key("catalog-wait")).Padding(20, 0).Children(func() { a.loadingBlock(c, cerr, 220) })
		return
	}
	v := viewCatalog(cat)
	if s.interval == "" {
		s.interval = "monthly"
	}
	if s.family == "" {
		s.family = "plans"
		if len(v.monthly)+len(v.yearly) == 0 || (len(v.packs) > 0 && v.hasPaidPlan) {
			s.family = "packs"
		}
	}
	plans := v.plans(s.interval)
	products := plans
	if s.family == "packs" {
		products = v.packs
	}
	var selected *catalogProduct
	for _, pr := range cat.Catalog.Products {
		if pr.Key == s.selected {
			selected = pr
		}
	}
	if selected == nil {
		pick := recommendedPlan(plans)
		if s.family == "packs" {
			pick = recommendedPack(v.packs)
		}
		if pick != nil {
			s.selected, selected = pick.Key, pick
		}
	}
	recPack := recommendedPack(v.packs)
	wide := a.width >= 960+240
	ui.Row(c.Key("purchase")).Gap(40).AlignItems(ui.Start).Padding(20, 0).Children(func() {
		ui.Column(c.Key("catalog")).Grow(1).Basis(0).MinWidth(0).Gap(18).Children(func() {
			ui.Row(c.Key("controls")).AlignItems(ui.Center).Gap(10).Children(func() {
				if len(v.monthly)+len(v.yearly) > 0 && len(v.packs) > 0 {
					a.segmentedControl(c, "family", []segment{{"plans", m("purchase_family_plans"), ""}, {"packs", m("purchase_family_packs"), ""}}, s.family, func(f string) {
						s.family = f
						s.selected = ""
					})
				}
				ui.Spacer(c)
				if s.family == "plans" && len(plans) > 0 {
					note := ""
					best := 0
					for _, y := range v.yearly {
						best = max(best, y.savings(v.monthly))
					}
					if best > 0 {
						note = fmt.Sprintf("−%d%%", best)
					}
					a.segmentedControl(c, "interval", []segment{{"monthly", m("billing_monthly"), ""}, {"yearly", m("billing_yearly"), note}}, s.interval, func(iv string) {
						prev := selected
						s.interval = iv
						if prev != nil && prev.Kind == "plan" {
							s.selected = ""
							for _, pl := range v.plans(iv) {
								if planTier(pl) == planTier(prev) {
									s.selected = pl.Key
								}
							}
						}
					})
				}
			})
			cols := 3
			if !wide {
				cols = 2
			}
			cols = min(cols, max(1, len(products)))
			cardGrid(c, cols, 14, len(products), func(i int) {
				pr := products[i]
				rec := pr.Key == recPack.keyOr()
				if pr.Kind == "plan" {
					rec = recommendedPlan(plans) == pr
				}
				a.productCard(c, pr, s.selected == pr.Key, rec, v.current == pr, v.monthly, func() { s.selected = pr.Key })
			})
			if s.family == "packs" && !v.hasPaidPlan && len(v.monthly)+len(v.yearly) > 0 {
				ui.Row(c.Key("hint")).Gap(4).Children(func() {
					ui.Text(c, m("purchase_hint_plans_cheaper")).FontSize(11.5).TextColor(p.textTertiary)
					l := ui.Text(c, m("purchase_family_plans")+" →").FontSize(11.5).TextColor(p.brandMutedFg).Cursor(ui.CursorPointer)
					l.OnClick(func() { s.family, s.selected = "plans", "" })
				})
			}
		})
		if wide {
			ui.Column(c.Key("aside")).Width(280).Shrink(0).Padding(20).Radius(10).Border(1, p.borderSubtle).Background(p.bgContent).Children(func() {
				a.checkoutSummary(c, cat, v, selected)
			})
		}
	})
	if !wide && selected != nil {
		ui.Column(c.Key("summary-inline")).Padding(16).Radius(10).Border(1, p.borderSubtle).Background(p.bgContent).Children(func() {
			a.checkoutSummary(c, cat, v, selected)
		})
	}
	a.subscriptionHistory(c)
}

func (p *catalogProduct) keyOr() string {
	if p == nil {
		return ""
	}
	return p.Key
}

type segment struct{ value, label, note string }

// segmented is .segmented: options on a sunken track, the chosen one on a
// raised thumb.
func (a *app) segmentedControl(c *ui.Context, key string, opts []segment, value string, choose func(string)) {
	p := a.pal
	thumb := p.bgInput
	if thumb.A < 128 {
		thumb = p.bgPrimary
	}
	ui.Row(c.Key(key)).Shrink(0).Padding(2).Radius(7).Border(1, p.borderSubtle).Background(p.bgSurfaceHover).Children(func() {
		for _, o := range opts {
			on := o.value == value
			b := ui.ButtonBase(c.Key(o.value)).Row().Gap(6).AlignItems(ui.Center).Height(30).Padding(0, 12).Radius(5).Label(o.label).FocusRing(false).Children(func() {
				col := p.textTertiary
				if on {
					col = p.text
				}
				ui.Text(c, o.label).FontSize(12).TextColor(col)
				if o.note != "" {
					ui.Text(c, o.note).FontSize(10).FontWeight(600).Font(fontMono).TextColor(p.brand)
				}
			})
			if on {
				b.Background(thumb).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.08))
			}
			value := o.value
			if b.Clicked() && !on {
				choose(value)
			}
		}
	})
}

// productCard is ProductCard.svelte, full variant.
func (a *app) productCard(c *ui.Context, pr *catalogProduct, selected, recommended, current bool, monthly []*catalogProduct, choose func()) {
	p := a.pal
	amount, was, percent, offer := pr.price()
	yearly := pr.Interval == "yearly"
	border := p.borderSubtle
	switch {
	case selected:
		border = p.brand
	case recommended:
		border = p.borderSubtle.MixOklab(p.brand, 0.4)
	}
	card := ui.ButtonBase(c.Key(pr.Key)).Column().AlignItems(ui.Stretch).Grow(1).Gap(14).Padding(18, 18, 16, 18).Radius(10).Border(1, border).Background(p.bgContent).FocusRing(false).Label(pr.Name)
	if selected {
		card.Border(2, p.brand).Padding(17, 17, 15, 17)
	}
	if current {
		card.Background(p.bgSurfaceHover)
	}
	card.Children(func() {
		if recommended && !current {
			a.flag(c, m("purchase_most_popular"), true)
		} else if current {
			a.flag(c, m("purchase_current_plan"), false)
		}
		ui.Row(c.Key("head")).Gap(10).AlignItems(ui.Start).Children(func() {
			ui.Text(c, pr.Name).FontSize(14).FontWeight(600).Grow(1).MinWidth(0)
			if offer && percent > 0 {
				ui.Text(c, m("purchase_first_order_off", "percent", percent)).FontSize(11).FontWeight(600).Font(fontMono).
					TextColor(p.brandMutedFg).Background(p.brandMuted).Padding(2, 6).Radius(4).Shrink(0)
			} else if sv := pr.savings(monthly); sv > 0 {
				ui.Text(c, fmt.Sprintf("−%d%%", sv)).FontSize(11).FontWeight(600).Font(fontMono).TextColor(p.textTertiary).Shrink(0)
			}
		})
		ui.Column(c.Key("price")).Gap(2).MinHeight(58).Children(func() {
			if pr.Pricing.AmountUSD == 0 {
				ui.Text(c, m("pricing_free")).FontSize(32).FontWeight(600).LetterSpacing(-1.4)
				return
			}
			if offer {
				ui.Text(c, formatPrice(was)).FontSize(13).Font(fontMono).TextColor(p.placeholder).Strikethrough()
			}
			ui.Row(c.Key("figure")).Gap(5).AlignItems(ui.End).Children(func() {
				ui.Text(c, formatPrice(amount)).FontSize(32).FontWeight(600).LetterSpacing(-1.4)
				if pr.Kind != "addon" {
					unit := m("purchase_per_month")
					if yearly {
						unit = m("purchase_per_year")
					}
					ui.Text(c, unit).FontSize(12).TextColor(p.textTertiary).Margin(0, 0, 3, 0)
				}
			})
			switch {
			case yearly:
				ui.Text(c, m("purchase_per_month_equiv", "amount", formatPrice(amount/12))).FontSize(11).TextColor(p.textTertiary)
			case offer:
				ui.Text(c, m("purchase_then_price", "amount", formatPrice(pr.Pricing.AmountUSD))).FontSize(11).TextColor(p.textTertiary)
			}
		})
		ui.Column(c.Key("facts")).Gap(7).Padding(12, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle.Alpha(0.6)).Children(func() {
			first := m("purchase_balance_monthly", "amount", formatPrice(pr.balance()))
			switch {
			case pr.Kind == "addon":
				first = m("purchase_balance_once", "amount", formatPrice(pr.balance()))
			case yearly:
				first = m("purchase_balance_yearly", "month", formatPrice(pr.balance()), "year", formatPrice(pr.periodBalance()))
			}
			fact := func(k, text string, lead bool) {
				ui.Row(c.Key(k)).Gap(8).AlignItems(ui.Start).Children(func() {
					ui.Icon(c, icon("check")).Size(13, 13).TextColor(p.brand).Margin(2, 0, 0, 0)
					t := ui.Text(c, text).FontSize(12).TextColor(p.textSecond).Grow(1).MinWidth(0)
					if lead {
						t.TextColor(p.text).FontWeight(500)
					}
				})
			}
			fact("balance", first, true)
			benefits := pr.Display.Benefits
			if len(benefits) > 4 {
				benefits = benefits[:4]
			}
			for i, b := range benefits {
				fact(fmt.Sprint("b", i), b, false)
			}
			desc := pr.Description
			if pr.Display.Description != nil {
				desc = *pr.Display.Description
			}
			if desc != "" && len(benefits) == 0 {
				ui.Text(c, desc).FontSize(12).TextColor(p.textTertiary)
			}
		})
	})
	if !selected && card.Hovered() {
		card.Border(1, p.border)
	}
	if card.Clicked() {
		choose()
	}
}

// flag is the .badge riding the card's top edge.
func (a *app) flag(c *ui.Context, label string, promo bool) {
	p := a.pal
	bg, fg := p.brand, p.brandFg
	if !promo {
		bg, fg = p.bgElevated, p.textSecond
	}
	ui.Text(c.Key("flag"), strings.ToUpper(label)).FontSize(10).FontWeight(600).LetterSpacing(0.6).TextColor(fg).
		Background(bg).Padding(2, 7).Radius(4).Absolute().Top(-9).Left(16)
}

// checkoutSummary is CheckoutSummary.svelte.
func (a *app) checkoutSummary(c *ui.Context, cat *billingCatalog, v catalogView, pr *catalogProduct) {
	p := a.pal
	if pr == nil {
		ui.Text(c, m("purchase_pick_prompt")).FontSize(12).TextColor(p.textTertiary)
		return
	}
	plan := pr.Kind == "plan"
	yearly := pr.Interval == "yearly"
	list, discount, due := pr.Pricing.AmountUSD, 0.0, pr.Pricing.AmountUSD
	if o := pr.Offer; o != nil {
		list, discount, due = o.Pricing.AmountUSD, o.Pricing.DiscountAmountUSD, o.Pricing.PaidAmountUSD
	}
	ui.Column(c.Key("summary")).Gap(16).Children(func() {
		ui.Column(c.Key("selection")).Gap(2).Children(func() {
			l := m("purchase_selected_pack")
			if plan {
				l = m("purchase_selected_plan")
			}
			ui.Text(c, strings.ToUpper(l)).FontSize(11).LetterSpacing(0.9).TextColor(p.textTertiary)
			ui.Text(c, pr.Name).FontSize(16).FontWeight(600)
		})
		ui.Column(c.Key("lines")).Gap(8).Children(func() {
			line := func(k, label, value string, valueCol ui.Color) {
				ui.Row(c.Key(k)).Gap(12).Children(func() {
					ui.Text(c, label).FontSize(12).TextColor(p.textTertiary).Grow(1)
					ui.Text(c, value).FontSize(12).Font(fontMono).TextColor(valueCol)
				})
			}
			kind := m("purchase_line_one_time")
			if plan {
				kind = m("purchase_line_monthly")
				if yearly {
					kind = m("purchase_line_yearly")
				}
			}
			line("list", kind, formatMoney(list), p.textSecond)
			if discount > 0 {
				line("discount", m("purchase_first_order_discount"), "−"+formatMoney(discount), p.success)
			}
			ui.Row(c.Key("total")).Gap(12).AlignItems(ui.End).Padding(10, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
				ui.Text(c, m("purchase_due_today")).FontSize(12).FontWeight(500).Grow(1).Margin(0, 0, 3, 0)
				ui.Text(c, formatMoney(due)).FontSize(22).FontWeight(600).Font(fontMono).LetterSpacing(-0.6)
			})
		})
		ui.Column(c.Key("notes")).Gap(4).Children(func() {
			note := func(k, t string) { ui.Text(c.Key(k), t).FontSize(11.5).TextColor(p.textTertiary) }
			switch {
			case !plan:
				note("bal", m("purchase_note_balance_once", "amount", formatPrice(pr.balance())))
			case yearly:
				note("bal", m("purchase_note_balance_yearly", "amount", formatPrice(pr.periodBalance())))
			default:
				note("bal", m("purchase_note_balance_monthly", "amount", formatPrice(pr.balance())))
			}
			if plan && discount > 0 {
				k := "purchase_renews_monthly"
				if yearly {
					k = "purchase_renews_yearly"
				}
				note("renew", m(k, "amount", formatPrice(pr.Pricing.AmountUSD)))
			}
			if plan {
				note("cancel", m("purchase_note_cancel_anytime"))
			} else {
				note("valid", m("purchase_note_pack_validity"))
			}
		})
		blocked := ""
		switch {
		case !cat.Catalog.Payment.Available:
			blocked = m("billing_payment_unavailable_plain")
			if r := cat.Catalog.Payment.Reason; r != nil {
				blocked = *r
			}
		case plan && v.current == pr:
			blocked = m("purchase_already_on_plan")
		case plan && cat.Catalog.HasActiveSubscription:
			blocked = m("purchase_active_subscription_blocks")
		}
		cta := m("purchase_cta_add_balance")
		if plan {
			cta = m("purchase_cta_subscribe")
		}
		b := ui.ButtonBase(c.Key("cta")).Height(44).Center().Radius(6).Background(p.brand).FocusRing(false).Label(cta).Children(func() {
			ui.Text(c, cta).FontSize(13).FontWeight(600).TextColor(p.brandFg)
		})
		if blocked != "" {
			b.Opacity(0.5)
		} else if b.Hovered() {
			b.Background(p.brandHover)
		}
		if b.Clicked() && blocked == "" {
			c.OpenURL("https://cohub.live/settings/billing?tab=plans")
		}
		if blocked != "" {
			ui.Text(c.Key("blocked"), blocked).FontSize(11).TextColor(p.textTertiary).Margin(-6, 0, 0, 0)
		}
		s := &a.purchase
		code := ui.Row(c.Key("code-toggle")).Gap(4).AlignItems(ui.Center).AlignSelf(ui.Start).Cursor(ui.CursorPointer).Children(func() {
			ui.Text(c, m("purchase_have_code")).FontSize(11).TextColor(p.textTertiary)
			ch := "chevron-down"
			if s.codeOpen {
				ch = "chevron-up"
			}
			ui.Icon(c, icon(ch)).Size(14, 14).TextColor(p.textTertiary)
		})
		code.OnClick(func() { s.codeOpen = !s.codeOpen })
		if s.codeOpen {
			ui.Row(c.Key("code-form")).Gap(6).Children(func() {
				ui.TextInputBase(c.Key("code"), &s.code).Placeholder(m("billing_checkout_enter_code")).PlaceholderColor(p.placeholder).
					FontSize(12).Font(fontMono).Height(36).Padding(0, 10).Radius(5).Border(1, p.borderSubtle).Background(p.bgInput).Grow(1).MinWidth(0)
				ui.ButtonBase(c.Key("apply")).Height(36).Padding(0, 10).MinWidth(64).Center().Radius(5).Border(1, p.borderSubtle).Background(p.bgInput).FocusRing(false).Children(func() {
					ui.Text(c, m("billing_checkout_apply")).FontSize(12).FontWeight(500)
				})
			})
		}
		ui.Text(c.Key("trust"), m("purchase_trust_line")).FontSize(11).TextColor(p.textTertiary)
	})
}

// subscriptionHistory is the plans tab's list of subscriptions, ten a page.
func (a *app) subscriptionHistory(c *ui.Context) {
	p := a.pal
	page := max(1, a.subsPage)
	h, herr, _ := loadJSON[subscriptionHistory](a, fmt.Sprint("subs-page-", page), fmt.Sprintf("/api/billing/subscriptions?page=%d&limit=10", page))
	ui.Column(c.Key("subs")).Margin(6, 0, 0, 0).Padding(20, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Row(c.Key("subs-head")).AlignItems(ui.Center).Children(func() {
			ui.Text(c, m("billing_subscriptions_heading")).FontSize(14).FontWeight(500).Grow(1)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				pager := func(k, ic string, ok bool, to int) {
					b := ui.ButtonBase(c.Key(k)).Size(28, 28).Center().Radius(5).Border(1, p.borderSubtle).Background(p.bgInput).FocusRing(false).Children(func() {
						ui.Icon(c, icon(ic)).Size(14, 14).TextColor(p.textTertiary)
					})
					if !ok {
						b.Opacity(0.45)
					} else if b.Clicked() {
						a.subsPage = to
					}
				}
				more := h != nil && h.Subscriptions.Pagination.HasMore
				pager("prev", "chevron-left", page > 1, page-1)
				ui.Text(c, m("billing_page", "page", page)).FontSize(11).TextColor(p.textTertiary)
				pager("next", "chevron-right", more, page+1)
			})
		})
		switch {
		case h == nil:
			ui.Column(c.Key("subs-wait")).Margin(12, 0, 0, 0).Children(func() { a.loadingBlock(c, herr, 128) })
		case len(h.Subscriptions.Items) == 0:
			ui.Text(c.Key("subs-none"), m("billing_no_subscriptions_yet")).FontSize(12).TextColor(p.textTertiary).Margin(16, 0, 0, 0)
		default:
			ui.Column(c.Key("subs-list")).Margin(12, 0, 0, 0).Radius(6).Border(1, p.borderSubtle).Children(func() {
				for i, s := range h.Subscriptions.Items {
					row := ui.Row(c.Key(s.ID)).Gap(12).AlignItems(ui.Center).Padding(12)
					if i > 0 {
						row.BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle)
					}
					row.Children(func() {
						ui.Column(c).Grow(1.2).Basis(0).MinWidth(0).Gap(2).Children(func() {
							ui.Text(c, s.ProductName).FontSize(12).FontWeight(500).SingleLine()
							ui.Text(c, shortID(s.ID)).FontSize(10).Font(fontMono).TextColor(p.placeholder).SingleLine()
						})
						ui.Column(c).Grow(0.9).Basis(0).MinWidth(0).Gap(2).Children(func() {
							ui.Text(c, historyStatus(s.Status)).FontSize(12).TextColor(p.textSecond).SingleLine()
							if s.CancelAtPeriodEnd {
								ui.Text(c, m("billing_auto_renew_canceled")).FontSize(11).TextColor(p.textTertiary)
							}
						})
						ui.Text(c, billingPeriod(s)).FontSize(12).TextColor(p.textTertiary).Grow(1.4).Basis(0).MinWidth(0).SingleLine()
						amt := s.AmountUSD
						if s.PaidAmountUSD > 0 {
							amt = s.PaidAmountUSD
						}
						ui.Text(c, formatMoney(amt)).FontSize(12).Font(fontMono).Grow(0.7).Basis(0).TextAlign(ui.End)
						ui.Text(c, m("billing_no_actions")).FontSize(11).TextColor(p.placeholder).Shrink(0)
					})
				}
			})
		}
	})
}

func shortID(s string) string {
	if len(s) <= 18 {
		return s
	}
	return s[:10] + "…" + s[len(s)-6:]
}

func historyStatus(v string) string {
	switch v {
	case "active":
		return m("billing_status_active")
	case "trialing":
		return m("billing_status_trialing")
	case "past_due":
		return m("billing_recovery_pending")
	case "pending_checkout":
		return m("billing_status_pending_checkout")
	}
	parts := strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '_' || r == '-' })
	for i, s := range parts {
		parts[i] = strings.ToUpper(s[:1]) + s[1:]
	}
	return strings.Join(parts, " ")
}

func billingPeriod(s subscriptionRow) string {
	switch {
	case s.CurrentPeriodStart == nil && s.CurrentPeriodEnd == nil:
		return m("billing_no_active_period")
	case s.CurrentPeriodStart == nil:
		return m("billing_until", "date", mediumDateTime(*s.CurrentPeriodEnd))
	case s.CurrentPeriodEnd == nil:
		return m("billing_from", "date", mediumDateTime(*s.CurrentPeriodStart))
	}
	return mediumDateTime(*s.CurrentPeriodStart) + " - " + mediumDateTime(*s.CurrentPeriodEnd)
}

// mediumDateTime is formatDateTime with dateStyle medium, timeStyle short.
func mediumDateTime(t time.Time) string {
	if i18n.Locale() == "zh-CN" {
		return t.Local().Format("2006年1月2日 15:04")
	}
	return t.Local().Format("Jan 2, 2006, 3:04 PM")
}

// redeemTab is the redemption code form.
func (a *app) redeemTab(c *ui.Context) {
	p := a.pal
	s := &a.redeem
	ui.Column(c.Key("redeem")).MaxWidth(576).Padding(20, 0).Gap(16).Children(func() {
		ui.Text(c, m("billing_redeem_code")).FontSize(14).FontWeight(500)
		ui.Row(c.Key("form")).Gap(8).Children(func() {
			in := ui.TextInputBase(c.Key("code"), &s.code).Placeholder(m("billing_redemption_code_placeholder")).PlaceholderColor(p.placeholder).
				FontSize(13).Font(fontMono).Height(36).Padding(0, 12).Radius(5).Border(1, p.borderSubtle).Background(p.bgInput).Grow(1).MinWidth(0).Label(m("billing_redemption_code_label"))
			if in.Focused() {
				in.Border(1, p.brand)
			}
			ready := strings.TrimSpace(s.code) != "" && !s.busy
			b := ui.ButtonBase(c.Key("submit")).Row().Gap(6).AlignItems(ui.Center).Height(36).Padding(0, 12).Radius(5).Border(1, p.borderSubtle).Background(p.bgInput).FocusRing(false).Children(func() {
				if s.busy {
					a.spinner(c, 14, p.text)
					ui.Text(c, m("billing_redeeming")).FontSize(12).FontWeight(500)
				} else {
					ui.Icon(c, icon("gift")).Size(14, 14).TextColor(p.text)
					ui.Text(c, m("billing_redeem")).FontSize(12).FontWeight(500)
				}
			})
			if !ready {
				b.Opacity(0.55)
			} else if b.Hovered() {
				b.Background(p.hover)
			}
			if (b.Clicked() || in.Submitted()) && ready {
				a.submitRedemption()
			}
		})
		if s.err != "" {
			ui.Row(c.Key("err")).Gap(8).Padding(12).Radius(6).Border(1, p.errorC.Alpha(0.3)).Background(p.errorC.Alpha(0.08)).Children(func() {
				ui.Icon(c, icon("circle-alert")).Size(14, 14).TextColor(p.errorC)
				ui.Text(c, s.err).FontSize(12).TextColor(p.errorC).Grow(1).MinWidth(0)
			})
		}
		if s.ok != "" {
			ui.Text(c.Key("ok"), s.ok).FontSize(12).TextColor(p.success).Padding(12).Radius(6).Border(1, p.success.Alpha(0.3)).Background(p.success.Alpha(0.08))
		}
	})
}

type redeemState struct {
	code, err, ok string
	busy          bool
}

func (a *app) submitRedemption() {
	s := &a.redeem
	code := strings.TrimSpace(s.code)
	s.busy, s.err, s.ok = true, "", ""
	if a.api == nil {
		s.busy, s.err = false, m("billing_redeem_failed")
		return
	}
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var out struct {
			Redemption struct {
				Redeemed bool    `json:"redeemed"`
				Message  *string `json:"message"`
			} `json:"redemption"`
		}
		err := a.api.Do(ctx, "POST", "/api/billing/redemptions", map[string]string{"code": code}, &out)
		a.update(func() {
			s.busy = false
			switch {
			case err != nil:
				s.err = err.Error()
			case !out.Redemption.Redeemed:
				s.err = m("billing_redeem_failed")
				if out.Redemption.Message != nil {
					s.err = *out.Redemption.Message
				}
			default:
				s.ok, s.code = m("billing_redeem_success"), ""
				a.reload("credits")
				delete(a.pages.json, "credits")
			}
		})
	})
}

// cardGrid lays cells out cols to a row, each row as tall as its tallest
// cell.
func cardGrid(c *ui.Context, cols int, gap float32, count int, cell func(i int)) {
	ui.Column(c.Key("cards")).Gap(gap).Children(func() {
		for r := 0; r*cols < count; r++ {
			ui.Row(c.Key(r)).Gap(gap).AlignItems(ui.Stretch).Children(func() {
				for i := r * cols; i < r*cols+cols; i++ {
					ui.Column(c.Key(i)).Grow(1).Basis(0).MinWidth(0).Children(func() {
						if i < count {
							cell(i)
						}
					})
				}
			})
		}
	})
}
