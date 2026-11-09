package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// sessionTurns is what the timeline of one session shows.
type sessionTurns struct {
	turns   []*Turn
	hasMore bool
	loaded  bool
	err     string
	// live is the reply of the running turn so far, from the stream snapshot.
	live      string
	liveTurn  string
	liveSteps []Block
	liveDone  []*StepMessage // the running turn's messages done so far
	patch     *patchState    // the running turn's message, as patches build it
	// steps are the loaded intermediate messages of turns, by turn ID.
	steps    map[string][]*StepMessage
	expanded map[string]bool
}

// app is the whole state of the window: data from the API and where the
// user is. Only the main thread touches it; work on goroutines hands its
// results over with update.
type app struct {
	win *mygo.Window
	api *client
	rt  *realtime
	now func() time.Time
	// sync runs loads when the test flushes them instead of on goroutines.
	sync    bool
	pending []func()

	status string // connection state, "" when fine
	me     *Me

	spaces        []*Space
	sessions      []*Session // across spaces, most recent first
	spaceSessions map[string][]*Session
	timelines     map[string]*sessionTurns
	usage7d       map[string]*SpaceUsage
	labels        map[string][]*Label
	labelItems    map[string][]LabelItem
	models        []*Model
	loading       map[string]bool
	seen          map[string]time.Time
	seenPath      string
	images        *imageCache
	pages         pageData

	route                        route
	back                         []route
	chip                         string // the chats sidebar's Space filter, "" for all
	spacesNav                    int
	chats                        chatsFilter           // the inbox's source, "web" at first
	chipSpaces                   []string              // the chips, fixed once shown
	chipChats                    map[string][]*Session // a chip's chats by chip and filter
	panelTab                     int
	theme                        string // a built-in theme's name, or "system"
	prefs                        prefs
	sidebarOpen                  bool
	attachments                  []*attachment // the composer's files, before they are sent
	voice                        *voiceInput
	toast                        string // the last failed action, shown for a while
	toastAt                      time.Time
	toastOK                      bool
	renaming                     string // the chat whose title is being edited
	renameText                   string
	shareOpen                    bool
	copiedAt                     time.Time
	labelTarget                  *labelTarget
	statsFor                     string               // the chat whose statistics are open
	editors                      map[string]*fileEdit // open files by space:path
	fsPending                    *fsCreate
	uploading                    int
	cron                         *cronForm
	cpTab                        map[string]int // a save's tab: changes, files
	copiedValue                  string
	forking                      string // the turn a fork is being made from
	spaceEdits                   map[string]*spaceEdit
	editingProfile, profileDraft string
	channelNew                   channelForm
	walletForms                  map[string]*walletForm
	confirmBox                   *confirmBox
	windowWide                   bool                  // the window takes the main column's room
	viewportOff                  map[string]bool       // windows the user took out of the prompt's context
	looseSessions                map[string]*Session   // chats opened that no list holds
	notices                      []*turnNotice         // finished turns the user did not see
	rowRenaming                  string                // the chat row whose title is being edited
	boardViews                   map[string]*boardView // each board's pan and zoom
	login                        loginState
	helpOpen                     bool
	commerce                     *commerceForm
	harnesses                    map[string]harnessState // the composer's harness by chat or Space
	newLabelFor, newLabelName    string
	compMenu                     composerMenu
	openWith                     *openWithState
	fileHandlers                 *fileHandlerCache
	handledCommands              map[string]bool
	displayTyped                 string
	displayState                 *displayView
	portToast                    *portToast
	publish                      *publishForm
	danmakuWidth                 float32
	danmakuFrame                 time.Time
	danmaku                      danmakuState
	forks                        map[string][]sessionFork
	runtimeDiag                  bool
	height                       float32
	draftCaret                   int
	inviteBusy                   bool
	inviteErr                    string
	selectedSpaces               map[string]bool
	feedback                     feedbackForm
	lastG                        time.Time
	inHeader                     bool    // building a header: compact icon buttons grow to 40px
	width                        float32 // the window's width, as media queries see it
	purchase                     purchaseState
	redeem                       redeemState
	subsPage                     int
	mediaFilter                  string // the media panel's type: all, image, video, audio
	compactPanel                 bool   // the side panel, over the chat in the compact shell
	panelOpen                    bool
	userMenu                     bool
	palette                      paletteState
	draft                        string
	sending                      bool
	sendErr                      string
	groupsOpen                   map[string]bool
	compactNav                   bool // the compact shell's drawer
	newSpace                     newSpaceForm
	menus                        map[string]bool
	foldOpen                     map[string]bool
	modelChoices                 map[string]modelChoice

	sessionList ui.ListState
	spaceList   ui.ListState
	timeline    ui.ListState
	md          markdown.Options
	mdTheme     *markdown.Theme
	mdThemeKey  string
	pal         *palette
	palKey      string
	customCSS   string // the Home Space's .cohub/theme.css
	homeSpace   string
	compact     bool
}

func newApp() *app {
	a := &app{
		now:            time.Now,
		spaceSessions:  map[string][]*Session{},
		forks:          map[string][]sessionFork{},
		timelines:      map[string]*sessionTurns{},
		usage7d:        map[string]*SpaceUsage{},
		labels:         map[string][]*Label{},
		labelItems:     map[string][]LabelItem{},
		loading:        map[string]bool{},
		seen:           map[string]time.Time{},
		groupsOpen:     map[string]bool{"labels": true, "chats": true, "all": true, "saves": true},
		menus:          map[string]bool{},
		foldOpen:       map[string]bool{},
		modelChoices:   map[string]modelChoice{},
		images:         newImageCache(),
		sidebarOpen:    true,
		chats:          chatsFilter{source: "web"},
		mediaFilter:    "all",
		chipChats:      map[string][]*Session{},
		editors:        map[string]*fileEdit{},
		viewportOff:    map[string]bool{},
		looseSessions:  map[string]*Session{},
		cpTab:          map[string]int{},
		boardViews:     map[string]*boardView{},
		harnesses:      map[string]harnessState{},
		selectedSpaces: map[string]bool{},
		draftCaret:     -1,
		spaceEdits:     map[string]*spaceEdit{},
		walletForms:    map[string]*walletForm{},
		panelOpen:      true,
		pal:            lightPalette(),
		route:          route{kind: -1},
		theme:          "system",
	}
	a.pages.init()
	a.md.Reveal = &markdown.Reveal{Unit: markdown.ByWord}
	a.md.OnLink = func(u string) { a.openLink(u) }
	a.md.Block = a.mdBlock
	a.images.onLoad = func(fn func()) { a.update(fn) }
	if dir, err := os.UserConfigDir(); err == nil {
		a.seenPath = filepath.Join(dir, "cohub-mygo", "seen.json")
		if b, err := os.ReadFile(a.seenPath); err == nil {
			json.Unmarshal(b, &a.seen)
		}
		a.prefs.path = filepath.Join(dir, "cohub-mygo", "prefs.json")
		a.prefs.load()
		if a.prefs.Theme != "" {
			a.theme = a.prefs.Theme
		}
		setLocale(a.prefs.Lang)
		if b, err := os.ReadFile(filepath.Join(dir, "cohub-mygo", "theme.css")); err == nil {
			a.customCSS = string(b)
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

// spawn runs slow work off the main thread, or queues it for flush in tests.
func (a *app) spawn(fn func()) {
	if a.sync {
		a.pending = append(a.pending, fn)
		return
	}
	go fn()
}

// flush runs the queued work, and the work it queues, in tests.
func (a *app) flush() bool {
	ran := false
	for i := 0; i < 8 && len(a.pending) > 0; i++ {
		work := a.pending
		a.pending = nil
		for _, fn := range work {
			fn()
		}
		ran = true
	}
	return ran
}

// once starts a load keyed by key unless one ran: views ask for what they
// show, and each piece loads once until reload drops its key.
func (a *app) once(key string, fn func(ctx context.Context)) {
	if a.api == nil || a.loading[key] {
		return
	}
	a.loading[key] = true
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		fn(ctx)
	})
}

func (a *app) reload(key string) { delete(a.loading, key) }

// start connects to the API and keeps the data fresh.
func (a *app) start() {
	cl, err := newClient()
	if err != nil {
		if errors.Is(err, errNoAccount) {
			// No account yet: sign in through the CLI's device login.
			a.login.needed = true
			return
		}
		a.status = err.Error()
		loadDemo(a)
		return
	}
	a.api = cl
	a.status = m("sync_connecting")
	a.spawn(a.refresh)
	a.spawn(a.loadCustomTheme)
	if !a.sync {
		go a.poll()
		a.startRealtime()
	}
}

func (a *app) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var (
		wg       sync.WaitGroup
		me       *Me
		spaces   []*Space
		sessions []*Session
		models   []*Model
		errs     [3]error
	)
	wg.Add(4)
	go func() { defer wg.Done(); me, errs[0] = a.api.me(ctx) }()
	go func() { defer wg.Done(); spaces, errs[1] = a.api.spaces(ctx) }()
	filter := a.chats
	go func() { defer wg.Done(); sessions, errs[2] = a.api.mySessions(ctx, filter) }()
	go func() { defer wg.Done(); models, _ = a.api.models(ctx) }()
	wg.Wait()
	a.update(func() {
		for _, err := range errs {
			if err != nil {
				a.status = m("x_offline") + err.Error()
				return
			}
		}
		a.status = ""
		a.me = me
		sort.SliceStable(spaces, func(i, j int) bool { return spaces[i].activity().After(spaces[j].activity()) })
		a.spaces = spaces
		sort.SliceStable(sessions, func(i, j int) bool { return sessions[i].when().After(sessions[j].when()) })
		if filter == a.chats {
			a.sessions = sessions
		}
		if models != nil {
			a.models = models
		}
		if a.rt != nil {
			a.rt.watch(a.currentSpace())
		}
		if a.route.kind == routeSessions && len(sessions) > 0 && !a.compact {
			a.go_(route{kind: routeSession, session: sessions[0].ID})
		}
	})
}

// poll refreshes the lists every 16 seconds and the open session every
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
			sessionID, spaceID = a.route.sessionID(), a.route.space
			if s := a.session(sessionID); s != nil {
				running = s.running()
			}
			running = running || lastRunning(a.timelines[sessionID])
			close(done)
		})
		<-done
		live := a.rt.live()
		if sessionID != "" && running && (!live || i%3 == 0) {
			a.loadTurns(sessionID)
			if i%3 == 0 {
				a.update(func() { a.forget("approvals:" + sessionID) })
			}
		}
		if i%8 == 0 && (!live || i%16 == 0) {
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
		tl := a.timelineOf(sessionID)
		tl.loaded = true
		if err != nil {
			tl.err = err.Error()
			return
		}
		tl.err = ""
		tl.turns, tl.hasMore = page.Turns, page.HasMore
		switch {
		case snap == nil || snap.Snapshot == nil || snap.Snapshot.Current == nil:
			tl.live, tl.liveSteps, tl.patch, tl.liveDone = "", nil, nil, nil
		case tl.patch != nil && tl.patch.turnID == snap.Snapshot.TurnID && snap.Snapshot.Seq < tl.patch.seq:
			// Patches got further than the snapshot, which was read
			// before them: keep what they built.
		default:
			ps := &patchState{turnID: snap.Snapshot.TurnID, seq: snap.Snapshot.Seq, blocks: snap.Snapshot.Current.Content}
			if ap := snap.Snapshot.Current.AppendPath; ap != nil {
				ps.appendPath = *ap
			}
			tl.patch = ps
			tl.liveDone = nil
			for i, im := range snap.Snapshot.IntermediateMessages {
				if len(im.Content) > 0 {
					tl.liveDone = append(tl.liveDone, &StepMessage{ID: firstNonEmpty(im.MessageID, fmt.Sprint("live", i)), Sequence: i, Content: im.Content})
				}
			}
			tl.liveSteps = ps.content()
			tl.live = joinText(tl.liveSteps)
			tl.liveTurn = snap.Snapshot.TurnID
		}
		if s := a.session(sessionID); s != nil && len(page.Turns) > 0 {
			if !page.Turns[len(page.Turns)-1].running() {
				s.ActiveTurn = nil
			}
		}
	})
}

func (a *app) timelineOf(id string) *sessionTurns {
	tl := a.timelines[id]
	if tl == nil {
		tl = &sessionTurns{steps: map[string][]*StepMessage{}, expanded: map[string]bool{}}
		a.timelines[id] = tl
	}
	return tl
}

func (a *app) loadSpaceSessions(spaceID string) {
	if a.api == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	list, forks, err := a.api.spaceSessions(ctx, spaceID)
	if err != nil {
		return
	}
	a.update(func() {
		sort.SliceStable(list, func(i, j int) bool { return list[i].when().After(list[j].when()) })
		a.spaceSessions[spaceID] = list
		a.forks[spaceID] = forks
	})
}

// navigate goes to a path of the web app's scheme.
func (a *app) navigate(path string) { a.go_(parseRoute(path)) }

// go_ shows r, remembering where the window was for goBack.
func (a *app) go_(r route) {
	if r == a.route {
		return
	}
	a.back = append(a.back, a.route)
	if len(a.back) > 50 {
		a.back = a.back[1:]
	}
	a.show(r)
}

func (a *app) goBack() {
	if len(a.back) == 0 {
		a.show(route{kind: routeSessions})
		return
	}
	r := a.back[len(a.back)-1]
	a.back = a.back[:len(a.back)-1]
	a.show(r)
}

func (a *app) show(r route) {
	prevSession := a.route.sessionID()
	a.route = r
	a.sendErr = ""
	a.userMenu = false
	if r.kind == routeSessions && !a.compact && len(a.sessions) > 0 {
		// /sessions opens the latest chat, as the web does.
		a.route = route{kind: routeSession, session: a.sessions[0].ID}
	}
	if r.kind == routeNewSession {
		a.palette.open(paletteSpaces)
	}
	if r.kind == routeSlug {
		a.resolveSlug(r.sub)
	}
	if id := a.route.sessionID(); id != "" && id != prevSession {
		a.timeline = ui.ListState{}
		if s := a.session(id); s != nil {
			a.markSeen(s)
		}
		a.timelineOf(id)
		a.fetchSession(id)
		if a.api != nil {
			a.spawn(func() { a.loadTurns(id) })
		}
	}
	switch a.route.kind {
	case routeSpaceCron:
		// The side panel shows the scheduled prompts beside one, the
		// apps beside an app.
		a.panelTab = 4
	case routeSpaceApp:
		a.panelTab = 3
	}
	if a.rt != nil {
		a.rt.watch(a.currentSpace())
	}
	if sp := a.route.space; sp != "" && a.route.area() == areaSpace {
		a.once("space-sessions:"+sp, func(context.Context) { a.loadSpaceSessions(sp) })
	}
}

// openSession shows a chat where the window is: in the Space's sidebar
// inside a Space, among all chats elsewhere.
func (a *app) openSession(s *Session) {
	if s == nil {
		return
	}
	if a.route.area() == areaSpace {
		r := route{kind: routeSpaceSession, space: s.SpaceID, session: s.ID}
		if a.route.space == s.SpaceID {
			// The window stays as the chat beside it changes.
			r.win = a.route.win
		}
		a.go_(r)
		return
	}
	a.go_(route{kind: routeSession, session: s.ID})
}

func (a *app) openSpace(id string) { a.go_(route{kind: routeSpace, space: id}) }

func (a *app) markSeen(s *Session) {
	a.seen[s.ID] = s.when()
	if a.seenPath == "" {
		return
	}
	b, _ := json.Marshal(a.seen)
	path := a.seenPath
	go func() {
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, b, 0o644)
	}()
}

func (a *app) unread(s *Session) bool {
	if s.ID == a.route.sessionID() || s.running() {
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
	for _, list := range a.chipChats {
		for _, s := range list {
			if s.ID == id {
				return s
			}
		}
	}
	if s := a.looseSessions[id]; s != nil {
		return s
	}
	return nil
}

// fetchSession loads a chat the lists do not hold, as one opened by a
// link, so that its Space and title are known.
func (a *app) fetchSession(id string) {
	if a.api == nil || id == "" || a.session(id) != nil {
		return
	}
	a.once("session-detail:"+id, func(ctx context.Context) {
		var out struct {
			Session *Session `json:"session"`
		}
		if a.api.do(ctx, "GET", "/api/sessions/"+url.PathEscape(id), nil, &out) != nil || out.Session == nil {
			return
		}
		a.update(func() {
			a.looseSessions[id] = out.Session
			if a.rt != nil {
				a.rt.watch(out.Session.SpaceID)
			}
		})
	})
}

func (a *app) space(id string) *Space {
	for _, s := range a.spaces {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// spaceName returns the name and picture of a Space, from the list or
// from a session's summary of it.
func (a *app) spaceName(id string) (name, avatar string) {
	if sp := a.space(id); sp != nil {
		return sp.Name, sp.avatar()
	}
	for _, s := range a.sessions {
		if s.SpaceID == id && s.Space != nil {
			return s.Space.Name, s.Space.avatarURL()
		}
	}
	return "", ""
}

// currentSpace is the Space of the route, or of the chat it shows.
func (a *app) currentSpace() string {
	if a.route.space != "" {
		return a.route.space
	}
	if s := a.session(a.route.sessionID()); s != nil {
		return s.SpaceID
	}
	return ""
}

// chatSpaces are the chips above the chats: Spaces with recent chats, in
// the order of their latest.
// chatSpaces are the chips: the six Spaces used last, the chosen one
// first when it is not among them (chats-inbox spaceChips).
func (a *app) chatSpaces() []*Space {
	if a.chipSpaces == nil || len(a.chipSpaces) == 0 {
		recent := a.filteredSpaces(0, "")
		a.chipSpaces = nil
		for _, sp := range recent {
			if len(a.chipSpaces) == 6 {
				break
			}
			a.chipSpaces = append(a.chipSpaces, sp.ID)
		}
	}
	var out []*Space
	if a.chip != "" && !contains(a.chipSpaces, a.chip) {
		if sp := a.space(a.chip); sp != nil {
			out = append(out, sp)
		}
	}
	for _, id := range a.chipSpaces {
		if sp := a.space(id); sp != nil {
			out = append(out, sp)
		}
	}
	return out
}

// visibleSessions are the inbox's chats: the user's, or the chosen Space's.
func (a *app) visibleSessions() []*Session {
	if a.chip == "" {
		return a.sessions
	}
	key := "chip:" + a.chip + ":" + a.chats.key()
	if list, ok := a.chipChats[key]; ok {
		return list
	}
	id, filter := a.chip, a.chats
	a.once(key, func(ctx context.Context) {
		list, err := a.api.spaceChats(ctx, id, filter)
		if err != nil {
			return
		}
		sort.SliceStable(list, func(i, j int) bool { return list[i].when().After(list[j].when()) })
		a.update(func() { a.chipChats[key] = list })
	})
	var out []*Session
	for _, s := range a.sessions {
		if s.SpaceID == a.chip {
			out = append(out, s)
		}
	}
	return out
}

// setChatsFilter lists another source's chats, or the archived ones.
func (a *app) setChatsFilter(f chatsFilter) {
	if f == a.chats {
		return
	}
	a.chats = f
	a.sessionList = ui.ListState{}
	if a.api != nil {
		a.spawn(a.refresh)
	}
}

func (a *app) filteredSpaces(nav int, query string) []*Space {
	var out []*Space
	q := strings.ToLower(strings.TrimSpace(query))
	for _, s := range a.spaces {
		switch nav {
		case 0, 1:
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
		if q != "" && !strings.Contains(strings.ToLower(s.Name+" "+s.Description), q) {
			continue
		}
		out = append(out, s)
	}
	// The web sorts "recent" by the user's own activity and every other
	// filter by when they joined the Space (spaces-filter.ts).
	at := func(s *Space) time.Time { return s.JoinedAt }
	if nav == 0 {
		at = (*Space).activity
	}
	sort.SliceStable(out, func(i, j int) bool {
		ti, tj := at(out[i]), at(out[j])
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return out[i].ID > out[j].ID
	})
	return out
}

// send sends the composer's text to the open chat, or starts one in the
// route's Space.
func (a *app) send() {
	text := strings.TrimSpace(a.draft)
	if (text == "" && len(a.attachments) == 0) || a.sending {
		return
	}
	for _, at := range a.attachments {
		if at.status == "uploading" {
			return
		}
	}
	sessionID := a.route.sessionID()
	spaceID := a.currentSpace()
	if spaceID == "" {
		a.sendErr = m("x_choose_space")
		return
	}
	if a.api == nil {
		a.sendErr = m("x_demo_no_send")
		return
	}
	a.sending, a.sendErr = true, ""
	a.draft = ""
	ats := a.attachments
	for _, at := range ats {
		at.status, at.progress = "uploading", 0
	}
	model := a.composerModel()
	if h := a.harnessChoice(); h.harness != "cohub" {
		model = modelChoice{provider: h.provider, id: h.model, harness: h.harness, approval: h.approval}
	}
	viewport := a.viewport()
	inSpace := a.route.area() == areaSpace
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		urls := map[string]string{}
		var err error
		for _, at := range ats {
			at := at
			var u string
			u, err = a.api.uploadAttachment(ctx, spaceID, sessionID, at, func(f float64) {
				a.update(func() { at.progress = f })
			})
			if err != nil {
				break
			}
			urls[at.id] = u
		}
		var sandbox []string
		if err == nil && len(ats) > 0 {
			// The sandbox copy is extra: the public URLs carry the files
			// without it.
			sandbox, _ = a.api.materialize(ctx, spaceID, sessionID, ats, urls)
		}
		var id string
		if err == nil {
			content := promptContent(text, ats, urls, sandbox)
			if viewport != nil {
				content = append(content, viewport)
			}
			id, err = a.api.prompt(ctx, spaceID, sessionID, content, model)
		}
		a.update(func() {
			a.sending = false
			if err != nil {
				a.sendErr = err.Error()
				a.draft = text
				for _, at := range ats {
					at.status = "failed"
				}
				return
			}
			a.attachments = nil
			if id != "" && sessionID == "" {
				if inSpace {
					a.go_(route{kind: routeSpaceSession, space: spaceID, session: id})
				} else {
					a.go_(route{kind: routeSession, session: id})
				}
			}
		})
		if id != "" {
			a.loadTurns(id)
		}
		a.refresh()
	})
}

func (a *app) abort() {
	id := a.route.sessionID()
	if a.api == nil || id == "" {
		return
	}
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		a.api.abort(ctx, id)
		a.loadTurns(id)
	})
}

// imageCache loads pictures from the CDN once each.
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
	if isWebP(b) {
		return decodeWebP(b)
	}
	return ui.DecodeBitmap(b)
}

// wait blocks until every image asked for has loaded or failed (tests).
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

func lastRunning(tl *sessionTurns) bool {
	return tl != nil && len(tl.turns) > 0 && tl.turns[len(tl.turns)-1].running()
}

// loadCustomTheme reads the Home Space's .cohub/theme.css, the custom theme
// the web lays over every page, and keeps a copy for the next start.
func (a *app) loadCustomTheme() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var home Space
	if err := a.api.get(ctx, "/api/me/spaces/by-slug/home", &home); err != nil || home.ID == "" {
		return
	}
	var f fsFile
	css := ""
	if err := a.api.get(ctx, sp(home.ID)+"/fs/file?path=.cohub/theme.css", &f); err == nil && f.Kind == "text" {
		css = f.Content
	}
	a.update(func() {
		a.homeSpace = home.ID
		a.customCSS = css
	})
	if dir, err := os.UserConfigDir(); err == nil && !a.sync {
		path := filepath.Join(dir, "cohub-mygo", "theme.css")
		if css == "" {
			os.Remove(path)
		} else {
			os.MkdirAll(filepath.Dir(path), 0o755)
			os.WriteFile(path, []byte(css), 0o644)
		}
	}
}

// setSpacePinned and setSpaceArchived are space-pins.svelte.ts: flip the
// flags at once, then write the system labels, putting them back if the
// write fails. Pinning unarchives and archiving unpins.
func (a *app) setSpacePinned(id string, pinned bool) {
	patch := map[string]any{"removeLabelRefs": []string{"Pinned"}}
	if pinned {
		patch = map[string]any{"addLabelRefs": []string{"Pinned"}, "removeLabelRefs": []string{"Archived"}}
	}
	a.setSpaceFlags(id, func(s *Space) {
		s.IsPinned = pinned
		if pinned {
			s.IsArchived = false
		}
	}, patch)
}

func (a *app) setSpaceArchived(id string, archived bool) {
	patch := map[string]any{"removeLabelRefs": []string{"Archived"}}
	if archived {
		patch = map[string]any{"addLabelRefs": []string{"Archived"}, "removeLabelRefs": []string{"Pinned"}}
	}
	a.setSpaceFlags(id, func(s *Space) {
		s.IsArchived = archived
		if archived {
			s.IsPinned = false
		}
	}, patch)
}

func (a *app) setSpaceFlags(id string, flip func(*Space), patch map[string]any) {
	sp := a.space(id)
	if sp == nil {
		return
	}
	pinned, archived := sp.IsPinned, sp.IsArchived
	flip(sp)
	if a.api == nil {
		return
	}
	patch["resourceRefs"] = []string{id}
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.api.do(ctx, "PATCH", "/api/me/resources/space/labels/batch", patch, nil); err != nil {
			a.update(func() {
				sp.IsPinned, sp.IsArchived = pinned, archived
				a.status = m("x_error") + ": " + err.Error()
			})
		}
	})
}

// resolveSlug opens a Space by its URL, /username/slug, accepting an
// invitation first when the link carries one.
func (a *app) resolveSlug(sub string) {
	if strings.Contains(sub, "/w/") {
		// A published web app runs in the browser, beside its Space.
		a.openURL("https://cohub.live/" + sub)
		sub, _, _ = strings.Cut(sub, "/w/")
	}
	a.act(func(ctx context.Context) error {
		user, slug, _ := strings.Cut(sub, "/")
		var sp Space
		if err := a.api.do(ctx, "GET", "/api/spaces/by-slug/"+url.PathEscape(user)+"/"+url.PathEscape(slug), nil, &sp); err != nil {
			return err
		}
		a.update(func() {
			a.route = route{kind: routeSpace, space: sp.ID}
			a.show(a.route)
		})
		return nil
	})
}

// appPath is where a link goes in the app: cohub://spaces/<id> (a
// mention), a cohub.live address or a path of the web's, or false for
// a link that leaves for the browser.
func appPath(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	path := u.Path
	switch {
	case u.Scheme == "cohub" && u.Host != "":
		path = "/" + u.Host + u.Path
	case u.Scheme == "" && u.Host == "" && strings.HasPrefix(raw, "/"):
	case (u.Scheme == "https" || u.Scheme == "http") && (u.Host == "cohub.live" || u.Host == "www.cohub.live"):
		if path == "" || path == "/" || strings.HasPrefix(path, "/docs") || strings.HasPrefix(path, "/zh") ||
			strings.HasPrefix(path, "/pricing") || strings.HasPrefix(path, "/changelog") || strings.HasPrefix(path, "/trending") {
			return "", false
		}
	default:
		return "", false
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return path, true
}

// openLink opens a link: in the app where it has the page, else in the
// browser. It reports whether the app took it.
func (a *app) openLink(raw string) bool {
	if path, ok := appPath(raw); ok {
		a.navigate(path)
		return true
	}
	a.openURL(raw)
	return false
}
