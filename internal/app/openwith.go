package app

import (
	"context"
	"encoding/json"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Files open with Apps, as the web's file-handlers.ts: an App declares the
// extensions it opens (meta.fileHandlers), a Space's .cohub/apps.json says
// which installed App opens each by default, and "Open with…" picks one.

const installedAppsPath = ".cohub/apps.json"

var extensionRe = regexp.MustCompile(`^\.[a-z0-9][a-z0-9_-]{0,31}$`)

// normalizeExtension makes "board", ".Board" or " .board " ".board".
func normalizeExtension(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if !strings.HasPrefix(v, ".") {
		v = "." + v
	}
	if extensionRe.MatchString(v) {
		return v
	}
	return ""
}

// fileExtensionOf is the extension of a path's last segment; dotfiles have none.
func fileExtensionOf(p string) string {
	name := path.Base(strings.TrimRight(p, "/"))
	if i := strings.LastIndex(name, "."); i > 0 {
		return normalizeExtension(name[i:])
	}
	return ""
}

// parseFileHandlers reads a list or a comma or space separated string.
func parseFileHandlers(v any) []string {
	var raw []string
	switch v := v.(type) {
	case []any:
		for _, e := range v {
			if s, ok := e.(string); ok {
				raw = append(raw, s)
			}
		}
	case string:
		raw = strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
	}
	var out []string
	seen := map[string]bool{}
	for _, e := range raw {
		if x := normalizeExtension(e); x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
			if len(out) >= 32 {
				break
			}
		}
	}
	return out
}

// canOpenFileWith tells a published App that declares the file's extension.
func canOpenFileWith(r *appRec, p string) bool {
	if r == nil || r.Status != "published" {
		return false
	}
	ext := fileExtensionOf(p)
	for _, h := range parseFileHandlers(r.Meta.FileHandlers) {
		if ext != "" && h == ext {
			return true
		}
	}
	return false
}

// appDetail is GET /api/apps/:id.
type appDetail struct {
	App       appRec  `json:"app"`
	PublicURL *string `json:"publicUrl"`
	Owner     struct {
		Username string `json:"username"`
	} `json:"owner"`
	Space struct {
		Slug string `json:"slug"`
	} `json:"space"`
}

// installedFile is .cohub/apps.json with the revision it was read at.
type installedFile struct {
	doc      installedApps
	mtimeMs  float64
	size     int64
	revision bool
}

// fileHandlerCache is what the file opens consult: each Space's installed
// Apps and the Apps' details.
type fileHandlerCache struct {
	installed map[string]*installedFile
	details   map[string]*appDetail // nil: gone
}

func (a *app) handlers() *fileHandlerCache {
	if a.fileHandlers == nil {
		a.fileHandlers = &fileHandlerCache{installed: map[string]*installedFile{}, details: map[string]*appDetail{}}
	}
	return a.fileHandlers
}

// readInstalled reads .cohub/apps.json; a missing file is an empty one.
func (a *app) readInstalled(ctx context.Context, spaceID string) (*installedFile, error) {
	var f fsFile
	err := a.api.Get(ctx, sp(spaceID)+"/fs/file?path="+url.QueryEscape(installedAppsPath), &f)
	if isStatus(err, 404) {
		return &installedFile{doc: installedApps{Format: "cohub.space-apps", Version: 1}}, nil
	}
	if err != nil {
		return nil, err
	}
	out := &installedFile{mtimeMs: f.MtimeMs, size: f.Size, revision: true}
	if err := json.Unmarshal([]byte(f.Content), &out.doc); err != nil {
		return nil, err
	}
	return out, nil
}

// fetchApp loads an App, publicly when it is not the viewer's; nil when gone.
func (a *app) fetchApp(ctx context.Context, id string) (*appDetail, error) {
	var d appDetail
	err := a.api.Get(ctx, "/api/apps/"+url.PathEscape(id), &d)
	if isStatus(err, 401) || isStatus(err, 403) {
		err = a.api.Get(ctx, "/api/apps/"+url.PathEscape(id)+"/public", &d)
	}
	if isStatus(err, 403) || isStatus(err, 404) {
		a.update(func() { delete(a.handlers().details, id) })
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.update(func() { a.handlers().details[id] = &d })
	return &d, nil
}

// warmFileHandlers reads a Space's installed Apps and their details, so
// opening a file can go straight to its App.
func (a *app) warmFileHandlers(spaceID string) {
	a.once("file-handlers:"+spaceID, func(ctx context.Context) {
		f, err := a.readInstalled(ctx, spaceID)
		if err != nil {
			return
		}
		for _, ap := range f.doc.Apps {
			if on, _ := ap["enabled"].(bool); on && len(installedOpens(ap)) > 0 {
				id, _ := ap["id"].(string)
				a.fetchApp(ctx, id)
			}
		}
		a.update(func() { a.handlers().installed[spaceID] = f })
	})
}

func installedOpens(ap map[string]any) []string {
	var out []string
	if list, ok := ap["opens"].([]any); ok {
		for _, e := range list {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// installedHandler is the enabled installed App that opens the file.
func installedHandler(doc installedApps, p string) (id, ref string) {
	ext := fileExtensionOf(p)
	if ext == "" {
		return "", ""
	}
	for _, ap := range doc.Apps {
		if on, _ := ap["enabled"].(bool); !on {
			continue
		}
		for _, o := range installedOpens(ap) {
			if o == ext {
				id, _ = ap["id"].(string)
				ref, _ = ap["ref"].(string)
				return id, ref
			}
		}
	}
	return "", ""
}

// defaultFileApp is the App that opens the file by default, from what is
// warm; "" opens it built in. A default that can no longer open it says so.
func (a *app) defaultFileApp(spaceID, p string) string {
	a.warmFileHandlers(spaceID)
	h := a.handlers()
	f := h.installed[spaceID]
	if f == nil {
		return ""
	}
	id, ref := installedHandler(f.doc, p)
	if id == "" {
		return ""
	}
	d, known := h.details[id]
	if !known {
		return ""
	}
	if d == nil || !canOpenFileWith(&d.App, p) {
		a.notify(m("open_with_unavailable", "app", firstNonEmpty(ref, id)))
		return ""
	}
	return id
}

// handlerApp is a choice in the Open with dialog.
type handlerApp struct {
	id, slug, title, icon string
}

type openWithState struct {
	space, path string
	cands       []handlerApp
	def         string
	loaded      bool
	always      bool
	open        bool
}

// openFileWith opens the Open with dialog for a file.
func (a *app) openFileWith(spaceID, p string) {
	st := &openWithState{space: spaceID, path: p, open: true}
	a.openWith = st
	if a.api == nil {
		st.loaded = true
		return
	}
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		var cands []handlerApp
		def := ""
		if fileExtensionOf(p) != "" {
			var own struct {
				Apps []*appRec `json:"apps"`
			}
			a.api.Get(ctx, "/api/apps/space/"+url.PathEscape(spaceID), &own)
			recs := own.Apps
			if f, err := a.readInstalled(ctx, spaceID); err == nil {
				a.update(func() { a.handlers().installed[spaceID] = f })
				for _, ap := range f.doc.Apps {
					if on, _ := ap["enabled"].(bool); !on {
						continue
					}
					id, _ := ap["id"].(string)
					if d, _ := a.fetchApp(ctx, id); d != nil {
						r := d.App
						recs = append(recs, &r)
					}
				}
				if id, _ := installedHandler(f.doc, p); id != "" {
					if d, _ := a.fetchApp(ctx, id); d != nil && canOpenFileWith(&d.App, p) {
						def = id
					}
				}
			}
			seen := map[string]bool{}
			for _, r := range recs {
				if r == nil || seen[r.ID] || !canOpenFileWith(r, p) {
					continue
				}
				seen[r.ID] = true
				cands = append(cands, handlerApp{id: r.ID, slug: r.Slug, title: r.title(), icon: r.Meta.Icon})
			}
		}
		a.update(func() {
			if a.openWith == st {
				st.cands, st.def, st.loaded = cands, def, true
			}
		})
	})
}

// pickOpenWith opens the file with the choice ("" built in), first making
// it the Space's default for the extension when asked.
func (a *app) pickOpenWith(appID string) {
	st := a.openWith
	a.openWith = nil
	if st == nil {
		return
	}
	if st.always {
		spaceID, p := st.space, st.path
		a.spawn(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if err := a.saveDefaultFileHandler(ctx, spaceID, p, appID); err != nil {
				a.update(func() { a.toast, a.toastAt, a.toastOK = m("open_with_save_failed"), time.Now(), false })
			}
		})
	}
	if appID == "" {
		a.openFileBuiltin(st.space, st.path)
		return
	}
	a.openFileInApp(st.space, st.path, appID)
}

// saveDefaultFileHandler is saveDefaultFileHandler: it installs the App when
// it is not yet, enables it, and moves the extension to it.
func (a *app) saveDefaultFileHandler(ctx context.Context, spaceID, p, appID string) error {
	ext := fileExtensionOf(p)
	if ext == "" {
		return nil
	}
	f, err := a.readInstalled(ctx, spaceID)
	if err != nil {
		return err
	}
	doc := f.doc
	if doc.Format == "" {
		doc.Format, doc.Version = "cohub.space-apps", 1
	}
	if appID != "" {
		has := false
		for _, ap := range doc.Apps {
			if id, _ := ap["id"].(string); id == appID {
				has = true
			}
		}
		if !has {
			d, err := a.fetchApp(ctx, appID)
			if err != nil {
				return err
			}
			if d == nil || d.PublicURL == nil {
				return errString("This App is not available.")
			}
			snap := map[string]any{"name": d.App.title()}
			if d.App.Meta.Description != "" {
				snap["description"] = d.App.Meta.Description
			}
			if ic := d.App.Meta.Icon; strings.HasPrefix(ic, "http://") || strings.HasPrefix(ic, "https://") {
				snap["icon"] = ic
			}
			doc.Apps = append(doc.Apps, map[string]any{
				"id": appID, "ref": d.Owner.Username + "/" + d.Space.Slug + "/" + d.App.Slug, "url": *d.PublicURL,
				"enabled": true, "source": map[string]any{"type": "url", "url": *d.PublicURL},
				"snapshot": snap, "installedAt": time.Now().UTC().Format(time.RFC3339Nano),
			})
		}
		// Enabling never takes back an extension another App holds.
		held := map[string]bool{}
		for _, ap := range doc.Apps {
			if id, _ := ap["id"].(string); id != appID {
				if on, _ := ap["enabled"].(bool); on {
					for _, o := range installedOpens(ap) {
						held[o] = true
					}
				}
			}
		}
		for _, ap := range doc.Apps {
			if id, _ := ap["id"].(string); id == appID {
				if on, _ := ap["enabled"].(bool); !on {
					ap["enabled"] = true
					var keep []any
					for _, o := range installedOpens(ap) {
						if !held[o] {
							keep = append(keep, o)
						}
					}
					setOpens(ap, keep)
				}
			}
		}
	}
	for _, ap := range doc.Apps {
		var opens []any
		for _, o := range installedOpens(ap) {
			if o != ext {
				opens = append(opens, o)
			}
		}
		if id, _ := ap["id"].(string); id == appID && appID != "" {
			opens = append(opens, ext)
		}
		setOpens(ap, opens)
	}
	b, _ := json.MarshalIndent(doc, "", "  ")
	in := map[string]any{"path": installedAppsPath, "content": string(b) + "\n", "encoding": "utf-8"}
	if f.revision {
		in["expected"] = map[string]any{"mtimeMs": f.mtimeMs, "size": f.size}
	}
	if err := a.api.Do(ctx, "PUT", sp(spaceID)+"/fs/file", in, nil); err != nil {
		return err
	}
	a.update(func() {
		a.handlers().installed[spaceID] = &installedFile{doc: doc}
		a.reload("file-handlers:" + spaceID)
		a.forget("file:" + spaceID + ":" + installedAppsPath)
	})
	return nil
}

func setOpens(ap map[string]any, opens []any) {
	if len(opens) == 0 {
		delete(ap, "opens")
		return
	}
	ap["opens"] = opens
}

type errString string

func (e errString) Error() string { return string(e) }

// openFileInApp opens the file in an App's window, ?window=app:<id>:<path>.
func (a *app) openFileInApp(spaceID, p, appID string) {
	r := a.fileRoute(spaceID)
	r.win = "app:" + appID + ":" + p
	a.go_(r)
}

// openWithDialog is OpenWithDialog.svelte.
func (a *app) openWithDialog(c *ui.Context) {
	st := a.openWith
	if st == nil {
		return
	}
	p := a.pal
	open := true
	a.dialog(c, &open, m("open_with_title", "name", path.Base(st.path)), 360, func() {
		ui.Column(c.Key("open-with")).Padding(4, 0).Children(func() {
			choice := func(key, label string, def bool, mark func()) {
				row := ui.ButtonBase(c.Key(key)).Row().Gap(10).AlignItems(ui.Center).MinHeight(36).Padding(6, 12).FocusRing(false).Children(func() {
					ui.Box(c).Size(20, 20).Center().Children(mark)
					ui.Text(c, label).FontSize(13).Grow(1).MinWidth(0).SingleLine()
					if def {
						ui.Text(c, m("open_with_default")).FontSize(10).FontWeight(500).TextColor(p.textTertiary).Padding(1, 6).Radius(4).Background(p.hover)
					}
				})
				if row.Hovered() {
					row.Background(p.hover)
				}
				if row.Clicked() {
					id := strings.TrimPrefix(key, "app-")
					if key == "builtin" {
						id = ""
					}
					a.pickOpenWith(id)
				}
			}
			choice("builtin", m("open_with_builtin"), st.loaded && st.def == "", func() {
				ui.Icon(c, icon("file-text")).Size(16, 16).TextColor(p.textTertiary)
			})
			for _, ap := range st.cands {
				ap := ap
				choice("app-"+ap.id, ap.title, ap.id == st.def, func() {
					a.avatarSeed(c, ap.icon, ap.title, ap.id, 16, 4)
				})
			}
			if !st.loaded {
				ui.Row(c.Key("loading")).Gap(8).AlignItems(ui.Center).Padding(8, 12).Children(func() {
					a.spinner(c, 14, p.textTertiary)
					ui.Text(c, m("open_with_loading")).FontSize(12).TextColor(p.textTertiary)
				})
			}
		})
		if ext := fileExtensionOf(st.path); ext != "" && a.can(st.space, "file.edit") {
			ui.Row(c.Key("always")).Gap(8).AlignItems(ui.Center).Padding(8, 12).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Children(func() {
				ui.Checkbox(c, &st.always, m("open_with_always", "extension", ext)).FontSize(12).TextColor(p.textSecond)
			})
		}
	})
	if !open && a.openWith == st {
		a.openWith = nil
	}
}
