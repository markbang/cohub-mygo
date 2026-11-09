package main

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
)

// relTime formats a time as the web's lists do: the time today, the date
// and time this year, the date before.
func relTime(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	t = t.Local()
	now = now.Local()
	switch {
	case t.Year() == now.Year() && t.YearDay() == now.YearDay():
		return t.Format("15:04")
	case t.Year() == now.Year():
		return t.Format("01-02 15:04")
	}
	return t.Format("2006.01.02")
}

func elapsed(since, now time.Time) string {
	d := now.Sub(since)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d秒", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d分", int(d.Minutes()))
	}
	return fmt.Sprintf("%d时%d分", int(d.Hours()), int(d.Minutes())%60)
}

func kilo(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

// oneLine collapses whitespace, for previews.
func oneLine(s string) string {
	return strings.Join(strings.FieldsFunc(s, unicode.IsSpace), " ")
}

var (
	mdLink   = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	mdMarks  = regexp.MustCompile("\\*\\*|__|`+|~~")
	mdPrefix = regexp.MustCompile(`(?m)^\s{0,3}(#{1,6}\s+|>\s?|[-*+]\s+|\d+\.\s+)`)
)

// preview is a reply as the lists show it: one line of its text, without
// the Markdown marks.
func preview(s string) string {
	s = mdLink.ReplaceAllString(s, "$1")
	s = mdPrefix.ReplaceAllString(s, "")
	s = mdMarks.ReplaceAllString(s, "")
	return oneLine(s)
}

func initials(name string) string {
	var out []rune
	for _, w := range strings.Fields(name) {
		r := []rune(w)
		out = append(out, unicode.ToUpper(r[0]))
		if len(out) == 2 {
			break
		}
	}
	if len(out) == 1 {
		if r := []rune(strings.Fields(name)[0]); len(r) > 1 {
			out = append(out, unicode.ToUpper(r[1]))
		}
	}
	if len(out) == 0 {
		return "?"
	}
	return string(out)
}

func hue(s string) float32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return float32(h.Sum32() % 360)
}

// avatar draws a picture from url, or initials on a tint of the name's hue.
func (a *app) avatar(c *ui.Context, url, name string, size, radius float32) ui.Element {
	if bmp := a.images.get(url); bmp != nil {
		return ui.Image(c, bmp).Size(size, size).Fit(ui.Cover).Radius(radius).Border(1, a.pal.borderSubtle.Alpha(0.6))
	}
	h := hue(name)
	bg, fg := ui.Oklch(0.94, 0.035, h), ui.Oklch(0.48, 0.11, h)
	if a.pal.dark {
		bg, fg = ui.Oklch(0.32, 0.05, h), ui.Oklch(0.82, 0.09, h)
	}
	return ui.Box(c).Size(size, size).Radius(radius).Background(bg).Center().Children(func() {
		ui.Text(c, initials(name)).FontSize(size * 0.36).FontWeight(600).TextColor(fg).Font(fontMono).SingleLine()
	})
}

// iconButton is a square, quiet button showing an icon, as the web's
// header and toolbar buttons.
func (a *app) iconButton(c *ui.Context, name, label string, active bool) ui.Element {
	p := a.pal
	b := ui.ButtonBase(c).Size(28, 28).Radius(6).Center().Label(label).Tooltip(label).FocusRing(false).Children(func() {
		col := p.textTertiary
		if active {
			col = p.text
		}
		ui.Icon(c, icon(name)).Size(16, 16).TextColor(col)
	})
	switch {
	case active:
		b.Background(p.hoverStrong)
	case b.Pressed():
		b.Background(p.active)
	case b.Hovered():
		b.Background(p.hover)
	}
	return b
}

// pressable is a row or box of the look of the web's list rows: a hover
// tint, a stronger one while chosen.
func (a *app) pressable(e ui.Element, selected bool) ui.Element {
	p := a.pal
	e.Cursor(ui.CursorPointer)
	switch {
	case selected:
		e.Background(p.active)
	case e.Pressed():
		e.Background(p.hoverStrong)
	case e.Hovered():
		e.Background(p.hover)
	}
	return e
}

func (a *app) dot(c *ui.Context, col ui.Color, size float32) ui.Element {
	return ui.Box(c).Size(size, size).Radius(size / 2).Background(col).Shrink(0)
}

// chip is a rounded control of the composer: an optional avatar, a label
// and a chevron.
func (a *app) pill(c *ui.Context, label string, lead func()) ui.Element {
	p := a.pal
	b := ui.ButtonBase(c).Row().Gap(5).AlignItems(ui.Center).Padding(4, 8, 4, 8).Radius(999).Border(1, p.borderSubtle).FocusRing(false).Children(func() {
		if lead != nil {
			lead()
		}
		ui.Text(c, label).FontSize(12).TextColor(p.textSecond).SingleLine()
		ui.Icon(c, icon("chevron-down")).Size(12, 12).TextColor(p.placeholder)
	})
	if b.Hovered() {
		b.Background(p.hover)
	}
	return b
}

func (a *app) spinner(c *ui.Context, size float32, col ui.Color) ui.Element {
	s := ui.Icon(c, icon("loader-circle")).Size(size, size).TextColor(col)
	s.Rotate(s.Loop("spin", time.Second, ui.Linear) * 360)
	return s
}

// logo is Cohub's mark: a red tile with a C.
func (a *app) logo(c *ui.Context, size float32) ui.Element {
	return ui.Box(c).Size(size, size).Radius(size * 0.22).Background(a.pal.brand).Center().Shrink(0).Children(func() {
		ui.Text(c, "C").FontSize(size * 0.5).FontWeight(600).TextColor(a.pal.brandFg)
	})
}
