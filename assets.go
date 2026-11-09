package main

import (
	"embed"
	"log"
	"strings"

	"github.com/egoist/mygo/ui"
)

//go:embed icons/*.svg
var iconFiles embed.FS

//go:embed fonts/*.ttf
var fontFiles embed.FS

// Icons are Lucide's (ISC), the set the web app draws with lucide-svelte.
var icons = map[string]*ui.SVG{}

func icon(name string) *ui.SVG {
	s, ok := icons[name]
	if !ok {
		panic("unknown icon " + name)
	}
	return s
}

func init() {
	entries, err := iconFiles.ReadDir("icons")
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range entries {
		b, _ := iconFiles.ReadFile("icons/" + e.Name())
		icons[strings.TrimSuffix(e.Name(), ".svg")] = ui.MustParseSVG(b)
	}
	for _, f := range []struct{ file, family string }{
		{"Geist-Regular.ttf", "Geist"},
		{"Geist-Medium.ttf", "Geist"},
		{"Geist-SemiBold.ttf", "Geist"},
		{"GeistMono-Regular.ttf", "Geist Mono"},
	} {
		b, _ := fontFiles.ReadFile("fonts/" + f.file)
		if err := ui.RegisterFont(b, f.family); err != nil {
			log.Printf("font %s: %v", f.file, err)
		}
	}
}
