package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

// palette mirrors the design tokens of apps/web/src/styles/themes/{light,dark}.css.
type palette struct {
	dark bool

	bgPrimary      ui.Color // sidebar, panels
	bgContent      ui.Color // main column
	bgSurface      ui.Color
	bgSurfaceHover ui.Color
	bgElevated     ui.Color
	bgInput        ui.Color
	bgCode         ui.Color

	text         ui.Color
	textReading  ui.Color
	textSecond   ui.Color
	textTertiary ui.Color
	placeholder  ui.Color
	disabled     ui.Color

	border       ui.Color
	borderSubtle ui.Color

	hover       ui.Color
	hoverStrong ui.Color
	active      ui.Color

	brand       ui.Color
	brandHover  ui.Color
	brandMuted  ui.Color
	brandBorder ui.Color
	brandFg     ui.Color
	userBubble  ui.Color

	success ui.Color
	errorC  ui.Color
}

func n(l, c float32) ui.Color { return ui.Oklch(l, c, 250) }

func lightPalette() *palette {
	brand := ui.Oklch(0.58, 0.18, 30)
	return &palette{
		bgPrimary:      n(1, 0),
		bgContent:      n(0.99, 0.001),
		bgSurface:      n(0.98, 0.002),
		bgSurfaceHover: n(0.96, 0.003),
		bgElevated:     n(1, 0),
		bgInput:        n(1, 0),
		bgCode:         n(0.98, 0.002),
		text:           n(0.08, 0.003),
		textReading:    n(0.18, 0.005),
		textSecond:     n(0.35, 0.008),
		textTertiary:   n(0.50, 0.01),
		placeholder:    n(0.65, 0.012),
		disabled:       n(0.78, 0.01),
		border:         n(0.85, 0.008),
		borderSubtle:   n(0.90, 0.006),
		hover:          ui.Oklch(0.2, 0.03, 250).Alpha(0.06),
		hoverStrong:    ui.Oklch(0.2, 0.03, 250).Alpha(0.11),
		active:         ui.Oklch(0.2, 0.03, 250).Alpha(0.17),
		brand:          brand,
		brandHover:     ui.Oklch(0.52, 0.2, 30),
		brandMuted:     brand.Alpha(0.10),
		brandBorder:    brand.Alpha(0.20),
		brandFg:        n(1, 0),
		userBubble:     brand.Alpha(0.05),
		success:        ui.Oklch(0.45, 0.12, 160),
		errorC:         ui.Oklch(0.45, 0.16, 25),
	}
}

func darkPalette() *palette {
	brand := ui.Oklch(0.60, 0.2, 30)
	return &palette{
		dark:           true,
		bgPrimary:      n(0.18, 0.004),
		bgContent:      n(0.22, 0.005),
		bgSurface:      n(0.27, 0.006),
		bgSurfaceHover: n(0.32, 0.007),
		bgElevated:     n(0.38, 0.008),
		bgInput:        n(0.22, 0.005),
		bgCode:         n(0.14, 0.003),
		text:           n(0.94, 0.004),
		textReading:    n(0.90, 0.005),
		textSecond:     n(0.82, 0.006),
		textTertiary:   n(0.70, 0.008),
		placeholder:    n(0.58, 0.01),
		disabled:       n(0.54, 0.012),
		border:         n(0.38, 0.008),
		borderSubtle:   n(0.32, 0.007),
		hover:          ui.Oklch(0.9, 0.02, 250).Alpha(0.11),
		hoverStrong:    ui.Oklch(0.9, 0.02, 250).Alpha(0.19),
		active:         ui.Oklch(0.9, 0.02, 250).Alpha(0.24),
		brand:          brand,
		brandHover:     ui.Oklch(0.55, 0.22, 30),
		brandMuted:     brand.Alpha(0.10),
		brandBorder:    brand.Alpha(0.20),
		brandFg:        n(1, 0),
		userBubble:     brand.Alpha(0.05),
		success:        ui.Oklch(0.78, 0.12, 160),
		errorC:         ui.Oklch(0.68, 0.18, 25),
	}
}

const (
	fontSans = "Geist, PingFang SC, Noto Sans CJK SC, Microsoft YaHei, sans-serif"
	fontMono = "Geist Mono, SF Mono, Noto Sans Mono CJK SC, monospace"
)

// apply makes the window's widgets follow the palette.
func (p *palette) apply(c *ui.Context) {
	t := *c.Theme()
	t.Background = p.bgContent
	t.Surface, t.SurfaceHover, t.SurfacePressed = p.bgSurface, p.bgSurfaceHover, p.bgElevated
	t.Border = p.border
	t.Text, t.TextMuted = p.text, p.textTertiary
	t.Accent, t.AccentHover, t.AccentPressed, t.AccentText = p.brand, p.brandHover, p.brandHover, p.brandFg
	t.InactiveAccent = p.brand
	t.Danger, t.Success = p.errorC, p.success
	t.Selection = ui.Oklch(0.85, 0.04, 250)
	if p.dark {
		t.Selection = ui.Oklch(0.4, 0.06, 250)
	}
	t.Focus = p.brand.Alpha(0.5)
	t.Scrollbar = p.disabled
	t.Radius = 6
	t.FontSize = 14
	t.Font = fontSans
	t.Dark = p.dark
	c.SetTheme(&t)
}

// markdownTheme styles replies as MarkdownView.svelte: the reading text
// color, quiet code blocks and tables with hairlines.
func (a *app) markdownTheme(c *ui.Context) *markdown.Theme {
	p := a.pal
	if a.mdTheme != nil && a.mdThemeDark == p.dark {
		return a.mdTheme
	}
	t := markdown.DefaultTheme(c.Theme())
	t.FontSize = 14
	t.LineHeight = 1.75
	t.Font = fontSans
	t.CodeFont = fontMono
	t.Text, t.Muted, t.Strong = p.textReading, p.textTertiary, p.text
	t.StrongWeight = 600
	t.Link = p.brand
	t.Code, t.CodeBackground = p.text, p.hover
	t.CodeBlockBackground, t.CodeBlockBorder = p.bgCode, p.borderSubtle
	t.CodeBlockRadius = 8
	t.TableBorder, t.TableHeader, t.TableStripe = p.borderSubtle, ui.Transparent, ui.Transparent
	t.TableCellPadding = [2]float32{0.9, 0.6}
	t.QuoteBar, t.QuoteText = p.border, p.textSecond
	t.Rule = p.borderSubtle
	a.mdTheme, a.mdThemeDark = t, p.dark
	return t
}
