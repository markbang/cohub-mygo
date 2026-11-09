package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
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
	// The user's own preferences and custom theme stay out of the tests.
	a.prefs.path, a.theme, a.customCSS = "", "system", ""
	setLocale("zh-CN")
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
		"/spaces/s1/wallet", "/spaces/s1?window=file%3Adocs%2Fa.md", "/spaces/s1/sessions/abc?window=file%3Aa.md", "/spaces/s1/checkpoints/new", "/spaces/s1/tasks/t1",
		"/spaces/s1/cronjobs/c1", "/spaces/s1/apps/a1", "/spaces/s1/settings/commerce", "/invite/tok", "/bangwu/router/w/site", "/bangwu/router", "/settings/general", "/settings/billing",
	} {
		r := parseRoute(path)
		if got := r.String(); got != path && !(path == "/spaces/s1/sessions/new" && got == "/spaces/s1/sessions/new") {
			t.Errorf("%s parses to %+v, which is %s", path, r, got)
		}
	}
	if r := parseRoute("/bangwu/router/join/abc"); r.kind != routeInvite || r.sub != "abc" {
		t.Errorf("join link %+v", r)
	}
	if r := parseRoute("/spaces/s1/files/docs/a.md"); r.kind != routeSpace || r.file() != "docs/a.md" {
		t.Errorf("files path %+v", r)
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
	tt.Advance(time.Second)
	tt.Frame()
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
			a.sidebarOpen, a.spacesNav, a.spaceList = true, 0, ui.ListState{}
			a.shareOpen, a.statsFor, a.labelTarget, a.renaming = false, "", nil, ""
			a.draft = ""
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
				case act["type"] != nil:
					tt.Type(act["type"].(string))
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

func TestEnglish(t *testing.T) {
	a := demoApp(t)
	setLocale("en")
	defer setLocale("zh-CN")
	tt := ui.NewTester(a.view, 1440, 900)
	for _, want := range []string{"Chats", "New chat"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	snapshot(t, tt, "desktop-chat-en")
}

func TestSpaceRowMenuArchives(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Click("Space")
	list := a.filteredSpaces(0, "")
	var target *Space
	for _, s := range list {
		if !s.IsPinned {
			target = s
			break
		}
	}
	if target == nil {
		t.Skip("every demo Space is pinned")
	}
	a.menus["space-row:"+target.ID] = true
	tt.Frame()
	if err := tt.Click(m("spaces_archive")); err != nil {
		t.Fatal(err)
	}
	if !target.IsArchived || a.menus["space-row:"+target.ID] {
		t.Fatalf("%s not archived, or the menu stayed open; route %v menus %v", target.Name, a.route, a.menus)
	}
}

func TestComposerTakesDroppedFiles(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	pic := filepath.Join(dir, "shot.png")
	os.WriteFile(pic, buf.Bytes(), 0o644)
	doc := filepath.Join(dir, "notes.txt")
	os.WriteFile(doc, []byte("hello"), 0o644)
	if !tt.DropFiles(600, 800, pic, doc) {
		t.Fatal("the composer did not take the files")
	}
	tt.Frame()
	if len(a.attachments) != 2 || a.attachments[0].kind != attachmentImage || a.attachments[0].mime != "image/jpeg" || a.attachments[1].kind != attachmentFile {
		t.Fatalf("attachments %+v", a.attachments)
	}
	if !tt.HasText("notes.txt") {
		t.Fatal("no file tile")
	}
	content := promptContent("look", a.attachments, map[string]string{a.attachments[0].id: "https://x/a.jpg", a.attachments[1].id: "https://x/n.txt"}, []string{"/tmp/uploads/u/shot.jpg", "/tmp/uploads/u/notes.txt"})
	if len(content) != 2 || !strings.Contains(content[0]["text"].(string), "Files:\n- `/tmp/uploads/u/shot.jpg`") || content[1]["type"] != "image" {
		t.Fatalf("content %v", content)
	}
	snapshot(t, tt, "desktop-composer-attachments")
}

func TestCronNextRuns(t *testing.T) {
	s, err := parseCron("0 9 * * 1-5")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC) // a Saturday
	got := s.next(from, 2)
	if len(got) != 2 || got[0] != time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC) || got[1] != time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC) {
		t.Fatalf("next %v", got)
	}
	if _, err := parseCron("0 9 * *"); err == nil {
		t.Fatal("four fields parse")
	}
	if got := mustCron(t, "*/15 * * * *").next(from, 3); got[2] != from.Add(45*time.Minute) {
		t.Fatalf("steps %v", got)
	}
}

func mustCron(t *testing.T, e string) *cronSpec {
	s, err := parseCron(e)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPatchReducer(t *testing.T) {
	var s patchState
	ev := func(js string) patchEvent {
		var e patchEvent
		if err := json.Unmarshal([]byte(js), &e); err != nil {
			t.Fatal(err)
		}
		return e
	}
	steps := []string{
		`{"turnId":"t1","seq":1,"baseSeq":0,"ops":[{"o":"add","p":"/message/content/blocks/0","v":{"type":"thinking","thinking":"Hm"}}]}`,
		`{"turnId":"t1","seq":2,"baseSeq":1,"ops":[{"o":"append","p":"/message/content/blocks/0/thinking","v":"m."},{"o":"append","p":"/message/content/blocks/1/text","v":"Hel"}]}`,
		`{"turnId":"t1","seq":3,"baseSeq":2,"ops":[{"v":"lo"}]}`,
	}
	for _, js := range steps {
		if !s.apply(ev(js)) {
			t.Fatalf("rejected %s", js)
		}
	}
	got := s.content()
	if len(got) != 2 || got[0].Thinking != "Hmm." || got[1].Text != "Hello" {
		t.Fatalf("content %+v", got)
	}
	if s.apply(ev(`{"turnId":"t1","seq":5,"baseSeq":4,"ops":[]}`)) {
		t.Fatal("a gap applied")
	}
	if !s.apply(ev(`{"turnId":"t1","seq":3,"baseSeq":2,"ops":[]}`)) {
		t.Fatal("a duplicate asked for the snapshot")
	}
}

func TestTurnNoticeStacks(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	other := a.sessions[1]
	a.turnNotified(json.RawMessage(`{"spaceId":"` + other.SpaceID + `","sessionId":"` + other.ID + `","turnId":"t9","status":"completed","userPreview":"写个总结","durationMs":12300,"stepCount":3}`))
	tt.Frame()
	if len(a.notices) != 1 || !tt.HasText("写个总结") {
		t.Fatalf("notices %d, texts %q", len(a.notices), tt.Texts())
	}
	if err := tt.Click(other.displayTitle()); err != nil {
		t.Fatal(err)
	}
	snapshot(t, tt, "desktop-notice")
}

func TestSessionRowActions(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	s := a.sessions[2]
	r, ok := tt.Find(s.displayTitle())
	if !ok {
		t.Fatal("no row")
	}
	tt.Move(r.X+10, r.Y+5)
	tt.Frame()
	if err := tt.Click(m("common_rename")); err != nil {
		t.Fatalf("%v: %q", err, tt.Texts())
	}
	if a.rowRenaming != s.ID {
		t.Fatal("not renaming")
	}
	snapshot(t, tt, "desktop-row-rename")
}

func TestLoginScreen(t *testing.T) {
	a := demoApp(t)
	a.login.needed = true
	tt := ui.NewTester(a.view, 1440, 900)
	if !tt.HasText(m("app_auth_sign_in")) {
		t.Fatalf("texts %q", tt.Texts())
	}
	snapshot(t, tt, "desktop-login")
	if err := tt.Click(m("x_login_demo")); err != nil {
		t.Fatal(err)
	}
	if a.login.needed {
		t.Fatal("the demo did not open")
	}
}

func TestTypingIsNotAShortcut(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Type("jkgG")
	if a.draft != "jkgG" {
		t.Fatalf("draft %q", a.draft)
	}
}

func TestAppPath(t *testing.T) {
	for in, want := range map[string]string{
		"cohub://spaces/s1":             "/spaces/s1",
		"cohub://spaces/s1/sessions/x":  "/spaces/s1/sessions/x",
		"https://cohub.live/spaces/s1":  "/spaces/s1",
		"/sessions/abc":                 "/sessions/abc",
		"https://example.com/a":         "",
		"https://cohub.live/docs/intro": "",
	} {
		got, ok := appPath(in)
		if !ok {
			got = ""
		}
		if got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

func TestDanmaku(t *testing.T) {
	a := demoApp(t)
	a.me = &Me{UUID: "me"}
	a.route = route{kind: routeSpace, space: a.spaces[0].ID}
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	payload := []byte(`{"turn":{"id":"t1","sequence":3,"userUuid":"u2","userText":"大家好，看看这个","authorProfile":{"displayName":"Mila"}}}`)
	a.danmakuTurn(a.spaces[0].ID, "s-other", payload)
	a.danmakuTurn(a.spaces[0].ID, "s-other", payload)
	if len(a.danmaku.items) != 1 {
		t.Fatalf("%d items, want one", len(a.danmaku.items))
	}
	tt.Advance(2 * time.Second)
	tt.Frame()
	if dir := os.Getenv("SNAPSHOT_DIR"); dir != "" {
		writePNG(t, tt, filepath.Join(dir, "desktop-danmaku.png"))
	}
	if err := tt.Click("大家好，看看这个"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.route.kind != routeSpaceSession || a.route.session != "s-other" {
		t.Fatalf("route %+v", a.route)
	}
	own := []byte(`{"turn":{"id":"t2","sequence":4,"userUuid":"me","userText":"mine"}}`)
	a.danmakuTurn(a.spaces[0].ID, "s-other", own)
	if len(a.danmaku.items) > 1 {
		t.Fatal("own message floated")
	}
}

// TestMessageKeys checks every m("key") the code asks for is a message.
func TestMessageKeys(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	re := regexp.MustCompile(`\bm\("([a-z0-9_]+)"`)
	for _, f := range files {
		b, _ := os.ReadFile(f)
		for _, mm := range re.FindAllStringSubmatch(string(b), -1) {
			k := mm[1]
			if strings.HasSuffix(k, "_") || k == "key" {
				continue // a prefix the code completes
			}
			if _, ok := messages["en"][k]; !ok {
				if _, ok := messages["zh-CN"][k]; !ok {
					t.Errorf("%s: no message %q", f, k)
				}
			}
		}
	}
}

// TestDisplayWindow draws a display's frames from /capture and sends a
// click on it to /input as the display's pointer.
func TestDisplayWindow(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for i := range frame.Pix {
		frame.Pix[i] = 200
	}
	var jpg bytes.Buffer
	jpeg.Encode(&jpg, frame, nil)
	var inputs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/displays"):
			w.Write([]byte(`{"displays":[{"id":"d1","name":"Screen","width":400,"height":300,"input":true}]}`))
		case strings.HasSuffix(r.URL.Path, "/capture"):
			fmt.Fprintf(w, `{"mimeType":"image/jpeg","data":%q,"width":400,"height":300}`, base64.StdEncoding.EncodeToString(jpg.Bytes()))
		case strings.HasSuffix(r.URL.Path, "/input"):
			b, _ := io.ReadAll(r.Body)
			inputs = append(inputs, string(b))
			w.Write([]byte(`{"applied":2}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	a := demoApp(t)
	a.api = &client{base: srv.URL, http: srv.Client(), token: "x", expires: time.Now().Add(time.Hour)}
	a.sync = true
	id := a.spaces[0].ID
	a.route = route{kind: routeSpace, space: id, win: "display:d1"}
	tt := ui.NewTester(a.view, 1440, 900)
	for i := 0; i < 4; i++ {
		tt.Frame()
		a.flush()
	}
	if a.displayState == nil || a.displayState.frame == nil {
		t.Fatal("no frame")
	}
	if err := tt.Click(m("display_live")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.flush()
	if len(inputs) != 1 || !strings.Contains(inputs[0], `"action":"down"`) || !strings.Contains(inputs[0], `"action":"up"`) {
		t.Fatalf("inputs %q", inputs)
	}
}
