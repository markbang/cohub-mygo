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
	ID            string    `json:"id"`
	Slug          string    `json:"slug"`
	Status        string    `json:"status"`
	Visibility    string    `json:"visibility"`
	TargetType    string    `json:"targetType"`
	TargetRef     string    `json:"targetRef"`
	LatestVersion int       `json:"latestVersion"`
	AppScopes     []string  `json:"appScopes"`
	PublishedAt   time.Time `json:"publishedAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	Meta          struct {
		Title        string `json:"title"`
		Icon         string `json:"icon"`
		Description  string `json:"description"`
		FileHandlers any    `json:"fileHandlers"`
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
	Path     string  `json:"path"`
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	MimeType string  `json:"mimeType"`
	Kind     string  `json:"kind"`
	Content  string  `json:"content"`
	MtimeMs  float64 `json:"mtimeMs"`
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
				on := a.route.kind == routeSpaceApp && a.route.sub == ap.ID
				a.pressable(row, on)
				appID := ap.ID
				row.OnClick(func() { a.go_(route{kind: routeSpaceApp, space: spaceID, sub: appID}) })
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
		add := ui.ButtonBase(c.Key("new-cron")).Size(18, 18).Center().Radius(4).Label(m("cron_new_scheduled")).FocusRing(false).Children(func() {
			ui.Icon(c, icon("plus")).Size(12, 12).TextColor(p.placeholder)
		})
		if add.Clicked() {
			a.cron = nil
			a.go_(route{kind: routeSpaceCron, space: spaceID, sub: "new"})
		}
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
func (a *app) panelContent(c *ui.Context, tab int) {
	p := a.pal
	spaceID := a.currentSpace()
	switch tab {
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
			a.appsPanel(c, spaceID)
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
	p := a.pal
	edit := a.can(spaceID, "file.edit")
	a.warmFileHandlers(spaceID)
	ui.Row(c.Key("files-head")).Height(36).Gap(2).AlignItems(ui.Center).Padding(0, 6, 0, 12).Children(func() {
		ui.Text(c, m("files_title")).FontSize(12).FontWeight(600).Grow(1)
		if a.uploading > 0 {
			a.spinner(c, 12, p.textTertiary)
		}
		if edit {
			nb := a.iconButton(c.Key("new"), "plus", m("files_new"), a.menus["files-new"])
			if nb.Clicked() {
				a.menus["files-new"] = !a.menus["files-new"]
			}
			open := a.menus["files-new"]
			a.menuWidth(c, nb, &open, 160, []menuItem{
				{icon: "file-plus", label: m("files_new_file"), run: func() { a.fsPending = &fsCreate{space: spaceID, kind: "file"} }},
				{icon: "folder-plus", label: m("files_new_folder"), run: func() { a.fsPending = &fsCreate{space: spaceID, kind: "dir"} }},
			})
			a.menus["files-new"] = open
			if a.iconButton(c.Key("upload"), "upload", m("files_upload_files"), false).Clicked() {
				a.uploadFiles(spaceID, "")
			}
		}
		if a.iconButton(c.Key("refresh"), "refresh-cw", m("files_refresh"), false).Clicked() {
			a.forgetPrefix("tree:" + spaceID)
		}
	})
	a.portChips(c, spaceID)
	tree := ui.Scroll(c.Key("tree")).Grow(1)
	if edit {
		if files := tree.DroppedFiles(); files != nil {
			a.uploadPaths(spaceID, "", files)
		}
		if tree.FileDragOver() {
			tree.Background(p.brandMuted)
		}
	}
	tree.Children(func() {
		ui.Column(c).Padding(0, 6, 12, 6).Children(func() {
			a.fsPendingRow(c, spaceID, "", 0)
			a.treeDir(c, spaceID, "", 0)
		})
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
		on := a.route.file() == e.Path
		row := ui.Row(c.Key(e.Path)).Gap(6).AlignItems(ui.Center).Height(30).Padding(0, 6, 0, 6+float32(depth)*14).Radius(5).Children(func() {
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
			a.openFile(spaceID, e.Path)
		})
		entry := e
		actions := a.fileActions(spaceID, entry.Path, isDir)
		if isDir && a.can(spaceID, "file.edit") {
			actions = append([]menuItem{
				{icon: "file-plus", label: m("files_new_file"), run: func() { a.fsPending = &fsCreate{space: spaceID, dir: entry.Path, kind: "file"} }},
				{icon: "folder-plus", label: m("files_new_folder"), run: func() { a.fsPending = &fsCreate{space: spaceID, dir: entry.Path, kind: "dir"} }},
				{icon: "upload", label: m("files_upload_files"), run: func() { a.uploadFiles(spaceID, entry.Path) }, divider: true},
			}, actions...)
		}
		row.PopoverMenu(func(mn *ui.Menu) {
			for _, it := range actions {
				item := mn.Item(it.label)
				if it.danger {
					item.Danger()
				}
				if item.Chosen() && it.run != nil {
					it.run()
				}
				if it.divider {
					mn.Separator()
				}
			}
		})
		if f := a.fsPending; f != nil && f.kind == "rename" && f.from == e.Path {
			continue
		}
		if isDir && open {
			a.fsPendingRow(c, spaceID, e.Path, depth+1)
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
	switch {
	case r.file() != "":
		title = path.Base(r.file())
	case r.port() != "":
		title, iconName = ":"+r.port(), "globe"
	case strings.HasPrefix(r.win, "app:") || strings.HasPrefix(r.win, "work:"):
		title, iconName = m("x_apps"), "panels-top-left"
	case r.display() != "":
		title, iconName = r.display(), "monitor"
		ds, _ := a.displays(r.space)
		for _, d := range ds {
			if d.ID == r.display() {
				title = firstNonEmpty(d.Name, d.ID)
				if d.Kind == "agent" {
					title = m("runtime_display_agent")
				}
			}
		}
	case r.kind == routeSpaceCheckpoint:
		title, iconName = m("sidebar_new_save"), "save"
		if r.sub != "new" {
			title = m("sidebar_saves")
		}
	case r.kind == routeSpaceApp:
		title, iconName = m("x_apps"), "panels-top-left"
	case r.kind == routeSpaceTask:
		title, iconName = m("sidebar_task_run"), "activity"
	case r.kind == routeSpaceCron:
		title, iconName = m("x_schedules"), "calendar-clock"
		if r.sub == "new" {
			title = m("cron_new_scheduled")
		} else if j, _, _ := loadJSON[struct {
			Job *cronRecord `json:"job"`
		}](a, "cron:"+r.sub, "/api/cron-jobs/"+url.PathEscape(r.sub)); j != nil && j.Job != nil {
			title = j.Job.Title
		}
	}
	pane := ui.Column(c.Key("pane"))
	if a.windowWide {
		pane.Grow(1)
	} else {
		pane.Width(480).Shrink(0)
	}
	pane.Background(p.bgPrimary).BorderWidth(0, 0, 0, 1).BorderColor(p.borderSubtle).Children(func() {
		ui.Row(c).Height(headerHeight).Shrink(0).Gap(4).AlignItems(ui.Center).Padding(0, 8).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).DragWindow().Children(func() {
			a.iconButton(c.Key("pane-side"), "panel-left", m("resource_app_files"), false)
			ui.Row(c.Key("tab")).Gap(6).AlignItems(ui.Center).Height(40).Padding(0, 8).BorderWidth(0, 0, 2, 0).BorderColor(p.brand).Children(func() {
				ui.Icon(c, icon(iconName)).Size(13, 13).TextColor(p.text)
				ui.Text(c, title).FontSize(12).FontWeight(500).SingleLine().MaxWidth(200)
				x := ui.ButtonBase(c.Key("close")).Size(18, 18).Center().Radius(4).Label(m("common_close")).FocusRing(false).Children(func() {
					ui.Icon(c, icon("x")).Size(12, 12).TextColor(p.textTertiary)
				})
				if x.Clicked() {
					a.closeWindow()
				}
			})
			ui.Spacer(c)
			if f := r.file(); f != "" {
				a.fileWindowActions(c, r.space, f)
			} else if r.port() != "" {
				a.portActions(c, r.space, r.port())
			} else {
				a.iconButton(c.Key("reload"), "refresh-cw", m("rules_refresh"), false)
			}
			ic := "maximize-2"
			if a.windowWide {
				ic = "minimize-2"
			}
			if a.iconButton(c.Key("wide"), ic, m("x_maximize"), a.windowWide).Clicked() {
				a.windowWide = !a.windowWide
			}
		})
		ui.Column(c).Grow(1).MinHeight(0).Children(func() {
			switch {
			case r.file() != "":
				a.fileView(c, r.space, r.file())
			case r.port() != "":
				a.portView(c, r.space, r.port())
			case r.display() != "":
				a.displayView(c, r.space, r.display())
			case strings.HasPrefix(r.win, "app:") || strings.HasPrefix(r.win, "work:"):
				a.webSurface(c, "https://cohub.live"+r.String())
			case r.kind == routeSpaceCheckpoint:
				a.checkpointView(c, r.space, r.sub)
			case r.kind == routeSpaceTask:
				a.taskView(c, r.space, r.sub)
			case r.kind == routeSpaceApp:
				a.appView(c, r.space, r.sub)
			case r.kind == routeSpaceCron:
				a.cronView(c, r.space, r.sub)
			}
		})
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
	if id != "new" {
		a.checkpointDetail(c, spaceID, id)
		return
	}
	p := a.pal
	name, _ := a.spaceName(spaceID)
	ui.Row(c.Key("back")).Height(36).Gap(4).AlignItems(ui.Center).Padding(0, 12).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Icon(c, icon("chevron-left")).Size(13, 13).TextColor(p.textSecond)
		ui.Text(c, m("sidebar_saves")).FontSize(12).TextColor(p.textSecond)
	})
	ui.Scroll(c.Key("cp")).Grow(1).Children(func() {
		ui.Column(c).Padding(24, 20).Gap(6).Children(func() {
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
			review := a.groupsOpen["cp-review:"+spaceID]
			rv := ui.Row(c.Key("review")).Gap(6).AlignItems(ui.Center).Padding(12, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Cursor(ui.CursorPointer).Children(func() {
				ch := "chevron-right"
				if review {
					ch = "chevron-down"
				}
				ui.Icon(c, icon(ch)).Size(12, 12).TextColor(p.textTertiary)
				ui.Text(c, m("cp_review_changes")).FontSize(12).TextColor(p.textSecond)
				ui.Text(c, m("cp_workspace_as_is")).FontSize(11).TextColor(p.textTertiary)
			})
			if rv.Clicked() {
				a.groupsOpen["cp-review:"+spaceID] = !review
			}
			if review {
				sum, err, ok := loadJSON[diffSummary](a, "ws-diff:"+spaceID, sp(spaceID)+"/fs/diff")
				switch {
				case !ok:
					a.spinner(c, 12, p.textTertiary)
				case err != nil:
					ui.Text(c, err.Error()).FontSize(11).TextColor(p.errorC)
				case len(sum.Files) == 0:
					ui.Text(c, m("cp_no_changes_since_save")).FontSize(12).TextColor(p.textTertiary)
				default:
					for _, f := range sum.Files {
						ui.Row(c.Key("chg-" + f.Path)).Gap(8).Children(func() {
							ui.Text(c, f.Status).FontSize(10).FontWeight(600).Font(fontMono).TextColor(p.warning).Width(12)
							ui.Text(c, f.Path).FontSize(12).Font(fontMono).TextColor(p.textSecond).SingleLine().Grow(1)
							ui.Text(c, fmt.Sprintf("+%d −%d", f.Additions, f.Deletions)).FontSize(11).Font(fontMono).TextColor(p.textTertiary)
						})
					}
				}
			}
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
