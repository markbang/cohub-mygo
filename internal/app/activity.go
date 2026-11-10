package app

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"cohub-mygo/internal/cohub"
)

// Activity is GET /api/me/activity or /api/spaces/:id/activity.
type Activity struct {
	Hourly []struct {
		BucketStartAt time.Time `json:"bucketStartAt"`
		TotalTokens   int64     `json:"totalTokens"`
		RequestCount  int64     `json:"requestCount"`
	} `json:"hourly"`
	Generation struct {
		Hourly []struct {
			BucketStartAt time.Time `json:"bucketStartAt"`
			RequestCount  int64     `json:"requestCount"`
		} `json:"hourly"`
	} `json:"generation"`
	Totals struct {
		TotalTokens  int64   `json:"totalTokens"`
		RequestCount int64   `json:"requestCount"`
		CostTotal    float64 `json:"costTotal"`
	} `json:"totals"`
	Range *struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
	} `json:"range"`
	Rankings struct {
		LLMModels []struct {
			Provider    string  `json:"provider"`
			Model       string  `json:"model"`
			TotalTokens int64   `json:"totalTokens"`
			CostTotal   float64 `json:"costTotal"`
		} `json:"llmModels"`
		GenerationModels []struct {
			Provider     string  `json:"provider"`
			Model        string  `json:"model"`
			RequestCount int64   `json:"requestCount"`
			CostTotal    float64 `json:"costTotal"`
		} `json:"generationModels"`
		Apps []struct {
			AppID     string `json:"appId"`
			SpaceID   string `json:"spaceId"`
			Title     string `json:"title"`
			SpaceName string `json:"spaceName"`
			ViewCount int64  `json:"viewCount"`
		} `json:"apps"`
	} `json:"rankings"`
	Contributors *struct {
		Items []struct {
			UserUUID  string         `json:"userUuid"`
			Role      string         `json:"role"`
			Tokens    int64          `json:"tokens"`
			CostTotal float64        `json:"costTotal"`
			Profile   *cohub.Profile `json:"profile"`
		} `json:"items"`
		MemberCount int `json:"memberCount"`
	} `json:"contributors"`
}

type activityDay struct {
	date        time.Time
	tokens      int64
	requests    int64
	generations int64
}

// days buckets the hourly rows into local days, the last n of them.
func (act *Activity) days(n int, now time.Time) []activityDay {
	key := func(t time.Time) string { return t.Local().Format("2006-01-02") }
	m := map[string]*activityDay{}
	get := func(t time.Time) *activityDay {
		k := key(t)
		if m[k] == nil {
			m[k] = &activityDay{}
		}
		return m[k]
	}
	for _, r := range act.Hourly {
		d := get(r.BucketStartAt)
		d.tokens += r.TotalTokens
		d.requests += r.RequestCount
	}
	for _, r := range act.Generation.Hourly {
		d := get(r.BucketStartAt)
		d.requests += r.RequestCount
		d.generations += r.RequestCount
	}
	out := make([]activityDay, n)
	for i := range out {
		day := now.AddDate(0, 0, i-n+1)
		d := get(day)
		out[i] = *d
		out[i].date = time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, time.Local)
	}
	return out
}

// formatCost is the activity's money: US$28,782.83.
func formatCost(v float64) string {
	switch {
	case v == 0:
		return "US$0.00"
	case v < 0.01:
		return "<US$0.01"
	}
	s := strconv.FormatFloat(v, 'f', 2, 64)
	whole, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, ch := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	return "US$" + b.String() + "." + frac
}

// activityPage is ActivityPage.svelte, for the account (spaceID "") or a Space.
func (a *app) activityPage(c *ui.Context, spaceID string) {
	p := a.pal
	key := "activity:" + spaceID
	daysKey := "activity-days:" + spaceID
	days, ok := a.pages.json[daysKey].(int)
	if !ok {
		days = 30
		if spaceID == "" {
			days = 365
		}
	}
	load := fmt.Sprintf("%s:%d", key, days)
	a.once(load, func(ctx context.Context) {
		var act Activity
		path := "/api/me/activity?days=" + strconv.Itoa(days)
		if spaceID != "" {
			path = "/api/spaces/" + url.PathEscape(spaceID) + "/activity?days=" + strconv.Itoa(days)
		}
		err := a.api.Get(ctx, path, &act)
		a.update(func() {
			if err != nil {
				a.pages.json[load] = err
				return
			}
			a.pages.json[load] = &act
		})
	})
	a.settingsHeaderless(c)
	ui.Scroll(c.Key("activity")).Grow(1).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(1024).Padding(28, 32).Children(func() {
			ui.Row(c).AlignItems(ui.End).Gap(16).Padding(0, 0, 20, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, "Cohub").FontSize(11).FontWeight(600).TextColor(p.brand)
					title, sub := m("nav_activity"), m("nav_activity")
					if spaceID == "" {
						who := m("activity_your_account")
						if a.me != nil && a.me.Profile != nil && a.me.Profile.Username != "" {
							who = "@" + a.me.Profile.Username
						}
						sub = who + " · " + map[bool]string{true: m("activity_last_year"), false: m("activity_last_days", "days", days)}[days == 365]
					}
					ui.Text(c, title).FontSize(20).FontWeight(600).SingleLine()
					ui.Text(c, sub).FontSize(12).TextColor(p.textTertiary)
				})
				a.segmented(c, "range", []string{"7D", "30D", "1Y"}, []int{7, 30, 365}, days, func(v int) { a.pages.json[daysKey] = v })
			})
			switch v := a.pages.json[load].(type) {
			case *Activity:
				a.activityBody(c, v, days, spaceID)
			case error:
				ui.Column(c).Padding(64, 0).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, m("activity_unavailable")).FontSize(13).TextColor(p.textSecond)
					ui.Text(c, v.Error()).FontSize(12).TextColor(p.errorC)
				})
			default:
				ui.Column(c).Height(288).Center().Children(func() { a.spinner(c, 20, p.placeholder) })
			}
		})
	})
}

// segmented is the activity pages' range switch: small buttons on a tray.
func (a *app) segmented(c *ui.Context, key string, labels []string, values []int, value int, set func(int)) {
	p := a.pal
	ui.Row(c.Key(key)).Padding(2).Radius(6).Background(p.bgSurface).Shrink(0).Children(func() {
		for i, l := range labels {
			on := values[i] == value
			b := ui.ButtonBase(c.Key(l)).MinWidth(44).Height(28).Padding(0, 10).Radius(4).Center().FocusRing(false).Children(func() {
				col := p.placeholder
				if on {
					col = p.text
				}
				ui.Text(c, l).FontSize(11).FontWeight(500).TextColor(col)
			})
			if on {
				b.Background(p.active)
			}
			if b.Clicked() {
				set(values[i])
			}
		}
	})
}

func (a *app) activityBody(c *ui.Context, act *Activity, n int, spaceID string) {
	p := a.pal
	days := act.days(n, a.now())
	active := 0
	for _, d := range days {
		if d.requests > 0 {
			active++
		}
	}
	// The stat strip.
	ui.Row(c).Padding(24, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
		stats := []struct{ v, l string }{
			{compactCount(act.Totals.TotalTokens), "Token"},
			{compactCount(act.Totals.RequestCount), m("activity_requests")},
			{strconv.Itoa(active), m("activity_active_days")},
			{formatCost(act.Totals.CostTotal), m("activity_cost")},
		}
		if spaceID != "" {
			stats = stats[:3]
		}
		for i, s := range stats {
			col := ui.Column(c.Key(i)).Grow(1).Basis(0).Gap(4).Padding(0, 20)
			if i == 0 {
				col.Padding(0, 20, 0, 0)
			}
			if i < len(stats)-1 {
				col.BorderWidth(0, 1, 0, 0).BorderColor(p.borderSubtle)
			}
			col.Children(func() {
				ui.Text(c, s.v).FontSize(22).Font(fontMono).TextColor(p.text)
				ui.Text(c, s.l).FontSize(11).TextColor(p.placeholder)
			})
		}
	})
	// The heatmap.
	modeKey := "heat-mode:" + spaceID
	gen, _ := a.pages.json[modeKey].(bool)
	ui.Column(c).Padding(28, 0).Gap(16).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Text(c, m("activity_daily_activity")).FontSize(13).FontWeight(500).Grow(1)
			mode := 0
			if gen {
				mode = 1
			}
			ui.Row(c.Key("mode")).Padding(2).Radius(6).Background(p.bgSurface).Children(func() {
				for i, l := range []string{"LLM", m("activity_mode_generation")} {
					on := i == mode
					b := ui.ButtonBase(c.Key(l)).Height(22).Padding(0, 10).Radius(4).Center().FocusRing(false).Children(func() {
						col := p.placeholder
						if on {
							col = p.text
						}
						ui.Text(c, l).FontSize(10).FontWeight(500).TextColor(col)
					})
					if on {
						b.Background(p.active)
					}
					if b.Clicked() {
						a.pages.json[modeKey] = i == 1
					}
				}
			})
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, m("activity_less")).FontSize(10).TextColor(p.placeholder)
				for lv := 0; lv <= 4; lv++ {
					ui.Box(c.Key(lv)).Size(11, 11).Radius(2).Background(a.heatColor(lv, gen))
				}
				ui.Text(c, m("activity_more")).FontSize(10).TextColor(p.placeholder)
			})
		})
		value := func(d activityDay) int64 {
			if gen {
				return d.generations
			}
			return d.tokens
		}
		var maxV int64
		for _, d := range days {
			maxV = max(maxV, value(d))
		}
		lead := int(days[0].date.Weekday())
		cells := lead + len(days)
		weeks := (cells + 6) / 7
		ui.ScrollHorizontal(c.Key("heat")).Scrollbars(false).Children(func() {
			ui.Row(c).Gap(4).Children(func() {
				for w := 0; w < weeks; w++ {
					ui.Column(c.Key(w)).Gap(4).Children(func() {
						for d := 0; d < 7; d++ {
							i := w*7 + d - lead
							if i < 0 || i >= len(days) {
								ui.Box(c.Key(d)).Size(13, 13)
								continue
							}
							lv := 0
							if v := value(days[i]); v > 0 && maxV > 0 {
								r := float64(v) / float64(maxV)
								switch {
								case r < 0.08:
									lv = 1
								case r < 0.25:
									lv = 2
								case r < 0.55:
									lv = 3
								default:
									lv = 4
								}
							}
							ui.Box(c.Key(d)).Size(13, 13).Radius(2).Background(a.heatColor(lv, gen)).
								Tooltip(fmt.Sprintf("%s · %s", days[i].date.Format("2006-01-02"), compactCount(value(days[i]))))
						}
					})
				}
			})
		})
	})
	if act.Contributors != nil {
		ui.Column(c).Padding(28, 0).Gap(16).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
			ui.Row(c).Children(func() {
				ui.Text(c, m("activity_contributors")).FontSize(13).FontWeight(500).Grow(1)
				ui.Text(c, m("activity_member_count", "count", act.Contributors.MemberCount)).FontSize(10).TextColor(p.placeholder)
			})
			ui.Row(c).Gap(8).Wrap().Children(func() {
				for _, it := range act.Contributors.Items {
					ui.Row(c.Key(it.UserUUID)).Gap(8).AlignItems(ui.Center).Padding(4, 12, 4, 4).Radius(999).Border(1, p.borderSubtle).Background(p.bgSurface).Children(func() {
						name := it.Profile.Name()
						av := ""
						if it.Profile != nil {
							av = it.Profile.AvatarURL
						}
						a.avatar(c, av, name, 20, 10)
						ui.Text(c, name).FontSize(12).TextColor(p.textSecond)
						if it.Role != "" {
							ui.Text(c, it.Role).FontSize(10).TextColor(p.textTertiary).Padding(1, 4).Radius(2).Background(p.hover)
						}
						ui.Text(c, compactCount(it.Tokens)).FontSize(11).Font(fontMono).TextColor(p.placeholder)
					})
				}
			})
		})
	}
	// The rankings.
	ui.Row(c).Gap(40).Padding(28, 0).AlignItems(ui.Start).Children(func() {
		rank := func(title, unit string, rows func(), empty bool, emptyText string) {
			ui.Column(c.Key(title)).Grow(1).Basis(0).Gap(12).MinWidth(0).Children(func() {
				ui.Row(c).Margin(0, 0, 4, 0).Children(func() {
					ui.Text(c, title).FontSize(13).FontWeight(500).Grow(1)
					ui.Text(c, unit).FontSize(10).TextColor(p.placeholder)
				})
				if empty {
					ui.Text(c, emptyText).FontSize(12).TextColor(p.placeholder)
					return
				}
				rows()
			})
		}
		row := func(i int, title, sub, value string) {
			ui.Row(c.Key(i)).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, strconv.Itoa(i+1)).FontSize(10).Font(fontMono).TextColor(p.placeholder).Width(18)
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, title).FontSize(12).TextColor(p.textSecond).SingleLine()
					if sub != "" {
						ui.Text(c, sub).FontSize(10).TextColor(p.placeholder).SingleLine()
					}
				})
				ui.Text(c, value).FontSize(12).Font(fontMono).TextColor(p.textSecond).Shrink(0)
			})
		}
		r := act.Rankings
		showCost := spaceID == ""
		unit := map[bool]string{true: m("activity_tokens_cost"), false: "Token"}[showCost]
		rank(m("activity_llm_models"), unit, func() {
			for i, m := range r.LLMModels {
				v := compactCount(m.TotalTokens)
				if showCost {
					v += " · " + formatCost(m.CostTotal)
				}
				row(i, m.Model, m.Provider, v)
			}
		}, len(r.LLMModels) == 0, m("activity_no_llm_usage"))
		rank(m("activity_generation_models"), map[bool]string{true: m("activity_calls_cost"), false: m("activity_calls")}[showCost], func() {
			for i, m := range r.GenerationModels {
				v := compactCount(m.RequestCount)
				if showCost {
					v += " · " + formatCost(m.CostTotal)
				}
				row(i, m.Model, "", v)
			}
		}, len(r.GenerationModels) == 0, m("activity_no_generation_usage"))
		apps := r.Apps
		sort.SliceStable(apps, func(i, j int) bool { return apps[i].ViewCount > apps[j].ViewCount })
		rank(m("side_panel_tab_apps"), m("activity_views"), func() {
			for i, m := range apps {
				row(i, m.Title, m.SpaceName, compactCount(m.ViewCount))
			}
		}, len(apps) == 0, m("activity_no_app_views"))
	})
}

// heatColor is a level of the heatmap: the brand (or the running green
// for generations) mixed into the page.
func (a *app) heatColor(level int, gen bool) ui.Color {
	p := a.pal
	base := p.brand
	if gen {
		base = p.success
	}
	switch level {
	case 0:
		return p.hover
	case 4:
		return base
	}
	return p.bgPrimary.Mix(base, []float32{0, 0.24, 0.42, 0.66}[level])
}
