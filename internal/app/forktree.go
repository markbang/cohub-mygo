package app

import (
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"cohub-mygo/internal/cohub"
)

// A Space's chats nest under the chat they were forked from, as the
// web's buildSessionForkTree: a fork's row is indented under its parent,
// with the tree's guides, at most four levels deep.

type treeRow struct {
	s           *cohub.Session
	fork        *cohub.SessionFork
	depth       int
	last        bool
	hasChildren bool
	rails       []int
}

const treeMaxDepth = 4

func forkTree(list []*cohub.Session, forks []cohub.SessionFork) []treeRow {
	byID := map[string]*cohub.Session{}
	for _, s := range list {
		byID[s.ID] = s
	}
	forkOf := map[string]*cohub.SessionFork{}
	for i := range forks {
		f := &forks[i]
		if byID[f.ParentSessionID] == nil || byID[f.ChildSessionID] == nil || f.ParentSessionID == f.ChildSessionID {
			continue
		}
		forkOf[f.ChildSessionID] = f
	}
	children := map[string][]*cohub.Session{}
	var roots []*cohub.Session
	for _, s := range list {
		if f := forkOf[s.ID]; f != nil {
			children[f.ParentSessionID] = append(children[f.ParentSessionID], s)
		} else {
			roots = append(roots, s)
		}
	}
	group := map[string]int64{}
	var timeOf func(s *cohub.Session, trail map[string]bool) int64
	timeOf = func(s *cohub.Session, trail map[string]bool) int64 {
		if t, ok := group[s.ID]; ok {
			return t
		}
		t := s.When().UnixMilli()
		if !trail[s.ID] {
			trail[s.ID] = true
			for _, ch := range children[s.ID] {
				t = max(t, timeOf(ch, trail))
			}
			delete(trail, s.ID)
		}
		group[s.ID] = t
		return t
	}
	less := func(xs []*cohub.Session) {
		sort.SliceStable(xs, func(i, j int) bool {
			a, b := timeOf(xs[i], map[string]bool{}), timeOf(xs[j], map[string]bool{})
			if a != b {
				return a > b
			}
			return xs[i].ID > xs[j].ID
		})
	}
	type raw struct {
		s     *cohub.Session
		depth int
	}
	var rows []raw
	placed := map[string]bool{}
	var add func(s *cohub.Session, depth int)
	add = func(s *cohub.Session, depth int) {
		if placed[s.ID] {
			return
		}
		placed[s.ID] = true
		rows = append(rows, raw{s, depth})
		kids := children[s.ID]
		less(kids)
		for _, ch := range kids {
			add(ch, depth+1)
		}
	}
	less(roots)
	for _, r := range roots {
		add(r, 0)
	}
	for _, s := range list {
		add(s, 0)
	}
	out := make([]treeRow, len(rows))
	next := make([]int, treeMaxDepth+1)
	for i := range next {
		next[i] = -1
	}
	nextDepth := 0
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		d := min(r.depth, treeMaxDepth)
		row := treeRow{s: r.s, depth: d, last: next[d] != d, hasChildren: nextDepth > d}
		if r.depth > 0 {
			row.fork = forkOf[r.s.ID]
		}
		for level := 1; level < d; level++ {
			if next[level] == level {
				row.rails = append(row.rails, level)
			}
		}
		out[i] = row
		for level := d; level <= treeMaxDepth; level++ {
			next[level] = d
		}
		nextDepth = d
	}
	return out
}

// treeTitle is a fork's title: what was asked after forking, while the
// fork has the parent's title.
func (r treeRow) title() string {
	if f := r.fork; f != nil && f.FirstUserTextAfterFork != nil {
		own := strings.TrimSpace(r.s.Title)
		parent := ""
		if f.ParentTitle != nil {
			parent = strings.TrimSpace(*f.ParentTitle)
		}
		if t := strings.Join(strings.Fields(*f.FirstUserTextAfterFork), " "); t != "" && (own == "" || own == parent) {
			if r := []rune(t); len(r) > 48 {
				t = string(r[:48])
			}
			return t
		}
	}
	return chatTitle(r.s)
}

// treeGuides draws a row's guides, as SessionRow's .tree: the rails of
// the levels going on past it, its elbow, and the stem to its forks.
func (a *app) treeGuides(c *ui.Context, r treeRow) {
	if r.depth == 0 && !r.hasChildren {
		return
	}
	col := a.pal.border
	const step, origin, titleY, lw = 12, 11, 15, 1.5
	railX := func(level int) float32 { return float32(origin + (level-1)*step) }
	textX := float32(origin + (r.depth-1)*step + 7)
	ui.Box(c.Key("tree")).Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).FillWidth().HeightPercent(100).PassThrough().Draw(func(pt *ui.Painter, b ui.Rect) {
		line := func(x, y0, y1 float32) { pt.Fill(ui.Rect{X: b.X + x, Y: b.Y + y0, W: lw, H: y1 - y0}, col, 0) }
		for _, l := range r.rails {
			line(railX(l), 0, b.H)
		}
		if r.depth > 0 {
			x := railX(r.depth)
			if r.last {
				// The elbow: down to the title, then across to it.
				const rad = 4
				x += lw / 2
				p := &ui.Path{}
				p.MoveTo(b.X+x, b.Y).LineTo(b.X+x, b.Y+titleY-rad).QuadTo(b.X+x, b.Y+titleY, b.X+x+rad, b.Y+titleY).LineTo(b.X+textX-3, b.Y+titleY)
				pt.StrokePath(p, lw, col)
			} else {
				line(x, 0, b.H)
			}
		}
		if r.hasChildren {
			line(railX(r.depth+1), b.H-4, b.H)
		}
	})
}
