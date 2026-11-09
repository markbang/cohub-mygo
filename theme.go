package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

// palette is a theme's design tokens, as the web's CSS variables
// (themes_gen.go has the built-in ones).
type palette struct {
	name string
	dark bool
	font string

	bgPrimary      ui.Color // sidebar, panels
	bgContent      ui.Color // main column
	bgSurface      ui.Color
	bgSurfaceHover ui.Color
	bgElevated     ui.Color
	bgInput        ui.Color
	bgCode         ui.Color
	sidebarBg      ui.Color
	chatBg         ui.Color

	text         ui.Color
	textReading  ui.Color
	textSecond   ui.Color
	textTertiary ui.Color
	placeholder  ui.Color
	disabled     ui.Color

	border       ui.Color
	borderSubtle ui.Color
	borderStrong ui.Color

	hover       ui.Color
	hoverStrong ui.Color
	active      ui.Color

	brand       ui.Color
	brandHover  ui.Color
	brandMuted  ui.Color
	brandBorder ui.Color
	brandFg     ui.Color
	brand900    ui.Color
	brandBg     ui.Color // a chosen option's background
	userBubble  ui.Color
	selection   ui.Color

	success ui.Color
	errorC  ui.Color
	warning ui.Color
	info    ui.Color

	popBg   ui.Color // the palette's panel
	popHead ui.Color
}

func lightPalette() *palette { return themePalette("light") }
func darkPalette() *palette  { return themePalette("dark") }

// themePalette returns a built-in theme with what it leaves out derived.
func themePalette(name string) *palette {
	mk, ok := builtinThemes[name]
	if !ok {
		mk = builtinThemes["light"]
	}
	p := mk()
	p.font = strings.NewReplacer("Geist Variable", "Geist", "Figtree Variable", "Figtree").Replace(p.font)
	if p.dark {
		p.font += ", Noto Sans CJK SC"
	}
	if p.sidebarBg == (ui.Color{}) {
		p.sidebarBg = p.bgPrimary
	}
	if p.chatBg == (ui.Color{}) {
		p.chatBg = p.bgContent
	}
	solid := p.bgSurface.Over(p.bgPrimary)
	p.popBg = solid.MixOklab(p.text, 0.06)
	p.popHead = solid.MixOklab(p.text, 0.035)
	return p
}

var fontSans = "Geist, PingFang SC, Noto Sans CJK SC, Microsoft YaHei, sans-serif"

const fontMono = "Geist Mono, SF Mono, Noto Sans Mono CJK SC, monospace"

// apply makes the window's widgets follow the palette.
func (p *palette) apply(c *ui.Context) {
	t := *c.Theme()
	t.Background = p.chatBg.Over(p.bgPrimary)
	t.Surface, t.SurfaceHover, t.SurfacePressed = p.bgSurface, p.bgSurfaceHover, p.bgElevated
	t.Border = p.border
	t.Text, t.TextMuted = p.text, p.textTertiary
	t.Accent, t.AccentHover, t.AccentPressed, t.AccentText = p.brand, p.brandHover, p.brandHover, p.brandFg
	t.InactiveAccent = p.brand
	t.Danger, t.Success, t.Warning = p.errorC, p.success, p.warning
	t.Selection = p.selection
	t.Focus = p.brand.Alpha(0.5)
	t.Scrollbar = p.disabled
	t.Radius = 6
	t.FontSize = 14
	t.Font = p.font
	t.Dark = p.dark
	t.Inverse, t.InverseText = p.text, p.bgPrimary
	c.SetTheme(&t)
}

// overlay is the scrim under drawers and sheets.
func (p *palette) overlay() ui.Color { return ui.RGBA(0, 0, 0, 0.5) }

// metaText is the meta bars' text: --text-placeholder at half.
func (p *palette) metaText() ui.Color { return p.placeholder.Alpha(0.55) }

// applyTheme picks the palette of the appearance and gives it to the window.
func (a *app) applyTheme(c *ui.Context) {
	name := a.theme
	if _, ok := builtinThemes[name]; !ok {
		name = "light"
		if c.Theme().Dark {
			name = "dark"
		}
	}
	if name != a.pal.name {
		a.pal = themePalette(name)
		fontSans = a.pal.font
	}
	a.pal.apply(c)
	a.md.Theme = a.markdownTheme(c)
}

// markdownTheme styles replies as MarkdownView.svelte: the reading text
// color, quiet code blocks and tables with hairlines.
func (a *app) markdownTheme(c *ui.Context) *markdown.Theme {
	p := a.pal
	key := p.name
	if a.mdTheme != nil && a.mdThemeKey == key {
		return a.mdTheme
	}
	t := markdown.DefaultTheme(c.Theme())
	t.FontSize = 14
	t.LineHeight = 1.68
	t.BlockSpacing = 0.85
	t.ItemSpacing = 0.18
	t.CodeSize = 0.86
	for i, h := range []struct{ size, top, bottom float32 }{{1.35, 0.2, 0.55}, {1.18, 1.35, 0.5}, {1.05, 1.15, 0.4}, {1, 1, 0.35}, {1, 1, 0.35}, {1, 1, 0.35}} {
		t.Headings[i] = markdown.HeadingStyle{Size: h.size, Weight: 600, Color: p.text, Top: h.top, Bottom: h.bottom}
	}
	t.Font = fontSans
	t.CodeFont = fontMono
	t.Text, t.Muted, t.Strong = p.textReading, p.textTertiary, p.text
	t.StrongWeight = 600
	t.Link = p.brand
	t.Code, t.CodeBackground = p.text, p.text.Alpha(0.06)
	t.CodeBlockBackground, t.CodeBlockBorder = p.bgCode, p.borderSubtle
	t.CodeBlockRadius = 8
	t.TableBorder, t.TableHeader, t.TableStripe = p.borderSubtle, p.bgContent, p.bgContent
	t.TableCellPadding = [2]float32{0.9, 0.6}
	t.QuoteBar, t.QuoteText = p.border, p.textSecond
	t.Rule = p.borderSubtle
	a.mdTheme, a.mdThemeKey = t, key
	return t
}
