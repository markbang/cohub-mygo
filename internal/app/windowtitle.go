package app

// windowTitle is what the window calls itself: the chat it shows, as a
// native document window names its document, or the app alone on the
// lists.
func (a *app) windowTitle() string {
	if id := a.route.sessionID(); id != "" {
		if s := a.session(id); s != nil {
			return chatTitle(s) + " — Cohub"
		}
	}
	if a.route.space != "" {
		if name, _ := a.spaceName(a.route.space); name != "" {
			return name + " — Cohub"
		}
	}
	return "Cohub"
}

// updateWindowTitle keeps the title the taskbar, Dock and window switcher
// read in step with the chat shown.
func (a *app) updateWindowTitle() {
	if a.win == nil {
		return
	}
	if title := a.windowTitle(); title != a.win.Title() {
		a.win.SetTitle(title)
	}
}
