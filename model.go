package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

type rail int

const (
	railChats rail = iota
	railSpaces
	railAccount // compact layouts only
)

type themeMode int

const (
	themeSystem themeMode = iota
	themeLight
	themeDark
)

// sessionTurns is what the timeline of one session shows.
type sessionTurns struct {
	turns   []*Turn
	hasMore bool
	loaded  bool
	err     string
	// live is the reply of the running turn so far, from the stream snapshot.
	live      string
	liveTurn  string
	liveSteps int
}

// app is the whole state of the window: data from the API and what the
// user chose. Only the main thread touches it; work on goroutines hands
// its results over with update.
type app struct {
	win *mygo.Window
	api *client
	now func() time.Time

	status string // connection state shown in headers, "" when fine
	me     *Me

	spaces        []*Space
	sessions      []*Session // across spaces, most recent first
	spaceSessions map[string][]*Session
	timelines     map[string]*sessionTurns
	seen          map[string]time.Time
	seenPath      string
	images        *imageCache

	// What the window shows.
	rail        rail
	chip        string // the chats sidebar's Space filter, "" for all
	selSpace    string
	selSession  string
	spacesNav   int
	panelTab    int
	themeMode   themeMode
	sidebarOpen bool
	panelOpen   bool
	userMenu    bool
	draft       string
	sending     bool
	sendErr     string
	autoPicked  bool

	sessionList ui.ListState
	spaceList   ui.ListState
	timeline    ui.ListState
	md          markdown.Options
	mdTheme     *markdown.Theme
	mdThemeDark bool
	pal         *palette
	compact     bool
}

func newApp() *app {
	a := &app{
		now:           time.Now,
		spaceSessions: map[string][]*Session{},
		timelines:     map[string]*sessionTurns{},
		seen:          map[string]time.Time{},
		images:        newImageCache(),
		sidebarOpen:   true,
		panelOpen:     true,
		pal:           lightPalette(),
	}
	a.md.Reveal = &markdown.Reveal{Unit: markdown.ByWord}
	a.images.onLoad = func(fn func()) { a.update(fn) }
	if dir, err := os.UserConfigDir(); err == nil {
		a.seenPath = filepath.Join(dir, "cohub-mygo", "seen.json")
		if b, err := os.ReadFile(a.seenPath); err == nil {
			json.Unmarshal(b, &a.seen)
		}
	}
	return a
}

// update runs fn on the main thread and redraws; without a window (tests)
// it runs fn at once.
func (a *app) update(fn func()) {
	if a.win == nil {
		fn()
		return
	}
	a.win.Update(fn)
}

// start connects to the API and keeps the data fresh.
func (a *app) start() {
	cl, err := newClient()
	if err != nil {
		a.status = err.Error()
		loadDemo(a)
		return
	}
	a.api = cl
	a.status = "连接中…"
	go a.refresh()
	go a.poll()
}

func (a *app) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var (
		wg       sync.WaitGroup
		me       *Me
		spaces   []*Space
		sessions []*Session
		errs     [3]error
	)
	wg.Add(3)
	go func() { defer wg.Done(); me, errs[0] = a.api.me(ctx) }()
	go func() { defer wg.Done(); spaces, errs[1] = a.api.spaces(ctx) }()
	go func() { defer wg.Done(); sessions, errs[2] = a.api.mySessions(ctx) }()
	wg.Wait()
	a.update(func() {
		for _, err := range errs {
			if err != nil {
				a.status = "离线：" + err.Error()
				return
			}
		}
		a.status = ""
		a.me = me
		sort.SliceStable(spaces, func(i, j int) bool { return spaces[i].activity().After(spaces[j].activity()) })
		a.spaces = spaces
		sort.SliceStable(sessions, func(i, j int) bool { return sessions[i].when().After(sessions[j].when()) })
		a.sessions = sessions
	})
}

// poll refreshes the lists every 15 seconds and the open session every
// two while it has a turn running.
func (a *app) poll() {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for i := 1; ; i++ {
		<-tick.C
		var sessionID, spaceID string
		running := false
		done := make(chan struct{})
		a.update(func() {
			sessionID, spaceID = a.selSession, a.selSpace
			if s := a.session(sessionID); s != nil {
				running = s.running()
			}
			if tl := a.timelines[sessionID]; tl != nil {
				for _, t := range tl.turns {
					running = running || t.running()
				}
			}
			close(done)
		})
		<-done
		if sessionID != "" && running {
			a.loadTurns(sessionID)
		}
		if i%8 == 0 {
			a.refresh()
			if spaceID != "" {
				a.loadSpaceSessions(spaceID)
			}
		}
	}
}

func (a *app) loadTurns(sessionID string) {
	if a.api == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	page, err := a.api.turns(ctx, sessionID, 30)
	var snap *streamSnapshot
	if err == nil && len(page.Turns) > 0 && page.Turns[len(page.Turns)-1].running() {
		snap, _ = a.api.snapshot(ctx, sessionID)
	}
	a.update(func() {
		tl := a.timelines[sessionID]
		if tl == nil {
			tl = &sessionTurns{}
			a.timelines[sessionID] = tl
		}
		tl.loaded = true
		if err != nil {
			tl.err = err.Error()
			return
		}
		tl.err = ""
		tl.turns, tl.hasMore = page.Turns, page.HasMore
		tl.live, tl.liveSteps = "", 0
		if snap != nil && snap.Snapshot != nil && snap.Snapshot.Current != nil {
			tl.live = joinText(snap.Snapshot.Current.Content)
			tl.liveTurn = snap.Snapshot.TurnID
			tl.liveSteps = len(snap.Snapshot.IntermediateMessages)
		}
		if s := a.session(sessionID); s != nil && len(page.Turns) > 0 {
			last := page.Turns[len(page.Turns)-1]
			if !last.running() && s.ActiveTurn != nil {
				s.ActiveTurn = nil
			}
		}
	})
}

func (a *app) loadSpaceSessions(spaceID string) {
	if a.api == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	list, err := a.api.spaceSessions(ctx, spaceID)
	if err != nil {
		return
	}
	a.update(func() {
		sort.SliceStable(list, func(i, j int) bool { return list[i].when().After(list[j].when()) })
		a.spaceSessions[spaceID] = list
	})
}

// openSession shows a session and loads its turns.
func (a *app) openSession(s *Session) {
	if s == nil {
		return
	}
	a.selSession = s.ID
	a.sendErr = ""
	if a.rail == railSpaces {
		a.selSpace = s.SpaceID
	}
	a.timeline = ui.ListState{}
	a.markSeen(s)
	if a.timelines[s.ID] == nil {
		a.timelines[s.ID] = &sessionTurns{}
	}
	if a.api != nil {
		go a.loadTurns(s.ID)
	}
}

func (a *app) openSpace(id string) {
	a.rail = railSpaces
	a.selSpace = id
	a.selSession = ""
	a.sendErr = ""
	go a.loadSpaceSessions(id)
}

func (a *app) markSeen(s *Session) {
	a.seen[s.ID] = s.when()
	if a.seenPath == "" {
		return
	}
	b, _ := json.Marshal(a.seen)
	go func() {
		os.MkdirAll(filepath.Dir(a.seenPath), 0o755)
		os.WriteFile(a.seenPath, b, 0o644)
	}()
}

func (a *app) unread(s *Session) bool {
	if s.ID == a.selSession || s.running() {
		return false
	}
	seen, ok := a.seen[s.ID]
	return !ok || s.when().After(seen.Add(time.Second))
}

func (a *app) session(id string) *Session {
	if id == "" {
		return nil
	}
	for _, s := range a.sessions {
		if s.ID == id {
			return s
		}
	}
	for _, list := range a.spaceSessions {
		for _, s := range list {
			if s.ID == id {
				return s
			}
		}
	}
	return nil
}

func (a *app) space(id string) *Space {
	for _, s := range a.spaces {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// chatSpaces are the chips above the chats: Spaces with recent chats, in
// the order of their latest.
func (a *app) chatSpaces() []*Space {
	var out []*Space
	seen := map[string]bool{}
	for _, s := range a.sessions {
		if seen[s.SpaceID] {
			continue
		}
		seen[s.SpaceID] = true
		if sp := a.space(s.SpaceID); sp != nil {
			out = append(out, sp)
		}
	}
	return out
}

func (a *app) visibleSessions() []*Session {
	if a.chip == "" {
		return a.sessions
	}
	var out []*Session
	for _, s := range a.sessions {
		if s.SpaceID == a.chip {
			out = append(out, s)
		}
	}
	return out
}

func (a *app) filteredSpaces() []*Space {
	var out []*Space
	for _, s := range a.spaces {
		switch a.spacesNav {
		case 0:
			if s.IsArchived {
				continue
			}
		case 2:
			if s.Relation != "owner" || s.IsArchived {
				continue
			}
		case 3:
			if !s.IsPinned {
				continue
			}
		case 4:
			if !s.IsArchived {
				continue
			}
		}
		out = append(out, s)
	}
	if a.spacesNav == 1 {
		out = append([]*Space(nil), out...)
		sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	}
	return out
}

// send sends the composer's text to the open session, or starts one in
// the open Space.
func (a *app) send() {
	text := strings.TrimSpace(a.draft)
	if text == "" || a.sending {
		return
	}
	spaceID, sessionID := a.selSpace, a.selSession
	if s := a.session(sessionID); s != nil {
		spaceID = s.SpaceID
	}
	if spaceID == "" {
		a.sendErr = "先选择一个 Space"
		return
	}
	if a.api == nil {
		a.sendErr = "演示模式不能发送"
		return
	}
	a.sending, a.sendErr = true, ""
	a.draft = ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		id, err := a.api.prompt(ctx, spaceID, sessionID, text)
		a.update(func() {
			a.sending = false
			if err != nil {
				a.sendErr = err.Error()
				a.draft = text
				return
			}
			if id != "" && a.selSession == "" {
				a.selSession = id
				a.timelines[id] = &sessionTurns{}
			}
		})
		if id != "" {
			a.loadTurns(id)
		}
		a.refresh()
	}()
}

func (a *app) abort() {
	id := a.selSession
	if a.api == nil || id == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		a.api.abort(ctx, id)
		a.loadTurns(id)
	}()
}

// imageCache loads avatars from the CDN once each.
type imageCache struct {
	mu     sync.Mutex
	m      map[string]*ui.Bitmap
	failed map[string]bool
	busy   map[string]bool
	sem    chan struct{}
	onLoad func(func())
	client *http.Client
}

func newImageCache() *imageCache {
	return &imageCache{
		m: map[string]*ui.Bitmap{}, failed: map[string]bool{}, busy: map[string]bool{},
		sem: make(chan struct{}, 6), client: &http.Client{Timeout: 20 * time.Second},
	}
}

// get returns the bitmap of url, or nil while it loads or when it fails.
func (ic *imageCache) get(url string) *ui.Bitmap {
	if url == "" {
		return nil
	}
	ic.mu.Lock()
	defer ic.mu.Unlock()
	if b := ic.m[url]; b != nil || ic.failed[url] || ic.busy[url] {
		return b
	}
	ic.busy[url] = true
	go ic.load(url)
	return nil
}

func (ic *imageCache) load(url string) {
	ic.sem <- struct{}{}
	defer func() { <-ic.sem }()
	bmp, err := ic.fetch(url)
	set := func() {
		ic.mu.Lock()
		defer ic.mu.Unlock()
		delete(ic.busy, url)
		if err != nil {
			ic.failed[url] = true
			return
		}
		ic.m[url] = bmp
	}
	if ic.onLoad != nil {
		ic.onLoad(set)
	} else {
		set()
	}
}

func (ic *imageCache) fetch(url string) (*ui.Bitmap, error) {
	res, err := ic.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, io.ErrUnexpectedEOF
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	return ui.DecodeBitmap(b)
}

// wait blocks until every image asked for has loaded or failed (snapshots).
func (ic *imageCache) wait(d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		ic.mu.Lock()
		n := len(ic.busy)
		ic.mu.Unlock()
		if n == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
