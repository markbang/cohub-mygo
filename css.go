package main

import (
	"embed"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// The web app's theme CSS (apps/web/src/styles), read as the browser reads
// its custom properties, so that the built-in themes and a Home Space's
// .cohub/theme.css color the window as they color the web.
//
//go:embed themes/*.css
var themeFiles embed.FS

var themeNames = []string{"light", "dark", "solarized-light", "solarized-dark", "neta-studio", "neta-pixel", "neta-pixel-dark"}

type cssRule struct {
	selectors []string
	decls     [][2]string // property, value in order
}

var cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

// parseCSS returns the top-level rules of css; at-rules and their blocks
// (@media, @supports, @font-face, @import) are left out.
func parseCSS(css string) []cssRule {
	css = cssComment.ReplaceAllString(css, "")
	var rules []cssRule
	i := 0
	for i < len(css) {
		open := strings.IndexByte(css[i:], '{')
		semi := strings.IndexByte(css[i:], ';')
		if open < 0 {
			break
		}
		head := strings.TrimSpace(css[i : i+open])
		if semi >= 0 && semi < open && strings.HasPrefix(strings.TrimSpace(css[i:i+semi]), "@") {
			i += semi + 1 // @import url(...);
			continue
		}
		// Find the matching brace.
		depth, j := 0, i+open
		for ; j < len(css); j++ {
			if css[j] == '{' {
				depth++
			} else if css[j] == '}' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		body := css[i+open+1 : min(j, len(css))]
		i = j + 1
		if strings.HasPrefix(head, "@") {
			continue
		}
		r := cssRule{selectors: splitTop(head, ',')}
		for _, d := range splitTop(body, ';') {
			k, v, ok := strings.Cut(d, ":")
			if !ok || strings.ContainsRune(k, '{') {
				continue
			}
			r.decls = append(r.decls, [2]string{strings.TrimSpace(k), strings.TrimSpace(v)})
		}
		rules = append(rules, r)
	}
	return rules
}

// splitTop splits s at sep outside parentheses and brackets.
func splitTop(s string, sep byte) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case sep:
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if t := strings.TrimSpace(s[start:]); t != "" {
		out = append(out, t)
	}
	return out
}

var attrTheme = regexp.MustCompile(`\[data-theme="([\w-]+)"\]`)

// appliesTo tells whether a selector styles the root in theme name: the
// root, any theme, or that one, with no descendant after it.
func appliesTo(sel, name string) bool {
	sel = strings.TrimSpace(sel)
	switch sel {
	case ":root", "[data-theme]", "html", ":where(:root, [data-theme])", ":where(:root,[data-theme])":
		return true
	case ":root:not([data-theme])":
		return false
	}
	if strings.HasPrefix(sel, ":root:is(") || strings.HasPrefix(sel, ":is(") {
		for _, m := range attrTheme.FindAllStringSubmatch(sel, -1) {
			if m[1] == name {
				return true
			}
		}
		return false
	}
	if m := attrTheme.FindStringSubmatchIndex(sel); m != nil {
		rest := strings.TrimSpace(sel[m[1]:])
		prefix := strings.TrimSpace(sel[:m[0]])
		if rest != "" || (prefix != "" && prefix != ":root" && prefix != "html") {
			return false
		}
		return sel[m[2]:m[3]] == name
	}
	return false
}

// cssVars collects the custom properties for theme name from the sheets in
// order, later ones overriding, and the color-scheme they declare.
func cssVars(name string, sheets ...string) (map[string]string, string) {
	vars := map[string]string{}
	scheme := ""
	for _, sheet := range sheets {
		for _, r := range parseCSS(sheet) {
			hit := false
			for _, s := range r.selectors {
				if appliesTo(s, name) {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
			for _, d := range r.decls {
				switch {
				case strings.HasPrefix(d[0], "--"):
					vars[d[0][2:]] = d[1]
				case d[0] == "color-scheme":
					scheme = strings.Fields(d[1])[0]
				}
			}
		}
	}
	return vars, scheme
}

var varRef = regexp.MustCompile(`var\(\s*--([\w-]+)\s*(?:,\s*([^()]*(?:\([^()]*\))?[^()]*))?\)`)

// resolveVar replaces var() references in v from vars.
func resolveVar(v string, vars map[string]string, depth int) string {
	if depth > 24 {
		return v
	}
	for i := 0; i < 8 && strings.Contains(v, "var("); i++ {
		v = varRef.ReplaceAllStringFunc(v, func(s string) string {
			m := varRef.FindStringSubmatch(s)
			if x, ok := vars[m[1]]; ok {
				return resolveVar(x, vars, depth+1)
			}
			return resolveVar(strings.TrimSpace(m[2]), vars, depth+1)
		})
	}
	return strings.TrimSpace(v)
}

type rgba struct{ r, g, b, a float64 }

func (c rgba) color() ui.Color {
	cl := func(x float64) uint8 { return uint8(math.Round(math.Max(0, math.Min(1, x)) * 255)) }
	return ui.RGBA(cl(c.r), cl(c.g), cl(c.b), float32(math.Max(0, math.Min(1, c.a))))
}

func oklchRGB(l, c, h float64) (float64, float64, float64) {
	a, b := c*math.Cos(h*math.Pi/180), c*math.Sin(h*math.Pi/180)
	l_ := l + 0.3963377774*a + 0.2158037573*b
	m_ := l - 0.1055613458*a - 0.0638541728*b
	s_ := l - 0.0894841775*a - 1.2914855480*b
	l3, m3, s3 := l_*l_*l_, m_*m_*m_, s_*s_*s_
	enc := func(x float64) float64 {
		x = math.Max(0, math.Min(1, x))
		if x <= 0.0031308 {
			return 12.92 * x
		}
		return 1.055*math.Pow(x, 1/2.4) - 0.055
	}
	return enc(4.0767416621*l3 - 3.3077115913*m3 + 0.2309699292*s3),
		enc(-1.2684380046*l3 + 2.6097574011*m3 - 0.3413193965*s3),
		enc(-0.0041960863*l3 - 0.7034186147*m3 + 1.7076147010*s3)
}

func cssNum(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	pct := strings.HasSuffix(s, "%")
	s = strings.TrimSuffix(s, "%")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	if pct {
		f /= 100
	}
	return f, true
}

var (
	reHex   = regexp.MustCompile(`^#([0-9a-fA-F]{3,8})$`)
	reOklch = regexp.MustCompile(`^oklch\(\s*([\d.]+%?)\s+([\d.]+)\s+([\d.]+)\s*(?:/\s*([\d.]+%?))?\s*\)$`)
	reRGB   = regexp.MustCompile(`^rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)\s*(?:[/,]\s*([\d.]+%?))?\s*\)$`)
	reMix   = regexp.MustCompile(`(?s)^color-mix\(\s*in\s+[\w-]+\s*,\s*(.+)\)$`)
	reStop  = regexp.MustCompile(`(?s)^(.+?)\s+([\d.]+)%$`)
)

// parseColor reads a CSS color, after resolving its var() references.
func parseColor(v string, vars map[string]string) (rgba, bool) {
	v = resolveVar(v, vars, 0)
	switch strings.ToLower(v) {
	case "transparent":
		return rgba{}, true
	case "white":
		return rgba{1, 1, 1, 1}, true
	case "black":
		return rgba{0, 0, 0, 1}, true
	}
	if m := reHex.FindStringSubmatch(v); m != nil {
		h := m[1]
		if len(h) == 3 || len(h) == 4 {
			var b strings.Builder
			for _, ch := range h {
				b.WriteRune(ch)
				b.WriteRune(ch)
			}
			h = b.String()
		}
		var vals []float64
		for i := 0; i+1 < len(h); i += 2 {
			n, _ := strconv.ParseUint(h[i:i+2], 16, 8)
			vals = append(vals, float64(n)/255)
		}
		if len(vals) == 3 {
			vals = append(vals, 1)
		}
		if len(vals) == 4 {
			return rgba{vals[0], vals[1], vals[2], vals[3]}, true
		}
	}
	if m := reOklch.FindStringSubmatch(v); m != nil {
		l, _ := cssNum(m[1])
		c, _ := cssNum(m[2])
		h, _ := cssNum(m[3])
		a := 1.0
		if m[4] != "" {
			a, _ = cssNum(m[4])
		}
		r, g, b := oklchRGB(l, c, h)
		return rgba{r, g, b, a}, true
	}
	if m := reRGB.FindStringSubmatch(v); m != nil {
		r, _ := cssNum(m[1])
		g, _ := cssNum(m[2])
		b, _ := cssNum(m[3])
		a := 1.0
		if m[4] != "" {
			a, _ = cssNum(m[4])
		}
		return rgba{r / 255, g / 255, b / 255, a}, true
	}
	if m := reMix.FindStringSubmatch(v); m != nil {
		parts := splitTop(m[1], ',')
		if len(parts) != 2 {
			return rgba{}, false
		}
		var cols [2]rgba
		var ps [2]float64
		var has [2]bool
		for i, p := range parts {
			if s := reStop.FindStringSubmatch(p); s != nil {
				ps[i], _ = cssNum(s[2] + "%")
				has[i] = true
				p = s[1]
			}
			c, ok := parseColor(p, vars)
			if !ok {
				return rgba{}, false
			}
			cols[i] = c
		}
		switch {
		case !has[0] && !has[1]:
			ps = [2]float64{0.5, 0.5}
		case !has[0]:
			ps[0] = 1 - ps[1]
		case !has[1]:
			ps[1] = 1 - ps[0]
		}
		a := cols[0].a*ps[0] + cols[1].a*ps[1]
		if a == 0 {
			return rgba{}, true
		}
		mix := func(x, y float64) float64 { return (x*cols[0].a*ps[0] + y*cols[1].a*ps[1]) / a }
		return rgba{mix(cols[0].r, cols[1].r), mix(cols[0].g, cols[1].g), mix(cols[0].b, cols[1].b), a}, true
	}
	return rgba{}, false
}

// paletteFields maps the palette's fields to the web's custom properties.
var paletteFields = []struct {
	css   string
	field func(p *palette) *ui.Color
}{
	{"bg-primary", func(p *palette) *ui.Color { return &p.bgPrimary }},
	{"bg-content", func(p *palette) *ui.Color { return &p.bgContent }},
	{"bg-surface", func(p *palette) *ui.Color { return &p.bgSurface }},
	{"bg-surface-hover", func(p *palette) *ui.Color { return &p.bgSurfaceHover }},
	{"bg-elevated", func(p *palette) *ui.Color { return &p.bgElevated }},
	{"bg-input", func(p *palette) *ui.Color { return &p.bgInput }},
	{"bg-code", func(p *palette) *ui.Color { return &p.bgCode }},
	{"sidebar-bg", func(p *palette) *ui.Color { return &p.sidebarBg }},
	{"chat-bg", func(p *palette) *ui.Color { return &p.chatBg }},
	{"text-primary", func(p *palette) *ui.Color { return &p.text }},
	{"text-reading", func(p *palette) *ui.Color { return &p.textReading }},
	{"text-secondary", func(p *palette) *ui.Color { return &p.textSecond }},
	{"text-tertiary", func(p *palette) *ui.Color { return &p.textTertiary }},
	{"text-placeholder", func(p *palette) *ui.Color { return &p.placeholder }},
	{"text-disabled", func(p *palette) *ui.Color { return &p.disabled }},
	{"border-primary", func(p *palette) *ui.Color { return &p.border }},
	{"border-subtle", func(p *palette) *ui.Color { return &p.borderSubtle }},
	{"border-strong", func(p *palette) *ui.Color { return &p.borderStrong }},
	{"bg-hover", func(p *palette) *ui.Color { return &p.hover }},
	{"bg-hover-strong", func(p *palette) *ui.Color { return &p.hoverStrong }},
	{"bg-active", func(p *palette) *ui.Color { return &p.active }},
	{"brand", func(p *palette) *ui.Color { return &p.brand }},
	{"brand-hover", func(p *palette) *ui.Color { return &p.brandHover }},
	{"brand-muted", func(p *palette) *ui.Color { return &p.brandMuted }},
	{"brand-muted-fg", func(p *palette) *ui.Color { return &p.brandMutedFg }},
	{"tab-key-selected-bg", func(p *palette) *ui.Color { return &p.tabKeyBg }},
	{"brand-border", func(p *palette) *ui.Color { return &p.brandBorder }},
	{"brand-contrast-fg", func(p *palette) *ui.Color { return &p.brandFg }},
	{"brand-900", func(p *palette) *ui.Color { return &p.brand900 }},
	{"brand-bg", func(p *palette) *ui.Color { return &p.brandBg }},
	{"provider-discord", func(p *palette) *ui.Color { return &p.providerDiscord }},
	{"provider-feishu", func(p *palette) *ui.Color { return &p.providerFeishu }},
	{"provider-wechat", func(p *palette) *ui.Color { return &p.providerWechat }},
	{"chat-user-message-bg", func(p *palette) *ui.Color { return &p.userBubble }},
	{"selection-bg", func(p *palette) *ui.Color { return &p.selection }},
	{"success-400", func(p *palette) *ui.Color { return &p.success }},
	{"error-400", func(p *palette) *ui.Color { return &p.errorC }},
	{"warning-400", func(p *palette) *ui.Color { return &p.warning }},
	{"info-400", func(p *palette) *ui.Color { return &p.info }},
}

func readTheme(name string) string {
	b, _ := themeFiles.ReadFile("themes/" + name + ".css")
	return string(b)
}

// buildPalette is the palette of theme name, with custom (a Home Space's
// theme.css) over it.
func buildPalette(name, custom string) *palette {
	sheets := []string{readTheme("theme"), readTheme(name)}
	if strings.HasPrefix(name, "neta-pixel") && name != "neta-pixel" {
		sheets = append(sheets, readTheme("neta-pixel"))
	}
	if custom != "" {
		sheets = append(sheets, custom)
	}
	vars, scheme := cssVars(name, sheets...)
	p := &palette{name: name, dark: scheme == "dark"}
	if scheme == "" {
		p.dark = strings.HasSuffix(name, "dark") || name == "neta-studio"
	}
	for _, f := range paletteFields {
		if v, ok := vars[f.css]; ok {
			if c, ok := parseColor(v, vars); ok {
				*f.field(p) = c.color()
			}
		}
	}
	p.board = map[string]ui.Color{}
	for _, tok := range []string{"brand", "neutral", "black", "white", "blue", "green", "amber", "violet", "rose"} {
		for _, part := range []string{"stroke", "fill", "label"} {
			if c, ok := parseColor("var(--board-color-"+tok+"-"+part+")", vars); ok {
				p.board[tok+"-"+part] = c.color()
			}
		}
	}
	for i := range p.identity {
		if c, ok := parseColor(fmt.Sprintf("var(--identity-%d)", i+1), vars); ok {
			p.identity[i] = c.color()
		} else {
			p.identity[i] = p.textTertiary
		}
	}
	font := resolveVar(vars["font-sans"], vars, 0)
	font = strings.ReplaceAll(font, `"`, "")
	font = strings.NewReplacer("Geist Variable", "Geist", "Figtree Variable", "Figtree").Replace(font)
	p.font = strings.Join(strings.Fields(font), " ")
	if p.font == "" {
		p.font = "Geist, sans-serif"
	}
	p.font += ", PingFang SC, Noto Sans CJK SC"
	mono := strings.ReplaceAll(resolveVar(vars["font-mono"], vars, 0), `"`, "")
	mono = strings.NewReplacer("Geist Mono Variable", "Geist Mono").Replace(mono)
	p.mono = strings.Join(strings.Fields(mono), " ")
	if p.mono == "" {
		p.mono = "Geist Mono, monospace"
	}
	p.mono += ", Noto Sans Mono CJK SC, monospace"
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
