package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// realtime keeps the gateway's WebSocket (packages/sdk/src/websocket.ts):
// it authenticates with the account's token, subscribes to the rooms of
// the Spaces the window shows, and turns their events into reloads of what
// changed, a stream snapshot for the open chat, the lists otherwise.
type realtime struct {
	a  *app
	mu sync.Mutex
	ws *websocket.Conn

	rooms     map[string]bool
	connected bool

	turnsDue   *time.Timer
	listsDue   *time.Timer
	turnsFor   string
	lastStatus string
	seen       map[string]int // events by type, for diagnostics
	patched    int            // patches applied, and patches that needed the snapshot
	resynced   int
}

func gatewayURL(apiBase string) string {
	if strings.Contains(apiBase, "api-dev.") {
		return "wss://gateway-dev.cohub.live/ws"
	}
	return "wss://gateway.cohub.live/ws"
}

func (a *app) startRealtime() {
	rt := &realtime{a: a, rooms: map[string]bool{}, seen: map[string]int{}}
	a.rt = rt
	go rt.run()
}

func (rt *realtime) run() {
	backoff := time.Second
	for {
		err := rt.session()
		rt.mu.Lock()
		rt.connected, rt.ws = false, nil
		rt.mu.Unlock()
		if err != nil {
			rt.lastStatus = err.Error()
		}
		time.Sleep(backoff)
		backoff = min(backoff*2, 30*time.Second)
	}
}

type wsEnvelope struct {
	Type      string          `json:"type"`
	SpaceID   string          `json:"spaceId"`
	SessionID string          `json:"sessionId"`
	Payload   json.RawMessage `json:"payload"`
}

func (rt *realtime) session() error {
	cfg, err := websocket.NewConfig(gatewayURL(rt.a.api.base), "https://cohub.live")
	if err != nil {
		return err
	}
	ws, err := websocket.DialConfig(cfg)
	if err != nil {
		return err
	}
	defer ws.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	token, err := rt.a.api.accessToken(ctx)
	cancel()
	if err != nil {
		return err
	}
	if err := websocket.JSON.Send(ws, map[string]any{
		"type":    "auth",
		"payload": map[string]any{"token": token, "capabilities": []string{"realtime.rooms.v1"}},
	}); err != nil {
		return err
	}
	rt.mu.Lock()
	rt.ws = ws
	for r := range rt.rooms {
		rt.rooms[r] = false
	}
	rt.mu.Unlock()
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		t := time.NewTicker(25 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				rt.send(map[string]any{"type": "ping", "payload": map[string]any{}})
			}
		}
	}()
	for {
		var raw []byte
		if err := websocket.Message.Receive(ws, &raw); err != nil {
			return err
		}
		var env wsEnvelope
		if json.Unmarshal(raw, &env) != nil {
			continue
		}
		rt.handle(env)
	}
}

func (rt *realtime) send(v any) {
	rt.mu.Lock()
	ws := rt.ws
	rt.mu.Unlock()
	if ws != nil {
		websocket.JSON.Send(ws, v)
	}
}

func (rt *realtime) handle(env wsEnvelope) {
	rt.mu.Lock()
	rt.seen[env.Type]++
	rt.mu.Unlock()
	switch {
	case env.Type == "system.auth.ok":
		rt.mu.Lock()
		rt.connected = true
		rt.mu.Unlock()
		rt.flush()
	case env.Type == "system.subscribe.ok":
		var p struct {
			Rooms []string `json:"rooms"`
		}
		json.Unmarshal(env.Payload, &p)
		rt.mu.Lock()
		for _, r := range p.Rooms {
			rt.rooms[r] = true
		}
		rt.mu.Unlock()
	case env.Type == "session.turn.notify":
		rt.a.turnNotified(env.Payload)
		rt.dueLists()
	case strings.HasPrefix(env.Type, "session."):
		if env.Type == "session.turn.created" {
			rt.a.update(func() { rt.a.danmakuTurn(env.SpaceID, env.SessionID, env.Payload) })
		}
		open := ""
		done := make(chan struct{})
		rt.a.update(func() { open = rt.a.route.sessionID(); close(done) })
		<-done
		if env.SessionID != "" && env.SessionID == open {
			if env.Type == "session.turn.patch" {
				rt.applyPatch(open, env.Payload)
			} else {
				rt.dueTurns(open, env.Type)
			}
		}
		if env.Type != "session.turn.patch" {
			rt.dueLists()
		}
	case strings.HasPrefix(env.Type, "space."):
		switch env.Type {
		case "space.list.changed", "space.updated":
			rt.dueLists()
		case "space.ports.changed":
			rt.a.update(func() { rt.a.portsChanged(env.SpaceID, env.Payload) })
		case "space.presence.updated":
			rt.presenceUpdated(env.Payload)
		case "space.session.approvals.changed":
			rt.a.update(func() { rt.a.forget("approvals:" + env.SessionID) })
		case "space.fs.changed":
			rt.a.update(func() { rt.a.forgetPrefix("tree:"+env.SpaceID, "file:"+env.SpaceID) })
		case "space.cronjobs.changed", "space.cronjob.updated":
			rt.a.update(func() { rt.a.forget("crons:" + env.SpaceID) })
		case "space.labels.changed", "label.assignments.updated":
			rt.a.update(func() { rt.a.forgetPrefix("labels:"+env.SpaceID, "res-labels:"+env.SpaceID) })
		}
	}
}

// dueTurns reloads the open chat soon: at most every 300 ms while it
// streams, so the reply comes in as it is written.
func (rt *realtime) dueTurns(id, kind string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.turnsDue != nil && rt.turnsFor == id {
		return
	}
	rt.turnsFor = id
	delay := 300 * time.Millisecond
	if kind != "session.turn.patch" {
		delay = 50 * time.Millisecond
	}
	rt.turnsDue = time.AfterFunc(delay, func() {
		rt.mu.Lock()
		rt.turnsDue = nil
		rt.mu.Unlock()
		rt.a.loadTurns(id)
	})
}

func (rt *realtime) dueLists() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.listsDue != nil {
		return
	}
	rt.listsDue = time.AfterFunc(1500*time.Millisecond, func() {
		rt.mu.Lock()
		rt.listsDue = nil
		rt.mu.Unlock()
		rt.a.refresh()
	})
}

// watch subscribes to the rooms of the Spaces the window shows.
func (rt *realtime) watch(spaceIDs ...string) {
	rt.mu.Lock()
	changed := false
	for _, id := range spaceIDs {
		if id == "" {
			continue
		}
		r := "space:" + id
		if _, ok := rt.rooms[r]; !ok {
			rt.rooms[r] = false
			changed = true
		}
	}
	rt.mu.Unlock()
	if changed {
		rt.flush()
	}
}

func (rt *realtime) flush() {
	rt.mu.Lock()
	if !rt.connected {
		rt.mu.Unlock()
		return
	}
	var rooms []string
	for r, ok := range rt.rooms {
		if !ok {
			rooms = append(rooms, r)
		}
	}
	rt.mu.Unlock()
	if len(rooms) > 0 {
		rt.send(map[string]any{"type": "subscribe", "payload": map[string]any{"rooms": rooms}})
	}
}

func (rt *realtime) live() bool {
	if rt == nil {
		return false
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.connected
}

var debugPatch func(string)

// applyPatch builds the running reply from a patch, or reloads the chat
// when the patch does not follow what the window has.
func (rt *realtime) applyPatch(sessionID string, payload json.RawMessage) {
	var ev patchEvent
	if json.Unmarshal(payload, &ev) != nil {
		rt.dueTurns(sessionID, "session.turn.patch")
		return
	}
	rt.a.update(func() {
		tl := rt.a.timelineOf(sessionID)
		if tl.patch == nil && ev.BaseSeq == 0 {
			tl.patch = &patchState{}
		}
		if tl.patch == nil || !tl.patch.apply(ev) {
			if debugPatch != nil {
				st := -1
				if tl.patch != nil {
					st = tl.patch.seq
				}
				ap := ""
				if tl.patch != nil {
					ap = tl.patch.appendPath
				}
				debugPatch(fmt.Sprintf("reject seq=%d base=%d state=%d append=%q ops=%s", ev.Seq, ev.BaseSeq, st, ap, string(payload)[:min(400, len(payload))]))
			}
			rt.resynced++
			go rt.dueTurns(sessionID, "session.turn.snapshot")
			return
		}
		rt.patched++
		tl.liveSteps = tl.patch.content()
		tl.live = joinText(tl.liveSteps)
		if ev.TurnID != "" {
			tl.liveTurn = ev.TurnID
		}
	})
}
