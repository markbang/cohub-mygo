package app

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// prefs are what the app keeps on this device, as the web keeps them in
// localStorage: the theme, and whether the Home Space's theme applies.
type prefs struct {
	path        string
	Theme       string `json:"theme,omitempty"`
	Lang        string `json:"lang,omitempty"`
	CustomTheme *bool  `json:"customTheme,omitempty"`
	Danmaku     *bool  `json:"danmaku,omitempty"` // live messages over the Space
	// The panes' widths, once the user dragged their edges.
	SidebarWidth float32 `json:"sidebarWidth,omitempty"`
	PanelWidth   float32 `json:"panelWidth,omitempty"`
	MainWidth    float32 `json:"mainWidth,omitempty"`
	// The chat and Space the window last showed, so that a launch opens
	// where the user left, as a native app restores its window.
	LastSession string `json:"lastSession,omitempty"`
	LastSpace   string `json:"lastSpace,omitempty"`
}

func (p *prefs) danmaku() bool { return p.Danmaku == nil || *p.Danmaku }

func (p *prefs) load() {
	if b, err := os.ReadFile(p.path); err == nil {
		json.Unmarshal(b, p)
	}
}

func (p *prefs) save() {
	if p.path == "" {
		return
	}
	b, _ := json.MarshalIndent(p, "", "  ")
	os.MkdirAll(filepath.Dir(p.path), 0o755)
	os.WriteFile(p.path, b, 0o644)
}

func (a *app) setTheme(name string) {
	a.theme = name
	a.prefs.Theme = name
	a.prefs.save()
}
