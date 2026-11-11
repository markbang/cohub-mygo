package app

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
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"cohub-mygo/internal/cohub"
	"cohub-mygo/internal/i18n"
)

func demoApp(t testing.TB) *app {
	t.Helper()
	a := newApp()
	a.seenPath, a.cachePath = "", ""
	a.seen = map[string]time.Time{}
	// The user's own preferences and custom theme stay out of the tests.
	a.prefs.path, a.theme, a.customCSS = "", "system", ""
	a.sidebarW, a.panelW, a.mainW = sidebarWidth, panelWidth, mainWidth
	i18n.SetLocale("zh-CN")
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

// compareDir is the repository's compare/, which tools/webshot also uses.
const compareDir = "../../compare"

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
//	COHUB_LIVE=1 go test ./internal/app -run Compare [-compare regexp]
func TestCompare(t *testing.T) {
	if os.Getenv("COHUB_LIVE") == "" {
		t.Skip("COHUB_LIVE not set")
	}
	var states []compareState
	var vars map[string]string
	b, _ := os.ReadFile(filepath.Join(compareDir, "states.json"))
	if err := json.Unmarshal(b, &states); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(compareDir, "vars.json"))
	json.Unmarshal(b, &vars)
	fill := func(s string) string {
		for k, v := range vars {
			s = strings.ReplaceAll(s, "{"+k+"}", v)
		}
		return s
	}
	only := regexp.MustCompile(os.Getenv("COMPARE"))
	cl, err := cohub.New(clientID())
	if err != nil {
		t.Skip(err)
	}
	a := newApp()
	a.seenPath, a.cachePath = "", ""
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
			for _, tl := range a.timelines {
				tl.expanded = map[string]bool{}
			}
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
				case act["textPart"] != nil:
					part := fill(act["textPart"].(string))
					for _, text := range tt.Texts() {
						if strings.Contains(text, part) {
							if err := tt.Click(text); err != nil {
								t.Log(err)
							}
							break
						}
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
				case act["clickAt"] != nil:
					xy := act["clickAt"].([]any)
					tt.ClickAt(float32(xy[0].(float64)), float32(xy[1].(float64)))
				}
				settle()
			}
			tt.Advance(3 * time.Second)
			settle()
			app := filepath.Join(compareDir, "app", s.Name+".png")
			writePNG(t, tt, app)
			web := filepath.Join(compareDir, "web", s.Name+".png")
			if _, err := os.Stat(web); err == nil {
				os.MkdirAll(filepath.Join(compareDir, "side"), 0o755)
				exec.Command("montage", web, app, "-tile", "2x1", "-geometry", "+6+0", "-background", "#888", filepath.Join(compareDir, "side", s.Name+".png")).Run()
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
	cl, err := cohub.New(clientID())
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
		t.Fatalf("not connected: %s", a.rt.conn.Status())
	}
	time.Sleep(6 * time.Second)
	t.Logf("events %v", a.rt.conn.Seen())
	if !a.rt.conn.Subscribed("space:daf9537a-1c37-4953-9499-8ad794d9d5f4") {
		t.Errorf("not subscribed to the Space's room")
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
	i18n.SetLocale("en")
	defer i18n.SetLocale("zh-CN")
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
	var target *cohub.Space
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

func TestTurnNoticeStacks(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	other := a.sessions[1]
	a.turnNotified(json.RawMessage(`{"spaceId":"` + other.SpaceID + `","sessionId":"` + other.ID + `","turnId":"t9","status":"completed","userPreview":"写个总结","durationMs":12300,"stepCount":3}`))
	tt.Frame()
	if len(a.notices) != 1 || !tt.HasText("写个总结") {
		t.Fatalf("notices %d, texts %q", len(a.notices), tt.Texts())
	}
	if err := tt.Click(chatTitle(other)); err != nil {
		t.Fatal(err)
	}
	snapshot(t, tt, "desktop-notice")
}

// drag presses at x, halfway across the window, and lets go dx further.
func drag(tt *ui.Tester, x, dx float32) {
	tt.Press(x, 450)
	tt.Move(x+dx/2, 450)
	tt.Move(x+dx, 450)
	tt.Release(x+dx, 450)
	tt.Frame()
}

// Dragging the edges of the sidebar and the right panel resizes them, as
// far as the web's limits, and keeps the widths for the next start.
func TestDraggingPaneEdges(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	drag(tt, sidebarWidth, 100)
	if a.sidebarW != sidebarWidth+100 || a.prefs.SidebarWidth != a.sidebarW {
		t.Fatalf("sidebar %v, kept %v", a.sidebarW, a.prefs.SidebarWidth)
	}
	drag(tt, a.sidebarW, 400)
	if a.sidebarW != sidebarMax {
		t.Fatalf("sidebar %v past its widest", a.sidebarW)
	}
	drag(tt, 1440-panelWidth, -100)
	if a.panelW != panelWidth+100 || a.prefs.PanelWidth != a.panelW {
		t.Fatalf("panel %v, kept %v", a.panelW, a.prefs.PanelWidth)
	}
	snapshot(t, tt, "desktop-panes-resized")
}

// Beside a window, dragging the main column's edge resizes it, and letting
// go far past its narrowest gives the window its room.
func TestWindowEdgeResizesMainColumn(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	a.navigate("/spaces/s1/checkpoints/new")
	tt.Frame()
	drag(tt, sidebarWidth+mainWidth, 100)
	if a.mainW != mainWidth+100 || a.windowWide {
		t.Fatalf("main column %v, wide %v", a.mainW, a.windowWide)
	}
	drag(tt, sidebarWidth+a.mainW, -400)
	if !a.windowWide || a.mainW != mainWidth+100 {
		t.Fatalf("main column %v, wide %v", a.mainW, a.windowWide)
	}
}

// While a turn runs, a message sent waits in the queue above the composer
// rather than in the timeline, and steering it asks the API to run it now.
func TestFollowupQueuesAndSteers(t *testing.T) {
	var mu sync.Mutex
	var prompt map[string]any
	steered := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/spaces/s1/prompt":
			json.NewDecoder(r.Body).Decode(&prompt)
			io.WriteString(w, `{"sessionId":"demo-0"}`)
		case r.URL.Path == "/api/sessions/demo-0/turns":
			cid, _ := prompt["clientMessageId"].(string)
			status, intent := "queued", "followup"
			if steered != "" {
				status, intent = "running", "steer"
			}
			fmt.Fprintf(w, `{"turns":[{"id":"t2","sequence":2,"status":"running"},`+
				`{"id":"t3","sequence":3,"status":%q,"intent":%q,"userText":"接着做","meta":{"clientMessageId":%q}}]}`, status, intent, cid)
		case r.Method == "POST" && r.URL.Path == "/api/spaces/s1/sessions/demo-0/turns/t3/steer":
			steered = "t3"
			io.WriteString(w, `{"ok":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	a := demoApp(t)
	a.sync = true
	a.api = cohub.NewWithToken(srv.URL, "x", "", srv.Client())
	tt := ui.NewTester(a.view, 1440, 900)
	if _, ok := tt.Find(m("board_session_stop")); !ok {
		t.Fatal("an empty composer does not stop the running turn")
	}
	tt.Type("接着做")
	tt.Frame()
	if err := tt.Click(m("feedback_send")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(m("chat_queued", "count", 1)) {
		t.Fatalf("the message waits nowhere: %q", tt.Texts())
	}
	for i := 0; i < 3 && a.flush(); i++ {
		tt.Frame()
	}
	tt.Frame()
	mu.Lock()
	if prompt["sessionId"] != "demo-0" || prompt["intent"] != "followup" {
		t.Fatalf("prompt %v", prompt)
	}
	mu.Unlock()
	tl := a.timelineOf("demo-0")
	shown := tl.shown()
	if len(tl.pending) != 0 || len(tl.queued()) != 1 || shown[len(shown)-1].ID != "t2" {
		t.Fatalf("pending %d, queued %d, last shown %s", len(tl.pending), len(tl.queued()), shown[len(shown)-1].ID)
	}
	snapshot(t, tt, "desktop-followup-queue")
	if err := tt.Click(m("chat_steer_now")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3 && a.flush(); i++ {
		tt.Frame()
	}
	tt.Frame()
	mu.Lock()
	defer mu.Unlock()
	if steered != "t3" || tt.HasText(m("chat_steer_now")) {
		t.Fatalf("steered %q, queue %v", steered, tt.HasText(m("chat_steer_now")))
	}
}

// A chat scrolled up to its first loaded turn loads the turns before it;
// they only ever loaded the latest 30.
func TestScrollingUpLoadsOlderTurns(t *testing.T) {
	var mu sync.Mutex
	cursor := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/api/sessions/demo-0/turns" && r.URL.Query().Get("cursor") != "" {
			cursor = r.URL.Query().Get("cursor")
			io.WriteString(w, `{"turns":[{"id":"t0","sequence":0,"status":"completed","userText":"最早的一句"}],"hasMore":false}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	a := demoApp(t)
	a.sync = true
	a.api = cohub.NewWithToken(srv.URL, "x", "", srv.Client())
	tl := a.timelineOf("demo-0")
	tl.hasMore = true
	tt := ui.NewTester(a.view, 1440, 900)
	a.timeline.ScrollTo(0, ui.Start)
	for i := 0; i < 3; i++ {
		tt.Frame()
		a.flush()
	}
	mu.Lock()
	defer mu.Unlock()
	if cursor != "1" || tl.turns[0].ID != "t0" || tl.hasMore {
		t.Fatalf("cursor %q, first turn %s, more %v", cursor, tl.turns[0].ID, tl.hasMore)
	}
}

// The tokens and time under a reply open the turn's statistics.
func TestReplyFiguresOpenTurnStats(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	if err := tt.Click(m("stats_turn")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.turnStatsFor != "t1" || !tt.HasText(m("stats_final_response")) {
		t.Fatalf("stats for %q", a.turnStatsFor)
	}
	snapshot(t, tt, "desktop-turn-stats")
}

// An archived chat says so above the composer, and its button restores it.
func TestArchivedChatUnarchives(t *testing.T) {
	a := demoApp(t)
	s := a.session("demo-0")
	at := a.now()
	s.ArchivedAt = &at
	tt := ui.NewTester(a.view, 1440, 900)
	if err := tt.Click(m("session_unarchive")); err != nil {
		t.Fatal(err)
	}
	if s.ArchivedAt != nil {
		t.Fatal("still archived")
	}
}

// A tool call opens on a click into what it ran and what came back; its
// row had a chevron that did nothing.
func TestToolCallOpens(t *testing.T) {
	a := demoApp(t)
	tl := a.timelineOf("demo-0")
	step := &cohub.StepMessage{ID: "m1", Content: []cohub.Block{
		{Type: "tool_use", ID: "c1", Name: "bash", Input: json.RawMessage(`{"command":"ls","timeout":20}`)},
		{Type: "tool_result", ToolUse: "c1", Content: json.RawMessage(`"a.txt"`)},
	}}
	tl.steps["t1"] = []*cohub.StepMessage{step}
	tl.expanded["t1"], a.foldOpen["m1"] = true, true
	tt := ui.NewTester(a.view, 1440, 900)
	a.timeline.ScrollTo(0, ui.Start)
	tt.Frame()
	if tt.HasText("a.txt") {
		t.Fatal("a finished call shows open")
	}
	if err := tt.Click("bash $ ls"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.foldOpen["tool:c1"] || !tt.HasText("a.txt") || !tt.HasText("20s") {
		t.Fatalf("open %v, texts %q", a.foldOpen["tool:c1"], tt.Texts())
	}
	snapshot(t, tt, "desktop-tool-open")
}

// A finished step's file leaves its tool results out; opening a call loads
// them from the step's tool calls file, where it showed no output.
func TestToolCallLoadsOmittedResult(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/sessions/demo-0/turns/t1/signed-urls":
			fmt.Fprintf(w, `{"urls":{"calls.json":%q}}`, srv.URL+"/obj/calls.json")
		case "/obj/calls.json":
			io.WriteString(w, `{"toolCalls":[{"id":"c1","name":"bash","input":{"command":"ls"},"result":{"content":"a.txt","isError":false}}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	a := demoApp(t)
	a.sync = true
	a.api = cohub.NewWithToken(srv.URL, "x", "", srv.Client())
	tl := a.timelineOf("demo-0")
	tl.steps["t1"] = []*cohub.StepMessage{{ID: "m1", ToolCallsObjectKey: "calls.json", Content: []cohub.Block{
		{Type: "tool_use", ID: "c1", Name: "bash", Input: json.RawMessage(`{"command":"ls"}`)},
		{Type: "tool_result", ToolUse: "c1", Content: json.RawMessage(`[]`), Meta: &cohub.BlockMeta{ResultDetail: "omitted"}},
	}}}
	tl.expanded["t1"], a.foldOpen["m1"], a.foldOpen["tool:c1"] = true, true, true
	tt := ui.NewTester(a.view, 1440, 900)
	a.timeline.ScrollTo(0, ui.Start)
	for i := 0; i < 3; i++ {
		tt.Frame()
		a.flush()
	}
	tt.Frame()
	if !tt.HasText("a.txt") {
		t.Fatalf("no output: %q", tt.Texts())
	}
}

// Dragging the thumb along the rail's track scrolls the timeline, as the
// web's scroll track does.
func TestRailTrackScrollsTimeline(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	sc := &a.timelineScroll
	r, ok := tt.Find(m("turnrail_scroll"))
	if !ok || sc.MaxY <= 0 || sc.Y < sc.MaxY-1 {
		t.Fatalf("track %v, scroll %v of %v", ok, sc.Y, sc.MaxY)
	}
	x := r.X + r.W/2
	tt.Press(x, r.Y+r.H/2)
	tt.Move(x, r.Y+r.H/4)
	tt.Move(x, r.Y)
	tt.Release(x, r.Y)
	tt.Frame()
	if sc.Y != 0 {
		t.Fatalf("scrolled to %v of %v", sc.Y, sc.MaxY)
	}
}

// In the compact shell, the pill over the timeline opens the turn list,
// and a turn in it jumps there and closes it.
func TestCompactTurnList(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 420, 860)
	a.navigate("/sessions/demo-0")
	tt.Frame()
	if err := tt.Click(m("chat_open_turn_list")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.turnSheetOpen {
		t.Fatal("the list stayed shut")
	}
	if err := tt.Click("#1"); err != nil {
		t.Fatal(err)
	}
	if a.turnSheetOpen {
		t.Fatal("the list stayed open after the jump")
	}
}

// The account menu checks for updates, as Windows and Linux show no menu
// bar to do it from.
func TestAccountMenuChecksUpdates(t *testing.T) {
	a := demoApp(t)
	checked := 0
	a.checkUpdates = func() { checked++ }
	tt := ui.NewTester(a.view, 1440, 900)
	if err := tt.Click(m("x_account")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(m("x_check_updates")); err != nil {
		t.Fatalf("%v: %q", err, tt.Texts())
	}
	if checked != 1 {
		t.Fatalf("checked %d times", checked)
	}
}

// Hovering a turn's mark opens the turn navigator: its search filters the
// turns, a turn in it closes it, and it closes a moment after the pointer
// leaves.
func TestTurnRailNavigator(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	r, ok := tt.Find(m("turnrail_jump_turn", "n", 1))
	if !ok {
		t.Fatal("no mark")
	}
	hover := func() {
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		tt.Frame()
		if !a.turnNav.open || a.turnNav.turn != "t1" {
			t.Fatalf("navigator %+v", a.turnNav)
		}
	}
	hover()
	tt.Move(400, 400)
	tt.Frame()
	if !a.turnNav.open {
		t.Fatal("closed as the pointer left")
	}
	tt.Advance(turnNavGrace)
	tt.Frame()
	if a.turnNav.open {
		t.Fatal("stayed open")
	}

	hover()
	if err := tt.Click(m("navigator_search")); err != nil {
		t.Fatal(err)
	}
	tt.Type("mygo")
	tt.Frame()
	if _, ok := tt.Find("#1"); ok || !a.turnNav.open {
		t.Fatalf("search %q kept turn 1, open %v", a.turnNav.query, a.turnNav.open)
	}
	snapshot(t, tt, "desktop-turn-navigator")
	if err := tt.Click("#2"); err != nil {
		t.Fatal(err)
	}
	if a.turnNav.open {
		t.Fatal("stayed open after the jump")
	}
}

func TestSessionRowActions(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	s := a.sessions[2]
	r, ok := tt.Find(chatTitle(s))
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
	a.me = &cohub.Me{UUID: "me"}
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
			if !i18n.Has(k) {
				t.Errorf("%s: no message %q", f, k)
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
	a.api = cohub.NewWithToken(srv.URL, "x", "", srv.Client())
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

// TestDesktopCommand opens the file an agent asks this client to show,
// and reports it applied.
func TestDesktopCommand(t *testing.T) {
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path+" "+r.Header.Get("X-Cohub-Source-Client"))
		if strings.HasSuffix(r.URL.Path, "/accept") {
			w.Write([]byte(`{"accepted":true}`))
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	a := demoApp(t)
	a.api = cohub.NewWithToken(srv.URL, "x", "mygo-test-client", srv.Client())
	a.sync = true
	id := a.spaces[0].ID
	payload := fmt.Sprintf(`{"commandId":"c1","targetClientId":"mygo-test-client","command":{"type":"desktop.open","target":{"kind":"file","path":"docs/a.md"}},"source":{"spaceId":%q}}`, id)
	a.desktopCommandDispatched([]byte(payload))
	a.flush()
	if a.route.file() != "docs/a.md" || a.route.space != id {
		t.Fatalf("route %+v", a.route)
	}
	if len(calls) < 2 || !strings.HasSuffix(calls[1], "/result mygo-test-client") {
		t.Fatalf("calls %q", calls)
	}
	n := len(calls)
	a.desktopCommandDispatched([]byte(strings.Replace(payload, `"mygo-test-client"`, `"other"`, 1)))
	a.flush()
	if len(calls) != n {
		t.Fatal("ran another client's command")
	}
}

// TestOpenWith lists the Apps that open a file's type, and choosing one
// "always" makes it the Space's default in .cohub/apps.json.
func TestOpenWith(t *testing.T) {
	const app1, app2 = "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	installed := `{"format":"cohub.space-apps","version":1,"apps":[{"id":"` + app2 + `","ref":"bob/s/viewer","url":"https://x","enabled":true,"opens":[".csv"]}]}`
	var written string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "PUT" && strings.HasSuffix(r.URL.Path, "/fs/file"):
			b, _ := io.ReadAll(r.Body)
			written = string(b)
			w.Write([]byte(`{"size":1,"mtimeMs":2}`))
		case strings.HasSuffix(r.URL.Path, "/fs/file"):
			fmt.Fprintf(w, `{"path":".cohub/apps.json","kind":"text","content":%q,"size":10,"mtimeMs":5}`, installed)
		case strings.HasPrefix(r.URL.Path, "/api/apps/space/"):
			fmt.Fprintf(w, `{"apps":[{"id":%q,"slug":"board","status":"published","meta":{"title":"Board","fileHandlers":".board .csv"}},{"id":"x","slug":"off","status":"disabled","meta":{"fileHandlers":[".csv"]}}]}`, app1)
		case strings.HasSuffix(r.URL.Path, "/"+app2):
			fmt.Fprintf(w, `{"app":{"id":%q,"slug":"viewer","status":"published","meta":{"title":"Viewer","fileHandlers":["csv"]}},"publicUrl":"https://x","owner":{"username":"bob"},"space":{"slug":"s"}}`, app2)
		case strings.HasSuffix(r.URL.Path, "/"+app1):
			fmt.Fprintf(w, `{"app":{"id":%q,"slug":"board","status":"published","meta":{"title":"Board","fileHandlers":".board .csv"}},"publicUrl":"https://b","owner":{"username":"me"},"space":{"slug":"home"}}`, app1)
		case strings.Count(r.URL.Path, "/") == 3 && strings.HasPrefix(r.URL.Path, "/api/spaces/"):
			w.Write([]byte(`{"access":{"permissions":["file.edit"]}}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	a := demoApp(t)
	a.api = cohub.NewWithToken(srv.URL, "x", "", srv.Client())
	a.sync = true
	id := a.spaces[0].ID
	a.route = route{kind: routeSpace, space: id}
	tt := ui.NewTester(a.view, 1440, 900)
	a.openFileWith(id, "data/q.csv")
	for i := 0; i < 3; i++ {
		a.flush()
		tt.Frame()
	}
	st := a.openWith
	if st == nil || !st.loaded || len(st.cands) != 2 || st.def != app2 {
		t.Fatalf("open with %+v", st)
	}
	snapshot(t, tt, "desktop-open-with")
	if err := tt.Click(m("open_with_always", "extension", ".csv")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !st.always {
		t.Fatal("always is off")
	}
	if err := tt.Click("Board"); err != nil {
		t.Fatal(err)
	}
	a.flush()
	tt.Frame()
	if a.route.win != "app:"+app1+":data/q.csv" {
		t.Fatalf("window %q", a.route.win)
	}
	var in struct {
		Content  string         `json:"content"`
		Expected map[string]any `json:"expected"`
	}
	json.Unmarshal([]byte(written), &in)
	var doc installedApps
	json.Unmarshal([]byte(in.Content), &doc)
	if in.Expected == nil || len(doc.Apps) != 2 {
		t.Fatalf("written %s", written)
	}
	if id, _ := installedHandler(doc, "x.csv"); id != app1 {
		t.Fatalf("default %q in %s", id, in.Content)
	}
	// The file now opens in its App.
	a.route = route{kind: routeSpace, space: id}
	a.openFile(id, "data/q.csv")
	if a.route.win != "app:"+app1+":data/q.csv" {
		t.Fatalf("open %q", a.route.win)
	}
}

// A launch shows the last refresh's Spaces, chats and the latest chat's
// turns from the startup cache before the API answers, and a refresh with
// a newer chat leaves the open chat alone, as a native window does.
func TestStartupCacheShowsLastChats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "startup.json")
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	prev := demoApp(t)
	prev.api, prev.cachePath, prev.sync = cohub.NewWithToken(srv.URL, "x", "", srv.Client()), path, true
	prev.saveStartupCache()
	prev.flush()

	a := newApp()
	a.seenPath, a.prefs.path, a.theme, a.customCSS = "", "", "system", ""
	a.seen = map[string]time.Time{}
	a.api, a.cachePath, a.sync = cohub.NewWithToken(srv.URL, "x", "", srv.Client()), path, true
	ui.NewTester(a.view, 1440, 900)
	a.loadStartupCache()
	a.flush()
	if len(a.spaces) != len(prev.spaces) || len(a.sessions) != len(prev.sessions) {
		t.Fatalf("%d spaces and %d chats, want %d and %d", len(a.spaces), len(a.sessions), len(prev.spaces), len(prev.sessions))
	}
	if a.route.sessionID() != "demo-0" {
		t.Fatalf("route %v, want the latest chat", a.route)
	}
	if tl := a.timelines["demo-0"]; tl == nil || !tl.loaded || len(tl.turns) != len(prev.timelines["demo-0"].turns) {
		t.Fatalf("timeline %+v", tl)
	}

	newer := &cohub.Session{ID: "newer", SpaceID: "s1", Title: "newer", LastMessageAt: a.now().Add(time.Minute)}
	a.showLists(a.me, a.spaces, append([]*cohub.Session{newer}, a.sessions...), a.models, true)
	if a.route.sessionID() != "demo-0" {
		t.Fatalf("route %v after a refresh with a newer chat, want the open chat", a.route)
	}
}

// Clicking a reply's steps unfolds them in the demo, whose timeline once
// lacked the map of unfolded turns and panicked on the click.
func TestDemoStepsUnfold(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	if err := tt.Click("19 步 · 19 个工具"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.timelines["demo-0"].expanded["t1"] {
		t.Fatal("the steps stayed folded")
	}
}

// fakeDisplayPage answers the display page's scripts as a page streaming
// would, and notes what the app asked of it.
type fakeDisplayPage struct {
	state  string
	answer string
	closed int
}

func (f *fakeDisplayPage) Eval(code string) (any, error) {
	switch {
	case strings.HasPrefix(code, "return cohubDisplay.offer("):
		return "v=0 offer", nil
	case strings.HasPrefix(code, "return cohubDisplay.answer("):
		json.NewDecoder(strings.NewReader(strings.TrimPrefix(code, "return cohubDisplay.answer("))).Decode(&f.answer)
	case code == "return cohubDisplay.info()":
		return map[string]any{"state": f.state}, nil
	case code == "cohubDisplay.close()":
		f.closed++
	}
	return nil, nil
}

// Opening a display that streams signals its page's offer and the
// display's answer through the API, shows it connected once the page is,
// and leaving it closes the page's connection and the display's session.
func TestDisplayStreamsOverWebRTC(t *testing.T) {
	var mu sync.Mutex
	var offers []string
	deleted := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/api/spaces/s1/displays":
			io.WriteString(w, `{"displays":[{"id":"screen","name":"Virtual display","width":1280,"height":800,"input":true,"stream":true}]}`)
		case r.URL.Path == "/api/spaces/s1/rtc/ice-servers":
			io.WriteString(w, `{"iceServers":[{"urls":["stun:stun.example:3478"]}]}`)
		case r.Method == "POST" && r.URL.Path == "/api/spaces/s1/displays/screen/sessions":
			var in struct{ Offer string }
			json.NewDecoder(r.Body).Decode(&in)
			offers = append(offers, in.Offer)
			io.WriteString(w, `{"sessionId":"sess1","answer":"v=0 answer"}`)
		case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/spaces/s1/displays/screen/sessions/"):
			deleted = strings.TrimPrefix(r.URL.Path, "/api/spaces/s1/displays/screen/sessions/")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	a := demoApp(t)
	clock := a.now()
	a.now = func() time.Time { return clock }
	a.sync = true
	a.api = cohub.NewWithToken(srv.URL, "x", "", srv.Client())
	page := &fakeDisplayPage{state: "connecting"}
	a.rtc = &displayRTC{page: page, failed: map[string]bool{}}
	tt := ui.NewTester(a.view, 1440, 900)
	a.go_(route{kind: routeSpace, space: "s1", win: "display:screen"})
	for i := 0; i < 4; i++ {
		tt.Frame()
		a.flush()
	}
	mu.Lock()
	if len(offers) != 1 || offers[0] != "v=0 offer" {
		t.Fatalf("offers %q", offers)
	}
	mu.Unlock()
	if page.answer != "v=0 answer" || a.rtc.session != "sess1" || a.rtc.state != "connecting" {
		t.Fatalf("answer %q, session %q, state %q", page.answer, a.rtc.session, a.rtc.state)
	}

	page.state = "connected"
	clock = clock.Add(2 * time.Second)
	tt.Frame()
	a.flush()
	if a.rtc.state != "connected" {
		t.Fatalf("state %q after the page connected", a.rtc.state)
	}

	a.go_(route{kind: routeSessions})
	a.flush()
	mu.Lock()
	defer mu.Unlock()
	if page.closed != 1 || deleted != "sess1" {
		t.Fatalf("closed %d times, deleted session %q", page.closed, deleted)
	}
}

// A chat forked from another lists right under it in the inbox, titled by
// what was said after forking while it has no title, and opens on a click.
func TestInboxNestsForks(t *testing.T) {
	a := demoApp(t)
	parent := a.sessions[3]
	fork := &cohub.Session{ID: "fork-1", SpaceID: parent.SpaceID, Source: "web", LatestMessageText: "继续",
		LastMessageAt: a.now().Add(-time.Minute), Space: parent.Space}
	a.sessions = append([]*cohub.Session{fork}, a.sessions...)
	a.inboxForks = []cohub.SessionFork{{ParentSessionID: parent.ID, ChildSessionID: fork.ID, Depth: 1}}
	rows := a.visibleRows()
	at := -1
	for i, r := range rows {
		if r.s == parent {
			at = i
		}
	}
	if at < 0 || at+1 >= len(rows) || rows[at+1].s != fork || rows[at+1].depth != 1 || rows[at+1].title() != "继续" {
		t.Fatalf("the fork isn't under its parent: %+v", rows)
	}
	tt := ui.NewTester(a.view, 1440, 900)
	pr, _ := tt.Find(chatTitle(parent))
	fr, ok := tt.Find("继续")
	if !ok || fr.Y <= pr.Y || fr.Y-pr.Y > 80 {
		t.Fatalf("the fork's row at %v, its parent's at %v", fr, pr)
	}
	if err := tt.Click("继续"); err != nil {
		t.Fatal(err)
	}
	if a.route.sessionID() != fork.ID {
		t.Fatalf("route %v after clicking the fork", a.route)
	}
}

// A video's first frame is its tile's poster rather than a tile of its own.
func TestVideoTakesItsFirstFrame(t *testing.T) {
	var r genRun
	if err := json.Unmarshal([]byte(`{"status":"completed","result":{"output":[
		{"type":"video","source":{"type":"url","url":"https://x/v.mp4"}},
		{"type":"image","meta":{"role":"first_frame"},"source":{"type":"url","url":"https://x/f.png"}}]}}`), &r); err != nil {
		t.Fatal(err)
	}
	out := r.outputs()
	if len(out) != 1 || out[0].kind != "video" || out[0].preview != "https://x/f.png" {
		t.Fatalf("outputs %+v", out)
	}
}

// Cmd+B toggles the sidebar, as the palette has always said.
func TestCmdBTogglesSidebar(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	tt.TypeKey(ui.Cmd, ui.KeyB, "")
	if a.sidebarOpen {
		t.Fatal("Cmd+B left the sidebar open")
	}
	tt.TypeKey(ui.Cmd, ui.KeyB, "")
	if !a.sidebarOpen {
		t.Fatal("Cmd+B did not bring the sidebar back")
	}
}

// Cmd+N starts a new chat, as native apps make a new document.
func TestCmdNStartsChat(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	tt.TypeKey(ui.Cmd, ui.KeyN, "")
	if a.route.kind != routeNewSession {
		t.Fatalf("Cmd+N left route %v", a.route)
	}
}

// The window names the chat it shows, as a document window does.
func TestWindowTitleNamesChat(t *testing.T) {
	a := demoApp(t)
	ui.NewTester(a.view, 1440, 900)
	if got, want := a.windowTitle(), "看看这个 https://nsh.dev/ — Cohub"; got != want {
		t.Errorf("title %q, want %q", got, want)
	}
	a.go_(route{kind: routeSettings, sub: "general"})
	if got, want := a.windowTitle(), "Cohub"; got != want {
		t.Errorf("settings title %q, want %q", got, want)
	}
	a.openSpace("s2")
	if got, want := a.windowTitle(), "cohub-mobile — Cohub"; got != want {
		t.Errorf("Space title %q, want %q", got, want)
	}
}

// A chat row's secondary click opens its actions, as native lists do.
func TestChatRowContextMenu(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	if err := tt.RightClick("执行 npx skills@latest update"); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	if len(menu) < 3 || menu[0] != "打开" || menu[1] != "重命名" {
		t.Fatalf("menu %q", menu)
	}
	if err := tt.ChooseMenuItem("重命名"); err != nil {
		t.Fatal(err)
	}
	if a.rowRenaming != "demo-3" {
		t.Fatalf("renaming %q, want demo-3", a.rowRenaming)
	}
}

// Coming back to a chat keeps the place the reader left, not its end.
func TestChatKeepsScrollPosition(t *testing.T) {
	a := demoApp(t)
	benchTimeline(a, "demo-0", 100)
	tt := ui.NewTester(a.view, 1440, 900)
	tt.Frame()
	a.timelineScroll.Y = 2000
	tt.Frame()
	a.go_(route{kind: routeSession, session: "demo-3"})
	tt.Frame()
	a.go_(route{kind: routeSession, session: "demo-0"})
	tt.Frame()
	if a.timelineScroll.Y < 1990 || a.timeline.AtEnd() {
		t.Fatalf("returned to %v of %v (end %v), want 2000",
			a.timelineScroll.Y, a.timelineScroll.MaxY, a.timeline.AtEnd())
	}
}

// The chat list takes the keyboard: arrows choose, Enter opens.
func TestChatListKeyboard(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	if a.selectedChat != 0 {
		t.Fatalf("selected %d, want the open chat", a.selectedChat)
	}
	tt.TypeKey(ui.Cmd|ui.Shift, ui.KeyU, "")
	first := a.route.sessionID()
	tt.Key(0, ui.KeyDown)
	if a.selectedChat != 1 {
		t.Fatalf("Down selected %d, want 1", a.selectedChat)
	}
	if a.route.sessionID() != first {
		t.Fatal("Down opened a chat instead of only choosing it")
	}
	tt.Key(0, ui.KeyEnter)
	if a.route.sessionID() == first {
		t.Fatal("Enter did not open the chosen chat")
	}
}

// The menu bar has File, View and Help beside the platform's menus.
func TestAppMenu(t *testing.T) {
	a := demoApp(t)
	items := appMenu(a).Items()
	labels := make([]string, len(items))
	for i, it := range items {
		labels[i] = it.Label
	}
	want := []string{"", "文件", "", "视图", "", "帮助"}
	if len(items) != len(want) {
		t.Fatalf("menus %q", labels)
	}
	for i := range want {
		if want[i] != "" && labels[i] != want[i] {
			t.Fatalf("menus %q, want %q at %d", labels, want, i)
		}
	}
	file := items[1].Submenu
	if len(file) < 1 || file[0].Label != "新建对话" {
		t.Fatalf("File menu %+v", file)
	}
	file[0].Click(file[0], nil)
	if a.route.kind != routeNewSession {
		t.Fatalf("New Chat left route %v", a.route)
	}
}

// A launch opens the chat the window last showed, not the newest one.
func TestRestoresLastChat(t *testing.T) {
	a := demoApp(t)
	a.prefs.LastSession, a.prefs.LastSpace = "demo-3", "s2"
	ui.NewTester(a.view, 1440, 900)
	if a.route.sessionID() != "demo-3" || a.route.kind != routeSpaceSession {
		t.Fatalf("opened %+v, want demo-3 in s2", a.route)
	}
	b := demoApp(t)
	ui.NewTester(b.view, 1440, 900)
	if b.route.sessionID() != "demo-0" {
		t.Fatalf("without a last chat opened %q", b.route.sessionID())
	}
	b.openSession(b.session("demo-5"))
	if b.prefs.LastSession != "demo-5" || b.prefs.LastSpace != "" {
		t.Fatalf("recorded %q/%q", b.prefs.LastSession, b.prefs.LastSpace)
	}
}

// Cmd+F finds text in the loaded turns; the query filters and Enter walks
// to the next match.
func TestFindInChat(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	tt.TypeKey(ui.Cmd, ui.KeyF, "")
	if !a.find.open {
		t.Fatal("Cmd+F did not open the find bar")
	}
	tt.Type("MyGo")
	tt.Frame()
	if len(a.find.turns) != 2 || a.find.turns[0] != 0 || a.find.turns[1] != 1 {
		t.Fatalf("found %v, want the first two turns", a.find.turns)
	}
	a.find.query = "nothing-matches-this"
	tt.Frame()
	if len(a.find.turns) != 0 {
		t.Fatalf("found %v for a missing query", a.find.turns)
	}
}

// Back and Forward walk the route history, and a new navigation drops the
// forward pages, as in a browser.
func TestForwardNavigation(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	a.openSession(a.session("demo-1"))
	tt.Frame()
	a.goBack()
	if a.route.sessionID() != "demo-0" {
		t.Fatalf("back went to %q", a.route.sessionID())
	}
	a.goForward()
	if a.route.sessionID() != "demo-1" {
		t.Fatalf("forward went to %q", a.route.sessionID())
	}
	a.goBack()
	a.go_(route{kind: routeSettings, sub: "general"})
	a.goForward()
	if a.route.kind != routeSettings {
		t.Fatalf("forward after a new page went to %+v", a.route)
	}
}

// The app icon counts unread chats, without counting a chat twice when it
// is in both the inbox and its Space.
func TestUnreadBadgeCount(t *testing.T) {
	a := demoApp(t)
	if got := a.unreadCount(); got != 1 {
		t.Fatalf("unread %d, want the one unseen chat", got)
	}
	a.markSeen(a.session("demo-2"))
	if got := a.unreadCount(); got != 0 {
		t.Fatalf("unread %d after reading", got)
	}
}

// Zoom follows the platform's keys and is kept for the next launch.
func TestZoomKeys(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	tt.TypeKey(ui.Cmd, ui.KeyEqual, "")
	if a.zoom <= 1 {
		t.Fatalf("zoom in left %v", a.zoom)
	}
	tt.TypeKey(ui.Cmd, ui.Key0, "")
	if a.zoom != 1 {
		t.Fatalf("reset left %v", a.zoom)
	}
	tt.TypeKey(ui.Cmd, ui.KeyMinus, "")
	if a.zoom >= 1 {
		t.Fatalf("zoom out left %v", a.zoom)
	}
}
