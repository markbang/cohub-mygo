package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	_ "golang.org/x/image/webp"
)

// Composer attachments, as composer-attachments.ts and the chat host's
// send: each file goes once to durable public storage, the server then
// copies those URLs into the sandbox, and the prompt names the sandbox
// paths (or the URLs) with image blocks for pictures.

const (
	maxAttachments  = 14
	maxUploadBytes  = 2 << 30
	maxImageEdge    = 2560
	attachmentImage = "image"
	attachmentFile  = "file"
)

type attachment struct {
	id       string
	name     string
	kind     string // image, file
	mime     string
	data     []byte
	width    int
	height   int
	preview  *ui.Bitmap
	status   string // ready, uploading, failed
	progress float64
}

func newID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// pickAttachments asks the system for files to attach.
func (a *app) pickAttachments() {
	if a.win == nil {
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Multiple: true, Title: m("feedback_add_files")})
		if err != nil || len(paths) == 0 {
			return
		}
		a.update(func() { a.addAttachmentPaths(paths) })
	}()
}

// addAttachmentPaths reads files from disk into the composer.
func (a *app) addAttachmentPaths(paths []string) {
	var notes []string
	for _, p := range paths {
		if len(a.attachments) >= maxAttachments {
			notes = append(notes, m("attach_limit_reached", "max", maxAttachments))
			break
		}
		st, err := os.Stat(p)
		switch {
		case err != nil || st.IsDir():
			notes = append(notes, m("attach_invalid_path"))
			continue
		case st.Size() == 0:
			notes = append(notes, m("attach_rejected_empty_one", "name", filepath.Base(p)))
			continue
		case st.Size() > maxUploadBytes:
			notes = append(notes, m("attach_rejected_too_large_one", "name", filepath.Base(p), "limit", "2 GB"))
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			notes = append(notes, m("attach_read_failed"))
			continue
		}
		a.addAttachmentBytes(filepath.Base(p), b)
	}
	a.sendErr = strings.Join(notes, " ")
}

// addAttachmentBytes adds a file's bytes; pictures become JPEG or stay
// WebP, no larger than 2560 on a side, with a preview.
func (a *app) addAttachmentBytes(name string, b []byte) {
	mt := http.DetectContentType(b)
	if ext := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); ext != "" && !strings.HasPrefix(mt, "image/") {
		mt = ext
	}
	if base, _, err := mime.ParseMediaType(mt); err == nil {
		mt = base
	}
	at := &attachment{id: newID(), name: name, kind: attachmentFile, mime: mt, data: b, status: "ready"}
	if strings.HasPrefix(mt, "image/") && mt != "image/svg+xml" {
		if img, _, err := image.Decode(bytes.NewReader(b)); err == nil {
			at.kind = attachmentImage
			r := img.Bounds()
			at.width, at.height = r.Dx(), r.Dy()
			if mt != "image/jpeg" && mt != "image/webp" || max(r.Dx(), r.Dy()) > maxImageEdge {
				img = shrink(img, maxImageEdge)
				var buf bytes.Buffer
				if jpeg.Encode(&buf, flatten(img), &jpeg.Options{Quality: 88}) == nil {
					at.data, at.mime = buf.Bytes(), "image/jpeg"
					at.name = strings.TrimSuffix(name, filepath.Ext(name)) + ".jpg"
					r = img.Bounds()
					at.width, at.height = r.Dx(), r.Dy()
				}
			}
			at.preview = ui.NewBitmap(shrink(img, 160))
		}
	}
	a.attachments = append(a.attachments, at)
}

// pasteImage attaches the clipboard's picture, when it holds one.
func (a *app) pasteImage() bool {
	png := mygo.Clipboard.ReadImage()
	if len(png) == 0 {
		return false
	}
	a.addAttachmentBytes(fmt.Sprintf("pasted-%s.png", time.Now().Format("20060102-150405")), png)
	return true
}

func shrink(img image.Image, edge int) image.Image {
	r := img.Bounds()
	w, h := r.Dx(), r.Dy()
	if max(w, h) <= edge {
		return img
	}
	scale := float64(edge) / float64(max(w, h))
	nw, nh := max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))
	out := image.NewRGBA(image.Rect(0, 0, nw, nh))
	// Box filter: each output pixel averages the source pixels it covers.
	for y := 0; y < nh; y++ {
		sy0, sy1 := r.Min.Y+y*h/nh, r.Min.Y+(y+1)*h/nh
		for x := 0; x < nw; x++ {
			sx0, sx1 := r.Min.X+x*w/nw, r.Min.X+(x+1)*w/nw
			var cr, cg, cb, ca, n uint64
			for sy := sy0; sy < max(sy1, sy0+1); sy++ {
				for sx := sx0; sx < max(sx1, sx0+1); sx++ {
					pr, pg, pb, pa := img.At(sx, sy).RGBA()
					cr, cg, cb, ca, n = cr+uint64(pr), cg+uint64(pg), cb+uint64(pb), ca+uint64(pa), n+1
				}
			}
			out.Set(x, y, color.RGBA64{uint16(cr / n), uint16(cg / n), uint16(cb / n), uint16(ca / n)})
		}
	}
	return out
}

func flatten(img image.Image) image.Image {
	out := image.NewRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Over)
	return out
}

// attachmentStrip is the row of attachment tiles above the composer's text.
func (a *app) attachmentStrip(c *ui.Context) {
	if len(a.attachments) == 0 {
		return
	}
	p := a.pal
	ui.ScrollHorizontal(c.Key("attachments")).Scrollbars(false).Padding(0, 6).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			for i, at := range a.attachments {
				idx := i
				tile := ui.Box(c.Key(at.id)).Radius(16).Border(1, p.borderSubtle).Background(p.bgContent).Clip()
				if at.kind == attachmentImage {
					tile.Size(80, 80)
				} else {
					tile.Size(160, 80)
				}
				hover := tile.Hovered()
				tile.Children(func() {
					if at.kind == attachmentImage && at.preview != nil {
						ui.Image(c, at.preview).Fill().Fit(ui.Contain)
					} else {
						ui.Column(c).Fill().Padding(8, 12).Gap(4).Children(func() {
							ui.Text(c, at.name).FontSize(12).FontWeight(500).SingleLine()
							state := m("x_ready")
							switch at.status {
							case "uploading":
								state = fmt.Sprintf("Uploading %d%%", int(at.progress*100))
							case "failed":
								state = m("attach_upload_failed")
							}
							ui.Text(c, fmt.Sprintf("File · %d KB · %s", (len(at.data)+1023)/1024, state)).FontSize(11).TextColor(p.textTertiary).SingleLine()
						})
					}
					if at.status == "uploading" {
						ui.Box(c.Key("veil")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(p.bgPrimary.Alpha(0.55)).Center().Children(func() {
							a.spinner(c, 16, p.textSecond)
						})
					}
					if hover && at.status != "uploading" {
						x := ui.ButtonBase(c.Key("remove")).Absolute().Top(4).Right(4).Size(20, 20).Radius(10).Center().
							Background(ui.RGBA(0, 0, 0, 0.6)).Label(m("composer_remove_attachment")).FocusRing(false).Children(func() {
							ui.Icon(c, icon("x")).Size(12, 12).TextColor(ui.RGBA(255, 255, 255, 1))
						})
						if x.Clicked() {
							a.attachments = append(a.attachments[:idx:idx], a.attachments[idx+1:]...)
						}
					}
				})
			}
		})
	})
}

// uploadAttachment puts one attachment on durable public storage and
// returns its public URL.
func (cl *client) uploadAttachment(ctx context.Context, spaceID, sessionID string, at *attachment, progress func(float64)) (string, error) {
	in := map[string]any{
		"purpose": "chat_attachment", "uploadProtocol": "presigned_put_v1", "spaceId": spaceID,
		"file": map[string]any{"size": len(at.data), "mimeType": at.mime, "filename": at.name},
	}
	if sessionID != "" {
		in["sessionId"] = sessionID
	}
	var plan struct {
		Asset struct {
			PublicURL     string            `json:"publicUrl"`
			UploadURL     string            `json:"uploadUrl"`
			UploadHeaders map[string]string `json:"uploadHeaders"`
		} `json:"asset"`
	}
	if err := cl.do(ctx, "POST", "/api/public-assets/uploads", in, &plan); err != nil {
		return "", err
	}
	body := &progressReader{r: bytes.NewReader(at.data), n: len(at.data), on: progress}
	req, err := http.NewRequestWithContext(ctx, "PUT", plan.Asset.UploadURL, body)
	if err != nil {
		return "", err
	}
	req.ContentLength = int64(len(at.data))
	for k, v := range plan.Asset.UploadHeaders {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return "", fmt.Errorf("upload failed: HTTP %d %s", res.StatusCode, strings.TrimSpace(string(detail)))
	}
	return plan.Asset.PublicURL, nil
}

type progressReader struct {
	r    io.Reader
	n    int
	done int
	on   func(float64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	k, err := p.r.Read(b)
	p.done += k
	if p.on != nil && p.n > 0 {
		p.on(float64(p.done) / float64(p.n))
	}
	return k, err
}

// materialize copies uploaded URLs into the sandbox's /tmp/uploads and
// returns their paths there.
func (cl *client) materialize(ctx context.Context, spaceID, sessionID string, ats []*attachment, urls map[string]string) ([]string, error) {
	dest := map[string]any{"kind": "sandbox_tmp"}
	if sessionID != "" {
		dest["sessionId"] = sessionID
	}
	var entries []map[string]any
	seen := map[string]int{}
	for _, at := range ats {
		name := at.name
		if n := seen[name]; n > 0 {
			ext := filepath.Ext(name)
			name = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), n, ext)
		}
		seen[at.name]++
		entries = append(entries, map[string]any{
			"id": at.id, "name": at.name, "relativePath": name, "size": len(at.data), "mimeType": at.mime, "downloadUrl": urls[at.id],
		})
	}
	var plan struct {
		UploadID string `json:"uploadId"`
	}
	if err := cl.do(ctx, "POST", "/api/spaces/"+url.PathEscape(spaceID)+"/fs/uploads", map[string]any{"destination": dest, "entries": entries}, &plan); err != nil {
		return nil, err
	}
	var ids []map[string]string
	for _, at := range ats {
		ids = append(ids, map[string]string{"id": at.id})
	}
	var done struct {
		Uploaded []struct {
			Path string `json:"path"`
		} `json:"uploaded"`
	}
	if err := cl.do(ctx, "POST", "/api/spaces/"+url.PathEscape(spaceID)+"/fs/uploads/"+url.PathEscape(plan.UploadID)+"/complete", map[string]any{"entries": ids}, &done); err != nil {
		return nil, err
	}
	var paths []string
	for _, u := range done.Uploaded {
		paths = append(paths, u.Path)
	}
	return paths, nil
}

// promptContent is the prompt's blocks: the text with the attachments'
// references, then a block for each picture.
func promptContent(text string, ats []*attachment, urls map[string]string, sandbox []string) []map[string]any {
	clean := func(s string) string { return strings.NewReplacer("\r", "_", "\n", "_", "`", "_").Replace(s) }
	var refs []string
	if len(sandbox) > 0 {
		lines := []string{"Files:"}
		for _, p := range sandbox {
			lines = append(lines, "- `"+clean(p)+"`")
		}
		refs = append(refs, strings.Join(lines, "\n"))
	} else {
		var files, images []string
		for _, at := range ats {
			if at.kind == attachmentImage {
				images = append(images, "- "+clean(urls[at.id]))
			} else {
				files = append(files, "- `"+clean(urls[at.id])+"`")
			}
		}
		if len(files) > 0 {
			refs = append(refs, "Files:\n"+strings.Join(files, "\n"))
		}
		if len(images) > 0 {
			refs = append(refs, "Images:\n"+strings.Join(images, "\n"))
		}
	}
	full := strings.Join(append([]string{text}, refs...), "\n\n")
	full = strings.TrimSpace(full)
	var out []map[string]any
	if full != "" {
		out = append(out, map[string]any{"type": "text", "text": full})
	}
	for _, at := range ats {
		if at.kind != attachmentImage {
			continue
		}
		out = append(out, map[string]any{
			"type":   "image",
			"source": map[string]any{"type": "url", "url": urls[at.id]},
			"_meta":  map[string]any{"filename": at.name, "mediaType": at.mime, "size": len(at.data), "width": at.width, "height": at.height},
		})
	}
	return out
}

// viewport is the window the prompt carries as context, as the web's
// viewport block: the open file, unless the user took it out.
func (a *app) viewport() map[string]any {
	f := a.route.file()
	if f == "" || a.viewportOff[f] || !a.hasWindow() {
		return nil
	}
	clean := strings.NewReplacer("\r", "_", "\n", "_", "`", "_").Replace(f)
	return map[string]any{
		"type":  "text",
		"text":  "Viewport:\n- file: `" + clean + "`",
		"_meta": map[string]any{"attachmentKind": "viewport", "viewports": []map[string]any{{"kind": "file", "path": f}}},
	}
}

// viewportChips are ViewportContextBlocks: the window's file over the text.
func (a *app) viewportChips(c *ui.Context) {
	f := a.route.file()
	if f == "" || a.viewportOff[f] || !a.hasWindow() {
		return
	}
	p := a.pal
	ui.Row(c.Key("viewport")).Padding(0, 6).Children(func() {
		chip := ui.Row(c.Key(f)).Height(24).AlignItems(ui.Center).Radius(6).Border(1, p.borderSubtle.Alpha(0.7)).Background(p.bgContent.Alpha(0.55))
		hover := chip.Hovered()
		chip.Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(0, 8).Children(func() {
				ui.Icon(c, icon("file")).Size(12, 12).TextColor(p.textTertiary)
				ui.Text(c, filepath.Base(f)).FontSize(11).FontWeight(500).TextColor(p.textSecond).SingleLine().MaxWidth(240)
			})
			x := ui.ButtonBase(c.Key("x")).Size(24, 24).Center().Label(m("vctx_remove")).FocusRing(false).Children(func() {
				ui.Icon(c, icon("x")).Size(12, 12).TextColor(p.textTertiary)
			})
			if !hover {
				x.Opacity(0.45)
			}
			if x.Clicked() {
				a.viewportOff[f] = true
			}
		})
	})
}
