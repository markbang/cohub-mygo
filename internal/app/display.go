package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// A display of the Space's runtime, as DisplayWindow: the web streams it
// over WebRTC; a pure Go client takes its frames from /capture, about
// twice a second while the window shows, and sends what is pressed and
// typed over it to /input, as the display's own pointer and keys.

type displayInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Input  bool   `json:"input"`
}

type displayView struct {
	space, id string
	frame     *ui.Bitmap
	w, h      int
	at        time.Time
	fetching  bool
	problem   string // offline, unavailable, busy, outdated, failed
	sentErr   bool
}

func (r route) display() string {
	if d, ok := strings.CutPrefix(r.win, "display:"); ok {
		return d
	}
	return ""
}

func (a *app) displays(spaceID string) ([]displayInfo, string) {
	res, _, _ := loadJSON[struct {
		Displays []displayInfo `json:"displays"`
		Virtual  string        `json:"virtual"`
	}](a, "displays:"+spaceID, sp(spaceID)+"/displays")
	if res == nil {
		return nil, ""
	}
	return res.Displays, res.Virtual
}

func (a *app) openDisplay(spaceID, id string) {
	r := route{kind: routeSpace, space: spaceID, win: "display:" + id}
	if a.route.space == spaceID && a.route.session != "" {
		r.kind, r.session = routeSpaceSession, a.route.session
	}
	a.go_(r)
}

func displayProblem(err error) string {
	s := err.Error()
	for _, k := range []struct{ code, problem string }{
		{"sandbox_offline", "offline"}, {"display_unsupported", "outdated"}, {"display_busy", "busy"},
		{"display_unavailable", "unavailable"}, {"display_not_found", "unavailable"},
	} {
		if strings.Contains(s, k.code) {
			return k.problem
		}
	}
	return "failed"
}

// capture takes the next frame, while none is on its way.
func (a *app) captureDisplay(v *displayView) {
	if v.fetching || a.api == nil {
		return
	}
	v.fetching = true
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var out struct {
			MimeType string `json:"mimeType"`
			Data     string `json:"data"`
			Width    int    `json:"width"`
			Height   int    `json:"height"`
		}
		err := a.api.Do(ctx, "GET", sp(v.space)+"/displays/"+url.PathEscape(v.id)+"/capture?format=jpeg&quality=70&maxSize=1600", nil, &out)
		var bm *ui.Bitmap
		if err == nil {
			b, derr := base64.StdEncoding.DecodeString(out.Data)
			if derr == nil {
				if img, _, ierr := image.Decode(bytes.NewReader(b)); ierr == nil {
					bm = ui.NewBitmap(img)
				}
			}
		}
		a.update(func() {
			v.fetching, v.at = false, time.Now()
			if err != nil {
				v.problem = displayProblem(err)
				return
			}
			v.problem = ""
			if bm != nil {
				v.frame, v.w, v.h = bm, out.Width, out.Height
			}
		})
	})
}

func (a *app) displayInput(v *displayView, events ...map[string]any) {
	if a.api == nil {
		return
	}
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := a.api.Do(ctx, "POST", sp(v.space)+"/displays/"+url.PathEscape(v.id)+"/input", map[string]any{"events": events}, nil)
		a.update(func() {
			v.sentErr = err != nil
			// The frame after the input shows what it did.
			v.at = time.Time{}
		})
	})
}

var displayKeys = map[ui.Key]string{
	ui.KeyEnter: "Enter", ui.KeyBackspace: "Backspace", ui.KeyDelete: "Delete", ui.KeyEscape: "Escape", ui.KeyTab: "Tab",
	ui.KeyUp: "ArrowUp", ui.KeyDown: "ArrowDown", ui.KeyLeft: "ArrowLeft", ui.KeyRight: "ArrowRight",
	ui.KeyHome: "Home", ui.KeyEnd: "End", ui.KeyPageUp: "PageUp", ui.KeyPageDown: "PageDown",
}

func (a *app) displayView(c *ui.Context, spaceID, id string) {
	p := a.pal
	v := a.displayState
	if v == nil || v.space != spaceID || v.id != id {
		v = &displayView{space: spaceID, id: id}
		a.displayState = v
	}
	if time.Since(v.at) > 500*time.Millisecond {
		a.captureDisplay(v)
	}
	c.After(600 * time.Millisecond)
	if v.frame == nil {
		ui.Column(c.Key("display-state")).Grow(1).Center().Padding(24).Background(p.bgContent).Children(func() {
			ui.Column(c).MaxWidth(360).Gap(6).AlignItems(ui.Center).Children(func() {
				if v.problem == "" {
					a.spinner(c, 16, p.textTertiary)
					ui.Text(c, m("display_connecting")).FontSize(12).TextColor(p.textTertiary)
					return
				}
				ui.Icon(c, icon("monitor")).Size(20, 20).TextColor(p.textTertiary)
				title := "display_failed_title"
				if v.problem == "offline" || v.problem == "unavailable" {
					title = "display_" + v.problem + "_title"
				}
				ui.Text(c, m(title)).FontSize(14).FontWeight(500)
				ui.Text(c, m("display_"+v.problem+"_hint")).FontSize(12).LineHeight(1.6).TextColor(p.textTertiary).TextAlign(ui.Center)
				if a.button(c, "", m("display_retry"), false).Clicked() {
					v.problem, v.at = "", time.Time{}
				}
			})
		})
		return
	}
	stage := ui.Box(c.Key("display")).Grow(1).MinHeight(0).Background(ui.RGB(0, 0, 0)).Focusable().Label(m("display_live"))
	b := stage.LayoutBounds()
	// The frame contained in the stage, as containedRect.
	fit := func() ui.Rect {
		if v.w <= 0 || v.h <= 0 || b.W <= 0 || b.H <= 0 {
			return b
		}
		s := min(b.W/float32(v.w), b.H/float32(v.h))
		w, h := float32(v.w)*s, float32(v.h)*s
		return ui.Rect{X: b.X + (b.W-w)/2, Y: b.Y + (b.H-h)/2, W: w, H: h}
	}
	point := func() (float32, float32, bool) {
		x, y, ok := stage.PointerPosition()
		if !ok {
			return 0, 0, false
		}
		r := fit()
		px, py := (x+b.X-r.X)/r.W, (y+b.Y-r.Y)/r.H
		if px < 0 || px > 1 || py < 0 || py > 1 {
			return 0, 0, false
		}
		return px, py, true
	}
	stage.Children(func() {
		ui.Image(c, v.frame).FillWidth().HeightPercent(100).Fit(ui.Contain)
	})
	if stage.Clicked() {
		if x, y, ok := point(); ok {
			button := "primary"
			if bt, ok := stage.PressedButton(); ok && bt == 2 {
				button = "secondary"
			}
			a.displayInput(v,
				map[string]any{"type": "pointer", "action": "down", "x": x, "y": y, "button": button},
				map[string]any{"type": "pointer", "action": "up", "x": x, "y": y, "button": button})
		}
	}
	// What is typed goes to the display from the bar under it: text as
	// it is entered, and the editing keys as they are pressed.
	ui.Row(c.Key("keys")).Height(40).Gap(6).Padding(0, 8).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Background(p.bgContent).Children(func() {
		ui.Icon(c, icon("keyboard")).Size(14, 14).TextColor(p.textTertiary)
		in := ui.TextInputBase(c.Key("type"), &a.displayTyped).Placeholder(m("x_display_type")).PlaceholderColor(p.placeholder).
			FontSize(13).Height(28).Padding(0, 8).Radius(6).Border(1, p.borderSubtle).Grow(1).Label(m("x_display_type"))
		in.OnTextKey(func(k ui.TextKey) bool {
			name, named := displayKeys[k.Key]
			if !named || k.Mods != 0 {
				return false
			}
			if t := k.Text(); t != "" {
				a.displayInput(v, map[string]any{"type": "text", "text": t})
				k.SetText("")
				a.displayTyped = ""
				if k.Key == ui.KeyEnter {
					a.displayInput(v, map[string]any{"type": "key", "action": "press", "key": "Enter"})
				}
				return true
			}
			a.displayInput(v, map[string]any{"type": "key", "action": "press", "key": name})
			return true
		})
		for _, sc := range []struct {
			glyph string
			dy    float32
		}{{"chevron-up", -3}, {"chevron-down", 3}} {
			sc := sc
			if a.iconButton(c.Key(sc.glyph), sc.glyph, m("x_display_scroll"), false).Clicked() {
				a.displayInput(v, map[string]any{"type": "scroll", "x": 0.5, "y": 0.5, "dy": sc.dy})
			}
		}
	})
	if v.sentErr {
		ui.Text(c.Key("err"), m("display_action_failed")).FontSize(12).TextColor(p.errorC).Padding(6, 12)
	}
}
