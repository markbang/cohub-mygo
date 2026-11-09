package main

import (
	"net/url"

	"github.com/egoist/mygo/ui"
)

// A Space whose runtime is a computer of the user's (Cohub Local) runs
// chats with the harnesses installed there (Pi, Codex, Claude Code) and
// their models, as ComposerHarnessPicker and the local model selector:
// the composer chooses the harness, its model and, for Codex and Claude,
// how much it may do without asking.

type runtimeInfo struct {
	Kind         string `json:"kind"`
	Online       bool   `json:"online"`
	Capabilities *struct {
		Harnesses     []string            `json:"harnesses"`
		ApprovalModes map[string][]string `json:"approvalModes"`
		Models        []struct {
			Harness  string `json:"harness"`
			Provider string `json:"provider"`
			ID       string `json:"id"`
			Name     string `json:"name"`
		} `json:"models"`
	} `json:"capabilities"`
}

// harnessState is what the composer chose for a chat or a Space's new one.
type harnessState struct {
	harness, provider, model, approval string
}

func (a *app) runtimeOf(spaceID string) *runtimeInfo {
	if spaceID == "" {
		return nil
	}
	r, _, _ := loadJSON[runtimeInfo](a, "runtime:"+spaceID, "/api/spaces/"+url.PathEscape(spaceID)+"/runtime")
	return r
}

func (a *app) composerKey() string {
	if id := a.route.sessionID(); id != "" {
		return id
	}
	return "space:" + a.currentSpace()
}

// harnessChoice is the composer's harness: the one chosen, else the
// chat's last, else Cohub's own.
func (a *app) harnessChoice() harnessState {
	if h, ok := a.harnesses[a.composerKey()]; ok {
		return h
	}
	h := harnessState{harness: a.composerHarness()}
	if h.harness == "" {
		h.harness = "cohub"
	}
	if tl := a.timelines[a.route.sessionID()]; tl != nil && h.harness != "cohub" {
		for i := len(tl.turns) - 1; i >= 0; i-- {
			if t := tl.turns[i]; t.Meta.Harness == h.harness && t.Model != "" {
				h.provider, h.model = t.Provider, t.Model
				break
			}
		}
	}
	return h
}

var approvalLabels = map[string]string{
	"inherit": "runtime_approval_inherit", "auto": "runtime_approval_auto",
	"full_access": "runtime_approval_full_access", "read_only": "runtime_approval_read_only",
}

// harnessPicker is the composer's harness, local model and approval chips.
// It shows only for a Space with a local runtime, or a chat that ran on one.
func (a *app) harnessPicker(c *ui.Context) {
	rt := a.runtimeOf(a.currentSpace())
	cur := a.harnessChoice()
	local := rt != nil && rt.Kind == "local" && rt.Capabilities != nil
	if !local && cur.harness == "cohub" {
		return
	}
	key := a.composerKey()
	set := func(h harnessState) { a.harnesses[key] = h }
	hb := a.pill(c.Key("harness"), harnessName(cur.harness), "", func() { a.harnessIcon(c, cur.harness, 14) })
	if hb.Clicked() && local {
		a.menus["harness"] = !a.menus["harness"]
	}
	if local {
		open := a.menus["harness"]
		items := []menuItem{{label: harnessName("cohub"), checked: cur.harness == "cohub", run: func() { set(harnessState{harness: "cohub"}) }}}
		for _, h := range rt.Capabilities.Harnesses {
			h := h
			items = append(items, menuItem{label: harnessName(h), checked: cur.harness == h, run: func() {
				next := harnessState{harness: h}
				for _, md := range rt.Capabilities.Models {
					if md.Harness == h {
						next.provider, next.model = md.Provider, md.ID
						break
					}
				}
				set(next)
			}})
		}
		if !rt.Online {
			items[len(items)-1].divider = true
			items = append(items, menuItem{label: m("x_runtime_offline")})
		}
		a.menuWidth(c, hb, &open, 200, items)
		a.menus["harness"] = open
	}
	if cur.harness == "cohub" || !local {
		return
	}
	name := cur.model
	for _, md := range rt.Capabilities.Models {
		if md.Harness == cur.harness && md.ID == cur.model && md.Name != "" {
			name = md.Name
		}
	}
	if name == "" {
		name = m("runtime_model")
	}
	mb := a.pill(c.Key("local-model"), name, "", nil)
	if mb.Clicked() {
		a.menus["local-model"] = !a.menus["local-model"]
	}
	open := a.menus["local-model"]
	var items []menuItem
	for _, md := range rt.Capabilities.Models {
		if md.Harness != cur.harness {
			continue
		}
		md := md
		label := md.Name
		if label == "" {
			label = md.ID
		}
		items = append(items, menuItem{label: label, hint: md.Provider, checked: md.ID == cur.model, run: func() {
			next := cur
			next.provider, next.model = md.Provider, md.ID
			set(next)
		}})
	}
	a.menuWidth(c, mb, &open, 300, items)
	a.menus["local-model"] = open
	modes := rt.Capabilities.ApprovalModes[cur.harness]
	if len(modes) == 0 {
		return
	}
	approval := cur.approval
	if approval == "" {
		approval = modes[0]
	}
	ab := a.pill(c.Key("approval"), m(approvalLabels[approval]), "", func() {
		ui.Icon(c, icon("shield")).Size(13, 13).TextColor(a.pal.textTertiary)
	})
	if ab.Clicked() {
		a.menus["approval"] = !a.menus["approval"]
	}
	open = a.menus["approval"]
	items = items[:0]
	for _, md := range modes {
		md := md
		items = append(items, menuItem{label: m(approvalLabels[md]), checked: md == approval, run: func() {
			next := cur
			next.approval = md
			set(next)
		}})
	}
	a.menuWidth(c, ab, &open, 200, items)
	a.menus["approval"] = open
}
