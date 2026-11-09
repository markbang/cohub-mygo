package main

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// A save of the workspace, as the checkpoint explorer: what it changed
// against its parent, file by file with the diff, the files it holds, and
// a new Space from it.

type checkpointDetail struct {
	Checkpoint struct {
		ID                 string    `json:"id"`
		Description        string    `json:"description"`
		CommitHash         string    `json:"commitHash"`
		ParentCheckpointID *string   `json:"parentCheckpointId"`
		ForkCount          int       `json:"forkCount"`
		CreatedAt          time.Time `json:"createdAt"`
		Meta               struct {
			Stats *struct {
				FileCount        int64 `json:"fileCount"`
				ChangedFileCount int64 `json:"changedFileCount"`
				FileBytes        int64 `json:"fileBytes"`
			} `json:"stats"`
		} `json:"meta"`
	} `json:"checkpoint"`
}

type diffFile struct {
	Path      string  `json:"path"`
	OldPath   *string `json:"oldPath"`
	Status    string  `json:"status"` // A, M, D, R
	Binary    bool    `json:"binary"`
	Additions int     `json:"additions"`
	Deletions int     `json:"deletions"`
}

type diffSummary struct {
	Files []diffFile `json:"files"`
	Stats struct {
		Additions        int `json:"additions"`
		Deletions        int `json:"deletions"`
		ChangedFileCount int `json:"changedFileCount"`
	} `json:"stats"`
	Truncated bool `json:"truncated"`
}

type diffPatch struct {
	Kind  string `json:"kind"`
	Lines []struct {
		Text string `json:"text"`
		Type string `json:"type"` // meta, hunk, add, del, context
	} `json:"lines"`
}

func (a *app) checkpointDetail(c *ui.Context, spaceID, id string) {
	p := a.pal
	base := sp(spaceID) + "/checkpoints/" + url.PathEscape(id)
	d, err, ok := loadJSON[checkpointDetail](a, "cp:"+id, base)
	if !ok || err != nil {
		a.loadingBlock(c, err, 200)
		return
	}
	cp := d.Checkpoint
	tab := a.cpTab[id]
	ui.Scroll(c.Key("cp-" + id)).Grow(1).Children(func() {
		ui.Column(c).Padding(20).Gap(20).Children(func() {
			ui.Column(c.Key("head")).Gap(10).Padding(0, 0, 18, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle.Alpha(0.7)).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(4, 8).Radius(99).Background(p.brandMuted).Children(func() {
						ui.Icon(c, icon("git-commit-horizontal")).Size(12, 12).TextColor(p.brand)
						ui.Text(c, strings.ToUpper(m("common_save"))).FontSize(10).FontWeight(500).LetterSpacing(0.6).TextColor(p.brand)
					})
					ui.Text(c, shortDateTime(cp.CreatedAt.Local())).FontSize(11).Font(fontMono).TextColor(p.placeholder)
				})
				title := cp.Description
				if title == "" {
					title = cp.CommitHash[:min(12, len(cp.CommitHash))]
				}
				ui.Text(c, title).FontSize(22).FontWeight(600).LetterSpacing(-0.3)
				ui.Row(c).Gap(12).Wrap().Children(func() {
					a.copyChip(c, "id", cp.ID, cp.ID)
					a.copyChip(c, "hash", cp.CommitHash[:min(12, len(cp.CommitHash))], cp.CommitHash)
				})
				nb := ui.ButtonBase(c.Key("new-space")).Row().Gap(6).AlignItems(ui.Center).AlignSelf(ui.Start).Height(34).Padding(0, 12).Radius(6).Background(p.brand).FocusRing(false).Children(func() {
					ui.Icon(c, icon("rocket")).Size(14, 14).TextColor(p.brandFg)
					ui.Text(c, m("cp_new_space")).FontSize(12).FontWeight(500).TextColor(p.brandFg)
				})
				if nb.Clicked() {
					a.newSpace.checkpoint = cp.ID
					a.go_(route{kind: routeNewSpace})
				}
			})
			ui.Row(c.Key("tabs")).Gap(20).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle.Alpha(0.7)).Children(func() {
				for i, l := range []string{m("resource_app_changes"), m("resource_app_files")} {
					on := tab == i
					b := ui.ButtonBase(c.Key(l)).Height(34).Center().FocusRing(false).Children(func() {
						col := p.textTertiary
						if on {
							col = p.text
						}
						ui.Text(c, l).FontSize(12).FontWeight(500).TextColor(col)
					})
					if on {
						b.BorderWidth(0, 0, 2, 0).BorderColor(p.brand)
					}
					idx := i
					if b.Clicked() {
						a.cpTab[id] = idx
					}
				}
			})
			if tab == 1 {
				a.checkpointFiles(c, spaceID, id, "", 0)
				return
			}
			if s := cp.Meta.Stats; s != nil {
				ui.Row(c.Key("stats")).Gap(12).Children(func() {
					for _, kv := range [][2]string{{m("cp_files"), groupThousands(fmt.Sprint(s.FileCount))}, {m("cp_changed"), groupThousands(fmt.Sprint(s.ChangedFileCount))}, {m("cp_size"), byteSize(s.FileBytes)}} {
						ui.Column(c.Key(kv[0])).Grow(1).Basis(0).Gap(4).Children(func() {
							ui.Text(c, strings.ToUpper(kv[0])).FontSize(10).LetterSpacing(0.6).TextColor(p.placeholder)
							ui.Text(c, kv[1]).FontSize(15).FontWeight(600).Font(fontMono)
						})
					}
				})
			}
			a.checkpointDiff(c, spaceID, id, cp.ParentCheckpointID != nil)
		})
	})
}

// copyChip is CopyButton: mono text that copies a value.
func (a *app) copyChip(c *ui.Context, key, text, value string) {
	p := a.pal
	b := ui.Row(c.Key("copy-" + key)).Gap(4).AlignItems(ui.Center).Cursor(ui.CursorPointer).Children(func() {
		ui.Text(c, text).FontSize(11).Font(fontMono).TextColor(p.placeholder)
		ic := "copy"
		if a.copiedValue == value && time.Since(a.copiedAt) < 2*time.Second {
			ic = "check"
		}
		ui.Icon(c, icon(ic)).Size(12, 12).TextColor(p.placeholder)
	})
	if b.Clicked() {
		c.WriteClipboard(value)
		a.copiedAt, a.copiedValue = time.Now(), value
	}
}

func (a *app) checkpointDiff(c *ui.Context, spaceID, id string, hasParent bool) {
	p := a.pal
	base := sp(spaceID) + "/checkpoints/" + url.PathEscape(id)
	sum, err, ok := loadJSON[diffSummary](a, "cp-diff:"+id, base+"/fs/diff")
	ui.Column(c.Key("diff")).Gap(8).Children(func() {
		if hasParent {
			ui.Text(c, m("cp_vs_parent")).FontSize(12).TextColor(p.placeholder)
		}
		switch {
		case !ok:
			a.spinner(c, 14, p.textTertiary)
			return
		case err != nil:
			ui.Text(c, err.Error()).FontSize(12).TextColor(p.errorC)
			return
		case len(sum.Files) == 0:
			msg := m("cp_initial_save")
			if hasParent {
				msg = m("cp_no_changes_from_parent")
			}
			ui.Text(c, msg).FontSize(12).TextColor(p.textTertiary)
			return
		}
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, fmt.Sprintf("%d %s", sum.Stats.ChangedFileCount, m("cp_changed"))).FontSize(12).TextColor(p.textSecond)
			ui.Text(c, fmt.Sprintf("+%d", sum.Stats.Additions)).FontSize(12).Font(fontMono).TextColor(p.success)
			ui.Text(c, fmt.Sprintf("−%d", sum.Stats.Deletions)).FontSize(12).Font(fontMono).TextColor(p.errorC)
		})
		files := append([]diffFile(nil), sum.Files...)
		sort.SliceStable(files, func(i, j int) bool { return files[i].Path < files[j].Path })
		ui.Column(c).Radius(6).Border(1, p.borderSubtle).Clip().Children(func() {
			for i, f := range files {
				key := "cp-file:" + id + ":" + f.Path
				open := a.groupsOpen[key]
				row := ui.Row(c.Key(f.Path)).Gap(8).AlignItems(ui.Center).Padding(7, 10).Cursor(ui.CursorPointer)
				if i > 0 {
					row.BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle)
				}
				row.Children(func() {
					ch := "chevron-right"
					if open {
						ch = "chevron-down"
					}
					ui.Icon(c, icon(ch)).Size(12, 12).TextColor(p.textTertiary)
					col := map[string]ui.Color{"A": p.success, "D": p.errorC, "R": p.info}[f.Status]
					if col == (ui.Color{}) {
						col = p.warning
					}
					ui.Text(c, f.Status).FontSize(10).FontWeight(600).Font(fontMono).TextColor(col).Width(12)
					ui.Text(c, f.Path).FontSize(12).Font(fontMono).TextColor(p.textSecond).SingleLine().Grow(1).MinWidth(0)
					if f.Binary {
						ui.Text(c, "binary").FontSize(10).TextColor(p.placeholder)
					} else {
						ui.Text(c, fmt.Sprintf("+%d", f.Additions)).FontSize(11).Font(fontMono).TextColor(p.success)
						ui.Text(c, fmt.Sprintf("−%d", f.Deletions)).FontSize(11).Font(fontMono).TextColor(p.errorC)
					}
				})
				if row.Hovered() {
					row.Background(p.hover)
				}
				if row.Clicked() {
					a.groupsOpen[key] = !open
				}
				if open && !f.Binary {
					a.diffLines(c, base, id, f.Path)
				}
			}
		})
	})
}

func (a *app) diffLines(c *ui.Context, base, id, path string) {
	p := a.pal
	patch, err, ok := loadJSON[diffPatch](a, "cp-patch:"+id+":"+path, base+"/fs/diff/file?path="+url.QueryEscape(path))
	ui.Column(c.Key("patch-"+path)).BorderWidth(1, 0, 0, 0).BorderColor(p.borderSubtle).Background(p.bgCode).Children(func() {
		switch {
		case !ok:
			ui.Row(c).Padding(8).Children(func() { a.spinner(c, 12, p.textTertiary) })
			return
		case err != nil:
			ui.Text(c, err.Error()).FontSize(11).TextColor(p.errorC).Padding(8)
			return
		}
		ui.ScrollHorizontal(c.Key("h")).Children(func() {
			ui.Column(c).Padding(4, 0).Children(func() {
				for i, l := range patch.Lines {
					if l.Type == "meta" {
						continue
					}
					bg, fg, mark := ui.Color{}, p.textSecond, " "
					switch l.Type {
					case "add":
						bg, fg, mark = p.success.Alpha(0.12), p.text, "+"
					case "del":
						bg, fg, mark = p.errorC.Alpha(0.12), p.text, "−"
					case "hunk":
						fg = p.info
					}
					text := l.Text
					if (l.Type == "add" || l.Type == "del" || l.Type == "context") && len(text) > 0 && (text[0] == '+' || text[0] == '-' || text[0] == ' ') {
						text = text[1:]
					}
					r := ui.Row(c.Key(i)).Padding(0, 10).Children(func() {
						if l.Type != "hunk" {
							ui.Text(c, mark).FontSize(11.5).Font(fontMono).TextColor(fg.Alpha(0.6)).Width(14)
						}
						ui.Text(c, text).FontSize(11.5).Font(fontMono).LineHeight(1.55).TextColor(fg).SingleLine()
					})
					if bg != (ui.Color{}) {
						r.Background(bg)
					}
				}
			})
		})
	})
}

// checkpointFiles is the save's tree; a file opens below its row.
func (a *app) checkpointFiles(c *ui.Context, spaceID, id, dir string, depth int) {
	p := a.pal
	base := sp(spaceID) + "/checkpoints/" + url.PathEscape(id)
	q := url.Values{"pagination": {"true"}}
	if dir != "" {
		q.Set("path", dir)
	}
	page, err, ok := loadJSON[struct {
		Entries []fsEntry `json:"entries"`
	}](a, "cp-tree:"+id+":"+dir, base+"/fs/tree?"+q.Encode())
	switch {
	case !ok:
		ui.Row(c.Key("loading-"+dir)).Padding(4, 8+float32(depth)*14).Children(func() { a.spinner(c, 12, p.textTertiary) })
		return
	case err != nil:
		ui.Text(c.Key("err-"+dir), err.Error()).FontSize(11).TextColor(p.errorC)
		return
	}
	entries := append([]fsEntry(nil), page.Entries...)
	sort.SliceStable(entries, func(i, j int) bool {
		if (entries[i].Type == "dir") != (entries[j].Type == "dir") {
			return entries[i].Type == "dir"
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	for _, e := range entries {
		key := "cp-fs:" + id + ":" + e.Path
		open := a.groupsOpen[key]
		isDir := e.Type == "dir"
		row := ui.Row(c.Key(e.Path)).Gap(6).AlignItems(ui.Center).Height(26).Padding(0, 6, 0, 6+float32(depth)*14).Radius(5).Cursor(ui.CursorPointer).Children(func() {
			ic := "file"
			if isDir {
				ic = "chevron-right"
				if open {
					ic = "chevron-down"
				}
			}
			ui.Icon(c, icon(ic)).Size(12, 12).TextColor(p.textTertiary)
			ui.Text(c, e.Name).FontSize(12).TextColor(p.textSecond).SingleLine().Grow(1)
			if !isDir {
				ui.Text(c, byteSize(e.Size)).FontSize(10).TextColor(p.placeholder)
			}
		})
		a.pressable(row, open && !isDir)
		if row.Clicked() {
			a.groupsOpen[key] = !open
		}
		if !open {
			continue
		}
		if isDir {
			a.checkpointFiles(c, spaceID, id, e.Path, depth+1)
			continue
		}
		f, ferr, fok := loadJSON[fsFile](a, "cp-file-body:"+id+":"+e.Path, base+"/fs/file?path="+url.QueryEscape(e.Path))
		ui.Column(c.Key("body-"+e.Path)).Margin(2, 0, 6, 6+float32(depth)*14).Radius(6).Background(p.bgCode).Padding(8, 10).Children(func() {
			switch {
			case !fok:
				a.spinner(c, 12, p.textTertiary)
			case ferr != nil:
				ui.Text(c, ferr.Error()).FontSize(11).TextColor(p.errorC)
			case f.Kind != "text":
				ui.Text(c, f.MimeType+" · "+byteSize(f.Size)).FontSize(11).TextColor(p.textTertiary)
			default:
				body := f.Content
				if len(body) > 20000 {
					body = body[:20000] + "\n…"
				}
				ui.Text(c, body).FontSize(11.5).Font(fontMono).LineHeight(1.55).TextColor(p.textSecond).Selectable()
			}
		})
	}
}
