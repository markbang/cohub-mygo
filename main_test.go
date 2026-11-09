package main

import (
	"image/png"
	"os"
	"path/filepath"
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

// snapshot writes the tester's frame to $SNAPSHOT_DIR/name.png when set.
func snapshot(t *testing.T, tt *ui.Tester, name string) {
	dir := os.Getenv("SNAPSHOT_DIR")
	if dir == "" {
		return
	}
	os.MkdirAll(dir, 0o755)
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

func TestDesktopOpensLatestChat(t *testing.T) {
	a := demoApp(t)
	tt := ui.NewTester(a.view, 1440, 900)
	if a.selSession != "demo-0" {
		t.Fatalf("selected %q, want the latest chat", a.selSession)
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
	a.selSession = ""
	tt.Frame()
	if !tt.HasText("全部 Space") || !tt.HasText("Personal Home Space") {
		t.Fatalf("texts %q", tt.Texts())
	}
	snapshot(t, tt, "desktop-spaces-light")
	if err := tt.Click("Personal Home Space"); err != nil {
		t.Fatal(err)
	}
	if a.selSpace != "s5" || !tt.HasText("新建对话") || !tt.HasText("Created by") {
		t.Fatalf("space %q, texts %q", a.selSpace, tt.Texts())
	}
	snapshot(t, tt, "desktop-space-light")
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
	if a.selSession != "" {
		t.Fatalf("compact layout opened %q by itself", a.selSession)
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
	if a.selSession != "demo-0" || !tt.HasText("消息") {
		t.Fatalf("session %q", a.selSession)
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

// TestLiveSnapshot renders the signed-in account's real data, for comparing
// with the web app: COHUB_LIVE=1 SNAPSHOT_DIR=screenshots go test -run Live.
func TestLiveSnapshot(t *testing.T) {
	if os.Getenv("COHUB_LIVE") == "" {
		t.Skip("COHUB_LIVE not set")
	}
	a := newApp()
	a.seenPath = ""
	cl, err := newClient()
	if err != nil {
		t.Skip(err)
	}
	a.api = cl
	a.images.onLoad = nil
	a.refresh()
	if a.status != "" {
		t.Fatal(a.status)
	}
	session := os.Getenv("COHUB_LIVE_SESSION")
	if session == "" {
		session = a.sessions[0].ID
	}
	a.autoPicked = true
	a.selSession = session
	a.loadTurns(session)
	tt := ui.NewTester(a.view, 1440, 900)
	a.images.wait(20 * time.Second)
	tt.Frame()
	snapshot(t, tt, "live-chat")
	a.rail, a.selSession = railSpaces, ""
	tt.Frame()
	a.images.wait(20 * time.Second)
	tt.Frame()
	snapshot(t, tt, "live-spaces")
	a.rail, a.selSpace = railSpaces, a.spaces[0].ID
	a.loadSpaceSessions(a.spaces[0].ID)
	tt.Frame()
	a.images.wait(10 * time.Second)
	tt.Frame()
	snapshot(t, tt, "live-space")
	tc := ui.NewTester(a.view, 390, 844)
	a.rail, a.selSpace = railChats, ""
	tc.Frame()
	a.images.wait(10 * time.Second)
	tc.Frame()
	snapshot(t, tc, "live-compact")
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
