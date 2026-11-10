package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

// palette is a theme's design tokens, as the web's CSS variables
// (themes_gen.go has the built-in ones).
type palette struct {
	name string
	dark bool
	font string
	mono string

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

	brand        ui.Color
	brandHover   ui.Color
	brandMuted   ui.Color
	brandMutedFg ui.Color
	brandBorder  ui.Color
	brandFg      ui.Color
	brand900     ui.Color
	brandBg      ui.Color // a chosen option's background

	providerDiscord, providerFeishu, providerWechat ui.Color
	userBubble                                      ui.Color
	selection                                       ui.Color

	success ui.Color
	errorC  ui.Color
	warning ui.Color
	info    ui.Color

	identity [8]ui.Color
	tabKeyBg ui.Color            // --tab-key-selected-bg
	board    map[string]ui.Color // --board-color-<token>-<part> // --identity-1…8, the avatar tones

	popBg   ui.Color // the palette's panel
	popHead ui.Color
}

func lightPalette() *palette { return buildPalette("light", "") }
func darkPalette() *palette  { return buildPalette("dark", "") }

// fontSans and fontMono are the palette's families, for text set apart.
var (
	fontSans = "Geist, PingFang SC, Noto Sans CJK SC, sans-serif"
	fontMono = "Geist Mono, Noto Sans Mono CJK SC, monospace"
)

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
	if !contains(themeNames, name) {
		name = "light"
		if c.Theme().Dark {
			name = "dark"
		}
	}
	custom := ""
	if a.customThemeOn() {
		custom = a.customCSS
	}
	if key := name + "\x00" + custom; key != a.palKey {
		a.palKey = key
		a.pal = buildPalette(name, custom)
		fontSans, fontMono = a.pal.font, a.pal.mono
	}
	a.pal.apply(c)
	a.md.Theme = a.markdownTheme(c)
}

// markdownTheme styles replies as MarkdownView.svelte: the reading text
// color, quiet code blocks and tables with hairlines.
func (a *app) markdownTheme(c *ui.Context) *markdown.Theme {
	p := a.pal
	key := a.palKey
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
	t.Font = p.font
	t.CodeFont = p.mono
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
