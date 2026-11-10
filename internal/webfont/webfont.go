// Package webfont loads the web fonts of a stylesheet, as a browser would
// for its @font-face rules and those of the stylesheets it @imports, for an
// app that draws text itself: each face decoded from WOFF2 or WOFF to the
// TrueType or OpenType data it can register, and kept on disk.
package webfont

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/tdewolff/font"
)

// A Face is a font of an @font-face rule.
type Face struct {
	Family string
	Style  string // normal or italic
	URL    string
	Data   []byte // TrueType or OpenType, once loaded
}

var (
	comment  = regexp.MustCompile(`(?s)/\*.*?\*/`)
	importRe = regexp.MustCompile(`@import\s+(?:url\(\s*)?["']?([^"')\s;]+)["']?\s*\)?[^;]*;`)
	faceRe   = regexp.MustCompile(`(?s)@font-face\s*\{(.*?)\}`)
	declRe   = regexp.MustCompile(`(?s)([a-z-]+)\s*:\s*([^;]+);?`)
	srcRe    = regexp.MustCompile(`url\(\s*["']?([^"')]+)["']?\s*\)(?:\s*format\(\s*["']?([^"')]+)["']?\s*\))?`)
)

// Imports returns the stylesheets css imports, resolved against base.
func Imports(css, base string) []string {
	var out []string
	for _, m := range importRe.FindAllStringSubmatch(comment.ReplaceAllString(css, ""), -1) {
		if u := resolve(base, m[1]); u != "" {
			out = append(out, u)
		}
	}
	return out
}

// Faces returns the faces of css's @font-face rules, each with the first
// of its sources a browser would take that this package reads.
func Faces(css, base string) []Face {
	var out []Face
	for _, m := range faceRe.FindAllStringSubmatch(comment.ReplaceAllString(css, ""), -1) {
		f := Face{Style: "normal"}
		for _, d := range declRe.FindAllStringSubmatch(m[1], -1) {
			v := strings.TrimSpace(d[2])
			switch d[1] {
			case "font-family":
				f.Family = strings.Trim(v, `"' `)
			case "font-style":
				f.Style = v
			case "src":
				for _, s := range srcRe.FindAllStringSubmatch(v, -1) {
					switch s[2] {
					case "", "woff2", "woff", "truetype", "opentype":
						if f.URL == "" {
							f.URL = resolve(base, s[1])
						}
					}
				}
			}
		}
		if f.Family != "" && f.URL != "" {
			out = append(out, f)
		}
	}
	return out
}

func resolve(base, ref string) string {
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	if b, err := url.Parse(base); err == nil && base != "" {
		u = b.ResolveReference(u)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return ""
	}
	return u.String()
}

// Load returns the faces of the families, of the normal style, that css
// and the stylesheets it imports declare, with their data: decoded from
// dir, where it keeps what it fetches, or else fetched.
func Load(ctx context.Context, client *http.Client, css string, families []string, dir string) ([]Face, error) {
	want := map[string]bool{}
	for _, f := range families {
		want[strings.ToLower(f)] = true
	}
	faces := Faces(css, "")
	for _, u := range Imports(css, "") {
		b, err := get(ctx, client, u)
		if err != nil {
			return nil, err
		}
		faces = append(faces, Faces(string(b), u)...)
	}
	var keep []Face
	for _, f := range faces {
		if want[strings.ToLower(f.Family)] && f.Style == "normal" {
			keep = append(keep, f)
		}
	}
	var wg sync.WaitGroup
	errs := make([]error, len(keep))
	sem := make(chan struct{}, 8)
	for i := range keep {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			keep[i].Data, errs[i] = load(ctx, client, keep[i].URL, dir)
		}()
	}
	wg.Wait()
	out := keep[:0]
	for i, f := range keep {
		if errs[i] != nil {
			return nil, errs[i]
		}
		out = append(out, f)
	}
	return out, nil
}

func load(ctx context.Context, client *http.Client, u, dir string) ([]byte, error) {
	sum := sha256.Sum256([]byte(u))
	path := filepath.Join(dir, hex.EncodeToString(sum[:12])+".otf")
	if b, err := os.ReadFile(path); err == nil {
		return b, nil
	}
	b, err := get(ctx, client, u)
	if err != nil {
		return nil, err
	}
	sfnt, err := font.ToSFNT(b)
	if err != nil {
		return nil, fmt.Errorf("webfont %s: %w", u, err)
	}
	if os.MkdirAll(dir, 0o755) == nil {
		os.WriteFile(path, sfnt, 0o644)
	}
	return sfnt, nil
}

func get(ctx context.Context, client *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webfont %s: HTTP %d", u, res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 32<<20))
}
