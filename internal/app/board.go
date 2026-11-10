package app

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Boards, the Space's canvases: a .board file names a board, whose
// document holds the items. This draws them as the web's board renderer
// does (shapes, text, arrows, drawings, frames, pictures, cards), and
// pans with scrolling and zooms with a pinch or Ctrl/Cmd scrolling.

type boardDoc struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Version int    `json:"version"`
	Board   struct {
		Background struct {
			Color   any    `json:"color"`
			Pattern string `json:"pattern"`
		} `json:"background"`
	} `json:"board"`
	Items map[string]*boardItem `json:"items"`
}

type boardVec struct{ X, Y float32 }

type boardItem struct {
	id       string
	Type     string   `json:"type"`
	Z        float64  `json:"z"`
	Position boardVec `json:"position"`
	Rotation float32  `json:"rotation"`
	Opacity  *float32 `json:"opacity"`
	Hidden   bool     `json:"hidden"`
	Size     *struct {
		Width  float32 `json:"width"`
		Height float32 `json:"height"`
	} `json:"size"`
	Style struct {
		Fill        any      `json:"fill"`
		FillOpacity *float32 `json:"fillOpacity"`
		Stroke      any      `json:"stroke"`
		StrokeWidth *float32 `json:"strokeWidth"`
		Dash        string   `json:"dash"`
		Radius      *float32 `json:"radius"`
	} `json:"style"`
	Props map[string]any `json:"props"`
}

type boardView struct {
	x, y, zoom float32
	fitted     bool
}

// boardColor resolves a board color: a palette token, a CSS color, or
// one for each scheme.
func (a *app) boardColor(v any, fallback, part string) ui.Color {
	p := a.pal
	switch c := v.(type) {
	case map[string]any:
		key := "light"
		if p.dark {
			key = "dark"
		}
		v = c[key]
	}
	s, _ := v.(string)
	if s == "" {
		s = fallback
	}
	if s == "neutral" && part != "fill" {
		return p.text
	}
	if col, ok := p.board[s+"-"+part]; ok {
		return col
	}
	if col, ok := parseColor(s, nil); ok {
		return col.color()
	}
	return p.textTertiary
}

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func num(m map[string]any, k string, def float32) float32 {
	if v, ok := m[k].(float64); ok {
		return float32(v)
	}
	return def
}

func vec(v any) (boardVec, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return boardVec{}, false
	}
	return boardVec{num(m, "x", 0), num(m, "y", 0)}, true
}

// bounds is the item's box on the board.
func (it *boardItem) bounds() (x, y, w, h float32) {
	x, y = it.Position.X, it.Position.Y
	if it.Size != nil {
		return x, y, it.Size.Width, it.Size.Height
	}
	switch it.Type {
	case "arrow":
		s, _ := vec(it.Props["start"])
		e, _ := vec(it.Props["end"])
		return x + min(s.X, e.X), y + min(s.Y, e.Y), float32(math.Abs(float64(e.X - s.X))), float32(math.Abs(float64(e.Y - s.Y)))
	case "text":
		size := num(it.Props, "fontSize", 24)
		lines := strings.Count(str(it.Props, "text"), "\n") + 1
		return x, y, num(it.Props, "width", size*float32(len([]rune(str(it.Props, "text"))))*0.6), size * 1.34 * float32(lines)
	}
	return x, y, 160, 100
}

func (a *app) boardWindow(c *ui.Context, spaceID string, f *fsFile) {
	p := a.pal
	var manifest struct {
		BoardID string `json:"boardId"`
	}
	json.Unmarshal([]byte(f.Content), &manifest)
	if manifest.BoardID == "" {
		ui.Text(c, f.Content).FontSize(12).Font(fontMono).Padding(12, 16)
		return
	}
	doc, err, ok := loadJSON[boardDoc](a, "board:"+manifest.BoardID, sp(spaceID)+"/boards/"+url.PathEscape(manifest.BoardID))
	if !ok || err != nil {
		a.loadingBlock(c, err, 200)
		return
	}
	v := a.boardViews[manifest.BoardID]
	if v == nil {
		v = &boardView{zoom: 1}
		a.boardViews[manifest.BoardID] = v
	}
	items := make([]*boardItem, 0, len(doc.Items))
	for id, it := range doc.Items {
		if it != nil && !it.Hidden {
			it.id = id
			items = append(items, it)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Z < items[j].Z })
	bg := p.bgContent
	if c := doc.Board.Background.Color; c != nil {
		bg = a.boardColor(c, "white", "fill")
	}
	fit := func(r ui.Rect) {
		if len(items) == 0 {
			v.x, v.y, v.zoom = r.W/2, r.H/2, 1
			return
		}
		minX, minY, maxX, maxY := float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(-1))
		for _, it := range items {
			x, y, w, h := it.bounds()
			minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x+w), max(maxY, y+h)
		}
		w, h := max(maxX-minX, 1), max(maxY-minY, 1)
		v.zoom = min((r.W-48)/w, (r.H-48)/h, 2)
		v.x = r.W/2 - (minX+w/2)*v.zoom
		v.y = r.H/2 - (minY+h/2)*v.zoom
	}
	canvas := ui.Box(c.Key("board-" + manifest.BoardID)).Grow(1).MinHeight(0).Clip().Background(bg)
	canvas.HandleInput(func(ev ui.InputEvent) bool {
		switch {
		case ev.Kind == ui.InputZoom || (ev.Kind == ui.InputScroll && (ev.Mods&(ui.Ctrl|ui.Cmd)) != 0):
			k := ev.Scale
			if ev.Kind == ui.InputScroll {
				k = float32(math.Exp(float64(-ev.DY) / 300))
			}
			nz := min(max(v.zoom*k, 0.05), 8)
			v.x = ev.X - (ev.X-v.x)*nz/v.zoom
			v.y = ev.Y - (ev.Y-v.y)*nz/v.zoom
			v.zoom = nz
		case ev.Kind == ui.InputScroll:
			v.x -= ev.DX
			v.y -= ev.DY
		default:
			return false
		}
		return true
	})
	canvas.Draw(func(pt *ui.Painter, r ui.Rect) {
		if !v.fitted && r.W > 0 {
			fit(r)
			v.fitted = true
		}
		if doc.Board.Background.Pattern == "dots" || doc.Board.Background.Pattern == "" {
			step := 24 * v.zoom
			for step < 12 {
				step *= 2
			}
			dot := p.border.Alpha(0.6)
			for x := float32(math.Mod(float64(v.x), float64(step))); x < r.W; x += step {
				for y := float32(math.Mod(float64(v.y), float64(step))); y < r.H; y += step {
					pt.Fill(ui.Rect{X: r.X + x - 0.75, Y: r.Y + y - 0.75, W: 1.5, H: 1.5}, dot, 0.75)
				}
			}
		}
		pt.Transform(ui.Scaling(v.zoom, v.zoom).Then(ui.Translation(r.X+v.x, r.Y+v.y)), 0, 0, func() {
			for _, it := range items {
				a.drawBoardItem(pt, it)
			}
		})
	})
	ui.Row(c.Key("board-tools")).Attach(ui.AnchorBottomRight, ui.AnchorBottomRight).Bottom(12).Right(12).Gap(2).Padding(2).Radius(8).
		Background(p.bgElevated).Border(1, p.borderSubtle).Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, 0.08)).Children(func() {
		if a.iconButton(c.Key("out"), "minus", "Zoom out", false).Clicked() {
			v.zoom = max(v.zoom/1.25, 0.05)
		}
		ui.Text(c, fmtPercent(v.zoom)).FontSize(11).Font(fontMono).TextColor(p.textSecond).Width(44).TextAlign(ui.Center).AlignSelf(ui.Center)
		if a.iconButton(c.Key("in"), "plus", "Zoom in", false).Clicked() {
			v.zoom = min(v.zoom*1.25, 8)
		}
		if a.iconButton(c.Key("fit"), "maximize", "Fit", false).Clicked() {
			v.fitted = false
		}
	})
}

func fmtPercent(z float32) string {
	return fmt.Sprintf("%.0f%%", z*100)
}

func (a *app) drawBoardItem(pt *ui.Painter, it *boardItem) {
	p := a.pal
	x, y, w, h := it.bounds()
	alpha := float32(1)
	if it.Opacity != nil {
		alpha = *it.Opacity
	}
	draw := func() {
		switch it.Type {
		case "shape":
			stroke := a.boardColor(firstOf(it.Style.Stroke, it.Style.Fill), "brand", "stroke")
			fill := a.boardColor(firstOf(it.Style.Fill, it.Style.Stroke), "brand", "fill")
			fo := float32(0)
			if it.Style.Fill != nil {
				fo = 1
			}
			if it.Style.FillOpacity != nil {
				fo = *it.Style.FillOpacity
			}
			sw := float32(1.75)
			if it.Style.StrokeWidth != nil {
				sw = *it.Style.StrokeWidth
			}
			path := shapePath(str(it.Props, "geometry"), x, y, w, h, it.Style.Radius)
			if fo > 0 {
				pt.FillPath(path, fill.Alpha(fo*alpha))
			}
			if sw > 0 {
				if it.Style.Dash == "dashed" {
					path.Dash(0, sw*4, sw*3)
				} else if it.Style.Dash == "dotted" {
					path.Dash(0, sw, sw*2)
				}
				pt.StrokePath(path, sw, stroke.Alpha(0.85*alpha))
			}
			if t := str(it.Props, "text"); t != "" {
				size := num(it.Props, "fontSize", 20)
				col := a.boardColor(it.Style.Stroke, "neutral", "label")
				tw, th := pt.MeasureText(w-16, ui.Span{Text: t, Size: size, Font: fontSans})
				pt.RichText(x+(w-tw)/2, y+(h-th)/2, w-16, ui.Span{Text: t, Size: size, Font: fontSans, Color: col.Alpha(alpha)})
			}
		case "text":
			size := num(it.Props, "fontSize", 24)
			col := a.boardColor(it.Style.Fill, "neutral", "label")
			font := fontSans
			switch str(it.Props, "font") {
			case "mono":
				font = fontMono
			case "serif":
				font = "serif"
			}
			width := num(it.Props, "width", 0)
			pt.RichText(x, y, width, ui.Span{Text: str(it.Props, "text"), Size: size, Weight: int(num(it.Props, "fontWeight", 500)), Font: font, Color: col.Alpha(alpha)})
		case "arrow":
			col := a.boardColor(firstOf(it.Style.Stroke, it.Style.Fill), "neutral", "stroke").Alpha(alpha)
			sw := float32(2.5)
			if it.Style.StrokeWidth != nil {
				sw = *it.Style.StrokeWidth
			}
			s, _ := vec(it.Props["start"])
			e, _ := vec(it.Props["end"])
			pts := []boardVec{{it.Position.X + s.X, it.Position.Y + s.Y}}
			if wps, ok := it.Props["waypoints"].([]any); ok {
				for _, wp := range wps {
					if v, ok := vec(wp); ok {
						pts = append(pts, boardVec{it.Position.X + v.X, it.Position.Y + v.Y})
					}
				}
			}
			pts = append(pts, boardVec{it.Position.X + e.X, it.Position.Y + e.Y})
			path := new(ui.Path).MoveTo(pts[0].X, pts[0].Y)
			for _, q := range pts[1:] {
				path.LineTo(q.X, q.Y)
			}
			path.Cap(ui.RoundCap).Join(ui.RoundJoin)
			if it.Style.Dash == "dashed" {
				path.Dash(0, sw*4, sw*3)
			}
			pt.StrokePath(path, sw, col)
			head := func(from, to boardVec) {
				dx, dy := to.X-from.X, to.Y-from.Y
				l := float32(math.Hypot(float64(dx), float64(dy)))
				if l == 0 {
					return
				}
				ux, uy := dx/l, dy/l
				size := 6 + sw*2.5
				hp := new(ui.Path).MoveTo(to.X, to.Y).
					LineTo(to.X-ux*size-uy*size*0.55, to.Y-uy*size+ux*size*0.55).
					LineTo(to.X-ux*size+uy*size*0.55, to.Y-uy*size-ux*size*0.55).Close()
				pt.FillPath(hp, col)
			}
			if v, ok := it.Props["arrowEnd"].(bool); !ok || v {
				head(pts[len(pts)-2], pts[len(pts)-1])
			}
			if v, _ := it.Props["arrowStart"].(bool); v {
				head(pts[1], pts[0])
			}
			if l := str(it.Props, "label"); l != "" {
				mid := pts[len(pts)/2]
				if len(pts) == 2 {
					mid = boardVec{(pts[0].X + pts[1].X) / 2, (pts[0].Y + pts[1].Y) / 2}
				}
				size := num(it.Props, "fontSize", 14)
				tw, th := pt.MeasureText(0, ui.Span{Text: l, Size: size})
				pt.Fill(ui.Rect{X: mid.X - tw/2 - 4, Y: mid.Y - th/2 - 2, W: tw + 8, H: th + 4}, p.bgContent, 4)
				pt.RichText(mid.X-tw/2, mid.Y-th/2, 0, ui.Span{Text: l, Size: size, Color: p.text})
			}
		case "draw":
			col := a.boardColor(firstOf(it.Style.Stroke, it.Style.Fill), "neutral", "stroke").Alpha(alpha)
			sw := float32(4)
			if it.Style.StrokeWidth != nil {
				sw = *it.Style.StrokeWidth
			}
			var pts [][2]float32
			if ps, ok := it.Props["points"].([]any); ok {
				for _, q := range ps {
					if v, ok := vec(q); ok {
						pts = append(pts, [2]float32{it.Position.X + v.X, it.Position.Y + v.Y})
					}
				}
			}
			if len(pts) > 1 {
				pt.StrokePolyline(pts, sw, col)
			}
		case "frame":
			if str(it.Props, "decoration") == "none" {
				return
			}
			pt.Fill(ui.Rect{X: x, Y: y, W: w, H: h}, p.bgPrimary.Alpha(alpha), 6)
			pt.Stroke(ui.Rect{X: x, Y: y, W: w, H: h}, p.border.Alpha(alpha), 6, 1)
			if l := str(it.Props, "label"); l != "" {
				pt.RichText(x, y-18, 0, ui.Span{Text: l, Size: 12, Color: p.textTertiary})
			}
		case "image", "video":
			src := str(it.Props, "src")
			if src == "" {
				src = str(it.Props, "poster")
			}
			var bmp *ui.Bitmap
			if strings.HasPrefix(src, "https://") {
				bmp = a.images.get(imageVariant(src, max(w, h)))
			}
			if bmp != nil {
				pt.Clip(ui.Rect{X: x, Y: y, W: w, H: h}, 6, func() {
					pt.Image(bmp, ui.Rect{X: x, Y: y, W: w, H: h}, ui.Cover)
				})
			} else {
				pt.Fill(ui.Rect{X: x, Y: y, W: w, H: h}, p.bgElevated, 6)
				pt.Stroke(ui.Rect{X: x, Y: y, W: w, H: h}, p.borderSubtle, 6, 1)
			}
		default:
			pt.Fill(ui.Rect{X: x, Y: y, W: w, H: h}, p.bgElevated.Alpha(alpha), 8)
			pt.Stroke(ui.Rect{X: x, Y: y, W: w, H: h}, p.borderSubtle, 8, 1)
			title := str(it.Props, "title")
			if title == "" {
				title = str(it.Props, "name")
			}
			if title == "" {
				title = it.Type
			}
			pt.RichText(x+12, y+10, w-24, ui.Span{Text: title, Size: 14, Weight: 500, Color: p.text})
		}
	}
	if it.Rotation != 0 {
		pt.Transform(ui.Rotation(it.Rotation), x+w/2, y+h/2, draw)
	} else {
		draw()
	}
}

func firstOf(vs ...any) any {
	for _, v := range vs {
		if v != nil {
			return v
		}
	}
	return nil
}

// shapePath is a shape's outline in its box.
func shapePath(geometry string, x, y, w, h float32, radius *float32) *ui.Path {
	p := new(ui.Path)
	switch geometry {
	case "ellipse":
		const k = 0.5523
		cx, cy, rx, ry := x+w/2, y+h/2, w/2, h/2
		p.MoveTo(cx+rx, cy).
			CubeTo(cx+rx, cy+ry*k, cx+rx*k, cy+ry, cx, cy+ry).
			CubeTo(cx-rx*k, cy+ry, cx-rx, cy+ry*k, cx-rx, cy).
			CubeTo(cx-rx, cy-ry*k, cx-rx*k, cy-ry, cx, cy-ry).
			CubeTo(cx+rx*k, cy-ry, cx+rx, cy-ry*k, cx+rx, cy).Close()
	case "diamond":
		p.MoveTo(x+w/2, y).LineTo(x+w, y+h/2).LineTo(x+w/2, y+h).LineTo(x, y+h/2).Close()
	case "triangle":
		p.MoveTo(x+w/2, y).LineTo(x+w, y+h).LineTo(x, y+h).Close()
	default:
		r := float32(0)
		if geometry == "rounded" {
			r = min(w, h) * 0.12
		}
		if radius != nil {
			r = *radius
		}
		r = min(r, w/2, h/2)
		if r <= 0 {
			p.MoveTo(x, y).LineTo(x+w, y).LineTo(x+w, y+h).LineTo(x, y+h).Close()
			break
		}
		p.MoveTo(x+r, y).LineTo(x+w-r, y).QuadTo(x+w, y, x+w, y+r).
			LineTo(x+w, y+h-r).QuadTo(x+w, y+h, x+w-r, y+h).
			LineTo(x+r, y+h).QuadTo(x, y+h, x, y+h-r).
			LineTo(x, y+r).QuadTo(x, y, x+r, y).Close()
	}
	return p
}
