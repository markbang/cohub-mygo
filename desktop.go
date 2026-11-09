package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Desktop commands are what an agent asks the client its chat came from
// to show, as the web's desktop-command bus: every request carries the
// client's id (X-Cohub-Source-Client), so a chat started here can open a
// file or an app in this window. A command comes as the user's
// desktop.command.dispatched event; the client accepts it, runs it and
// reports how it went.

// clientID is this install's id, kept across launches.
func clientID() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return newClientID()
	}
	p := filepath.Join(dir, "cohub-mygo", "client-id")
	if b, err := os.ReadFile(p); err == nil {
		if id := strings.TrimSpace(string(b)); len(id) >= 8 {
			return id
		}
	}
	id := newClientID()
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(id+"\n"), 0o644)
	return id
}

func newClientID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "mygo-" + hex.EncodeToString(b)
}

type desktopCommand struct {
	CommandID      string `json:"commandId"`
	TargetClientID string `json:"targetClientId"`
	Command        struct {
		Type   string `json:"type"`
		Target *struct {
			Kind  string `json:"kind"`
			Path  string `json:"path"`
			AppID string `json:"appId"`
			Label string `json:"label"`
		} `json:"target"`
		Preview *struct {
			Kind  string `json:"kind"`
			Path  string `json:"path"`
			AppID string `json:"appId"`
		} `json:"preview"`
	} `json:"command"`
	Source *struct {
		SpaceID   string `json:"spaceId"`
		SessionID string `json:"sessionId"`
	} `json:"source"`
}

// desktopCommandDispatched takes a command for this client.
func (a *app) desktopCommandDispatched(payload json.RawMessage) {
	var d desktopCommand
	if json.Unmarshal(payload, &d) != nil || d.CommandID == "" || d.TargetClientID != a.api.clientID {
		return
	}
	if a.handledCommands == nil {
		a.handledCommands = map[string]bool{}
	}
	if a.handledCommands[d.CommandID] {
		return
	}
	a.handledCommands[d.CommandID] = true
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		path := "/api/desktop/commands/" + url.PathEscape(d.CommandID)
		var acc struct {
			Accepted bool `json:"accepted"`
		}
		if err := a.api.do(ctx, "POST", path+"/accept", nil, &acc); err != nil || !acc.Accepted {
			return
		}
		status, errBody := "applied", map[string]any(nil)
		done := make(chan struct{})
		a.update(func() {
			defer close(done)
			if s, e := a.runDesktopCommand(&d); s != "" {
				status = s
				if e != "" {
					errBody = map[string]any{"code": s, "message": e}
				}
			}
		})
		<-done
		var body = map[string]any{"status": status, "error": nil}
		if errBody != nil {
			body["error"] = errBody
		}
		a.api.do(ctx, "POST", path+"/result", body, nil)
	})
}

// runDesktopCommand opens what the command asks for, in the Space the
// chat runs in, and returns its status, "" for applied.
func (a *app) runDesktopCommand(d *desktopCommand) (string, string) {
	kind, p, appID := "", "", ""
	switch {
	case d.Command.Target != nil:
		kind, p, appID = d.Command.Target.Kind, d.Command.Target.Path, d.Command.Target.AppID
	case d.Command.Preview != nil:
		kind, p, appID = d.Command.Preview.Kind, d.Command.Preview.Path, d.Command.Preview.AppID
	}
	space := ""
	if d.Source != nil {
		space = d.Source.SpaceID
	}
	if space == "" {
		space = a.currentSpace()
	}
	if space == "" {
		return "desktop_host_unavailable", "This Cohub window is not showing a Space."
	}
	session := ""
	if d.Source != nil {
		session = d.Source.SessionID
	}
	r := route{kind: routeSpace, space: space}
	if session != "" {
		r.kind, r.session = routeSpaceSession, session
	}
	switch kind {
	case "file":
		r.win = "file:" + p
		if strings.HasSuffix(strings.ToLower(p), ".board") {
			r.win = "board:" + p
		}
	case "app", "work":
		r.win = "app:" + appID
	default:
		return "unsupported", "command.target.kind must be one of: app, file"
	}
	a.go_(r)
	return "", ""
}
