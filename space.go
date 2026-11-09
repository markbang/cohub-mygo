package main

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

type checkpointRec struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	CommitHash  string    `json:"commitHash"`
	CreatedAt   time.Time `json:"createdAt"`
}

type taskRun struct {
	ID         string     `json:"id"`
	TaskType   string     `json:"taskType"`
	Status     string     `json:"status"`
	CronJobID  string     `json:"cronJobId"`
	Error      string     `json:"errorMessage"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
	Payload    struct {
		Data struct {
			Title   string  `json:"title"`
			Name    string  `json:"name"`
			Content []Block `json:"content"`
			Command string  `json:"command"`
		} `json:"data"`
	} `json:"payload"`
	Result map[string]any `json:"result"`
}

func (r *taskRun) title() string {
	d := r.Payload.Data
	switch {
	case d.Title != "":
		return d.Title
	case d.Name != "":
		return d.Name
	case joinText(d.Content) != "":
		return oneLine(joinText(d.Content))
	case d.Command != "":
		return d.Command
	}
	return strings.ReplaceAll(r.TaskType, "_", " ")
}

func (r *taskRun) when() time.Time {
	if r.FinishedAt != nil {
		return *r.FinishedAt
	}
	if r.StartedAt != nil {
		return *r.StartedAt
	}
	return r.CreatedAt
}

type cronJob struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Title     string     `json:"title"`
	Cron      string     `json:"cronExpression"`
	Pattern   string     `json:"pattern"`
	Enabled   bool       `json:"enabled"`
	NextRunAt *time.Time `json:"nextRunAt"`
}

type appRec struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Status      string    `json:"status"`
	Visibility  string    `json:"visibility"`
	PublishedAt time.Time `json:"publishedAt"`
	Meta        struct {
		Title string `json:"title"`
	} `json:"meta"`
}

func (r *appRec) title() string {
	if r.Meta.Title != "" {
		return r.Meta.Title
	}
	return r.Slug
}

type fsEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

type fsFile struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
	Kind     string `json:"kind"`
	Content  string `json:"content"`
}

func sp(id string) string { return "/api/spaces/" + url.PathEscape(id) }

// spaceSections are the sidebar's sections after the chats: apps, saves,
// scheduled prompts and tasks.
func (a *app) spaceSections(c *ui.Context, spaceID string) {
	p := a.pal
	apps, _, _ := loadJSON[struct {
		Apps []*appRec `json:"apps"`
	}](a, "apps:"+spaceID, "/api/apps/space/"+url.PathEscape(spaceID))
	if apps != nil && len(apps.Apps) > 0 {
		ui.Box(c.Key("gap-apps")).Height(12)
		if a.sectionHead(c, "apps", "rocket", m("x_apps"), nil) {
			for _, ap := range apps.Apps {
				row := ui.Row(c.Key(ap.ID)).Gap(8).AlignItems(ui.Center).Height(30).Padding(0, 6).Radius(6).Children(func() {
					ui.Text(c, ap.title()).FontSize(13).TextColor(p.textTertiary).SingleLine().Grow(1).MinWidth(0)
					ui.Text(c, relTime(ap.PublishedAt, a.now())).FontSize(9.5).TextColor(p.placeholder.Alpha(0.7)).Shrink(0)
				})
				a.pressable(row, false)
			}
		}
	}
	saves, _, _ := loadJSON[struct {
		Checkpoints []*checkpointRec `json:"checkpoints"`
	}](a, "saves:"+spaceID, sp(spaceID)+"/checkpoints?limit=20")
	ui.Box(c.Key("gap-saves")).Height(12)
	if a.sectionHead(c, "saves", "history", m("sidebar_saves"), nil) {
		switch {
		case saves == nil:
			a.sideEmpty(c, m("sidebar_loading_saves"), true)
		case len(saves.Checkpoints) == 0:
			a.sideEmpty(c, m("sidebar_no_saves"), false)
		}
		if saves != nil {
			for _, cp := range saves.Checkpoints {
				on := a.route.kind == routeSpaceCheckpoint && a.route.sub == cp.ID
				row := ui.Row(c.Key(cp.ID)).Gap(8).AlignItems(ui.Center).Height(30).Padding(0, 6).Radius(6).Children(func() {
					title := cp.Description
					if title == "" && len(cp.CommitHash) >= 7 {
						title = cp.CommitHash[:7]
					}
					t := ui.Text(c, title).FontSize(13).TextColor(p.textTertiary).SingleLine().Grow(1).MinWidth(0)
					if on {
						t.FontWeight(500).TextColor(p.text)
					}
					ui.Text(c, relTime(cp.CreatedAt, a.now())).FontSize(9.5).TextColor(p.placeholder.Alpha(0.7)).Shrink(0)
				})
				a.pressable(row, on)
				row.OnClick(func() { a.go_(route{kind: routeSpaceCheckpoint, space: spaceID, sub: cp.ID}) })
			}
		}
	}
	crons, _, _ := loadJSON[struct {
		Jobs []*cronJob `json:"jobs"`
	}](a, "crons:"+spaceID, "/api/cron-jobs?spaceId="+url.QueryEscape(spaceID))
	ui.Box(c.Key("gap-cron")).Height(12)
	if a.sectionHead(c, "cron", "calendar-clock", m("x_schedules"), func() {
		ui.Icon(c, icon("plus")).Size(12, 12).TextColor(p.placeholder)
	}) {
		switch {
		case crons == nil:
			a.sideEmpty(c, m("common_loading"), true)
		case len(crons.Jobs) == 0:
			a.sideEmpty(c, m("x_no_schedules"), false)
		}
		if crons != nil {
			for _, j := range crons.Jobs {
				row := ui.Row(c.Key(j.ID)).Gap(8).AlignItems(ui.Center).Height(30).Padding(0, 6).Radius(6).Children(func() {
					name := j.Title
					if name == "" {
						name = j.Name
					}
					ui.Text(c, name).FontSize(13).TextColor(p.textTertiary).SingleLine().Grow(1).MinWidth(0)
				})
				a.pressable(row, a.route.kind == routeSpaceCron && a.route.sub == j.ID)
				row.OnClick(func() { a.go_(route{kind: routeSpaceCron, space: spaceID, sub: j.ID}) })
			}
		}
	}
	tasks, _, _ := loadJSON[struct {
		Runs []*taskRun `json:"runs"`
	}](a, "tasks:"+spaceID, "/api/tasks?spaceId="+url.QueryEscape(spaceID)+"&limit=20")
	ui.Box(c.Key("gap-tasks")).Height(12)
	if a.sectionHead(c, "tasks", "activity", m("sidebar_tasks"), nil) {
		switch {
		case tasks == nil:
			a.sideEmpty(c, m("sidebar_loading_tasks"), true)
		case len(tasks.Runs) == 0:
			a.sideEmpty(c, m("sidebar_no_tasks"), false)
		}
		if tasks != nil {
			for _, r := range tasks.Runs {
				on := a.route.kind == routeSpaceTask && a.route.sub == r.ID
				col := a.taskColor(r.Status)
				row := ui.Row(c.Key(r.ID)).Gap(8).AlignItems(ui.Center).Padding(6, 6).Radius(6).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, r.title()).FontSize(12).TextColor(p.textSecond).SingleLine()
						ui.Text(c, taskTypeLabel(r.TaskType)+" · "+r.Status+" · "+relTime(r.when(), a.now())).FontSize(10).TextColor(col).SingleLine()
					})
					a.dot(c, col, 6)
				})
				a.pressable(row, on)
				row.OnClick(func() { a.go_(route{kind: routeSpaceTask, space: spaceID, sub: r.ID}) })
			}
		}
	}
}

func (a *app) sideEmpty(c *ui.Context, text string, loading bool) {
	p := a.pal
	ui.Row(c.Key("empty-"+text)).Gap(6).AlignItems(ui.Center).Padding(6, 24).Children(func() {
		if loading {
			a.spinner(c, 11, p.textTertiary)
		}
		ui.Text(c, text).FontSize(12).TextColor(p.textTertiary)
	})
}

func (a *app) taskColor(status string) ui.Color {
	p := a.pal
	switch status {
	case "completed":
		return p.success
	case "failed":
		return p.errorC
	case "running":
		return p.info
	}
	return p.placeholder
}

func taskTypeLabel(t string) string {
	switch t {
	case "send_message":
		return m("sidebar_send_message")
	case "run_command":
		return m("sidebar_run_command")
	case "save_checkpoint":
		return m("sidebar_save_checkpoint")
	case "generation":
		return m("sidebar_generation")
	}
	return t
}

// panelContent is the right panel's tab: the chat's output, the Space's
// files, its media or its apps.
func (a *app) panelContent(c *ui.Context) {
	p := a.pal
	spaceID := a.currentSpace()
	switch a.panelTab {
	case 1:
		if spaceID != "" {
			a.filesPanel(c, spaceID)
			return
		}
	case 2:
		if spaceID != "" {
			a.mediaPanel(c, spaceID, "", true)
		}
		return
	case 4:
		if spaceID != "" {
			a.scheduledPanel(c, spaceID)
			return
		}
	case 3:
		if spaceID != "" {
			apps, _, ok := loadJSON[struct {
				Apps []*appRec `json:"apps"`
			}](a, "apps:"+spaceID, "/api/apps/space/"+url.PathEscape(spaceID))
			ui.Column(c.Key("apps")).Padding(8).Gap(2).Children(func() {
				if !ok {
					a.spinner(c, 14, p.textTertiary)
					return
				}
				if apps == nil || len(apps.Apps) == 0 {
					ui.Text(c, m("x_apps_empty")).FontSize(12).TextColor(p.textTertiary).Padding(8)
					return
				}
				for _, ap := range apps.Apps {
					row := ui.Row(c.Key(ap.ID)).Gap(10).AlignItems(ui.Center).Padding(8).Radius(6).Children(func() {
						ui.Box(c).Size(32, 32).Radius(7).Background(p.hoverStrong).Center().Children(func() {
							ui.Icon(c, icon("layout-template")).Size(15, 15).TextColor(p.textTertiary)
						})
						ui.Column(c).Grow(1).MinWidth(0).Children(func() {
							ui.Text(c, ap.title()).FontSize(13).SingleLine()
							ui.Text(c, ap.Visibility+" · "+relTime(ap.PublishedAt, a.now())).FontSize(11).TextColor(p.textTertiary)
						})
					})
					a.pressable(row, false)
				}
			})
			return
		}
	}
	if spaceID != "" {
		a.sessionOutputs(c, spaceID, a.route.sessionID())
		return
	}
	ui.Column(c).Grow(1).Padding(16, 14).Children(func() {
		ui.Text(c, m("session_outputs_no_chat")).FontSize(12).LineHeight(1.6).TextColor(p.textTertiary)
	})
}

// filesPanel is the panel's file tree, as SpaceFileSidebar.svelte.
func (a *app) filesPanel(c *ui.Context, spaceID string) {
	ui.Row(c.Key("files-head")).Height(36).Gap(2).AlignItems(ui.Center).Padding(0, 6, 0, 12).Children(func() {
		ui.Text(c, m("x_files")).FontSize(12).FontWeight(600).Grow(1)
		a.iconButton(c.Key("new"), "plus", m("files_new"), false)
		a.iconButton(c.Key("upload"), "upload", m("common_upload"), false)
		if a.iconButton(c.Key("refresh"), "refresh-cw", m("rules_refresh"), false).Clicked() {
			for k := range a.loading {
				if strings.HasPrefix(k, "tree:"+spaceID) {
					delete(a.loading, k)
				}
			}
		}
	})
	ui.Scroll(c.Key("tree")).Grow(1).Children(func() {
		ui.Column(c).Padding(0, 6, 12, 6).Children(func() { a.treeDir(c, spaceID, "", 0) })
	})
}

func (a *app) treeDir(c *ui.Context, spaceID, dir string, depth int) {
	p := a.pal
	key := "tree:" + spaceID + ":" + dir
	q := url.Values{"pagination": {"true"}}
	if dir != "" {
		q.Set("path", dir)
	}
	page, err, ok := loadJSON[struct {
		Entries []fsEntry `json:"entries"`
	}](a, key, sp(spaceID)+"/fs/tree?"+q.Encode())
	if !ok {
		ui.Row(c.Key("loading-"+dir)).Padding(4, 8+float32(depth)*14).Children(func() { a.spinner(c, 12, p.textTertiary) })
		return
	}
	if err != nil {
		ui.Text(c.Key("err-"+dir), err.Error()).FontSize(11).TextColor(p.errorC).Padding(4, 8)
		return
	}
	entries := append([]fsEntry(nil), page.Entries...)
	sort.SliceStable(entries, func(i, j int) bool {
		if (entries[i].Type == "dir") != (entries[j].Type == "dir") {
			return entries[i].Type == "dir"
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	for _, e := range entries {
		isDir := e.Type == "dir"
		open := a.groupsOpen["fs:"+spaceID+":"+e.Path]
		on := a.route.kind == routeSpaceFiles && a.route.sub == e.Path
		row := ui.Row(c.Key(e.Path)).Gap(6).AlignItems(ui.Center).Height(26).Padding(0, 6, 0, 6+float32(depth)*14).Radius(5).Children(func() {
			if isDir {
				name := "chevron-right"
				if open {
					name = "chevron-down"
				}
				ui.Icon(c, icon(name)).Size(12, 12).TextColor(p.textTertiary)
			} else {
				ui.Icon(c, icon("file")).Size(13, 13).TextColor(p.textTertiary)
			}
			ui.Text(c, e.Name).FontSize(12).TextColor(p.textSecond).SingleLine()
		})
		a.pressable(row, on)
		row.OnClick(func() {
			if isDir {
				a.groupsOpen["fs:"+spaceID+":"+e.Path] = !open
				return
			}
			a.go_(route{kind: routeSpaceFiles, space: spaceID, sub: e.Path})
		})
		if isDir && open {
			a.treeDir(c, spaceID, e.Path, depth+1)
		}
	}
}

// windowPane is the Space's window beside the main column, as the web's
// WorkspaceWindowsPane: a file, a save, a task or a scheduled prompt.
func (a *app) windowPane(c *ui.Context) {
	p := a.pal
	r := a.route
	title, iconName := "", "file"
	switch r.kind {
	case routeSpaceFiles:
		title = path.Base(r.sub)
	case routeSpaceCheckpoint:
		title, iconName = m("sidebar_new_save"), "save"
		if r.sub != "new" {
			title = m("sidebar_saves")
		}
	case routeSpaceTask:
		title, iconName = m("sidebar_task_run"), "activity"
	case routeSpaceCron:
		title, iconName = m("x_schedules"), "calendar-clock"
	}
	ui.Column(c.Key("pane")).Width(480).Shrink(0).Background(p.bgPrimary).BorderWidth(0, 0, 0, 1).BorderColor(p.borderSubtle).Children(func() {
		ui.Row(c).Height(headerHeight).Shrink(0).Gap(4).AlignItems(ui.Center).Padding(0, 8).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).DragWindow().Children(func() {
			a.iconButton(c.Key("pane-side"), "panel-left", m("resource_app_files"), false)
			ui.Row(c.Key("tab")).Gap(6).AlignItems(ui.Center).Height(40).Padding(0, 8).BorderWidth(0, 0, 2, 0).BorderColor(p.brand).Children(func() {
				ui.Icon(c, icon(iconName)).Size(13, 13).TextColor(p.text)
				ui.Text(c, title).FontSize(12).FontWeight(500).SingleLine().MaxWidth(200)
				x := ui.ButtonBase(c.Key("close")).Size(18, 18).Center().Radius(4).Label(m("common_close")).FocusRing(false).Children(func() {
					ui.Icon(c, icon("x")).Size(12, 12).TextColor(p.textTertiary)
				})
				if x.Clicked() {
					a.go_(route{kind: routeSpace, space: r.space})
				}
			})
			ui.Spacer(c)
			a.iconButton(c.Key("reload"), "refresh-cw", m("rules_refresh"), false)
			a.iconButton(c.Key("popout"), "external-link", m("x_new_window"), false)
		})
		ui.Column(c).Grow(1).MinHeight(0).Children(func() {
			switch r.kind {
			case routeSpaceFiles:
				a.fileView(c, r.space, r.sub)
			case routeSpaceCheckpoint:
				a.checkpointView(c, r.space, r.sub)
			case routeSpaceTask:
				a.taskView(c, r.space, r.sub)
			case routeSpaceCron:
				a.cronView(c, r.space, r.sub)
			}
		})
	})
}

func (a *app) fileView(c *ui.Context, spaceID, file string) {
	p := a.pal
	if file == "" {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, m("x_select_file")).FontSize(12).TextColor(p.textTertiary)
		})
		return
	}
	f, err, ok := loadJSON[fsFile](a, "file:"+spaceID+":"+file, sp(spaceID)+"/fs/file?path="+url.QueryEscape(file))
	if !ok || err != nil {
		a.loadingBlock(c, err, 200)
		return
	}
	ui.Row(c.Key("crumbs")).Height(32).Gap(4).AlignItems(ui.Center).Padding(0, 12).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Text(c, f.Path).FontSize(11).Font(fontMono).TextColor(p.textTertiary).SingleLine().Grow(1)
		ui.Text(c, fmt.Sprintf("%s · %s", f.MimeType, byteSize(f.Size))).FontSize(10).TextColor(p.placeholder)
	})
	ui.Scroll(c.Key("file-" + file)).Grow(1).Children(func() {
		switch {
		case f.Kind != "text":
			ui.Text(c, f.MimeType).FontSize(12).TextColor(p.textTertiary).Padding(24)
		case strings.HasSuffix(strings.ToLower(f.Name), ".md"):
			ui.Column(c).Padding(20, 24).Children(func() { markdown.View(c, f.Content, &a.md) })
		default:
			ui.Text(c, f.Content).FontSize(12).Font(fontMono).LineHeight(1.6).TextColor(p.textSecond).Padding(12, 16).Selectable()
		}
	})
}

func byteSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// checkpointView is the new save's form, or a save's record.
func (a *app) checkpointView(c *ui.Context, spaceID, id string) {
	p := a.pal
	name, _ := a.spaceName(spaceID)
	ui.Row(c.Key("back")).Height(36).Gap(4).AlignItems(ui.Center).Padding(0, 12).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Icon(c, icon("chevron-left")).Size(13, 13).TextColor(p.textSecond)
		ui.Text(c, m("sidebar_saves")).FontSize(12).TextColor(p.textSecond)
	})
	ui.Scroll(c.Key("cp")).Grow(1).Children(func() {
		ui.Column(c).Padding(24, 20).Gap(6).Children(func() {
			if id != "new" {
				saves, _, _ := loadJSON[struct {
					Checkpoints []*checkpointRec `json:"checkpoints"`
				}](a, "saves:"+spaceID, sp(spaceID)+"/checkpoints?limit=20")
				var cp *checkpointRec
				if saves != nil {
					for _, x := range saves.Checkpoints {
						if x.ID == id {
							cp = x
						}
					}
				}
				if cp == nil {
					a.spinner(c, 16, p.textTertiary)
					return
				}
				ui.Text(c, m("sidebar_saves")).FontSize(11).TextColor(p.textTertiary)
				ui.Text(c, cp.Description).FontSize(20).FontWeight(600)
				ui.Text(c, cp.CreatedAt.Local().Format("2006-01-02 15:04")+" · "+cp.CommitHash[:min(12, len(cp.CommitHash))]).FontSize(12).Font(fontMono).TextColor(p.textTertiary)
				return
			}
			ui.Text(c, m("sidebar_new_save")).FontSize(11).TextColor(p.textTertiary)
			ui.Text(c, m("cp_save_workspace")).FontSize(20).FontWeight(600)
			ui.Text(c, m("cp_create_save_of")+" "+name+".").FontSize(13).TextColor(p.textSecond)
			ui.Row(c).Gap(4).Margin(16, 0, 6, 0).Children(func() {
				ui.Text(c, m("cp_description")).FontSize(11).FontWeight(500).TextColor(p.textSecond)
				ui.Text(c, m("cp_optional")).FontSize(11).TextColor(p.textTertiary)
			})
			desc, _ := a.pages.json["cp-desc:"+spaceID].(string)
			ui.TextArea(c.Key("desc"), &desc).Lines(3, 6).Placeholder(m("cp_what_changed_ph")).FontSize(13)
			a.pages.json["cp-desc:"+spaceID] = desc
			ui.Text(c, m("cp_desc_fallback_hint")).FontSize(11).TextColor(p.textTertiary).Margin(6, 0, 16, 0)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(12, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
				ui.Icon(c, icon("chevron-right")).Size(12, 12).TextColor(p.textTertiary)
				ui.Text(c, m("cp_review_changes")).FontSize(12).TextColor(p.textSecond)
				ui.Text(c, m("cp_workspace_as_is")).FontSize(11).TextColor(p.textTertiary)
			})
			save := ui.ButtonBase(c.Key("save")).Row().Gap(6).Height(36).Radius(6).Background(p.brand).Center().FocusRing(false).Margin(12, 0, 0, 0).Children(func() {
				ui.Icon(c, icon("save")).Size(14, 14).TextColor(p.brandFg)
				ui.Text(c, m("common_save")).FontSize(13).FontWeight(500).TextColor(p.brandFg)
			})
			if save.Hovered() {
				save.Background(p.brandHover)
			}
			if save.Clicked() && a.api != nil {
				d := desc
				a.spawn(func() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					var out struct {
						Checkpoint *checkpointRec `json:"checkpoint"`
					}
					err := a.api.do(ctx, "POST", sp(spaceID)+"/checkpoints", map[string]any{"description": nilIfEmpty(d)}, &out)
					a.update(func() {
						if err == nil {
							a.reload("saves:" + spaceID)
							a.pages.json["cp-desc:"+spaceID] = ""
							if out.Checkpoint != nil {
								a.go_(route{kind: routeSpaceCheckpoint, space: spaceID, sub: out.Checkpoint.ID})
							}
						}
					})
				})
			}
			cancel := ui.ButtonBase(c.Key("cancel")).Height(32).Center().FocusRing(false).Children(func() {
				ui.Text(c, m("common_cancel")).FontSize(12).TextColor(p.textSecond)
			})
			if cancel.Clicked() {
				a.go_(route{kind: routeSpace, space: spaceID})
			}
		})
	})
}

func nilIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func (a *app) taskView(c *ui.Context, spaceID, id string) {
	p := a.pal
	out, err, ok := loadJSON[struct {
		Run *taskRun `json:"run"`
	}](a, "task:"+id, "/api/tasks/"+url.PathEscape(id))
	if !ok || err != nil || out.Run == nil {
		a.loadingBlock(c, err, 200)
		return
	}
	r := out.Run
	ui.Scroll(c.Key("task")).Grow(1).Children(func() {
		ui.Column(c).Padding(20).Gap(12).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				a.dot(c, a.taskColor(r.Status), 8)
				ui.Text(c, r.title()).FontSize(16).FontWeight(600).SingleLine()
			})
			kv := func(k, v string) {
				ui.Row(c.Key(k)).Gap(12).Children(func() {
					ui.Text(c, k).FontSize(12).TextColor(p.textTertiary).Width(72)
					ui.Text(c, v).FontSize(12).Font(fontMono).TextColor(p.textSecond).Grow(1)
				})
			}
			kv(m("cron_type"), taskTypeLabel(r.TaskType))
			kv(m("channels_col_status"), r.Status)
			kv(m("x_created"), r.CreatedAt.Local().Format("2006-01-02 15:04:05"))
			if r.FinishedAt != nil {
				kv(m("tool_status_done"), r.FinishedAt.Local().Format("2006-01-02 15:04:05"))
			}
			if r.Error != "" {
				ui.Text(c, r.Error).FontSize(12).TextColor(p.errorC).Padding(8, 10).Radius(6).Background(p.errorC.Alpha(0.08))
			}
			if t := joinText(r.Payload.Data.Content); t != "" {
				ui.Text(c, t).FontSize(12).Font(fontMono).LineHeight(1.6).Padding(10, 12).Radius(6).Background(p.bgSurface).Selectable()
			}
		})
	})
}

func (a *app) cronView(c *ui.Context, spaceID, id string) {
	p := a.pal
	out, err, ok := loadJSON[struct {
		Job map[string]any `json:"job"`
	}](a, "cron:"+id, "/api/cron-jobs/"+url.PathEscape(id))
	if !ok || err != nil {
		a.loadingBlock(c, err, 200)
		return
	}
	ui.Scroll(c.Key("cron")).Grow(1).Children(func() {
		ui.Column(c).Padding(20).Gap(8).Children(func() {
			keys := make([]string, 0, len(out.Job))
			for k := range out.Job {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				ui.Row(c.Key(k)).Gap(12).Children(func() {
					ui.Text(c, k).FontSize(12).TextColor(p.textTertiary).Width(120)
					ui.Text(c, fmt.Sprint(out.Job[k])).FontSize(12).Font(fontMono).TextColor(p.textSecond).Grow(1)
				})
			}
		})
	})
}
