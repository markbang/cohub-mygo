package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
)

// A Space's files: the tree's new file, new folder, upload, rename,
// delete and its row menu, and the file window, which edits text in the
// code editor and saves it with Cmd+S, shows pictures and downloads the
// rest, as SpaceFileSidebar and SpaceFileDomain.

// fileEdit is an open file's editor, kept across frames.
type fileEdit struct {
	state   *editor.State
	mtimeMs float64
	size    int64
	saving  bool
	preview bool // a Markdown file shows rendered until edited
}

// fsCreate is the tree's pending new file or folder, or a rename.
type fsCreate struct {
	space, dir, kind string // kind: file, dir, rename
	from             string // the renamed path
	name             string
}

func (a *app) editorFor(spaceID string, f *fsFile) *fileEdit {
	key := spaceID + ":" + f.Path
	if e := a.editors[key]; e != nil {
		return e
	}
	buf := editor.NewBuffer(f.Content)
	buf.SetLanguage(editor.LanguageFor(f.Name))
	buf.MarkSaved()
	e := &fileEdit{
		state:   &editor.State{Buffer: buf, Font: ui.Font{Family: fontMono, Size: 12.5}, LineHeight: 1.6, SoftWrap: true, Label: f.Path},
		mtimeMs: f.MtimeMs, size: f.Size,
		preview: strings.HasSuffix(strings.ToLower(f.Name), ".md"),
	}
	a.editors[key] = e
	return e
}

// raw downloads a file's bytes from the Space.
func (cl *client) raw(ctx context.Context, spaceID, p string) ([]byte, string, error) {
	token, err := cl.accessToken(ctx)
	if err != nil {
		return nil, "", err
	}
	for i := 0; i < 20; i++ {
		req, _ := http.NewRequestWithContext(ctx, "GET", cl.base+sp(spaceID)+"/fs/download?path="+url.QueryEscape(p), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, "", err
		}
		if res.StatusCode == http.StatusAccepted {
			res.Body.Close()
			time.Sleep(time.Second)
			continue
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return nil, "", fmt.Errorf("download %s: HTTP %d", p, res.StatusCode)
		}
		b, err := io.ReadAll(io.LimitReader(res.Body, 256<<20))
		return b, res.Header.Get("Content-Type"), err
	}
	return nil, "", fmt.Errorf("download %s: still preparing", p)
}

func (a *app) fileView(c *ui.Context, spaceID, file string) {
	p := a.pal
	if file == "" {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, m("file_viewer_pick_hint")).FontSize(12).TextColor(p.textTertiary)
		})
		return
	}
	key := "file:" + spaceID + ":" + file
	f, err, ok := loadJSON[fsFile](a, key, sp(spaceID)+"/fs/file?path="+url.QueryEscape(file))
	if !ok || err != nil {
		a.loadingBlock(c, err, 200)
		return
	}
	if strings.HasSuffix(strings.ToLower(f.Name), ".board") {
		a.boardWindow(c, spaceID, f)
		return
	}
	editable := f.Kind == "text" && a.can(spaceID, "file.edit")
	var ed *fileEdit
	if f.Kind == "text" {
		ed = a.editorFor(spaceID, f)
		ed.state.ReadOnly = !editable
	}
	switch {
	case ed != nil && ed.preview:
		ui.Scroll(c.Key("md-" + file)).Grow(1).Children(func() {
			ui.Column(c).Padding(20, 24).Children(func() { markdown.View(c, ed.state.Buffer.String(), &a.md) })
		})
	case ed != nil:
		view := editor.View(c, ed.state).Grow(1).FillWidth()
		view.OnShortcut(ui.Cmd, ui.KeyS, func() { a.saveFile(spaceID, f, ed) })
	case strings.HasPrefix(f.MimeType, "image/") && f.MimeType != "image/svg+xml":
		bmp := a.fileBitmap(spaceID, f.Path)
		ui.Scroll(c.Key("img-" + file)).Grow(1).Children(func() {
			ui.Column(c).Padding(16).AlignItems(ui.Center).Children(func() {
				if bmp == nil {
					a.spinner(c, 16, p.textTertiary)
					return
				}
				w, h := bmp.Size()
				ui.Image(c, bmp).Width(min(float32(w), 440)).AspectRatio(float32(w) / float32(max(h, 1))).Fit(ui.Contain).Radius(6)
				ui.Text(c, fmt.Sprintf("%d × %d", w, h)).FontSize(11).TextColor(p.textTertiary).Margin(8, 0, 0, 0)
			})
		})
	default:
		ui.Column(c).Grow(1).Center().Gap(10).Children(func() {
			ui.Icon(c, icon("file")).Size(28, 28).TextColor(p.placeholder)
			ui.Text(c, f.MimeType+" · "+byteSize(f.Size)).FontSize(12).TextColor(p.textTertiary)
			if a.button(c, "download", m("file_download"), false).Clicked() {
				a.downloadFile(spaceID, f.Path)
			}
		})
	}
}

// fileBitmap loads a picture of the Space once.
func (a *app) fileBitmap(spaceID, p string) *ui.Bitmap {
	key := "raw:" + spaceID + ":" + p
	if b, ok := a.pages.json[key].(*ui.Bitmap); ok {
		return b
	}
	a.once(key, func(ctx context.Context) {
		b, _, err := a.api.raw(ctx, spaceID, p)
		if err != nil {
			return
		}
		var bmp *ui.Bitmap
		if isWebP(b) {
			bmp, err = decodeWebP(b)
		} else {
			bmp, err = ui.DecodeBitmap(b)
		}
		if err == nil {
			a.update(func() { a.pages.json[key] = bmp })
		}
	})
	return nil
}

func (a *app) saveFile(spaceID string, f *fsFile, ed *fileEdit) {
	if ed.saving || !ed.state.Buffer.Modified() {
		return
	}
	content := ed.state.Buffer.String()
	ed.saving = true
	in := map[string]any{"path": f.Path, "content": content, "encoding": "utf-8"}
	if ed.mtimeMs > 0 {
		in["expected"] = map[string]any{"mtimeMs": ed.mtimeMs, "size": ed.size}
	}
	a.act(func(ctx context.Context) error {
		var out struct {
			Size    int64   `json:"size"`
			MtimeMs float64 `json:"mtimeMs"`
		}
		err := a.api.do(ctx, "PUT", sp(spaceID)+"/fs/file", in, &out)
		a.update(func() {
			ed.saving = false
			if err == nil {
				ed.mtimeMs, ed.size = out.MtimeMs, out.Size
				if ed.state.Buffer.String() == content {
					ed.state.Buffer.MarkSaved()
				}
			}
		})
		return err
	})
}

// downloadFile saves a Space file where the user says.
func (a *app) downloadFile(spaceID, p string) {
	if a.win == nil || a.api == nil {
		return
	}
	go func() {
		dest, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.win, DefaultPath: path.Base(p)})
		if err != nil || dest == "" {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		b, _, err := a.api.raw(ctx, spaceID, p)
		if err == nil {
			err = os.WriteFile(dest, b, 0o644)
		}
		if err != nil {
			a.update(func() { a.toast, a.toastAt, a.toastOK = err.Error(), time.Now(), false })
		}
	}()
}

// fileMenu is a file's or folder's actions: insert, rename, download,
// label, delete. row is true for the tree's right-click menu.
func (a *app) fileMenu(c *ui.Context, spaceID, p string, dir bool) {
	key := "file-menu:" + p
	b := a.iconButton(c.Key("file-more"), "ellipsis", m("file_more"), a.menus[key])
	if b.Clicked() {
		a.menus[key] = !a.menus[key]
	}
	open := a.menus[key]
	a.menuWidth(c, b, &open, 180, a.fileActions(spaceID, p, dir))
	a.menus[key] = open
	if a.labelTarget != nil && a.labelTarget.kind == "file" && a.labelTarget.ref == p {
		a.labelPicker(c, b)
	}
}

func (a *app) fileActions(spaceID, p string, dir bool) []menuItem {
	items := []menuItem{
		{icon: "text-cursor-input", label: m("file_insert"), run: func() { a.insertReference(p) }},
	}
	if a.can(spaceID, "file.edit") {
		items = append(items, menuItem{icon: "pencil", label: m("file_rename"), run: func() {
			a.fsPending = &fsCreate{space: spaceID, dir: path.Dir(p), kind: "rename", from: p, name: path.Base(p)}
		}})
	}
	if !dir {
		items = append(items, menuItem{icon: "download", label: m("file_download"), run: func() { a.downloadFile(spaceID, p) }})
		items = append(items, menuItem{icon: "list-tree", label: m("inline_label_as"), run: func() {
			a.labelTarget = &labelTarget{space: spaceID, kind: "file", ref: p}
		}})
	}
	if a.can(spaceID, "file.edit") {
		items[len(items)-1].divider = true
		items = append(items, menuItem{icon: "trash-2", label: m("file_delete"), danger: true, run: func() { a.deleteNode(spaceID, p, dir) }})
	}
	return items
}

func (a *app) deleteNode(spaceID, p string, dir bool) {
	a.confirm(m("file_delete")+" "+path.Base(p)+"?", p, m("common_delete"), func() {
		q := url.Values{"path": {p}}
		if dir {
			q.Set("recursive", "true")
		}
		a.act(func(ctx context.Context) error {
			err := a.api.do(ctx, "DELETE", sp(spaceID)+"/fs/node?"+q.Encode(), nil, nil)
			if err == nil {
				a.update(func() {
					a.forgetPrefix("tree:"+spaceID, "file:"+spaceID+":"+p)
					if a.route.file() == p {
						a.closeWindow()
					}
				})
			}
			return err
		})
	})
}

// fsPendingRow is the tree's text box for a new file or folder, or a
// rename, in the folder it goes into.
func (a *app) fsPendingRow(c *ui.Context, spaceID, dir string, depth int) {
	f := a.fsPending
	if f == nil || f.space != spaceID || f.dir != dir && !(f.dir == "." && dir == "") {
		return
	}
	p := a.pal
	ui.Row(c.Key("pending")).Gap(6).AlignItems(ui.Center).Height(28).Padding(0, 6, 0, 6+float32(depth)*14).Children(func() {
		ic := map[string]string{"file": "file-plus", "dir": "folder-plus", "rename": "pencil"}[f.kind]
		ui.Icon(c, icon(ic)).Size(13, 13).TextColor(p.textTertiary)
		in := ui.TextInputBase(c.Key("name"), &f.name).FontSize(12).Height(24).Padding(0, 6).Radius(4).Border(1, p.brand).
			Background(p.bgInput).Grow(1).AutoFocus().Label(m("files_new"))
		in.OnShortcut(0, ui.KeyEscape, func() { a.fsPending = nil })
		if in.Submitted() {
			a.commitFsPending()
		}
	})
}

func (a *app) commitFsPending() {
	f := a.fsPending
	a.fsPending = nil
	name := strings.Trim(strings.TrimSpace(f.name), "/")
	if f == nil || name == "" {
		return
	}
	dir := f.dir
	if dir == "." {
		dir = ""
	}
	target := path.Join(dir, name)
	spaceID := f.space
	a.groupsOpen["fs:"+spaceID+":"+dir] = true
	a.act(func(ctx context.Context) error {
		var err error
		switch f.kind {
		case "file":
			err = a.api.do(ctx, "PUT", sp(spaceID)+"/fs/file", map[string]any{"path": target, "content": "", "encoding": "utf-8"}, nil)
		case "dir":
			err = a.api.do(ctx, "POST", sp(spaceID)+"/fs/dir", map[string]any{"path": target}, nil)
		case "rename":
			if target == f.from {
				return nil
			}
			err = a.api.do(ctx, "POST", sp(spaceID)+"/fs/move", map[string]any{"fromPath": f.from, "toPath": target}, nil)
		}
		if err == nil {
			a.update(func() {
				a.forgetPrefix("tree:"+spaceID, "file:"+spaceID+":"+f.from)
				switch {
				case f.kind == "file":
					a.openFile(spaceID, target)
				case f.kind == "rename" && a.route.file() == f.from:
					delete(a.editors, spaceID+":"+f.from)
					a.openFile(spaceID, target)
				}
			})
		}
		return err
	})
}

// uploadFiles asks for files and puts them in a folder of the Space.
func (a *app) uploadFiles(spaceID, dir string) {
	if a.win == nil || a.api == nil {
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Multiple: true, Title: m("files_upload_files")})
		if err != nil || len(paths) == 0 {
			return
		}
		a.update(func() { a.uploadPaths(spaceID, dir, paths) })
	}()
}

// uploadPaths puts local files in a folder of the Space: each goes to
// the upload's presigned URL, then the upload completes into the
// workspace.
func (a *app) uploadPaths(spaceID, dir string, paths []string) {
	a.uploading++
	a.act(func(ctx context.Context) error {
		defer a.update(func() { a.uploading-- })
		type entry struct {
			id, name string
			data     []byte
		}
		var entries []entry
		var plan []map[string]any
		for _, p := range paths {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			e := entry{newID(), filepath.Base(p), b}
			entries = append(entries, e)
			mt := http.DetectContentType(b)
			if i := strings.Index(mt, ";"); i >= 0 {
				mt = mt[:i]
			}
			plan = append(plan, map[string]any{"id": e.id, "name": e.name, "relativePath": e.name, "size": len(b), "mimeType": mt})
		}
		var up struct {
			UploadID string `json:"uploadId"`
			Entries  []struct {
				ID        string            `json:"id"`
				UploadURL string            `json:"uploadUrl"`
				Headers   map[string]string `json:"headers"`
			} `json:"entries"`
		}
		dest := map[string]any{"kind": "workspace", "targetDir": strings.Trim(dir, "/")}
		if err := a.api.do(ctx, "POST", sp(spaceID)+"/fs/uploads", map[string]any{"destination": dest, "entries": plan}, &up); err != nil {
			return err
		}
		var done []map[string]any
		for _, pe := range up.Entries {
			for _, e := range entries {
				if e.id != pe.ID || pe.UploadURL == "" {
					continue
				}
				req, _ := http.NewRequestWithContext(ctx, "PUT", pe.UploadURL, bytes.NewReader(e.data))
				req.ContentLength = int64(len(e.data))
				for k, v := range pe.Headers {
					req.Header.Set(k, v)
				}
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					return err
				}
				res.Body.Close()
				if res.StatusCode/100 != 2 {
					return fmt.Errorf("upload %s: HTTP %d", e.name, res.StatusCode)
				}
				item := map[string]any{"id": e.id}
				if et := res.Header.Get("ETag"); et != "" {
					item["etag"] = et
				}
				done = append(done, item)
			}
		}
		if err := a.api.do(ctx, "POST", sp(spaceID)+"/fs/uploads/"+url.PathEscape(up.UploadID)+"/complete", map[string]any{"entries": done}, nil); err != nil {
			return err
		}
		a.update(func() { a.forgetPrefix("tree:" + spaceID) })
		return nil
	})
}

// openFile shows a Space file in the window, beside the chat or the Space
// the main column shows, as the web's ?window=file:.
func (a *app) openFile(spaceID, p string) {
	r := a.route
	switch {
	case r.space != spaceID:
		r = route{kind: routeSpace, space: spaceID}
	case r.kind == routeSpaceCheckpoint || r.kind == routeSpaceTask || r.kind == routeSpaceCron || r.kind == routeSpaceApp:
		r = route{kind: routeSpace, space: spaceID}
	}
	r.win = "file:" + p
	a.go_(r)
}

// closeWindow takes the window away, keeping the main column.
func (a *app) closeWindow() {
	r := a.route
	switch r.kind {
	case routeSpaceCheckpoint, routeSpaceTask, routeSpaceCron, routeSpaceApp:
		r = route{kind: routeSpace, space: r.space}
	}
	r.win = ""
	a.go_(r)
}

// fileWindowActions are the file window's header buttons: source or
// preview for Markdown, save, copy, the file's menu.
func (a *app) fileWindowActions(c *ui.Context, spaceID, file string) {
	p := a.pal
	f, _, _ := loadJSON[fsFile](a, "file:"+spaceID+":"+file, sp(spaceID)+"/fs/file?path="+url.QueryEscape(file))
	if f == nil {
		return
	}
	var ed *fileEdit
	if f.Kind == "text" && !strings.HasSuffix(strings.ToLower(f.Name), ".board") {
		ed = a.editorFor(spaceID, f)
	}
	if ed != nil && strings.HasSuffix(strings.ToLower(f.Name), ".md") {
		if a.iconButton(c.Key("source"), "code", m("x_source"), !ed.preview).Clicked() {
			ed.preview = false
		}
		if a.iconButton(c.Key("preview"), "eye", m("attach_preview_file"), ed.preview).Clicked() {
			ed.preview = true
		}
	}
	if ed != nil && ed.state.Buffer.Modified() {
		save := ui.ButtonBase(c.Key("save")).Row().Gap(4).AlignItems(ui.Center).Height(24).Padding(0, 8).Radius(5).Background(p.brand).FocusRing(false).Label(m("common_save")).Children(func() {
			if ed.saving {
				a.spinner(c, 12, p.brandFg)
			}
			ui.Text(c, m("common_save")).FontSize(12).FontWeight(500).TextColor(p.brandFg)
		})
		if save.Clicked() {
			a.saveFile(spaceID, f, ed)
		}
	}
	if ed != nil {
		ic := "copy"
		if time.Since(a.copiedAt) < 2*time.Second {
			ic = "check"
		}
		if a.iconButton(c.Key("copy"), ic, m("copy"), false).Clicked() {
			c.WriteClipboard(ed.state.Buffer.String())
			a.copiedAt = time.Now()
		}
	}
	a.fileMenu(c, spaceID, f.Path, false)
}
