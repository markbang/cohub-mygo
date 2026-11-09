#!/usr/bin/env python3
"""Generates themes_gen.go from the web app's theme CSS (apps/web/src/styles/themes):
each built-in theme's tokens resolved to sRGB, so the app's palettes are the web's.

    python3 tools/gen_themes.py /path/to/cohub/apps/web/src/styles > themes_gen.go
"""
import math
import re
import sys
from pathlib import Path

THEMES = ["light", "dark", "solarized-light", "solarized-dark", "neta-studio", "neta-pixel", "neta-pixel-dark"]
FIELDS = {
    "bgPrimary": "bg-primary", "bgContent": "bg-content", "bgSurface": "bg-surface",
    "bgSurfaceHover": "bg-surface-hover", "bgElevated": "bg-elevated", "bgInput": "bg-input", "bgCode": "bg-code",
    "text": "text-primary", "textReading": "text-reading", "textSecond": "text-secondary",
    "textTertiary": "text-tertiary", "placeholder": "text-placeholder", "disabled": "text-disabled",
    "border": "border-primary", "borderSubtle": "border-subtle", "borderStrong": "border-strong",
    "hover": "bg-hover", "hoverStrong": "bg-hover-strong", "active": "bg-active",
    "brand": "brand", "brandHover": "brand-hover", "brandMuted": "brand-muted", "brandBorder": "brand-border",
    "brandFg": "brand-contrast-fg", "brand900": "brand-900", "brandBg": "brand-bg",
    "userBubble": "chat-user-message-bg", "success": "success-400", "errorC": "error-400",
    "warning": "warning-400", "info": "info-400", "sidebarBg": "sidebar-bg", "chatBg": "chat-bg",
    "selection": "selection-bg",
}


def oklch_to_srgb(l, c, h):
    a, b = c * math.cos(math.radians(h)), c * math.sin(math.radians(h))
    l_ = l + 0.3963377774 * a + 0.2158037573 * b
    m_ = l - 0.1055613458 * a - 0.0638541728 * b
    s_ = l - 0.0894841775 * a - 1.2914855480 * b
    l3, m3, s3 = l_ ** 3, m_ ** 3, s_ ** 3
    r = 4.0767416621 * l3 - 3.3077115913 * m3 + 0.2309699292 * s3
    g = -1.2684380046 * l3 + 2.6097574011 * m3 - 0.3413193965 * s3
    bl = -0.0041960863 * l3 - 0.7034186147 * m3 + 1.7076147010 * s3

    def enc(x):
        x = max(0.0, min(1.0, x))
        return 12.92 * x if x <= 0.0031308 else 1.055 * x ** (1 / 2.4) - 0.055

    return [enc(r), enc(g), enc(bl)]


def num(s):
    s = s.strip()
    if s.endswith("%"):
        return float(s[:-1]) / 100
    return float(s)


def parse_color(v, vars_):
    v = v.strip().rstrip(";").strip()
    v = resolve(v, vars_)
    if v is None:
        return None
    if v in ("transparent",):
        return [0, 0, 0, 0]
    if v in ("white", "#fff"):
        return [1, 1, 1, 1]
    if v in ("black", "#000"):
        return [0, 0, 0, 1]
    m = re.fullmatch(r"#([0-9a-fA-F]{3,8})", v)
    if m:
        h = m.group(1)
        if len(h) in (3, 4):
            h = "".join(ch * 2 for ch in h)
        vals = [int(h[i:i + 2], 16) / 255 for i in range(0, len(h), 2)]
        return vals + ([1] if len(vals) == 3 else [])
    m = re.fullmatch(r"oklch\(\s*([\d.]+%?)\s+([\d.]+)\s+([\d.]+)\s*(?:/\s*([\d.]+%?))?\s*\)", v)
    if m:
        l = num(m.group(1))
        rgb = oklch_to_srgb(l, float(m.group(2)), float(m.group(3)))
        return rgb + [num(m.group(4)) if m.group(4) else 1]
    m = re.fullmatch(r"rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)\s*(?:[/,]\s*([\d.]+%?))?\s*\)", v)
    if m:
        return [float(m.group(i)) / 255 for i in (1, 2, 3)] + [num(m.group(4)) if m.group(4) else 1]
    m = re.fullmatch(r"color-mix\(\s*in\s+\w+\s*,\s*(.+)\)", v, re.S)
    if m:
        parts = split_top(m.group(1))
        cols = []
        for p in parts:
            p = p.strip()
            pm = re.fullmatch(r"(.+?)\s+([\d.]+)%", p, re.S)
            if pm:
                cols.append((parse_color(pm.group(1), vars_), float(pm.group(2)) / 100))
            else:
                cols.append((parse_color(p, vars_), None))
        (c1, p1), (c2, p2) = cols
        if c1 is None or c2 is None:
            return None
        if p1 is None and p2 is None:
            p1 = 0.5
        if p1 is None:
            p1 = 1 - p2
        if p2 is None:
            p2 = 1 - p1
        a = c1[3] * p1 + c2[3] * p2
        if a == 0:
            return [0, 0, 0, 0]
        rgb = [(c1[i] * c1[3] * p1 + c2[i] * c2[3] * p2) / a for i in range(3)]
        return rgb + [a]
    return None


def split_top(s):
    out, depth, cur = [], 0, ""
    for ch in s:
        if ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
        if ch == "," and depth == 0:
            out.append(cur)
            cur = ""
        else:
            cur += ch
    out.append(cur)
    return out


def resolve(v, vars_, depth=0):
    v = v.strip()
    m = re.fullmatch(r"var\(\s*--([\w-]+)\s*(?:,\s*(.+))?\)", v, re.S)
    if m:
        if depth > 20:
            return None
        if m.group(1) in vars_:
            return resolve(vars_[m.group(1)], vars_, depth + 1)
        return resolve(m.group(2), vars_, depth + 1) if m.group(2) else None
    return re.sub(r"var\(\s*--([\w-]+)\s*\)", lambda mm: resolve("var(--" + mm.group(1) + ")", vars_, depth + 1) or mm.group(0), v)


def declarations(css):
    out = {}
    for m in re.finditer(r"--([\w-]+)\s*:\s*([^;]+);", css):
        out[m.group(1)] = re.sub(r"/\*.*?\*/", "", m.group(2), flags=re.S).strip()
    return out


def theme_vars(styles, name):
    base = declarations(re.sub(r"/\*.*?\*/", "", (styles / "theme.css").read_text(), flags=re.S))
    css = re.sub(r"/\*.*?\*/", "", (styles / "themes" / f"{name}.css").read_text(), flags=re.S)
    vars_ = dict(base)
    vars_.update(declarations(css))
    if name.startswith("neta-pixel"):
        pixel = re.sub(r"/\*.*?\*/", "", (styles / "themes" / "neta-pixel.css").read_text(), flags=re.S)
        m = re.search(r':root:is\(\[data-theme="neta-pixel"\][^{]*\{(.*?)\n\}', pixel, re.S)
        if m:
            vars_.update(declarations(m.group(1)))
    dark = "color-scheme: dark" in css
    return vars_, dark


def go_color(c):
    r, g, b, a = c
    return f"ui.RGBA({round(r * 255)}, {round(g * 255)}, {round(b * 255)}, {a:.3g})"


def main():
    styles = Path(sys.argv[1])
    print("// Code generated by tools/gen_themes.py from the web app's theme CSS; DO NOT EDIT.\n")
    print("package main\n\nimport \"github.com/egoist/mygo/ui\"\n")
    print("// builtinThemes are the web's themes (apps/web/src/styles/themes), by name.")
    print("var builtinThemes = map[string]func() *palette{")
    for name in THEMES:
        vars_, dark = theme_vars(styles, name)
        print(f'\t"{name}": func() *palette {{')
        print("\t\treturn &palette{")
        print(f'\t\t\tname: "{name}",')
        print(f"\t\t\tdark: {str(dark).lower()},")
        for field, var in FIELDS.items():
            if var not in vars_:
                continue
            c = parse_color(vars_[var], vars_)
            if c is None:
                print(f"\t\t\t// {field}: {vars_[var]!r} not resolved", file=sys.stderr)
                continue
            print(f"\t\t\t{field}: {go_color(c)},")
        font = resolve(vars_.get("font-sans", ""), vars_) or ""
        font = " ".join(font.replace('"', "").split())
        print(f"\t\t\tfont: {font!r},".replace("'", '"'))
        print("\t\t}")
        print("\t},")
    print("}")


main()
