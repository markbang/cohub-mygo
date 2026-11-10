package app

import (
	"encoding/json"
	"os"
	"path/filepath"

	"cohub-mygo/internal/cohub"
)

// startupCache is what the window showed at the last refresh, so the next
// launch shows it at once while the API loads. A refresh replaces all of
// it, so a stale or another account's cache shows only until then.
type startupCache struct {
	Version  int                 `json:"v"`
	API      string              `json:"api"`
	Me       *cohub.Me           `json:"me"`
	Spaces   []*cohub.Space      `json:"spaces"`
	Filter   string              `json:"filter"`
	Sessions []*cohub.Session    `json:"sessions"`
	Forks    []cohub.SessionFork `json:"forks,omitempty"`
	Models   []*cohub.Model      `json:"models"`
	Chat     string              `json:"chat,omitempty"`
	Turns    []*cohub.Turn       `json:"turns,omitempty"`
	HasMore  bool                `json:"hasMore,omitempty"`
}

const startupCacheVersion = 1

func startupCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "cohub-mygo", "startup.json")
}

// loadStartupCache shows the cache of the client's API, unless a refresh
// came first.
func (a *app) loadStartupCache() {
	path, api := a.cachePath, a.api.Base()
	if path == "" {
		return
	}
	a.spawn(func() {
		b, err := os.ReadFile(path)
		if err != nil {
			return
		}
		var c startupCache
		if json.Unmarshal(b, &c) != nil || c.Version != startupCacheVersion || c.API != api || c.Me == nil {
			return
		}
		a.update(func() {
			if a.me != nil {
				return
			}
			if c.Chat != "" && len(c.Turns) > 0 {
				tl := a.timelineOf(c.Chat)
				tl.turns, tl.hasMore, tl.loaded = c.Turns, c.HasMore, true
			}
			if c.Filter == a.chats.key() {
				a.inboxForks = c.Forks
			}
			a.showLists(c.Me, c.Spaces, c.Sessions, c.Models, c.Filter == a.chats.key())
			if len(a.sessions) > 0 {
				a.cacheOpened = route{kind: routeSession, session: a.sessions[0].ID}
			}
		})
	})
}

// saveStartupCache keeps the lists, and the turns of the latest chat, which
// a launch opens, for the next launch. It encodes on the main thread, which
// owns the data, and writes off it.
func (a *app) saveStartupCache() {
	if a.cachePath == "" || a.api == nil || a.me == nil {
		return
	}
	c := startupCache{
		Version: startupCacheVersion, API: a.api.Base(), Me: a.me, Spaces: a.spaces,
		Filter: a.chats.key(), Sessions: a.sessions, Forks: a.inboxForks, Models: a.models,
	}
	if len(a.sessions) > 0 {
		id := a.sessions[0].ID
		if tl := a.timelines[id]; tl != nil && tl.loaded && tl.err == "" {
			c.Chat, c.Turns, c.HasMore = id, tl.turns, tl.hasMore
		}
	}
	b, err := json.Marshal(c)
	if err != nil {
		return
	}
	path := a.cachePath
	a.spawn(func() {
		os.MkdirAll(filepath.Dir(path), 0o700)
		tmp := path + ".tmp"
		if os.WriteFile(tmp, b, 0o600) == nil {
			os.Rename(tmp, path)
		}
	})
}

func (a *app) dropStartupCache() {
	if a.cachePath != "" {
		os.Remove(a.cachePath)
	}
}
