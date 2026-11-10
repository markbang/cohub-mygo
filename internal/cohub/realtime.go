package cohub

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// Realtime keeps the gateway's WebSocket (packages/sdk/src/websocket.ts):
// it authenticates with the account's token, subscribes to the rooms it's
// asked to watch, and hands every event to its handler. It reconnects with
// backoff and subscribes to the rooms again.
type Realtime struct {
	cl     *Client
	handle func(Event)

	mu        sync.Mutex
	ws        *websocket.Conn
	rooms     map[string]bool // room → subscribed on this connection
	connected bool
	status    string         // the last connection error
	seen      map[string]int // events by type, for diagnostics
}

// Event is a message of the gateway.
type Event struct {
	Type      string          `json:"type"`
	SpaceID   string          `json:"spaceId"`
	SessionID string          `json:"sessionId"`
	Payload   json.RawMessage `json:"payload"`
}

// NewRealtime connects in the background. handle runs on the connection's
// goroutine, one event at a time.
func NewRealtime(cl *Client, handle func(Event)) *Realtime {
	rt := &Realtime{cl: cl, handle: handle, rooms: map[string]bool{}, seen: map[string]int{}}
	go rt.run()
	return rt
}

func gatewayURL(apiBase string) string {
	if strings.Contains(apiBase, "api-dev.") {
		return "wss://gateway-dev.cohub.live/ws"
	}
	return "wss://gateway.cohub.live/ws"
}

func (rt *Realtime) run() {
	backoff := time.Second
	for {
		err := rt.session()
		rt.mu.Lock()
		rt.connected, rt.ws = false, nil
		if err != nil {
			rt.status = err.Error()
		}
		rt.mu.Unlock()
		time.Sleep(backoff)
		backoff = min(backoff*2, 30*time.Second)
	}
}

func (rt *Realtime) session() error {
	cfg, err := websocket.NewConfig(gatewayURL(rt.cl.base), "https://cohub.live")
	if err != nil {
		return err
	}
	ws, err := websocket.DialConfig(cfg)
	if err != nil {
		return err
	}
	defer ws.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	token, err := rt.cl.AccessToken(ctx)
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
		var ev Event
		if json.Unmarshal(raw, &ev) != nil {
			continue
		}
		rt.receive(ev)
	}
}

func (rt *Realtime) send(v any) {
	rt.mu.Lock()
	ws := rt.ws
	rt.mu.Unlock()
	if ws != nil {
		websocket.JSON.Send(ws, v)
	}
}

func (rt *Realtime) receive(ev Event) {
	rt.mu.Lock()
	rt.seen[ev.Type]++
	rt.mu.Unlock()
	switch ev.Type {
	case "system.auth.ok":
		rt.mu.Lock()
		rt.connected = true
		rt.mu.Unlock()
		rt.flush()
	case "system.subscribe.ok":
		var p struct {
			Rooms []string `json:"rooms"`
		}
		json.Unmarshal(ev.Payload, &p)
		rt.mu.Lock()
		for _, r := range p.Rooms {
			rt.rooms[r] = true
		}
		rt.mu.Unlock()
	}
	rt.handle(ev)
}

// Watch subscribes to the rooms of the Spaces.
func (rt *Realtime) Watch(spaceIDs ...string) {
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

func (rt *Realtime) flush() {
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

// Live reports whether the gateway has accepted the connection.
func (rt *Realtime) Live() bool {
	if rt == nil {
		return false
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.connected
}

// Subscribed reports whether the gateway confirmed a room, like "space:<id>".
func (rt *Realtime) Subscribed(room string) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.rooms[room]
}

// Status is the last connection error.
func (rt *Realtime) Status() string {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.status
}

// Seen counts the events received by type.
func (rt *Realtime) Seen() map[string]int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	out := make(map[string]int, len(rt.seen))
	for k, v := range rt.seen {
		out[k] = v
	}
	return out
}
