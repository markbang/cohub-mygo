package main

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
}

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
