package main

import (
	"net/url"
	"strings"
)

type routeKind int

const (
	routeSessions        routeKind = iota // /sessions: the latest chat
	routeSession                          // /sessions/:id
	routeNewSession                       // /sessions/new
	routeSpaces                           // /spaces
	routeNewSpace                         // /spaces/new
	routeSpace                            // /spaces/:id
	routeSpaceSession                     // /spaces/:id/sessions/:sid
	routeSpaceNewSession                  // /spaces/:id/sessions/new
	routeSpaceSettings                    // /spaces/:id/settings#section
	routeSpaceActivity                    // /spaces/:id/activity
	routeSpaceWallet                      // /spaces/:id/wallet
	routeSpaceFiles                       // /spaces/:id/files/*path
	routeSpaceCheckpoint                  // /spaces/:id/checkpoints/:cid (or new)
	routeSpaceTask                        // /spaces/:id/tasks/:tid
	routeSpaceCron                        // /spaces/:id/cronjobs/:cid (or new)
	routeSettings                         // /settings/:section
)

// route is where the window is, in the web app's URL scheme, so that
// cohub:// links and the web's paths open the same places.
type route struct {
	kind    routeKind
	space   string
	session string
	sub     string // checkpoint, task or cron job ID, files path or settings section
}

func parseRoute(p string) route {
	frag := ""
	if u, err := url.Parse(p); err == nil {
		p, frag = u.Path, u.Fragment
	}
	parts := strings.Split(strings.Trim(p, "/"), "/")
	at := func(i int) string {
		if i < len(parts) {
			return parts[i]
		}
		return ""
	}
	switch at(0) {
	case "sessions":
		switch at(1) {
		case "":
			return route{kind: routeSessions}
		case "new":
			return route{kind: routeNewSession}
		}
		return route{kind: routeSession, session: at(1)}
	case "spaces":
		id := at(1)
		switch id {
		case "":
			return route{kind: routeSpaces}
		case "new":
			return route{kind: routeNewSpace}
		}
		r := route{kind: routeSpace, space: id}
		switch at(2) {
		case "sessions":
			if at(3) == "" || at(3) == "new" {
				r.kind = routeSpaceNewSession
			} else {
				r.kind, r.session = routeSpaceSession, at(3)
			}
		case "settings":
			r.kind, r.sub = routeSpaceSettings, frag
			if r.sub == "" {
				r.sub = "profile"
			}
		case "activity":
			r.kind = routeSpaceActivity
		case "wallet":
			r.kind = routeSpaceWallet
		case "files":
			r.kind, r.sub = routeSpaceFiles, strings.Join(parts[3:], "/")
		case "checkpoints":
			r.kind, r.sub = routeSpaceCheckpoint, at(3)
		case "tasks":
			r.kind, r.sub = routeSpaceTask, at(3)
		case "cronjobs":
			r.kind, r.sub = routeSpaceCron, at(3)
		}
		return r
	case "settings":
		s := at(1)
		if s == "" {
			s = "general"
		}
		return route{kind: routeSettings, sub: s}
	}
	return route{kind: routeSessions}
}

func (r route) String() string {
	switch r.kind {
	case routeSessions:
		return "/sessions"
	case routeSession:
		return "/sessions/" + r.session
	case routeNewSession:
		return "/sessions/new"
	case routeSpaces:
		return "/spaces"
	case routeNewSpace:
		return "/spaces/new"
	case routeSpace:
		return "/spaces/" + r.space
	case routeSpaceSession:
		return "/spaces/" + r.space + "/sessions/" + r.session
	case routeSpaceNewSession:
		return "/spaces/" + r.space + "/sessions/new"
	case routeSpaceSettings:
		return "/spaces/" + r.space + "/settings"
	case routeSpaceActivity:
		return "/spaces/" + r.space + "/activity"
	case routeSpaceWallet:
		return "/spaces/" + r.space + "/wallet"
	case routeSpaceFiles:
		return "/spaces/" + r.space + "/files/" + r.sub
	case routeSpaceCheckpoint:
		return "/spaces/" + r.space + "/checkpoints/" + r.sub
	case routeSpaceTask:
		return "/spaces/" + r.space + "/tasks/" + r.sub
	case routeSpaceCron:
		return "/spaces/" + r.space + "/cronjobs/" + r.sub
	case routeSettings:
		return "/settings/" + r.sub
	}
	return "/sessions"
}

// area is which sidebar a route shows, as the web's resolveAppArea.
type area int

const (
	areaChats area = iota
	areaSpaces
	areaSpace
	areaSettings
)

func (r route) area() area {
	switch r.kind {
	case routeSessions, routeSession, routeNewSession:
		return areaChats
	case routeSpaces, routeNewSpace:
		return areaSpaces
	case routeSettings:
		return areaSettings
	}
	return areaSpace
}

// sessionID is the chat a route shows, "" for none.
func (r route) sessionID() string {
	if r.kind == routeSession || r.kind == routeSpaceSession {
		return r.session
	}
	return ""
}
