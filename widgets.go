package main

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/egoist/mygo"
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

// dateTime is the palette's time: the time today, the date before.
func dateTime(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	t, now = t.Local(), now.Local()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	return t.Format("2006.01.02")
}

// units are the duration units of the window's language.
func units() (ms, s, mi, h string) {
	if locale == "zh-CN" {
		return "毫秒", "秒", "分", "时"
	}
	return "ms", "s", "m", "h"
}

func elapsed(since, now time.Time) string {
	_, s, mi, h := units()
	d := now.Sub(since)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d%s", int(d.Seconds()), s)
	case d < time.Hour:
		return fmt.Sprintf("%d%s", int(d.Minutes()), mi)
	}
	return fmt.Sprintf("%d%s %d%s", int(d.Hours()), h, int(d.Minutes())%60, mi)
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

// initials is avatar-initials.ts: two letters for Latin-like names (from
// two words, else the first word), one glyph for CJK or emoji.
func initials(name string) string {
	var words [][]rune
	var cur []rune
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || wide(r) {
			cur = append(cur, r)
		} else if len(cur) > 0 {
			words, cur = append(words, cur), nil
		}
	}
	if len(cur) > 0 {
		words = append(words, cur)
	}
	if len(words) == 0 {
		return "SP"
	}
	first := words[0][0]
	if wide(first) {
		return string(first)
	}
	var second rune
	if len(words) > 1 && !wide(words[1][0]) {
		second = words[1][0]
	} else if len(words[0]) > 1 && !wide(words[0][1]) {
		second = words[0][1]
	}
	if second == 0 {
		return strings.ToUpper(string(first))
	}
	return strings.ToUpper(string([]rune{first, second}))
}

func wide(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul, unicode.Bopomofo) || r >= 0x1F300
}

func hue(s string) float32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return float32(h.Sum32() % 360)
}

// avatar draws a picture from url, or initials on a tint of the name's hue.
func (a *app) avatar(c *ui.Context, url, name string, size, radius float32) ui.Element {
	return a.avatarSeed(c, url, name, name, size, radius)
}

// avatarSeed is Avatar.svelte: the image, or a tile toned by the seed (a
// Space's id) with the name's monogram. A radius under half the size asks
// for a tile, which takes the web's radius for that size.
func (a *app) avatarSeed(c *ui.Context, url, name, seed string, size, radius float32) ui.Element {
	p := a.pal
	if radius*2 < size {
		radius = tileRadius(size)
	}
	if bmp := a.images.get(url); bmp != nil {
		return ui.Image(c, bmp).Size(size, size).Fit(ui.Cover).Radius(radius).Border(1, p.borderSubtle)
	}
	mark := initials(name)
	tone := p.identity[identityTone(seed)]
	pair, single := monoSizes(size)
	fs, fw := pair, 600
	if len([]rune(mark)) == 1 {
		fs, fw = single, 500
	}
	return ui.Box(c).Size(size, size).Radius(radius).Background(p.bgPrimary.MixOklab(tone, 0.16)).
		Border(1, tone.Alpha(0.24)).Center().Shrink(0).Children(func() {
		ui.Text(c, mark).FontSize(fs).FontWeight(fw).TextColor(tone).SingleLine()
	})
}

// identityTone is avatar-identity.ts: FNV-1a over the seed's UTF-16 units,
// one of eight tones.
func identityTone(seed string) int {
	seed = strings.TrimSpace(seed)
	h := uint32(0x811c9dc5)
	for _, u := range utf16.Encode([]rune(seed)) {
		h ^= uint32(u)
		h *= 0x01000193
	}
	return int(h % 8)
}

// tileRadius and monoSizes are Avatar.svelte's SIZES.
func tileRadius(size float32) float32 {
	switch {
	case size <= 16:
		return 5
	case size <= 20:
		return 6
	case size <= 28:
		return 8
	case size <= 40:
		return 10
	}
	return 14
}

func monoSizes(size float32) (pair, single float32) {
	switch {
	case size <= 16:
		return 7, 9
	case size <= 20:
		return 8, 11
	case size <= 28:
		return 11, 14
	case size <= 40:
		return 13, 18
	}
	return 17, 24
}

// iconButton is a square, quiet button showing an icon, as the web's
// header and toolbar buttons.
func (a *app) iconButton(c *ui.Context, name, label string, active bool) ui.Element {
	p := a.pal
	size, radius, glyph := float32(28), float32(6), float32(16)
	if a.compact && a.inHeader {
		size, radius, glyph = 40, 8, 20
	}
	b := ui.ButtonBase(c).Size(size, size).Radius(radius).Center().Label(label).Tooltip(label).FocusRing(false).Children(func() {
		col := p.textTertiary
		if active {
			col = p.text
		}
		ui.Icon(c, icon(name)).Size(glyph, glyph).TextColor(col)
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

// pill is a rounded control of the composer: an optional lead, a label,
// a quieter second label and a chevron.
func (a *app) pill(c *ui.Context, label, sub string, lead func()) ui.Element {
	p := a.pal
	b := ui.ButtonBase(c).Row().Gap(5).AlignItems(ui.Center).Height(28).Padding(0, 8, 0, 9).Radius(999).Border(1, p.borderSubtle).Label(label).FocusRing(false).Children(func() {
		if lead != nil {
			lead()
		}
		ui.Text(c, label).FontSize(12).TextColor(p.textSecond).SingleLine()
		if sub != "" {
			ui.Text(c, sub).FontSize(11).TextColor(p.placeholder).SingleLine()
		}
		ui.Icon(c, icon("chevron-down")).Size(12, 12).TextColor(p.placeholder)
	})
	if b.Hovered() {
		b.Background(p.hover)
	}
	return b
}

type menuItem struct {
	icon, label, hint string
	checked           bool
	danger            bool
	divider           bool // a line below the item
	run               func()
}

// menu is the web's .menu-surface below (or above) anchor: rows of an
// icon and a label.
func (a *app) menu(c *ui.Context, anchor ui.Element, open *bool, items []menuItem) {
	a.menuWidth(c, anchor, open, 0, items)
}

func (a *app) menuWidth(c *ui.Context, anchor ui.Element, open *bool, width float32, items []menuItem) {
	p := a.pal
	ui.PopoverBase(c, anchor, open, func(panel ui.Element) {
		panel.MinWidth(180).Padding(4, 0).Radius(8).Background(p.bgElevated).Border(1, p.borderSubtle).
			Shadow(0, 10, 24, 0, ui.RGBA(0, 0, 0, 0.12)).Margin(4, 0, 4, 0)
		if width > 0 {
			panel.Width(width)
		}
		for _, it := range items {
			if it.label == "" {
				ui.Box(c.Key(fmt_("div", len(it.hint)))).Height(1).Background(p.borderSubtle).Margin(4, 0)
				continue
			}
			row := ui.Row(c.Key(it.label)).Gap(8).AlignItems(ui.Center).Height(30).Padding(0, 10).Children(func() {
				col := p.textTertiary
				if it.danger {
					col = p.errorC
				}
				if it.icon != "" {
					ui.Icon(c, icon(it.icon)).Size(14, 14).TextColor(col)
				}
				ui.Text(c, it.label).FontSize(12).TextColor(col).Grow(1).SingleLine()
				if it.hint != "" {
					ui.Text(c, it.hint).FontSize(11).Font(fontMono).TextColor(p.textSecond)
				}
				if it.checked {
					ui.Icon(c, icon("check")).Size(14, 14).TextColor(p.brand)
				}
			})
			if it.divider {
				row.BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle)
			}
			if row.Hovered() {
				row.Background(p.hover)
			}
			// Clicked, not OnClick: OnClick runs after the build, when the
			// caller has already stored *open.
			row.Cursor(ui.CursorPointer)
			if row.Clicked() {
				*open = false
				if it.run != nil {
					it.run()
				}
			}
		}
	})
}

func fmt_(a string, n int) string { return fmt.Sprint(a, n) }

// clock is a message's time: the time today, the day and time before.
func clock(t, now time.Time) string { return relTime(t, now) }

// duration is formatDurationMs.
func duration(ms int64) string {
	ums, us, umi, uh := units()
	if ms < 1000 {
		return fmt.Sprintf("%d%s", max(1, ms), ums)
	}
	if ms < 10_000 {
		return fmt.Sprintf("%.1f%s", float64(ms)/1000, us)
	}
	sec := (ms + 500) / 1000
	if sec < 60 {
		return fmt.Sprintf("%d%s", sec, us)
	}
	mn, sec := sec/60, sec%60
	if mn < 60 {
		if sec > 0 {
			return fmt.Sprintf("%d%s %d%s", mn, umi, sec, us)
		}
		return fmt.Sprintf("%d%s", mn, umi)
	}
	h, mn := mn/60, mn%60
	if mn > 0 {
		return fmt.Sprintf("%d%s %d%s", h, uh, mn, umi)
	}
	return fmt.Sprintf("%d%s", h, uh)
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

// adaptivePopover is AdaptivePopover.svelte above lg: a menu surface 6px
// from the anchor, padded 4px, scrolling past 420px.
func (a *app) adaptivePopover(c *ui.Context, anchor ui.Element, open *bool, width float32, build func()) {
	p := a.pal
	ui.PopoverBase(c, anchor, open, func(panel ui.Element) {
		panel.Width(width).Radius(8).Background(p.bgElevated).Border(1, p.borderSubtle).
			Shadow(0, 10, 24, 0, ui.RGBA(0, 0, 0, 0.12)).Margin(6, 0, 6, 0)
		ui.Scroll(c.Key("pop-scroll")).MaxHeight(420).Padding(4).Children(build)
	})
}

// openURL opens a link in the browser from outside a build.
func (a *app) openURL(u string) {
	if a.win != nil {
		go mygo.Shell.OpenExternal(u)
	}
}
