package main

import (
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// The side panel's media: a Space's (or a chat's) generation tasks as a
// grid of tiles, as side-panel/{MediaPanel, GenerationGrid, GenerationTile}
// and the SDK's generation-task.ts.

type genRun struct {
	ID           string    `json:"id"`
	Status       string    `json:"status"`
	ErrorMessage *string   `json:"errorMessage"`
	CreatedAt    time.Time `json:"createdAt"`
	StartedAt    *time.Time
	Payload      struct {
		Data struct {
			Model   string `json:"model"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			Prompt string `json:"prompt"`
		} `json:"data"`
	} `json:"payload"`
	Result *struct {
		Model  string     `json:"model"`
		Output []genBlock `json:"output"`
	} `json:"result"`
}

type genBlock struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	Source *struct {
		URL  string `json:"url"`
		Type string `json:"type"`
	} `json:"source"`
	PreviewURL string `json:"previewUrl"`
	DurationMs *int64 `json:"durationMs"`
}

func (b genBlock) url() string {
	if b.URL != "" {
		return b.URL
	}
	if b.Source != nil {
		return b.Source.URL
	}
	return ""
}

// genOutput is a GenerationTaskOutput: an image, video or audio block.
type genOutput struct {
	kind, url, preview string
	durationMs         int64
}

func (r *genRun) outputs() []genOutput {
	if r.Status != "completed" || r.Result == nil {
		return nil
	}
	var out []genOutput
	for _, b := range r.Result.Output {
		switch b.Type {
		case "image", "video", "audio":
			o := genOutput{kind: b.Type, url: b.url(), preview: b.PreviewURL}
			if b.DurationMs != nil {
				o.durationMs = *b.DurationMs
			}
			out = append(out, o)
		}
	}
	return out
}

func (r *genRun) prompt() string {
	if r.Payload.Data.Prompt != "" {
		return r.Payload.Data.Prompt
	}
	for _, c := range r.Payload.Data.Content {
		if c.Type == "text" && c.Text != "" {
			return c.Text
		}
	}
	return ""
}

func (r *genRun) model() string {
	if r.Payload.Data.Model != "" {
		return r.Payload.Data.Model
	}
	if r.Result != nil {
		return r.Result.Model
	}
	return ""
}

type genFeed struct {
	Runs     []*genRun `json:"runs"`
	PageInfo struct {
		HasMore    bool   `json:"hasMore"`
		NextCursor string `json:"nextCursor"`
	} `json:"pageInfo"`
}

// genTasks are the tasks worth a tile: completed ones without media (a
// refusal) have nothing to show.
func (f *genFeed) tasks() []*genRun {
	var out []*genRun
	for _, r := range f.Runs {
		if r.Status == "completed" && len(r.outputs()) == 0 {
			continue
		}
		out = append(out, r)
	}
	return out
}

// imageVariant is media.ts imageVariantUrl for the hosts that resize:
// the smallest fixed edge covering the tile, as WebP.
func imageVariant(raw string, px float32) string {
	edge := 1536
	for _, s := range []int{192, 384, 768, 1536} {
		if float32(s) >= px {
			edge = s
			break
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || strings.HasSuffix(strings.ToLower(u.Path), ".gif") {
		return raw
	}
	switch u.Host {
	case "router-files.neta.art", "oss.talesofai.com", "public.cohub.live", "public.cohub.run", "sessions.cohub.live", "works.cohub.live", "works.cohub.run":
		if strings.Contains(raw, "x-oss-process=") {
			return raw
		}
		sep := "?"
		if u.RawQuery != "" {
			sep = "&"
		}
		return raw + sep + fmt.Sprintf("x-oss-process=image/resize,m_mfit,w_%d,h_%d/quality,q_82/format,webp", edge, edge)
	case "cca.neta.art", "static.neta.art", "pages-r2.neta.art":
		if strings.HasPrefix(u.Path, "/cdn-cgi/") {
			return raw
		}
		return fmt.Sprintf("%s://%s/cdn-cgi/image/width=%d,height=%d,fit=scale-down,quality=82,format=webp,onerror=redirect%s", u.Scheme, u.Host, edge*2, edge*2, u.RequestURI())
	}
	return raw
}

// mediaPanel is MediaPanel.svelte for the Space, or for a chat when
// sessionID is set.
func (a *app) mediaPanel(c *ui.Context, spaceID, sessionID string, filters bool) {
	p := a.pal
	key := "gen:" + spaceID + ":" + sessionID
	path := "/api/tasks?taskType=generation&limit=30&spaceId=" + url.QueryEscape(spaceID)
	if sessionID != "" {
		path += "&sessionId=" + url.QueryEscape(sessionID)
	}
	feed, ferr, _ := loadJSON[genFeed](a, key, path)
	filter := a.mediaFilter
	if !filters {
		filter = "all"
	}
	if filters {
		ui.Row(c.Key("media-tabs")).Height(36).Shrink(0).Gap(2).AlignItems(ui.Center).Padding(0, 8).Children(func() {
			for _, f := range []struct{ id, label string }{{"all", m("media_filter_all")}, {"image", m("media_filter_image")}, {"video", m("media_filter_video")}, {"audio", m("media_filter_audio")}} {
				on := filter == f.id
				id := f.id
				b := ui.ButtonBase(c.Key(f.id)).Height(24).Padding(0, 8).Radius(5).Center().Label(f.label).FocusRing(false).Children(func() {
					col := p.textTertiary
					if on {
						col = p.text
					}
					ui.Text(c, f.label).FontSize(11).FontWeight(500).TextColor(col)
				})
				if on {
					b.Background(p.tabKeyBg)
				}
				if b.Clicked() {
					a.mediaFilter = id
				}
			}
		})
	}
	body := func() {
		switch {
		case feed == nil && ferr == nil:
			cardGrid(c, 3, 4, 6, func(i int) {
				ui.Box(c.Key(i)).AspectRatio(1).Radius(6).Background(p.bgElevated)
			})
		case feed == nil:
			ui.Text(c.Key("err"), m("generation_load_failed")).FontSize(12).TextColor(p.errorC).Margin(4, 4)
		default:
			tasks := feed.tasks()
			type item struct {
				run *genRun
				out *genOutput
			}
			var items []item
			for _, r := range tasks {
				outs := r.outputs()
				if r.Status != "completed" {
					if filter == "all" {
						items = append(items, item{run: r})
					}
					continue
				}
				for i := range outs {
					if filter == "all" || outs[i].kind == filter {
						items = append(items, item{r, &outs[i]})
					}
				}
			}
			if len(items) == 0 {
				msg := m("media_empty")
				if len(tasks) > 0 {
					msg = m("media_empty_filtered")
				}
				ui.Text(c.Key("empty"), msg).FontSize(12).TextColor(p.textTertiary).Margin(12, 4, 0, 4)
				break
			}
			cols := max(1, int((a.panelInner()+4)/(88+4)))
			tile := (a.panelInner() - 4*float32(cols-1)) / float32(cols)
			cardGrid(c, cols, 4, len(items), func(i int) {
				it := items[i]
				a.genTile(c, it.run, it.out, tile)
			})
		}
		if sessionID == "" {
			ui.Row(c.Key("browser")).Margin(12, 0, 0, 0).Gap(6).AlignItems(ui.Center).Padding(6).Radius(6).Children(func() {
				ui.Spacer(c)
				ui.Icon(c, icon("external-link")).Size(12, 12).TextColor(p.placeholder)
				ui.Text(c, m("media_open_task_browser")).FontSize(11).TextColor(p.placeholder)
				ui.Spacer(c)
			})
		}
	}
	if !filters {
		ui.Column(c.Key("media")).Children(body)
		return
	}
	ui.Scroll(c.Key("media-scroll")).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Padding(0, 8, 12, 8).Children(body)
	})
}

// panelInner is the width inside the panel's 8px gutters.
func (a *app) panelInner() float32 { return panelWidth - 1 - 16 }

// genTile is GenerationTile.svelte: the media, or the task's state.
func (a *app) genTile(c *ui.Context, r *genRun, o *genOutput, size float32) {
	p := a.pal
	bg := p.bgElevated
	if r.Status == "failed" {
		bg = p.errorC.Alpha(0.1).Over(p.bgElevated)
	}
	t := ui.ButtonBase(c.Key("tile")).AspectRatio(1).Radius(6).Background(bg).Clip().FocusRing(false).Label(r.prompt())
	t.Children(func() {
		switch {
		case r.Status == "pending" || r.Status == "running":
			ui.Column(c).Fill().Padding(8).Gap(2).Children(func() {
				ui.Text(c, r.prompt()).FontSize(11).TextColor(p.textTertiary).MaxLines(2)
				ui.Spacer(c)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					col := p.placeholder
					if r.Status == "running" {
						col = p.success
					}
					a.dot(c, col, 6)
					st := m("generation_status_queued")
					if r.Status == "running" {
						st = m("generation_status_running")
					}
					ui.Text(c, st).FontSize(10).TextColor(p.textSecond).SingleLine()
				})
				since := r.CreatedAt
				if r.StartedAt != nil {
					since = *r.StartedAt
				}
				ui.Text(c, elapsed(since, a.now())).FontSize(10).Font(fontMono).TextColor(p.placeholder)
			})
		case r.Status == "failed":
			ui.Column(c).Fill().Padding(8).Gap(4).Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icon("circle-alert")).Size(12, 12).TextColor(p.errorC)
					ui.Text(c, m("generation_status_failed")).FontSize(10).FontWeight(500).TextColor(p.errorC)
				})
				msg := r.prompt()
				if msg == "" && r.ErrorMessage != nil {
					msg = *r.ErrorMessage
				}
				ui.Text(c, msg).FontSize(11).TextColor(p.textTertiary).MaxLines(3)
			})
		case o != nil:
			src := o.preview
			if src == "" && o.kind == "image" {
				src = o.url
			}
			var bmp *ui.Bitmap
			if src != "" {
				bmp = a.images.get(imageVariant(src, size*2))
			}
			if bmp != nil {
				ui.Image(c, bmp).Fill().Fit(ui.Cover)
			} else {
				name := map[string]string{"audio": "audio-lines", "video": "film", "image": "image"}[o.kind]
				ui.Box(c).Fill().Center().Children(func() {
					ui.Icon(c, icon(name)).Size(20, 20).TextColor(p.placeholder)
				})
			}
			if o.kind == "video" || o.kind == "audio" {
				ui.Box(c.Key("badge")).Absolute().Top(4).Left(4).Size(20, 20).Radius(10).Center().Background(ui.RGBA(0, 0, 0, 0.55)).Children(func() {
					ui.Icon(c, icon("play")).Size(12, 12).TextColor(ui.RGBA(255, 255, 255, 1))
				})
				if o.durationMs > 0 {
					ui.Text(c.Key("dur"), duration(o.durationMs)).FontSize(10).Font(fontMono).TextColor(ui.RGBA(255, 255, 255, 1)).
						Padding(1, 4).Radius(4).Background(ui.RGBA(0, 0, 0, 0.55)).Absolute().Bottom(4).Right(4)
				}
			}
		}
	})
	if t.Hovered() && o != nil {
		t.Border(1, p.borderStrong)
	}
	if t.Clicked() {
		if o != nil && o.url != "" {
			c.OpenURL(o.url)
		} else {
			a.go_(route{kind: routeSpaceTask, space: a.currentSpace(), sub: r.ID})
		}
	}
}

type sessionFile struct {
	Path             string    `json:"path"`
	LastKind         string    `json:"lastKind"`
	LastChangedAt    time.Time `json:"lastChangedAt"`
	LastTurnSequence *int      `json:"lastTurnSequence"`
}

// sessionOutputs is SessionOutputsPanel.svelte: what the chat generated,
// then the files it changed, each a folding section.
func (a *app) sessionOutputs(c *ui.Context, spaceID, sessionID string) {
	p := a.pal
	ui.Scroll(c.Key("outputs")).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Padding(4, 6, 12, 6).Children(func() {
			if sessionID == "" {
				ui.Text(c, m("session_outputs_no_chat")).FontSize(12).LineHeight(1.5).TextColor(p.textTertiary).Margin(8, 6)
				return
			}
			gen, gerr, _ := loadJSON[genFeed](a, "gen:"+spaceID+":"+sessionID,
				"/api/tasks?taskType=generation&limit=30&spaceId="+url.QueryEscape(spaceID)+"&sessionId="+url.QueryEscape(sessionID))
			files, ferr, _ := loadJSON[struct {
				Files []sessionFile `json:"files"`
			}](a, "session-files:"+sessionID, "/api/sessions/"+url.PathEscape(sessionID)+"/files?limit=200")
			if a.api != nil && ((gen == nil && gerr == nil) || (files == nil && ferr == nil)) {
				cardGrid(c, 3, 4, 3, func(i int) { ui.Box(c.Key(i)).AspectRatio(1).Radius(6).Background(p.bgElevated) })
				return
			}
			var tasks []*genRun
			if gen != nil {
				tasks = gen.tasks()
			}
			var changed []sessionFile
			if files != nil {
				changed = files.Files
			}
			if len(tasks) == 0 && len(changed) == 0 {
				ui.Text(c, m("session_outputs_empty")).FontSize(12).LineHeight(1.5).TextColor(p.textTertiary).Margin(8, 6)
				return
			}
			if len(tasks) > 0 {
				count := fmt.Sprint(len(tasks))
				if gen.PageInfo.HasMore {
					count += "+"
				}
				a.panelSection(c, "media", m("session_outputs_media"), count, func() {
					ui.Column(c).Padding(0, 6, 8, 6).Children(func() {
						inner := a.panelInner() - 8
						cols := max(1, int((inner+4)/(88+4)))
						tile := (inner - 4*float32(cols-1)) / float32(cols)
						type item struct {
							run *genRun
							out *genOutput
						}
						var items []item
						for _, r := range tasks {
							if r.Status != "completed" {
								items = append(items, item{run: r})
								continue
							}
							outs := r.outputs()
							for i := range outs {
								items = append(items, item{r, &outs[i]})
							}
						}
						cardGrid(c, cols, 4, len(items), func(i int) { a.genTile(c, items[i].run, items[i].out, tile) })
					})
				})
			}
			if len(changed) > 0 {
				a.panelSection(c, "files", m("session_outputs_files"), fmt.Sprint(len(changed)), func() {
					for _, f := range changed {
						a.sessionFileRow(c, spaceID, f)
					}
				})
			}
		})
	})
}

// panelSection is SidePanelSection.svelte: a 32px folding title.
func (a *app) panelSection(c *ui.Context, key, title, count string, body func()) {
	a.panelSectionMeta(c, key, title, count, nil, body)
}

// panelSectionMeta is panelSection with actions at the title's end.
func (a *app) panelSectionMeta(c *ui.Context, key, title, count string, meta func(), body func()) {
	p := a.pal
	k := "panel-section:" + key
	collapsed := a.groupsOpen[k]
	ui.Column(c.Key(key)).Children(func() {
		head := ui.Row(c.Key("head")).Height(32).Gap(6).AlignItems(ui.Center).Padding(0, 6).Radius(5).Cursor(ui.CursorPointer).Children(func() {
			ch := ui.Icon(c, icon("chevron-down")).Size(12, 12).TextColor(p.textTertiary)
			if collapsed {
				ch.Rotate(-90)
			}
			ui.Text(c, title).FontSize(11).FontWeight(500).TextColor(p.textTertiary).SingleLine()
			if count != "" && count != "0" {
				ui.Text(c, count).FontSize(11).TextColor(p.placeholder)
			}
			if meta != nil {
				ui.Spacer(c)
				meta()
			}
		})
		if head.Hovered() {
			head.Background(p.hover)
		}
		if head.Clicked() {
			a.groupsOpen[k] = !collapsed
		}
		if !collapsed {
			body()
		}
	})
}

// sessionFileRow is a SessionFileList row: the name, its folder, and how
// the chat changed it when.
func (a *app) sessionFileRow(c *ui.Context, spaceID string, f sessionFile) {
	p := a.pal
	dir, name := "", f.Path
	if i := strings.LastIndex(f.Path, "/"); i >= 0 {
		dir, name = f.Path[:i], f.Path[i+1:]
	}
	kind := m("session_file_change_edit")
	if f.LastKind == "write" {
		kind = m("session_file_change_write")
	}
	meta := kind + " · " + timeAgo(f.LastChangedAt, a.now())
	metaW, _ := c.MeasureText(0, ui.Span{Text: meta, Size: 10})
	room := a.panelInner() + 4 - 12 - 14 - 8 - 8 - metaW
	row := ui.Row(c.Key(f.Path)).Gap(8).AlignItems(ui.Center).Padding(6).Radius(5).Cursor(ui.CursorPointer).Children(func() {
		ui.Icon(c, icon("file")).Size(14, 14).TextColor(p.placeholder)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, name).FontSize(12).LineHeight(16.0 / 12).TextColor(p.textSecond).SingleLine()
			if dir != "" {
				// dir="rtl" in the web: a long folder keeps its end.
				ui.Text(c, truncateStart(c, dir, 10, room)).FontSize(10).LineHeight(1.6).TextColor(p.placeholder).SingleLine()
			}
		})
		ui.Text(c, meta).FontSize(10).TextColor(p.placeholder).Shrink(0)
	})
	if row.Hovered() {
		row.Background(p.hover)
	}
	if row.Clicked() {
		a.openFile(spaceID, f.Path)
	}
}

// timeAgo is resource-kit formatTimeAgo.
func timeAgo(at, now time.Time) string {
	d := now.Sub(at)
	if d < 45*time.Second {
		return m("time_just_now")
	}
	mins := int(math.Round(d.Minutes()))
	if mins < 60 {
		return m("time_ago_min", "n", mins)
	}
	hours := int(math.Round(float64(mins) / 60))
	if hours < 24 {
		return m("time_ago_hour", "n", hours)
	}
	days := int(math.Round(float64(hours) / 24))
	if days <= 7 {
		return m("time_ago_day", "n", days)
	}
	if locale == "zh-CN" {
		if at.Year() == now.Year() {
			return at.Local().Format("1月2日")
		}
		return at.Local().Format("2006年1月2日")
	}
	if at.Year() == now.Year() {
		return at.Local().Format("Jan 2")
	}
	return at.Local().Format("Jan 2, 2006")
}

// scheduledPanel is ScheduledPanel.svelte: the Space's scheduled prompts.
func (a *app) scheduledPanel(c *ui.Context, spaceID string) {
	p := a.pal
	crons, cerr, _ := loadJSON[struct {
		Jobs []*cronJob `json:"jobs"`
	}](a, "crons:"+spaceID, "/api/cron-jobs?spaceId="+url.QueryEscape(spaceID))
	ui.Row(c.Key("sched-head")).Height(36).Shrink(0).Gap(4).AlignItems(ui.Center).Padding(0, 8).Children(func() {
		if a.can(spaceID, "cronjob.manage") {
			b := ui.ButtonBase(c.Key("new")).Row().Gap(4).AlignItems(ui.Center).Height(24).Padding(0, 6).Radius(5).FocusRing(false).Children(func() {
				ui.Icon(c, icon("plus")).Size(14, 14).TextColor(p.textTertiary)
				ui.Text(c, m("cron_new_scheduled")).FontSize(11).FontWeight(500).TextColor(p.textTertiary)
			})
			if b.Hovered() {
				b.Background(p.hover)
			}
			if b.Clicked() {
				a.cron = nil
				a.go_(route{kind: routeSpaceCron, space: spaceID, sub: "new"})
			}
		}
	})
	ui.Scroll(c.Key("sched")).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Padding(0, 6, 12, 6).Children(func() {
			switch {
			case crons == nil && cerr == nil:
				for i := 0; i < 3; i++ {
					ui.Row(c.Key(i)).Height(36).Gap(8).AlignItems(ui.Center).Padding(0, 6).Children(func() {
						ui.Box(c).Size(28, 28).Radius(8).Background(p.bgElevated)
						ui.Box(c).Width(120).Height(10).Radius(3).Background(p.bgElevated)
					})
				}
			case crons == nil:
				ui.Text(c, m("scheduled_panel_load_failed")).FontSize(12).TextColor(p.errorC).Margin(4, 6)
			case len(crons.Jobs) == 0:
				ui.Text(c, m("scheduled_panel_empty")).FontSize(12).LineHeight(1.5).TextColor(p.textTertiary).Margin(4, 6)
			default:
				for _, j := range crons.Jobs {
					on := a.route.kind == routeSpaceCron && a.route.sub == j.ID
					row := ui.Row(c.Key(j.ID)).Gap(8).AlignItems(ui.Center).Padding(4, 6).Radius(6).Cursor(ui.CursorPointer).Children(func() {
						col := p.placeholder
						if j.Enabled {
							col = p.success
						}
						ui.Box(c).Size(28, 28).Center().Shrink(0).Children(func() {
							ui.Icon(c, icon("clock")).Size(14, 14).TextColor(col)
						})
						ui.Column(c).Grow(1).MinWidth(0).Children(func() {
							name := j.Title
							if name == "" {
								name = j.Name
							}
							ui.Text(c, name).FontSize(12).TextColor(p.textSecond).SingleLine()
							ui.Text(c, j.Cron).FontSize(10).Font(fontMono).TextColor(p.placeholder).SingleLine()
						})
						if !j.Enabled {
							ui.Text(c, m("cron_paused")).FontSize(10).TextColor(p.placeholder).Padding(0, 4)
						}
					})
					a.pressable(row, on)
					id := j.ID
					if row.Clicked() {
						a.go_(route{kind: routeSpaceCron, space: spaceID, sub: id})
					}
				}
			}
		})
	})
}

// truncateStart cuts text from its start, behind "…", to fit width.
func truncateStart(c *ui.Context, text string, size, width float32) string {
	if w, _ := c.MeasureText(0, ui.Span{Text: text, Size: size}); w <= width {
		return text
	}
	r := []rune(text)
	lo, hi := 0, len(r)
	for lo < hi {
		mid := (lo + hi) / 2
		if w, _ := c.MeasureText(0, ui.Span{Text: "…" + string(r[mid:]), Size: size}); w <= width {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return "…" + string(r[lo:])
}
