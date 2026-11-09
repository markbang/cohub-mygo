package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// mdBlock builds the blocks Cohub styles apart from MyGo's markdown: tables,
// which the web draws with lines under its rows only, headers small,
// uppercase and tertiary (.markdown-content table in app.css).
func (a *app) mdBlock(c *ui.Context, d *parser.Document, i int) bool {
	b := d.Block(i)
	if b.Kind != parser.Table {
		return false
	}
	p := a.pal
	size := float32(14) * 0.93
	if a.md.Theme != nil && a.md.Theme.FontSize > 0 {
		size = a.md.Theme.FontSize * 0.93
	}
	cols := len(b.Align)
	type cell struct {
		spans []ui.Span
		link  string
	}
	var rows [][]cell
	var header []bool
	for r := i + 1; r < b.Next; r = d.Block(r).Next {
		row := d.Block(r)
		var cells []cell
		for k := r + 1; k < row.Next && len(cells) < cols; k = d.Block(k).Next {
			base := ui.Span{Size: size, Color: p.textSecond}
			if row.Header {
				base = ui.Span{Size: size * 0.85, Weight: 500, Color: p.textTertiary, LetterSpacing: size * 0.85 * 0.02}
			}
			spans, link := a.mdInlines(d.Inlines(k), base, row.Header)
			cells = append(cells, cell{spans, link})
		}
		for len(cells) < cols {
			cells = append(cells, cell{})
		}
		rows = append(rows, cells)
		header = append(header, row.Header)
	}
	// Columns as wide as their text, between 9rem and 24rem.
	widths := make([]float32, cols)
	for _, cells := range rows {
		for k, ce := range cells {
			w, _ := c.MeasureText(0, ce.spans...)
			widths[k] = max(widths[k], min(w+24, 384))
		}
	}
	for k := range widths {
		widths[k] = max(widths[k], 144)
	}
	ui.ScrollHorizontal(c.Key(b.Start)).AlignSelf(ui.Stretch).Children(func() {
		ui.Column(c).Shrink(0).Role(ui.RoleTable).Children(func() {
			for r, cells := range rows {
				rowEl := ui.Row(c).Role(ui.RoleRow).AlignItems(ui.Stretch)
				rowEl.Children(func() {
					for k, ce := range cells {
						role := ui.RoleCell
						if header[r] {
							role = ui.RoleColumnHeader
						}
						box := ui.Column(c).Width(widths[k]).Padding(9.6, 12).Role(role).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle)
						if !header[r] && rowEl.Hovered() {
							box.Background(p.hover.Alpha(0.5))
						}
						align := ui.Start
						switch b.Align[k] {
						case parser.AlignCenter:
							align = ui.Center
						case parser.AlignRight:
							align = ui.End
						}
						box.Children(func() {
							ui.RichText(c, ce.spans...).LineHeight(1.5).TextAlign(align).Selectable()
						})
						if ce.link != "" {
							box.Cursor(ui.CursorPointer)
							if box.Clicked() {
								a.openLink(ce.link)
							}
						}
					}
				})
			}
		})
	})
	return true
}

// mdInlines turns a cell's inlines into spans, with the first link's address.
func (a *app) mdInlines(ins []parser.Inline, base ui.Span, upper bool) ([]ui.Span, string) {
	p := a.pal
	var out []ui.Span
	link := ""
	var walk func(from, to int, s ui.Span)
	walk = func(from, to int, s ui.Span) {
		for j := from; j < to; {
			in := ins[j]
			next := in.Next
			if next <= j {
				next = j + 1
			}
			add := func(t string, s ui.Span) {
				if upper {
					t = strings.ToUpper(t)
				}
				s.Text = t
				out = append(out, s)
			}
			switch in.Kind {
			case parser.Text, parser.HTML, parser.Mention, parser.Reference:
				add(in.Text, s)
			case parser.SoftBreak, parser.LineBreak:
				add(" ", s)
			case parser.Code:
				cs := s
				cs.Font, cs.Size, cs.Color = fontMono, s.Size*0.86, p.text
				cs.Background, cs.Radius, cs.PaddingX = p.text.Alpha(0.06), 4, 3
				add(in.Text, cs)
			case parser.Math:
				cs := s
				cs.Font = fontMono
				add(in.Text, cs)
			case parser.Emphasis:
				es := s
				es.Italic = true
				walk(j+1, next, es)
			case parser.Strong:
				ss := s
				ss.Weight, ss.Color = 600, p.text
				walk(j+1, next, ss)
			case parser.Strikethrough:
				ss := s
				ss.Strikethrough = true
				walk(j+1, next, ss)
			case parser.Link, parser.Image:
				if link == "" {
					link = in.Dest
				}
				ls := s
				ls.Color = p.brand
				n := len(out)
				walk(j+1, next, ls)
				if len(out) == n {
					add(in.Dest, ls)
				}
			case parser.FootnoteRef:
				add("["+in.Text+"]", s)
			}
			j = next
		}
	}
	walk(0, len(ins), base)
	return out, link
}
