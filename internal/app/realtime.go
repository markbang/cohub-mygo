package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"cohub-mygo/internal/cohub"
)

// realtime is the app's side of the gateway's connection.
type realtime struct {
	a    *app
	conn *cohub.Realtime

	mu       sync.Mutex
	turnsDue *time.Timer
	listsDue *time.Timer
	turnsFor string
	patched  int // patches applied, and patches that needed the snapshot
	resynced int
}

func (a *app) startRealtime() {
	rt := &realtime{a: a}
	rt.conn = cohub.NewRealtime(a.api, rt.handle)
	a.rt = rt
}

// handle turns the gateway's events into reloads of what changed: a
// stream snapshot or patch for the open chat, the lists otherwise.
func (rt *realtime) handle(env cohub.Event) {
	switch {
	case env.Type == "desktop.command.dispatched":
		rt.a.update(func() { rt.a.desktopCommandDispatched(env.Payload) })
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
			switch {
			case env.Type == "session.turn.patch":
				rt.applyPatch(open, env.Payload)
			case env.Type == "session.turn.lifecycle":
				// A model call starting changes nothing shown: its patches do.
			case env.Type == "session.message.persisted" && rt.persisted(open, env.Payload):
			default:
				rt.dueTurns(open, env.Type)
			}
		}
		switch env.Type {
		case "session.turn.patch", "session.turn.lifecycle", "session.message.persisted":
		default:
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
func (rt *realtime) watch(spaceIDs ...string) { rt.conn.Watch(spaceIDs...) }

func (rt *realtime) live() bool { return rt != nil && rt.conn.Live() }

var debugPatch func(string)

// applyPatch builds the running reply from a patch, or reloads the chat
// when the patch does not follow what the window has.
func (rt *realtime) applyPatch(sessionID string, payload json.RawMessage) {
	var ev cohub.PatchEvent
	if json.Unmarshal(payload, &ev) != nil {
		rt.dueTurns(sessionID, "session.turn.patch")
		return
	}
	rt.a.update(func() {
		tl := rt.a.timelineOf(sessionID)
		if tl.stream == nil {
			tl.stream = &cohub.Stream{}
		}
		if !tl.stream.Patch(ev) {
			if debugPatch != nil {
				debugPatch(fmt.Sprintf("reject seq=%d base=%d message=%s ops=%s", ev.Seq, ev.BaseSeq, ev.Message(), string(payload)[:min(400, len(payload))]))
			}
			rt.resynced++
			go rt.dueTurns(sessionID, "session.turn.snapshot")
			return
		}
		rt.patched++
	})
}

// persisted takes a step of the running turn the API saved, and reports
// whether it did: other messages, as the user's or the final reply, reload
// the chat.
func (rt *realtime) persisted(sessionID string, payload json.RawMessage) bool {
	var p struct {
		Message *struct {
			cohub.StepMessage
			Role string `json:"role"`
		} `json:"message"`
	}
	if json.Unmarshal(payload, &p) != nil || p.Message == nil || p.Message.Role != "assistant" ||
		p.Message.Meta.MessageKind != "assistant_intermediate" {
		return false
	}
	m := p.Message.StepMessage
	rt.a.update(func() {
		tl := rt.a.timelineOf(sessionID)
		if tl.stream == nil {
			tl.stream = &cohub.Stream{}
		}
		tl.stream.Persisted(&m)
	})
	return true
}
