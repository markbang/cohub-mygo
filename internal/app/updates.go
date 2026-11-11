package app

import (
	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/locales/zh-Hans" // MyGo's own texts, as its menus', in Chinese
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"

	"cohub-mygo/internal/i18n"
)

// The app updates itself from the signed releases that mygo.json's updates
// name, through MyGo's update window drawn in native UI: it checks a little
// after launch and once a day, offers a new version with its notes, and
// installs it. Builds that cannot update, as `go run` ones and .deb
// installs, never check by themselves.

// useUpdates sets up the update window and the languages MyGo shows, before
// the app runs.
func useUpdates(a *app) {
	mygo.Use(native.Plugin)
	a.checkUpdates = updater.CheckForUpdates
	// MyGo's own texts, the update window's and the menus', follow the
	// language chosen in the settings.
	mygo.App.SetLanguages("zh-Hans", "en")
	a.langChanged = func() { mygo.App.SetLocale(i18n.Locale()); setMenu(a) }
	a.langChanged()
	mygo.App.OnLocaleChanged(func(string) { setMenu(a) })
}

// setMenu is the application menu. Its "Check for Updates…" says it in the
// language shown when it is made: the menu is made again when that changes.
func setMenu(a *app) { mygo.App.SetMenu(appMenu(a)) }

// appMenu is the native menu bar: the app's own items, the platform's
// editing and window menus, and the views that move around the app.
func appMenu(a *app) *mygo.Menu {
	settings := func(*mygo.MenuItem, *mygo.Window) {
		a.go_(route{kind: routeSettings, sub: "general"})
	}
	newChat := func(*mygo.MenuItem, *mygo.Window) { a.newChat() }
	toggleSidebar := func(*mygo.MenuItem, *mygo.Window) { a.sidebarOpen = !a.sidebarOpen }
	app := []*mygo.MenuItem{{Role: mygo.RoleAbout}}
	if a.checkUpdates != nil {
		app = append(app, updater.MenuItem())
	}
	app = append(app,
		&mygo.MenuItem{Label: m("nav_settings"), Accelerator: "CmdOrCtrl+,", Click: settings},
		mygo.Separator(),
		&mygo.MenuItem{Role: mygo.RoleServices},
		mygo.Separator(),
		&mygo.MenuItem{Role: mygo.RoleHide},
		&mygo.MenuItem{Role: mygo.RoleHideOthers},
		&mygo.MenuItem{Role: mygo.RoleUnhide},
		mygo.Separator(),
		&mygo.MenuItem{Role: mygo.RoleQuit},
	)
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: app},
		{Label: m("x_menu_file"), Submenu: []*mygo.MenuItem{
			{Label: m("sidebar_new_chat"), Accelerator: "CmdOrCtrl+N", Click: newChat},
			mygo.Separator(),
			{Role: mygo.RoleClose},
		}},
		{Role: mygo.RoleEditMenu},
		{Label: m("x_menu_view"), Submenu: []*mygo.MenuItem{
			{Label: m("x_toggle_sidebar"), Accelerator: "CmdOrCtrl+B", Click: toggleSidebar},
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Role: mygo.RoleWindowMenu},
		{Label: m("x_menu_help"), Submenu: []*mygo.MenuItem{
			{Label: m("help_title"), Accelerator: "Shift+/", Click: func(*mygo.MenuItem, *mygo.Window) { a.helpOpen = true }},
			{Label: m("help_docs"), Click: func(*mygo.MenuItem, *mygo.Window) { a.openURL("https://cohub.live/docs") }},
			{Label: m("help_changelog"), Click: func(*mygo.MenuItem, *mygo.Window) { a.openURL("https://cohub.live/changelog") }},
		}},
	})
}
