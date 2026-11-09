package main

import (
	"encoding/json"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func demoApp(t *testing.T) *app {
	t.Helper()
	a := newApp()
	a.seenPath = ""
	a.seen = map[string]time.Time{}
	fixed := time.Date(2026, 11, 9, 8, 0, 0, 0, time.Local)
	a.now = func() time.Time { return fixed }
	loadDemo(a)
	return a
}

func writePNG(t *testing.T, tt *ui.Tester, path string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

// snapshot writes the tester's frame to $SNAPSHOT_DIR/name.png when set.
func snapshot(t *testing.T, tt *ui.Tester, name string) {
	if dir := os.Getenv("SNAPSHOT_DIR"); dir != "" {
		writePNG(t, tt, filepath.Join(dir, name+".png"))
	}
}

func TestRoutes(t *testing.T) {
	for _, path := range []string{
		"/sessions", "/sessions/new", "/sessions/abc", "/spaces", "/spaces/new", "/spaces/s1",
		"/spaces/s1/sessions/abc", "/spaces/s1/sessions/new", "/spaces/s1/settings", "/spaces/s1/activity",
		"/spaces/s1/wallet", "/spaces/s1/files/docs/a.md", "/spaces/s1/checkpoints/new", "/spaces/s1/tasks/t1",
		"/spaces/s1/cronjobs/c1", "/settings/general", "/settings/billing",
	} {
		r := parseRoute(path)
		if got := r.String(); got != path && !(path == "/spaces/s1/sessions/new" && got == "/spaces/s1/sessions/new") {
			t.Errorf("%s parses to %+v, which is %s", path, r, got)
		}
	}
	if r := parseRoute("/spaces/s1/settings#access"); r.sub != "access" {
		t.Errorf("settings section %q", r.sub)
	}
}

func TestDesktopOpensLatestChat(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	if a.route.sessionID() != "demo-0" {
		t.Fatalf("route %v, want the latest chat", a.route)
	}
	for _, want := range []string{"对话", "全部", "看看这个 https://nsh.dev/", "消息", "本对话", "bangwu"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	snapshot(t, tt, "desktop-chat-light")
}

func TestChipFiltersChats(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	if err := tt.Click("cohub-mobile"); err != nil {
		t.Fatal(err)
	}
	if a.chip != "s2" || len(a.visibleSessions()) != 4 {
		t.Fatalf("chip %q shows %d chats", a.chip, len(a.visibleSessions()))
	}
}

func TestSpacesDirectoryOpensSpace(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	if err := tt.Click("Space"); err != nil {
		t.Fatal(err)
	}
	if a.route.kind != routeSpaces || !tt.HasText("全部 Space") || !tt.HasText("Personal Home Space") {
		t.Fatalf("route %v, texts %q", a.route, tt.Texts())
	}
	snapshot(t, tt, "desktop-spaces-light")
	if err := tt.Click("Personal Home Space"); err != nil {
		t.Fatal(err)
	}
	if a.route.kind != routeSpace || a.route.space != "s5" || !tt.HasText("新建对话") || !tt.HasText("Created by") {
		t.Fatalf("route %v, texts %q", a.route, tt.Texts())
	}
	snapshot(t, tt, "desktop-space-light")
}

func TestPaletteOpensSpace(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	tt.TypeKey(ui.Cmd, ui.KeyK, "")
	if !a.palette.shown || !tt.HasText("Run Command") {
		t.Fatalf("palette %v, texts %q", a.palette.shown, tt.Texts())
	}
	tt.Type("worldloom")
	tt.TypeKey(0, ui.KeyEnter, "")
	if a.palette.shown || a.route.kind != routeSpace || a.route.space != "s6" {
		t.Fatalf("route %v after choosing", a.route)
	}
}

func TestDarkTheme(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTesterWith(a.view, 1440, 900, ui.TesterOptions{Dark: true})
	if !a.pal.dark {
		t.Fatal("palette is not dark in a dark window")
	}
	snapshot(t, tt, "desktop-chat-dark")
}

func TestCompactLayout(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 390, 844)
	if a.route.sessionID() != "" {
		t.Fatalf("compact layout opened %v by itself", a.route)
	}
	for _, want := range []string{"对话", "Space", "账户", "修复详情页按钮放大"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q", want)
		}
	}
	snapshot(t, tt, "compact-chats")
	if err := tt.Click("看看这个 https://nsh.dev/"); err != nil {
		t.Fatal(err)
	}
	if a.route.sessionID() != "demo-0" || !tt.HasText("消息") {
		t.Fatalf("route %v", a.route)
	}
	snapshot(t, tt, "compact-chat")
}

func TestComposerSendsWithEnter(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	if err := tt.Click("消息"); err != nil {
		t.Fatal(err)
	}
	tt.Type("你好")
	if a.draft != "你好" {
		t.Fatalf("draft %q", a.draft)
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	// Demo mode has no API: the text stays, with why it was not sent.
	if a.sendErr == "" || a.draft != "你好" {
		t.Fatalf("draft %q, error %q", a.draft, a.sendErr)
	}
}

func TestPreviewDropsMarkdown(t *testing.T) {
	for in, want := range map[string]string{
		"## 执行结果 **结论**: 76 个":         "执行结果 结论: 76 个",
		"`impeccable` 其实没丢":            "impeccable 其实没丢",
		"见 [PR #12](https://x.dev/12)": "见 PR #12",
	} {
		if got := preview(in); got != want {
			t.Errorf("preview(%q) = %q, want %q", in, got, want)
		}
	}
}

// compareState is a state of compare/states.json, which the web's
// screenshots (tools/webshot) and the app's are taken in.
type compareState struct {
	Name    string           `json:"name"`
	Path    string           `json:"path"`
	Size    []int            `json:"size"`
	Dark    bool             `json:"dark"`
	Custom  bool             `json:"custom"`
	Actions []map[string]any `json:"actions"`
	App     []map[string]any `json:"app"`
}

// TestCompare renders the states of compare/states.json with the signed-in
// account into compare/app, and puts each beside the web's in compare/side:
//
//	COHUB_LIVE=1 go test -run Compare [-compare regexp]
func TestCompare(t *testing.T) {
	if os.Getenv("COHUB_LIVE") == "" {
		t.Skip("COHUB_LIVE not set")
	}
	var states []compareState
	var vars map[string]string
	b, _ := os.ReadFile("compare/states.json")
	if err := json.Unmarshal(b, &states); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile("compare/vars.json")
	json.Unmarshal(b, &vars)
	fill := func(s string) string {
		for k, v := range vars {
			s = strings.ReplaceAll(s, "{"+k+"}", v)
		}
		return s
	}
	only := regexp.MustCompile(os.Getenv("COMPARE"))
	cl, err := newClient()
	if err != nil {
		t.Skip(err)
	}
	a := newApp()
	a.seenPath = ""
	a.sync = true
	a.api = cl
	a.images.onLoad = nil
	a.refresh()
	if a.status != "" {
		t.Fatal(a.status)
	}
	a.loadCustomTheme()
	custom := a.customCSS
	for _, s := range states {
		if !only.MatchString(s.Name) {
			continue
		}
		t.Run(s.Name, func(t *testing.T) {
			w, h := 1440, 900
			if len(s.Size) == 2 {
				w, h = s.Size[0], s.Size[1]
			}
			a.route, a.back = route{kind: -1}, nil
			a.palette, a.menus, a.userMenu, a.compactNav, a.panelTab = paletteState{}, map[string]bool{}, false, false, 0
			a.theme = "light"
			if s.Dark {
				a.theme = "dark"
			}
			a.compact = w < compactWidth
			a.customCSS = ""
			if s.Custom {
				a.customCSS = custom
			}
			a.navigate(fill(s.Path))
			tt := ui.NewTesterWith(a.view, w, h, ui.TesterOptions{Dark: s.Dark})
			settle := func() {
				for i := 0; i < 6; i++ {
					tt.Frame()
					a.images.wait(15 * time.Second)
					if !a.flush() {
						break
					}
				}
				tt.Frame()
			}
			settle()
			actions := s.App
			if actions == nil {
				actions = s.Actions
			}
			for _, act := range actions {
				switch {
				case act["click"] != nil:
					if err := tt.Click(fill(act["click"].(string))); err != nil {
						t.Log(err)
					}
				case act["clickLabel"] != nil:
					if err := tt.Click(fill(act["clickLabel"].(string))); err != nil {
						t.Log(err)
					}
				case act["key"] == "Control+k":
					tt.TypeKey(ui.Cmd, ui.KeyK, "")
				case act["scroll"] != nil:
					tt.Scroll(float32(w)/2, float32(h)/2, 0, float32(act["scroll"].(float64)))
				}
				settle()
			}
			tt.Advance(3 * time.Second)
			settle()
			app := filepath.Join("compare", "app", s.Name+".png")
			writePNG(t, tt, app)
			web := filepath.Join("compare", "web", s.Name+".png")
			if _, err := os.Stat(web); err == nil {
				os.MkdirAll("compare/side", 0o755)
				exec.Command("montage", web, app, "-tile", "2x1", "-geometry", "+6+0", "-background", "#888", filepath.Join("compare", "side", s.Name+".png")).Run()
			}
		})
	}
}

// TestRealtimeConnects checks the gateway takes the account's token:
// COHUB_LIVE=1 go test -run Realtime.
func TestRealtimeConnects(t *testing.T) {
	if os.Getenv("COHUB_LIVE") == "" {
		t.Skip("COHUB_LIVE not set")
	}
	cl, err := newClient()
	if err != nil {
		t.Skip(err)
	}
	a := newApp()
	a.api = cl
	a.startRealtime()
	a.rt.watch("daf9537a-1c37-4953-9499-8ad794d9d5f4", "e0b76505-50ad-4899-8ff6-22e8c874569f")
	deadline := time.Now().Add(15 * time.Second)
	for !a.rt.live() && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if !a.rt.live() {
		t.Fatalf("not connected: %s", a.rt.lastStatus)
	}
	time.Sleep(6 * time.Second)
	a.rt.mu.Lock()
	defer a.rt.mu.Unlock()
	t.Logf("events %v", a.rt.seen)
	if !a.rt.rooms["space:daf9537a-1c37-4953-9499-8ad794d9d5f4"] {
		t.Errorf("rooms %v", a.rt.rooms)
	}
}

func TestThemesMatchWeb(t *testing.T) {
	// Values the web computes (getComputedStyle) for the light theme.
	p := buildPalette("light", "")
	if p.dark || p.bgPrimary != ui.RGBA(255, 255, 255, 1) || p.brand != ui.RGBA(207, 66, 50, 1) {
		t.Errorf("light: dark %v bg %v brand %v", p.dark, p.bgPrimary, p.brand)
	}
	for _, name := range themeNames {
		p := buildPalette(name, "")
		if p.text == (ui.Color{}) || p.bgPrimary == (ui.Color{}) || p.brand == (ui.Color{}) {
			t.Errorf("%s: unresolved %+v", name, p)
		}
	}
	if !buildPalette("dark", "").dark || buildPalette("solarized-light", "").dark || !buildPalette("neta-pixel-dark", "").dark {
		t.Error("color schemes")
	}
	custom := `:root,[data-theme]{color-scheme:light;--font-sans:"Maple Mono NF CN";--brand:#0088cc}
:root:not([data-theme]),[data-theme="dark"]{color-scheme:dark}`
	c := buildPalette("dark", custom)
	if !c.dark || c.brand != ui.RGBA(0, 136, 204, 1) || !strings.HasPrefix(c.font, "Maple Mono NF CN") {
		t.Errorf("custom over dark: dark %v brand %v font %q", c.dark, c.brand, c.font)
	}
}
