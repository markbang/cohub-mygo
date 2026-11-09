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
	routeSpaceCheckpoint                  // /spaces/:id/checkpoints/:cid (or new)
	routeSpaceTask                        // /spaces/:id/tasks/:tid
	routeSpaceCron                        // /spaces/:id/cronjobs/:cid (or new)
	routeSpaceApp                         // /spaces/:id/apps/:appId
	routeSettings                         // /settings/:section
	routeSlug                             // /:username/:slug, a Space by its URL
	routeInvite                           // /invite/:token, /spaces/:id/join/:token, /:username/:slug/join/:token
)

// route is where the window is, in the web app's URL scheme, so that
// cohub:// links and the web's paths open the same places.
type route struct {
	kind    routeKind
	space   string
	session string
	sub     string // checkpoint, task or cron job ID, or settings section
	win     string // the Space's window, ?window=file:path as the web's
}

func parseRoute(p string) route {
	frag, win := "", ""
	if u, err := url.Parse(p); err == nil {
		p, frag = u.Path, u.Fragment
		win = u.Query().Get("window")
		if win == "" {
			win = u.Query().Get("preview")
		}
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
		r := route{kind: routeSpace, space: id, win: win}
		switch at(2) {
		case "sessions":
			if at(3) == "" || at(3) == "new" {
				r.kind = routeSpaceNewSession
			} else {
				r.kind, r.session = routeSpaceSession, at(3)
			}
		case "settings":
			r.kind, r.sub = routeSpaceSettings, frag
			if at(3) == "commerce" {
				r.sub = "commerce"
			}
			if r.sub == "" {
				r.sub = "profile"
			}
		case "activity":
			r.kind = routeSpaceActivity
		case "wallet", "fund":
			r.kind = routeSpaceWallet
		case "join":
			return route{kind: routeInvite, sub: at(3)}
		case "files":
			if f := strings.Join(parts[3:], "/"); f != "" {
				r.win = "file:" + f
			}
		case "checkpoints":
			r.kind, r.sub = routeSpaceCheckpoint, at(3)
		case "tasks":
			r.kind, r.sub = routeSpaceTask, at(3)
		case "cronjobs":
			r.kind, r.sub = routeSpaceCron, at(3)
		case "apps":
			r.kind, r.sub = routeSpaceApp, at(3)
		}
		return r
	case "invite":
		return route{kind: routeInvite, sub: at(1)}
	case "referrals":
		return route{kind: routeSettings, sub: "referrals"}
	case "settings":
		s := at(1)
		if s == "" {
			s = "general"
		}
		if s == "channels" && at(2) == "new" {
			s = "channels/new"
		}
		return route{kind: routeSettings, sub: s}
	}
	if at(0) != "" && at(1) != "" {
		if at(2) == "join" && at(3) != "" {
			return route{kind: routeInvite, sub: at(3)}
		}
		if at(2) == "w" && at(3) != "" {
			return route{kind: routeSlug, sub: at(0) + "/" + at(1) + "/w/" + at(3)}
		}
		return route{kind: routeSlug, sub: at(0) + "/" + at(1)}
	}
	return route{kind: routeSessions}
}

func (r route) String() string {
	s := r.path()
	if r.win != "" {
		s += "?window=" + url.QueryEscape(r.win)
	}
	return s
}

func (r route) path() string {
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
		if r.sub == "commerce" {
			return "/spaces/" + r.space + "/settings/commerce"
		}
		return "/spaces/" + r.space + "/settings"
	case routeSpaceActivity:
		return "/spaces/" + r.space + "/activity"
	case routeSpaceWallet:
		return "/spaces/" + r.space + "/wallet"
	case routeSpaceCheckpoint:
		return "/spaces/" + r.space + "/checkpoints/" + r.sub
	case routeSpaceTask:
		return "/spaces/" + r.space + "/tasks/" + r.sub
	case routeSpaceCron:
		return "/spaces/" + r.space + "/cronjobs/" + r.sub
	case routeSpaceApp:
		return "/spaces/" + r.space + "/apps/" + r.sub
	case routeSettings:
		return "/settings/" + r.sub
	case routeSlug:
		return "/" + r.sub
	case routeInvite:
		return "/invite/" + r.sub
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
	case routeSpaces, routeNewSpace, routeInvite:
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

// file is the path of the file the route's window shows, "" for none.
func (r route) file() string {
	if f, ok := strings.CutPrefix(r.win, "file:"); ok {
		return f
	}
	if f, ok := strings.CutPrefix(r.win, "board:"); ok {
		return f
	}
	return ""
}
