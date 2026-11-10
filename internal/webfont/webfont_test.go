package webfont

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// Faces follow @import to the stylesheets of a font service, resolve
// their relative sources, and take a source format this package reads.
func TestFaces(t *testing.T) {
	css := `/* theme */ @import url("https://fonts.example/442/main/result.css");
@import 'https://fonts.example/other.css' screen;
:root { --font-sans: "Maple Mono NF CN"; }`
	if got := Imports(css, ""); len(got) != 2 || got[0] != "https://fonts.example/442/main/result.css" || got[1] != "https://fonts.example/other.css" {
		t.Fatalf("imports %q", got)
	}
	faces := Faces(`@font-face { font-family: "Maple Mono NF CN"; src: url("./a1.woff2") format("woff2"); font-display: swap; unicode-range: U+4E00-4E3F; }
@font-face { font-family: 'Maple Mono NF CN'; font-style: italic; src: local("x"), url(b.eot) format("embedded-opentype"), url(./b.woff) format("woff"); }`, "https://fonts.example/442/main/result.css")
	if len(faces) != 2 || faces[0].URL != "https://fonts.example/442/main/a1.woff2" || faces[0].Family != "Maple Mono NF CN" || faces[0].Style != "normal" {
		t.Fatalf("faces %+v", faces)
	}
	if faces[1].URL != "https://fonts.example/442/main/b.woff" || faces[1].Style != "italic" {
		t.Fatalf("second face %+v", faces[1])
	}
}

// Load fetches the faces of the families asked for, of the normal style,
// and reads them from its directory once it has them.
func TestLoad(t *testing.T) {
	ttf, err := os.ReadFile("../app/fonts/Figtree.ttf")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/fonts/result.css", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`@font-face { font-family: "Fig"; src: url(./fig.ttf) format("truetype"); }
@font-face { font-family: "Fig"; font-style: italic; src: url(./missing.ttf); }
@font-face { font-family: "Other"; src: url(./missing.ttf); }`))
	})
	mux.HandleFunc("/fonts/fig.ttf", func(w http.ResponseWriter, r *http.Request) { w.Write(ttf) })
	srv := httptest.NewServer(mux)
	css := `@import url("` + srv.URL + `/fonts/result.css");`
	dir := t.TempDir()
	faces, err := Load(context.Background(), srv.Client(), css, []string{"fig"}, dir)
	if err != nil || len(faces) != 1 || !bytes.Equal(faces[0].Data, ttf) {
		t.Fatalf("faces %d, %v", len(faces), err)
	}
	srv.Close()
	if _, err := Load(context.Background(), srv.Client(), css, []string{"fig"}, dir); err == nil {
		t.Fatal("loaded the stylesheet from a closed server")
	}
	if b, err := load(context.Background(), srv.Client(), faces[0].URL, dir); err != nil || !bytes.Equal(b, ttf) {
		t.Fatalf("the cached face: %v", err)
	}
}
