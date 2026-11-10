package app

import (
	"context"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"cohub-mygo/internal/cohub"
)

// The composer's two menus above the box, as SlashCommandMenu.svelte and
// SpaceMentionMenu.svelte: a "/" at the start lists the Space's prompt
// templates and skills, an "@" lists Spaces to mention, as
// @[name](cohub://spaces/<id>). ↑↓ move, Tab or Enter inserts, Escape
// closes.

type slashItem struct {
	kind, name, desc, group, hint string
	score                         int
}

type mentionItem struct {
	spaceID, name, desc, avatar, owner string
	official                           bool
}

type composerMenu struct {
	selected  int
	dismissed string // the token the user closed the menu on
	mention   struct {
		query   string
		at      time.Time
		results []mentionItem
		loading bool
	}
}

const slashScanLimit, slashQueryLimit = 96, 64

// slashToken is the command typed: "/name" alone at the start of the draft.
func slashToken(s string) (string, bool) {
	t := strings.TrimLeft(s, " \t\n")
	if !strings.HasPrefix(t, "/") || len(s) > slashScanLimit {
		return "", false
	}
	q := t[1:]
	if strings.ContainsAny(q, " \t\n") || len(q) > slashQueryLimit {
		return "", false
	}
	return q, true
}

var mentionRe = regexp.MustCompile(`(^|\s)@([^@\s\[\]()]{0,80})$`)

// mentionToken is the "@query" before the caret, and where its @ starts,
// in runes.
func mentionToken(s string, caret int) (string, int, bool) {
	r := []rune(s)
	if caret < 0 || caret > len(r) {
		return "", 0, false
	}
	start := max(0, caret-120)
	mm := mentionRe.FindStringSubmatch(string(r[start:caret]))
	if mm == nil {
		return "", 0, false
	}
	q := mm[2]
	return q, caret - len([]rune(q)) - 1, true
}

func (a *app) slashItems(spaceID, q string) ([]slashItem, bool) {
	type prompt struct {
		Name, Description, Category, Scope, ArgumentHint string
	}
	type skill struct {
		Name, Description, Scope string
		Source                   *struct {
			Type, MountSlug string
		}
	}
	ps, _, pok := loadJSON[struct{ Prompts []prompt }](a, "prompts:"+spaceID, "/api/prompts?spaceId="+url.QueryEscape(spaceID))
	ss, _, sok := loadJSON[struct{ Skills []skill }](a, "skills:"+spaceID, "/api/skills?spaceId="+url.QueryEscape(spaceID))
	q = strings.ToLower(q)
	score := func(fields ...string) int {
		if q == "" {
			return 10
		}
		for i, f := range fields {
			f = strings.ToLower(f)
			if f == "" {
				continue
			}
			switch {
			case i == 0 && strings.HasPrefix(f, q):
				return 100
			case i == 0 && strings.Contains(f, q):
				return 80
			case strings.Contains(f, q):
				return []int{80, 64, 48}[min(i, 2)]
			}
		}
		return 0
	}
	var out []slashItem
	if ps != nil {
		for _, p := range ps.Prompts {
			if s := score(p.Name, p.Category, p.Description); s > 0 {
				g := p.Category
				if g == "" {
					g = p.Scope
				}
				out = append(out, slashItem{kind: "prompt", name: p.Name, desc: p.Description, group: g, hint: p.ArgumentHint, score: s})
			}
		}
	}
	if ss != nil {
		for _, sk := range ss.Skills {
			mount := ""
			g := m("slash_skill", "scope", sk.Scope)
			if sk.Source != nil && sk.Source.Type == "mod" {
				mount, g = sk.Source.MountSlug, m("slash_mod", "slug", sk.Source.MountSlug)
			}
			s := score("skill:"+sk.Name, mount, sk.Description)
			if q != "" && strings.HasPrefix(strings.ToLower(sk.Name), q) {
				s = 100
			}
			if s > 0 {
				if q == "" {
					s = 9
				}
				out = append(out, slashItem{kind: "skill", name: sk.Name, desc: sk.Description, group: g, score: s})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		x, y := out[i], out[j]
		if x.score != y.score {
			return x.score > y.score
		}
		if x.kind != y.kind {
			return x.kind < y.kind
		}
		if x.group != y.group {
			return x.group < y.group
		}
		return x.name < y.name
	})
	return out, pok && sok
}

// mentionItems is what "@q" finds: the Spaces joined, then, from two
// letters on, public ones the search finds.
func (a *app) mentionItems(q string) ([]mentionItem, bool) {
	cur := a.currentSpace()
	ql := strings.ToLower(strings.TrimSpace(q))
	var out []mentionItem
	seen := map[string]bool{}
	for _, sp := range a.sortedSpaces() {
		if sp.ID == cur || seen[sp.ID] {
			continue
		}
		if ql != "" && !strings.Contains(strings.ToLower(sp.Name), ql) && !strings.Contains(strings.ToLower(sp.Slug), ql) {
			continue
		}
		seen[sp.ID] = true
		owner := ""
		if sp.OwnerProfile != nil {
			owner = sp.OwnerProfile.Name()
		}
		out = append(out, mentionItem{spaceID: sp.ID, name: sp.Name, desc: sp.Description, avatar: sp.Avatar(), owner: owner})
		if len(out) >= 8 {
			break
		}
	}
	mn := &a.compMenu.mention
	loading := false
	if len([]rune(ql)) >= 2 && a.api != nil {
		if mn.query != ql {
			mn.query, mn.results, mn.loading = ql, nil, true
			at := time.Now()
			mn.at = at
			a.spawn(func() {
				time.Sleep(180 * time.Millisecond)
				var res struct {
					Items []struct {
						Type, SpaceID, Title string
						Excerpt              *string
						OwnerProfile         *cohub.Profile
						SpaceProfile         *struct{ AvatarURL string }
					}
				}
				err := a.api.Do(a.ctx(), "GET", "/api/search?q="+url.QueryEscape(ql)+"&limit=30&type=space", nil, &res)
				a.update(func() {
					if mn.at != at {
						return
					}
					mn.loading = false
					if err != nil {
						return
					}
					mn.results = nil
					for _, it := range res.Items {
						if it.Type != "space" {
							continue
						}
						x := mentionItem{spaceID: it.SpaceID, name: it.Title, desc: deref(it.Excerpt)}
						if it.SpaceProfile != nil {
							x.avatar = it.SpaceProfile.AvatarURL
						}
						if it.OwnerProfile != nil {
							x.owner = it.OwnerProfile.Name()
						}
						mn.results = append(mn.results, x)
					}
				})
			})
		}
		loading = mn.loading
		for _, r := range mn.results {
			if !seen[r.spaceID] && r.spaceID != cur {
				seen[r.spaceID] = true
				out = append(out, r)
			}
		}
	}
	return out, !loading
}

func escapeMentionLabel(s string) string {
	s = strings.NewReplacer("[", "", "]", "", `\`, "").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// composerMenus draws the open menu above the composer card and returns
// how a key typed in the draft goes to it.
func (a *app) composerMenus(c *ui.Context, in ui.Element) func(k ui.TextKey) bool {
	cm := &a.compMenu
	spaceID := a.currentSpace()
	type entry struct {
		group, title, hint, desc, tag string
		avatar, seed, owner           string
		apply                         func(k *ui.TextKey)
	}
	var query string
	var entries []entry
	var head, status, emptyTitle, emptyHint, loadingText string
	ready := true
	token := ""
	if q, ok := slashToken(a.draft); ok && spaceID != "" {
		token, query = "/"+q, q
		items, done := a.slashItems(spaceID, q)
		ready = done
		head, emptyTitle, emptyHint, loadingText = m("slash_commands"), m("slash_none"), m("slash_hint_msg"), m("slash_loading")
		status = m("slash_narrow")
		if q != "" {
			status = m("slash_filtering", "q", strings.ToLower(q))
		}
		for _, it := range items {
			it := it
			label := it.name
			if it.kind == "skill" {
				label = "skill:" + it.name
			}
			entries = append(entries, entry{group: it.group, title: label, hint: it.hint, desc: it.desc, tag: it.group, apply: func(k *ui.TextKey) {
				t := strings.TrimLeft(a.draft, " \t\n")
				lead := a.draft[:len(a.draft)-len(t)]
				suffix := " "
				if i := strings.IndexByte(t, ' '); i >= 0 {
					suffix = t[i:]
				}
				a.setDraft(k, in, lead+"/"+label+suffix, -1)
			}})
		}
	} else if caret, _ := in.TextSelection(); true {
		if q, start, ok := mentionToken(a.draft, caret); ok {
			token, query = "@"+q+"#"+itoa(start), q
			items, done := a.mentionItems(q)
			ready = done
			head, emptyTitle, loadingText = m("mention_spaces"), m("mention_none"), m("mention_searching_spaces")
			emptyHint = m("mention_hint_typing")
			if len([]rune(q)) >= 2 {
				emptyHint = m("mention_hint_public")
			}
			// The web's own words, untranslated there.
			switch {
			case strings.TrimSpace(q) == "":
				status = "Mention another space"
			case !done:
				status = "Local " + itoa(len(items)) + " · syncing server…"
			case len(items) == 1:
				status = "1 space"
			default:
				status = itoa(len(items)) + " spaces"
			}
			for _, it := range items {
				it := it
				entries = append(entries, entry{group: m("mention"), title: it.name, desc: it.desc, owner: it.owner, avatar: it.avatar, seed: it.spaceID, apply: func(k *ui.TextKey) {
					r := []rune(a.draft)
					end := start + 1 + len([]rune(q))
					snippet := "@[" + escapeMentionLabel(it.name) + "](cohub://spaces/" + it.spaceID + ") "
					next := string(r[:start]) + snippet + string(r[end:])
					a.setDraft(k, in, next, start+len([]rune(snippet)))
				}})
			}
		}
	}
	if token == "" || token == cm.dismissed {
		if token == "" {
			cm.dismissed, cm.selected = "", 0
		}
		return nil
	}
	if cm.selected >= len(entries) {
		cm.selected = max(0, len(entries)-1)
	}
	p := a.pal
	ui.Column(c.Key("composer-menu")).Attach(ui.AnchorTopLeft, ui.AnchorBottomLeft).Width(min(560, a.width-48)).Top(-8).
		Radius(18).Border(1, p.borderSubtle).Background(p.bgContent).Shadow(0, 18, 60, 0, ui.RGBA(15, 23, 42, 0.18)).Clip().Children(func() {
		ui.Row(c.Key("head")).Gap(12).Padding(10, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, head).FontSize(12).FontWeight(500)
				ui.Text(c, status).FontSize(11).TextColor(p.textTertiary).SingleLine()
			})
			ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(4, 8).Radius(999).Border(1, p.borderSubtle).Background(p.bgPrimary).Children(func() {
				if !ready && len(entries) > 0 {
					a.spinner(c, 12, p.brand)
				}
				ui.Text(c, m("slash_tab")).FontSize(10).TextColor(p.textTertiary)
				ui.Text(c, m("slash_or")).FontSize(10).TextColor(p.placeholder)
				ui.Icon(c, icon("corner-down-left")).Size(12, 12).TextColor(p.textTertiary)
			})
		})
		ui.Scroll(c.Key("items")).MaxHeight(320).Padding(6, 0).Children(func() {
			switch {
			case len(entries) == 0 && !ready:
				ui.Row(c).Gap(8).Padding(12).AlignItems(ui.Center).Children(func() {
					a.spinner(c, 14, p.brand)
					ui.Text(c, loadingText).FontSize(12).TextColor(p.textTertiary)
				})
			case len(entries) == 0:
				ui.Column(c).Padding(28, 16).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(36, 36).Radius(18).Border(1, p.borderSubtle).Background(p.bgPrimary).Center().Children(func() {
						ui.Icon(c, icon("search-slash")).Size(16, 16).TextColor(p.textTertiary)
					})
					ui.Box(c).Height(8)
					ui.Text(c, emptyTitle).FontSize(12).FontWeight(500)
					ui.Text(c, emptyHint).FontSize(11).TextColor(p.textTertiary)
				})
			default:
				group := "\x00"
				for i, e := range entries {
					i, e := i, e
					if e.group != group {
						group = e.group
						ui.Text(c.Key("g:"+group), strings.ToUpper(group)).FontSize(10).FontWeight(500).LetterSpacing(1.6).TextColor(p.placeholder).Padding(4, 16)
					}
					on := i == cm.selected
					b := ui.ButtonBase(c.Key(itoa(i))).Row().Gap(12).AlignItems(ui.Center).Margin(0, 8).Padding(8, 10).Radius(11).FocusRing(false).Children(func() {
						if on {
							ui.Box(c.Key("bar")).Attach(ui.AnchorLeft, ui.AnchorLeft).Width(2).HeightPercent(60).Radius(1).Background(p.brand)
						}
						title := a.highlight(c, e.title, query, 13)
						if e.seed != "" {
							a.avatarSeed(c, e.avatar, e.title, e.seed, 32, 8)
							ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									title()
									ui.Text(c, strings.ToUpper(m("mention_space"))).FontSize(10).LetterSpacing(1.2).TextColor(p.placeholder).Shrink(0)
								})
								ui.Row(c).Gap(4).Children(func() {
									if e.owner != "" {
										ui.Text(c, m("mention_by")+" "+e.owner).FontSize(11).TextColor(p.textTertiary).SingleLine().Shrink(0)
									} else {
										ui.Text(c, m("mention_creator_unavailable")).FontSize(11).TextColor(p.placeholder).SingleLine().Shrink(0)
									}
									if e.desc != "" {
										ui.Text(c, "·").FontSize(11).TextColor(p.placeholder)
										ui.Text(c, e.desc).FontSize(11).TextColor(p.textTertiary).SingleLine().Grow(1)
									}
								})
							})
						} else {
							col, bg, bd := p.textTertiary, p.bgPrimary, p.borderSubtle
							if on {
								col, bg, bd = p.brand, p.brand.Alpha(0.1), p.brand.Alpha(0.25)
							}
							ui.Box(c).Size(28, 28).Radius(8).Border(1, bd).Background(bg).Center().Children(func() {
								ui.Text(c, "/").FontSize(13).FontWeight(600).TextColor(col)
							})
							ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
								ui.Row(c).Gap(8).AlignItems(ui.Baseline).Children(func() {
									ui.Row(c).Children(func() {
										ui.Text(c, "/").FontSize(13).FontWeight(500).TextColor(p.textTertiary)
										title()
									})
									if e.hint != "" {
										ui.Text(c, e.hint).FontSize(11).TextColor(p.textTertiary).SingleLine()
									}
								})
								ui.Text(c, e.desc).FontSize(11).TextColor(p.textTertiary).SingleLine()
							})
						}
						if e.tag != "" {
							a.tag(c, e.tag)
						}
						ui.Box(c).Size(20, 20).Radius(4).Border(1, p.borderSubtle).Center().Opacity(map[bool]float32{true: 1, false: 0}[on]).Children(func() {
							ui.Icon(c, icon("corner-down-left")).Size(12, 12).TextColor(p.textTertiary)
						})
					})
					if on {
						b.Background(p.brand.Alpha(0.07))
					} else if b.Hovered() {
						b.Background(p.hover)
					}
					if b.Hovered() {
						cm.selected = i
					}
					if b.Clicked() {
						e.apply(nil)
					}
				}
			}
		})
	})
	return func(k ui.TextKey) bool {
		if k.Mods != 0 {
			return false
		}
		switch k.Key {
		case ui.KeyDown:
			if len(entries) > 0 {
				cm.selected = (cm.selected + 1) % len(entries)
			}
			return true
		case ui.KeyUp:
			if len(entries) > 0 {
				cm.selected = (cm.selected - 1 + len(entries)) % len(entries)
			}
			return true
		case ui.KeyEscape:
			cm.dismissed = token
			return true
		case ui.KeyTab, ui.KeyEnter:
			if len(entries) == 0 {
				return false
			}
			a.draft = k.Text()
			entries[cm.selected].apply(&k)
			cm.selected = 0
			return true
		}
		return false
	}
}

// setDraft replaces the draft, from a key or a click, and puts the caret
// at caret runes, or at the end for -1.
func (a *app) setDraft(k *ui.TextKey, in ui.Element, s string, caret int) {
	a.draft = s
	if k != nil {
		k.SetText(s)
	}
	if caret < 0 {
		caret = len([]rune(s))
	}
	a.draftCaret = caret
}

func itoa(n int) string { return strconv.Itoa(n) }

// tag is the web's .tag: a small bordered label.
func (a *app) tag(c *ui.Context, s string) {
	p := a.pal
	ui.Box(c).Padding(2, 6).Radius(4).Border(1, p.borderSubtle).Children(func() {
		ui.Text(c, s).FontSize(10).TextColor(p.textTertiary).SingleLine()
	})
}

func (a *app) ctx() context.Context { return context.Background() }

// sortedSpaces is the Spaces joined, the latest used first.
func (a *app) sortedSpaces() []*cohub.Space {
	out := append([]*cohub.Space(nil), a.spaces...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Activity().After(out[j].Activity()) })
	return out
}

// highlight is a title with the query in it in the brand color, as the
// menus' highlightParts, built when called.
func (a *app) highlight(c *ui.Context, s, q string, size float32) func() {
	p := a.pal
	return func() {
		i := strings.Index(strings.ToLower(s), strings.ToLower(strings.TrimSpace(q)))
		if q == "" || i < 0 {
			ui.Text(c, s).FontSize(size).FontWeight(500).SingleLine()
			return
		}
		n := len(strings.TrimSpace(q))
		ui.RichText(c,
			ui.Span{Text: s[:i], Size: size, Weight: 500, Color: p.text},
			ui.Span{Text: s[i : i+n], Size: size, Weight: 500, Color: p.brand},
			ui.Span{Text: s[i+n:], Size: size, Weight: 500, Color: p.text},
		).SingleLine()
	}
}

var mentionLinkRe = regexp.MustCompile(`@\[([^\]\n]+)\]\((cohub://(?:spaces|apps)/[^\s)]+)\)`)

// userText is a message the user wrote, its mentions of Spaces and apps
// as the web's chips: "@name" on the brand's muted color. A click on a
// message that mentions one place opens it.
func (a *app) userText(c *ui.Context, s string) ui.Element {
	locs := mentionLinkRe.FindAllStringSubmatchIndex(s, -1)
	if locs == nil {
		return ui.Text(c, s)
	}
	p := a.pal
	var spans []ui.Span
	last := 0
	for _, l := range locs {
		if l[0] > last {
			spans = append(spans, ui.Span{Text: s[last:l[0]], Color: p.text})
		}
		spans = append(spans, ui.Span{Text: "@" + s[l[2]:l[3]], Weight: 500, Size: 14 * 0.92, Color: p.brandMutedFg,
			Background: p.brandMuted, Radius: 5, PaddingX: 6, PaddingY: 2})
		last = l[1]
	}
	if last < len(s) {
		spans = append(spans, ui.Span{Text: s[last:], Color: p.text})
	}
	e := ui.RichText(c, spans...)
	if len(locs) == 1 {
		e.Cursor(ui.CursorPointer)
	}
	if len(locs) == 1 && e.Clicked() {
		l := locs[0]
		a.openLink(s[l[4]:l[5]])
	}
	return e
}
