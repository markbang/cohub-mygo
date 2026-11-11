package app

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"cohub-mygo/internal/cohub"
)

// Headless baselines of the desktop window: frames built and drawn by
// ui.Tester on the CPU. Run them with:
//
//	CGO_ENABLED=0 GOMAXPROCS=1 go test ./internal/app -run '^$' \
//	  -bench 'Benchmark' -benchmem -count=6 -benchtime=300ms

const benchReply = demoReply + "\n\n" + demoReply + "\n\n" + demoReply

func benchTurn(id string, seq int, at time.Time) *cohub.Turn {
	done := at.Add(time.Second)
	return &cohub.Turn{
		ID: id, Sequence: seq, Status: "completed",
		UserContent:      []cohub.Block{{Type: "text", Text: fmt.Sprintf("Question %d: how does this render?", seq)}},
		AssistantContent: []cohub.Block{{Type: "text", Text: benchReply}},
		Model:            "claude-opus-5-5", ThinkingLevel: "high",
		StartedAt: at, CompletedAt: &done,
		TotalUsage: &cohub.Usage{Input: 1100, CacheRead: 137100, Output: 1300},
		Meta:       cohub.TurnMeta{Harness: "cohub", Source: "web", FinalMessageDurationMs: 19181},
	}
}

// benchTimeline gives the chat the demo opens n turns.
func benchTimeline(a *app, id string, n int) {
	tl := a.timelineOf(id)
	tl.loaded = true
	tl.turns = nil
	start := a.now().Add(-time.Duration(n) * time.Minute)
	for i := 1; i <= n; i++ {
		tl.turns = append(tl.turns, benchTurn(fmt.Sprintf("%s-%d", id, i), i, start.Add(time.Duration(i)*time.Minute)))
	}
}

// benchInbox fills the chats sidebar with n more chats.
func benchInbox(a *app, n int) {
	base := a.now().Add(-time.Hour)
	for i := 0; i < n; i++ {
		s := &cohub.Session{
			ID: fmt.Sprintf("bench-%d", i), SpaceID: "s1", Title: fmt.Sprintf("Benchmark chat %d", i),
			Source: "web", Status: "active",
			LatestMessageText: "a preview of the last message in this long list",
			LastMessageAt:     base.Add(-time.Duration(i) * time.Second),
		}
		a.sessions = append(a.sessions, s)
		a.spaceSessions["s1"] = append(a.spaceSessions["s1"], s)
	}
}

func benchTester(tb testing.TB, a *app) *ui.Tester {
	tb.Helper()
	tt := ui.NewTester(a.view, 1440, 900)
	for range 3 {
		tt.Frame()
	}
	return tt
}

// BenchmarkColdStartFrame is the first frames of the app with the demo
// data: the closest headless stand-in for launch to first paint.
func BenchmarkColdStartFrame(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		a := demoApp(b)
		benchTimeline(a, "demo-0", 40)
		tt := ui.NewTester(a.view, 1440, 900)
		runtime.KeepAlive(tt)
	}
}

// BenchmarkChatFrame is a steady frame of an open chat.
func BenchmarkChatFrame(b *testing.B) {
	a := demoApp(b)
	tt := benchTester(b, a)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		tt.Frame()
	}
}

// BenchmarkInboxFrame is a steady frame with 1,000 chats in the sidebar.
func BenchmarkInboxFrame(b *testing.B) {
	a := demoApp(b)
	benchInbox(a, 1000)
	tt := benchTester(b, a)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		tt.Frame()
	}
}

// BenchmarkSessionSwitchFrame switches chats, which rebuilds the timeline.
func BenchmarkSessionSwitchFrame(b *testing.B) {
	a := demoApp(b)
	benchTimeline(a, "demo-3", 100)
	tt := benchTester(b, a)
	ids := []string{"demo-0", "demo-3"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		a.go_(route{kind: routeSession, session: ids[i%2]})
		tt.Frame()
	}
}

// BenchmarkTimelineScrollFrame scrolls a 200-turn chat a turn at a time.
func BenchmarkTimelineScrollFrame(b *testing.B) {
	a := demoApp(b)
	benchTimeline(a, "demo-0", 200)
	tt := benchTester(b, a)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		a.timelineScroll.Y = float32(i%200) * 240
		tt.Frame()
	}
}

// BenchmarkStreamingFrame appends 16 bytes of a running reply each frame.
func BenchmarkStreamingFrame(b *testing.B) {
	a := demoApp(b)
	text := strings.Repeat(benchReply, 40)
	tl := a.timelineOf("demo-0")
	tl.turns = []*cohub.Turn{{
		ID: "stream", Sequence: 1, Status: "running",
		UserContent: []cohub.Block{{Type: "text", Text: "write a long answer"}},
		StartedAt:   a.now().Add(-time.Minute),
	}}
	tt := benchTester(b, a)
	n := 0
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		n += 16
		if n > len(text) {
			n = 16
		}
		tl.turns[0].AssistantContent = []cohub.Block{{Type: "text", Text: text[:n]}}
		tt.Frame()
	}
}

// BenchmarkTypingFrame types into the composer, which keeps a text input
// and its menus fresh each frame.
func BenchmarkTypingFrame(b *testing.B) {
	a := demoApp(b)
	tt := benchTester(b, a)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		tt.Type("a")
		tt.Frame()
	}
}

// TestChatRetainedHeap logs the Go heap a warmed chat window keeps, for
// the memory baseline in docs/native-audit.md:
//
//	COHUB_PERF=1 CGO_ENABLED=0 GOMAXPROCS=1 go test ./internal/app -run RetainedHeap -v
func TestChatRetainedHeap(t *testing.T) {
	if testing.Short() {
		t.Skip("measures retained heap")
	}
	warmA := demoApp(t)
	benchTimeline(warmA, "demo-0", 200)
	warm := benchTester(t, warmA)
	runtime.KeepAlive(warm)
	warm = nil
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	a := demoApp(t)
	benchTimeline(a, "demo-0", 200)
	tt := benchTester(t, a)
	runtime.GC()
	runtime.ReadMemStats(&after)
	t.Logf("retained Go heap for a 200-turn chat: %d bytes", int64(after.HeapAlloc)-int64(before.HeapAlloc))
	runtime.KeepAlive(tt)
}
