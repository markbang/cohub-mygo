package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLivePatchStream(t *testing.T) {
	if os.Getenv("COHUB_LIVE_STREAM") == "" {
		t.Skip()
	}
	n := 0
	debugPatch = func(s string) {
		if n < 8 {
			t.Log(s)
		}
		n++
	}
	a := newApp()
	a.start()
	time.Sleep(3 * time.Second)
	home := "0565aad4-5605-4ad0-a0fc-50be6dfce711"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sid, err := a.api.prompt(ctx, home, "", []map[string]any{{"type": "text", "text": "用中文写一段 300 字的短文，介绍 GPU 自绘界面的好处。不要使用任何工具。"}}, modelChoice{provider: "cohub", id: "deepseek-flash"})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("session", sid)
	a.update(func() { a.go_(route{kind: routeSpaceSession, space: home, session: sid}) })
	var maxLive int
	for i := 0; i < 60; i++ {
		time.Sleep(500 * time.Millisecond)
		done := make(chan struct{})
		a.update(func() {
			if l := len(a.timelineOf(sid).live); l > maxLive {
				maxLive = l
			}
			close(done)
		})
		<-done
	}
	done := make(chan struct{})
	a.update(func() {
		tl := a.timelineOf(sid)
		last := ""
		if len(tl.turns) > 0 {
			last = tl.turns[len(tl.turns)-1].Status
		}
		t.Logf("patched %d resynced %d max live %d, last turn %s, seen %v", a.rt.patched, a.rt.resynced, maxLive, last, a.rt.seen)
		close(done)
	})
	<-done
	// Archive the probe chat.
	a.api.do(context.Background(), "PUT", "/api/sessions/"+sid+"/archive", nil, nil)
	_ = strings.TrimSpace
}
