package app

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"cohub-mygo/internal/cohub"
	"cohub-mygo/internal/cron"
	"cohub-mygo/internal/i18n"
)

// Scheduled prompts, as CronjobView.svelte: a new prompt's form, a prompt
// with its pause, edit and delete, and the runs it made.

type cronRecord struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	TaskType       string         `json:"taskType"`
	Payload        map[string]any `json:"payload"`
	CronExpression string         `json:"cronExpression"`
	Timezone       string         `json:"timezone"`
	SpaceID        string         `json:"spaceId"`
	SessionID      *string        `json:"sessionId"`
	Enabled        bool           `json:"enabled"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	UserProfile    *cohub.Profile `json:"userProfile"`
}

// cronForm is the new or edited prompt.
type cronForm struct {
	id, title, prompt, expr, tz string
	model                       cohub.ModelChoice
	saving                      bool
	err                         string
	editing                     bool
}

func localZone() string {
	if z := time.Local.String(); z != "" && z != "Local" {
		return z
	}
	if tz := strings.TrimSpace(readTZ()); tz != "" {
		return tz
	}
	return "UTC"
}

func cronPrompt(payload map[string]any) (string, bool) {
	switch c := payload["content"].(type) {
	case string:
		return c, false
	case []any:
		var parts []string
		for _, b := range c {
			if m, ok := b.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					parts = append(parts, t)
				}
			}
		}
		return strings.Join(parts, "\n\n"), len(c) != 1
	}
	return "", false
}

func payloadString(payload map[string]any, key string) string {
	s, _ := payload[key].(string)
	return s
}

func (a *app) cronView(c *ui.Context, spaceID, id string) {
	p := a.pal
	if id == "new" {
		a.cronEditor(c, spaceID, nil)
		return
	}
	out, err, ok := loadJSON[struct {
		Job *cronRecord `json:"job"`
	}](a, "cron:"+id, "/api/cron-jobs/"+url.PathEscape(id))
	if !ok || err != nil || out.Job == nil {
		if ok && err == nil {
			err = fmt.Errorf("%s", m("cron_not_found"))
		}
		a.loadingBlock(c, err, 200)
		return
	}
	j := out.Job
	if f := a.cron; f != nil && f.id == j.ID && f.editing {
		a.cronEditor(c, spaceID, j)
		return
	}
	ui.Scroll(c.Key("cron-" + id)).Grow(1).Children(func() {
		ui.Column(c).Padding(20).Gap(20).Children(func() {
			ui.Column(c.Key("head")).Gap(12).Padding(0, 0, 18, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle.Alpha(0.7)).Children(func() {
				ui.Text(c, j.Title).FontSize(24).FontWeight(600).LetterSpacing(-0.3)
				ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						col, label := p.placeholder, m("cron_paused")
						if j.Enabled {
							col, label = p.success, m("cron_active")
						}
						a.dot(c, col, 6)
						ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(p.brand)
					})
					if pr := j.UserProfile; pr != nil {
						ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
							a.avatar(c, pr.AvatarURL, pr.Name(), 14, 7)
							ui.Text(c, pr.Name()).FontSize(11).TextColor(p.textTertiary)
						})
					}
					idb := ui.Row(c.Key("copy-id")).Gap(4).AlignItems(ui.Center).Cursor(ui.CursorPointer).Children(func() {
						ui.Text(c, j.ID).FontSize(11).Font(fontMono).TextColor(p.placeholder)
						ic := "copy"
						if time.Since(a.copiedAt) < 2*time.Second {
							ic = "check"
						}
						ui.Icon(c, icon(ic)).Size(12, 12).TextColor(p.placeholder)
					})
					if idb.Clicked() {
						c.WriteClipboard(j.ID)
						a.copiedAt = time.Now()
					}
				})
				ui.Row(c).Gap(8).Children(func() {
					act := func(key, ic, label string, col ui.Color) bool {
						b := ui.ButtonBase(c.Key(key)).Row().Gap(6).AlignItems(ui.Center).Height(32).Padding(0, 12).Radius(5).Background(p.bgElevated).FocusRing(false).Children(func() {
							ui.Icon(c, icon(ic)).Size(14, 14).TextColor(col)
							ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(col)
						})
						if b.Hovered() {
							b.Background(p.hover)
						}
						return b.Clicked()
					}
					if j.TaskType == "send_message" && act("edit", "pencil", m("common_edit"), p.textSecond) {
						prompt, _ := cronPrompt(j.Payload)
						a.cron = &cronForm{id: j.ID, title: j.Title, prompt: prompt, expr: j.CronExpression, tz: j.Timezone, editing: true,
							model: cohub.ModelChoice{Provider: payloadString(j.Payload, "provider"), ID: payloadString(j.Payload, "model"), Level: payloadString(j.Payload, "thinkingLevel")}}
					}
					label, ic, col := m("cron_resume"), "power-off", p.textSecond
					if j.Enabled {
						label, ic, col = m("cron_pause"), "power", p.success
					}
					if act("toggle", ic, label, col) {
						next := !j.Enabled
						j.Enabled = next
						a.act(func(ctx context.Context) error {
							return a.api.Do(ctx, "PATCH", "/api/cron-jobs/"+url.PathEscape(j.ID), map[string]any{"enabled": next}, nil)
						}, "cron:"+j.ID, "crons:"+spaceID)
					}
					if act("delete", "trash-2", m("cron_delete"), p.textTertiary) {
						a.confirm(m("resource_app_delete_schedule_title"), m("resource_app_delete_schedule_message", "title", j.Title), m("cron_delete"), func() {
							a.act(func(ctx context.Context) error {
								err := a.api.Do(ctx, "DELETE", "/api/cron-jobs/"+url.PathEscape(j.ID), nil, nil)
								if err == nil {
									a.update(func() { a.closeWindow() })
								}
								return err
							}, "crons:"+spaceID)
						})
					}
				})
			})
			if j.TaskType == "send_message" {
				prompt, _ := cronPrompt(j.Payload)
				ui.Column(c.Key("prompt")).Gap(8).Children(func() {
					ui.Text(c, strings.ToUpper(m("cron_prompt"))).FontSize(10).FontWeight(500).LetterSpacing(1.6).TextColor(p.placeholder)
					ui.Text(c, m("cron_prompt_chars", "count", len([]rune(prompt)))).FontSize(12).TextColor(p.textTertiary)
					ui.Row(c).Radius(8).Background(p.bgElevated.Alpha(0.4)).Border(1, p.borderSubtle.Alpha(0.6)).Clip().Children(func() {
						ui.Box(c).Width(3).Background(p.brand)
						ui.Text(c, prompt).FontSize(13).LineHeight(1.85).TextColor(p.textSecond).Padding(14, 18).Grow(1).Selectable()
					})
				})
				ui.Row(c.Key("model")).Gap(12).Children(func() {
					card := func(k, v string, mono bool) {
						ui.Column(c.Key(k)).Grow(1).Basis(0).Gap(4).Padding(10, 12).Radius(7).Background(p.bgElevated.Alpha(0.3)).Children(func() {
							ui.Text(c, strings.ToUpper(k)).FontSize(10).FontWeight(500).LetterSpacing(0.6).TextColor(p.placeholder)
							t := ui.Text(c, v).FontSize(13).SingleLine()
							if mono {
								t.Font(fontMono).FontSize(12).TextColor(p.textSecond)
							}
						})
					}
					model := payloadString(j.Payload, "model")
					if model == "" {
						model = m("cron_use_default_model")
					}
					card(m("cron_model"), model, false)
					prov := payloadString(j.Payload, "provider")
					if prov == "" {
						prov = "—"
					}
					card(m("cron_provider"), prov, true)
				})
			}
			ui.Column(c.Key("schedule")).Gap(10).Children(func() {
				ui.Text(c, strings.ToUpper(m("cron_schedule"))).FontSize(10).FontWeight(500).LetterSpacing(1.6).TextColor(p.placeholder)
				item := func(ic, k string, body func()) {
					ui.Column(c.Key(k)).Gap(6).Children(func() {
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							if ic != "" {
								ui.Icon(c, icon(ic)).Size(14, 14).TextColor(p.placeholder)
							}
							ui.Text(c, strings.ToUpper(k)).FontSize(11).FontWeight(500).LetterSpacing(0.6).TextColor(p.placeholder)
						})
						body()
					})
				}
				item("clock", m("cron_expression"), func() { ui.Text(c, j.CronExpression).FontSize(15).Font(fontMono) })
				item("clock-3", m("cron_timezone"), func() { ui.Text(c, j.Timezone).FontSize(13) })
				if runs, err := cron.NextRuns(j.CronExpression, j.Timezone, 3); j.Enabled && err == nil && len(runs) > 0 {
					item("", m("resource_app_next_runs"), func() {
						for _, r := range runs {
							ui.Text(c.Key(r.String()), shortDateTime(r)).FontSize(12).Font(fontMono).TextColor(p.textSecond)
						}
					})
				}
			})
			ui.Box(c.Key("rule")).Height(1).Background(p.borderSubtle.Alpha(0.7))
			ui.Column(c.Key("meta")).Gap(8).Children(func() {
				ui.Text(c, strings.ToUpper(m("cron_metadata"))).FontSize(10).FontWeight(500).LetterSpacing(1.6).TextColor(p.placeholder)
				kv := func(k, v string, mono bool) {
					ui.Row(c.Key(k)).Gap(12).Children(func() {
						ui.Text(c, k).FontSize(12).TextColor(p.placeholder).Width(76)
						t := ui.Text(c, v).FontSize(12).TextColor(p.textSecond).Grow(1)
						if mono {
							t.Font(fontMono)
						}
					})
				}
				kv(m("cron_type"), j.TaskType, true)
				sess := m("cron_new_session_on_run")
				if j.SessionID != nil {
					sess = (*j.SessionID)[:min(8, len(*j.SessionID))]
				}
				kv(m("cron_session"), sess, j.SessionID != nil)
				kv(m("cron_created"), i18n.LongDate(j.CreatedAt), false)
				kv(m("cron_updated"), i18n.LongDate(j.UpdatedAt), false)
			})
			a.cronRuns(c, spaceID, j.ID)
		})
	})
}

func (a *app) cronRuns(c *ui.Context, spaceID, id string) {
	p := a.pal
	runs, err, _ := loadJSON[struct {
		Runs []*taskRun `json:"runs"`
	}](a, "cron-runs:"+id, "/api/cron-jobs/"+url.PathEscape(id)+"/runs?limit=20")
	ui.Column(c.Key("runs")).Gap(8).Children(func() {
		ui.Text(c, strings.ToUpper(m("cron_runs"))).FontSize(10).FontWeight(500).LetterSpacing(1.6).TextColor(p.placeholder)
		switch {
		case runs == nil && err == nil:
			a.spinner(c, 14, p.textTertiary)
		case err != nil:
			ui.Text(c, err.Error()).FontSize(12).TextColor(p.errorC)
		case len(runs.Runs) == 0:
			ui.Text(c, m("cron_empty_runs")).FontSize(12).TextColor(p.textTertiary)
		default:
			ui.Text(c, m("cron_runs_loaded", "count", len(runs.Runs))).FontSize(11).TextColor(p.placeholder)
			ui.Column(c).Radius(6).Border(1, p.borderSubtle).Children(func() {
				for i, r := range runs.Runs {
					row := ui.Row(c.Key(r.ID)).Gap(10).AlignItems(ui.Center).Padding(8, 10).Cursor(ui.CursorPointer)
					if i > 0 {
						row.BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle)
					}
					row.Children(func() {
						col := p.placeholder
						switch r.Status {
						case "completed":
							col = p.success
						case "failed":
							col = p.errorC
						case "running":
							col = p.brand
						}
						a.dot(c, col, 6)
						ui.Text(c, r.Status).FontSize(12).TextColor(p.textSecond).Width(80)
						ui.Text(c, r.CreatedAt.Local().Format("2006-01-02 15:04")).FontSize(12).Font(fontMono).TextColor(p.textTertiary).Grow(1)
						if r.StartedAt != nil && r.FinishedAt != nil {
							ui.Text(c, duration(r.FinishedAt.Sub(*r.StartedAt).Milliseconds())).FontSize(11).TextColor(p.placeholder)
						}
					})
					if row.Hovered() {
						row.Background(p.hover)
					}
					rid := r.ID
					if row.Clicked() {
						a.go_(route{kind: routeSpaceTask, space: spaceID, sub: rid})
					}
				}
			})
		}
	})
}

// cronEditor is the new prompt's form, or the prompt's in edit mode.
func (a *app) cronEditor(c *ui.Context, spaceID string, j *cronRecord) {
	p := a.pal
	if a.cron == nil || (j == nil && a.cron.id != "") {
		a.cron = &cronForm{tz: localZone(), expr: "0 9 * * *"}
	}
	f := a.cron
	name, _ := a.spaceName(spaceID)
	a.cronBack(c)
	ui.Scroll(c.Key("cron-form")).Grow(1).Children(func() {
		ui.Column(c).Padding(20).Gap(20).Children(func() {
			if j == nil {
				ui.Column(c.Key("head")).Gap(8).Padding(0, 0, 18, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle.Alpha(0.7)).Children(func() {
					ui.Text(c, strings.ToUpper(m("cron_scheduled_prompt"))).FontSize(10).FontWeight(500).LetterSpacing(1.8).TextColor(p.placeholder)
					ui.Text(c, m("cron_new_scheduled")).FontSize(24).FontWeight(600)
					ui.Text(c, m("resource_app_empty_schedules_hint")).FontSize(13).TextColor(p.textTertiary)
					_ = name
				})
			}
			label := func(t string) {
				ui.Text(c, strings.ToUpper(t)).FontSize(10).FontWeight(500).LetterSpacing(0.6).TextColor(p.textTertiary)
			}
			ui.Column(c.Key("title")).Gap(6).Children(func() {
				label(m("cron_title"))
				a.field(c, "title", &f.title, m("cron_placeholder_name"))
			})
			ui.Column(c.Key("prompt")).Gap(6).Children(func() {
				label(m("cron_prompt"))
				in := ui.TextAreaBase(c.Key("prompt-text"), &f.prompt).Lines(6, 14).Placeholder(m("cron_placeholder_content")).PlaceholderColor(p.placeholder).
					FontSize(13).LineHeight(1.7).Padding(8, 12).Radius(6).Border(1, p.borderSubtle).Background(p.bgInput).Label(m("cron_prompt"))
				if in.Focused() {
					in.Border(1, p.brand.Alpha(0.5))
				}
			})
			ui.Row(c.Key("when")).Gap(12).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Gap(6).Children(func() {
					label(m("cron_schedule"))
					a.field(c, "expr", &f.expr, "0 9 * * *").Font(fontMono)
					ui.Text(c, m("cron_schedule_hint")).FontSize(11).TextColor(p.placeholder)
					ui.Row(c.Key("presets")).Gap(4).Wrap().Children(func() {
						for _, pr := range cronPresets {
							on := strings.TrimSpace(f.expr) == pr.expr
							expr := pr.expr
							b := ui.ButtonBase(c.Key(pr.key)).Height(24).Padding(0, 8).Radius(5).Center().FocusRing(false).Children(func() {
								col := p.textTertiary
								if on {
									col = p.brandMutedFg
								}
								ui.Text(c, m(pr.key)).FontSize(11).TextColor(col)
							})
							if on {
								b.Background(p.brandMuted)
							} else if b.Hovered() {
								b.Background(p.hover)
							}
							if b.Clicked() {
								f.expr = expr
							}
						}
					})
					if strings.TrimSpace(f.expr) != "" {
						runs, err := cron.NextRuns(f.expr, f.tz, 3)
						switch {
						case err != nil:
							ui.Text(c.Key("bad"), m("resource_app_cron_invalid")).FontSize(11).TextColor(p.errorC)
						case len(runs) > 0:
							ui.Text(c.Key("next"), strings.ToUpper(m("resource_app_next_runs"))).FontSize(10).FontWeight(500).LetterSpacing(0.6).TextColor(p.placeholder).Margin(6, 0, 0, 0)
							for _, r := range runs {
								ui.Text(c.Key(r.String()), shortDateTime(r)).FontSize(12).Font(fontMono).TextColor(p.textSecond)
							}
						}
					}
				})
				ui.Column(c).Grow(1).Basis(0).Gap(6).Children(func() {
					label(m("cron_timezone"))
					a.field(c, "tz", &f.tz, "UTC")
				})
			})
			ui.Column(c.Key("model")).Gap(6).Children(func() {
				label(m("cron_model"))
				cur := m("cron_use_default_model")
				if f.model.ID != "" {
					cur = f.model.ID
					if md := a.model(f.model.ID); md != nil && md.Model.Name != "" {
						cur = md.Model.Name
					}
				}
				mb := ui.ButtonBase(c.Key("pick")).Row().Gap(8).AlignItems(ui.Center).Height(40).Padding(0, 12).Radius(6).Border(1, p.borderSubtle).Background(p.bgElevated.Alpha(0.35)).FocusRing(false).Children(func() {
					ui.Text(c, cur).FontSize(13).Grow(1).SingleLine()
					ui.Icon(c, icon("chevron-down")).Size(14, 14).TextColor(p.placeholder)
				})
				if mb.Clicked() {
					a.menus["cron-model"] = !a.menus["cron-model"]
				}
				open := a.menus["cron-model"]
				items := []menuItem{{label: m("cron_use_default_model"), checked: f.model.ID == "", run: func() { f.model = cohub.ModelChoice{} }}}
				for _, md := range a.models {
					md := md
					name := md.Model.Name
					if name == "" {
						name = md.ID
					}
					items = append(items, menuItem{label: name, checked: f.model.ID == md.ID, run: func() {
						f.model = cohub.ModelChoice{Provider: md.Provider, ID: md.ID, Level: md.Model.DefaultLevel}
					}})
				}
				a.menuWidth(c, mb, &open, 280, items)
				a.menus["cron-model"] = open
			})
			if f.err != "" {
				ui.Text(c.Key("err"), f.err).FontSize(12).Font(fontMono).TextColor(p.errorC).Padding(10).Radius(6).Background(p.errorC.Alpha(0.08))
			}
			ui.Row(c.Key("foot")).Gap(8).Padding(14, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle.Alpha(0.7)).Children(func() {
				ui.Spacer(c)
				if a.button(c, "", m("cron_cancel"), false).Clicked() {
					if j != nil {
						f.editing = false
					} else {
						a.cron = nil
						a.closeWindow()
					}
				}
				label, ic := m("cron_create"), "plus"
				if j != nil {
					label, ic = m("cron_save_changes"), "check"
				}
				sb := ui.ButtonBase(c.Key("submit")).Row().Gap(6).AlignItems(ui.Center).Height(36).Padding(0, 12).Radius(5).Background(p.brand).FocusRing(false).Children(func() {
					if f.saving {
						a.spinner(c, 14, p.brandFg)
					} else {
						ui.Icon(c, icon(ic)).Size(14, 14).TextColor(p.brandFg)
					}
					ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(p.brandFg)
				})
				if sb.Clicked() && !f.saving {
					a.submitCron(spaceID, f, j)
				}
			})
		})
	})
}

func (a *app) submitCron(spaceID string, f *cronForm, j *cronRecord) {
	switch {
	case strings.TrimSpace(f.title) == "":
		f.err = m("cron_title") + "?"
		return
	case len(strings.Fields(f.expr)) != 5:
		f.err = m("cron_schedule_hint")
		return
	case strings.TrimSpace(f.prompt) == "":
		f.err = m("cron_prompt") + "?"
		return
	}
	f.saving, f.err = true, ""
	title, prompt, expr, tz, model := strings.TrimSpace(f.title), strings.TrimSpace(f.prompt), strings.TrimSpace(f.expr), strings.TrimSpace(f.tz), f.model
	a.act(func(ctx context.Context) error {
		var err error
		var newID string
		if j != nil {
			payload := map[string]any{}
			for k, v := range j.Payload {
				payload[k] = v
			}
			payload["content"] = []map[string]any{{"type": "text", "text": prompt}}
			delete(payload, "provider")
			delete(payload, "model")
			delete(payload, "thinkingLevel")
			if model.ID != "" {
				payload["provider"], payload["model"] = model.Provider, model.ID
				if model.Level != "" {
					payload["thinkingLevel"] = model.Level
				}
			}
			err = a.api.Do(ctx, "PATCH", "/api/cron-jobs/"+url.PathEscape(j.ID), map[string]any{
				"title": title, "cronExpression": expr, "timezone": tz, "payload": payload,
			}, nil)
		} else {
			in := map[string]any{
				"title": title, "content": []map[string]any{{"type": "text", "text": prompt}},
				"schedule": map[string]any{"mode": "repeat", "cronExpression": expr, "timezone": tz},
			}
			if model.ID != "" {
				in["provider"], in["model"] = model.Provider, model.ID
				if model.Level != "" {
					in["thinkingLevel"] = model.Level
				}
			}
			var out struct {
				Mode      string `json:"mode"`
				CronJobID string `json:"cronJobId"`
			}
			err = a.api.Do(ctx, "POST", sp(spaceID)+"/prompt", in, &out)
			newID = out.CronJobID
		}
		a.update(func() {
			f.saving = false
			if err != nil {
				f.err = err.Error()
				return
			}
			f.editing = false
			if j == nil {
				a.cron = nil
				if newID != "" {
					a.go_(route{kind: routeSpaceCron, space: spaceID, sub: newID})
				}
			}
		})
		return nil
	}, "crons:"+spaceID, func() string {
		if j != nil {
			return "cron:" + j.ID
		}
		return ""
	}())
}

// confirm asks before something that cannot be undone.
func (a *app) confirm(title, detail, action string, yes func()) {
	a.confirmBox = &confirmBox{title: title, detail: detail, action: action, yes: yes}
}

type confirmBox struct {
	title, detail, action string
	yes                   func()
}

// confirmView is AlertDialog: the question, Cancel and the action.
func (a *app) confirmView(c *ui.Context) {
	b := a.confirmBox
	if b == nil {
		return
	}
	p := a.pal
	open := true
	a.dialog(c, &open, "", 400, func() {
		ui.Column(c.Key("confirm")).Padding(18).Gap(8).Children(func() {
			ui.Text(c, b.title).FontSize(15).FontWeight(600)
			ui.Text(c, b.detail).FontSize(13).LineHeight(1.5).TextColor(p.textSecond)
			ui.Row(c).Gap(8).Margin(12, 0, 0, 0).Children(func() {
				ui.Spacer(c)
				if a.button(c, "", m("common_cancel"), false).Clicked() {
					a.confirmBox = nil
				}
				d := ui.ButtonBase(c.Key("yes")).Height(32).Padding(0, 12).Radius(5).Center().Background(p.errorC).FocusRing(false).Children(func() {
					ui.Text(c, b.action).FontSize(12).FontWeight(500).TextColor(ui.RGBA(255, 255, 255, 1))
				})
				if d.Clicked() {
					a.confirmBox = nil
					b.yes()
				}
			})
		})
	})
	if !open {
		a.confirmBox = nil
	}
}

// readTZ is the system's zone name where Go's Local has none.
func readTZ() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}
	if b, err := os.ReadFile("/etc/timezone"); err == nil {
		return string(b)
	}
	if l, err := os.Readlink("/etc/localtime"); err == nil {
		if i := strings.Index(l, "zoneinfo/"); i >= 0 {
			return l[i+len("zoneinfo/"):]
		}
	}
	return ""
}

// cronBack is the window's way back to the list of scheduled prompts.
func (a *app) cronBack(c *ui.Context) {
	p := a.pal
	b := ui.Row(c.Key("cron-back")).Height(36).Shrink(0).Gap(4).AlignItems(ui.Center).Padding(0, 12).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Cursor(ui.CursorPointer).Children(func() {
		ui.Icon(c, icon("chevron-left")).Size(14, 14).TextColor(p.textSecond)
		ui.Text(c, m("x_schedules")).FontSize(12).TextColor(p.textSecond)
	})
	if b.Clicked() {
		a.panelOpen, a.panelTab = true, 4
		a.closeWindow()
	}
}

// shortDateTime is formatShortDateTime: 11月10日 04:00, Nov 10, 04:00.
func shortDateTime(t time.Time) string {
	if i18n.Locale() == "zh-CN" {
		return t.Format("1月2日 15:04")
	}
	return t.Format("Jan 2, 15:04")
}

var cronPresets = []struct{ key, expr string }{
	{"resource_app_preset_hourly", "0 * * * *"},
	{"resource_app_preset_daily", "0 9 * * *"},
	{"resource_app_preset_weekdays", "0 9 * * 1-5"},
	{"resource_app_preset_weekly", "0 9 * * 1"},
}
