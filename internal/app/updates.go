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
	a.langChanged = func() { mygo.App.SetLocale(i18n.Locale()) }
	a.langChanged()
	mygo.App.OnLocaleChanged(func(string) { setMenu() })
}

// setMenu is the application menu. Its "Check for Updates…" says it in the
// language shown when it is made: the menu is made again when that changes.
func setMenu() {
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout},
			updater.MenuItem(),
			mygo.Separator(),
			{Role: mygo.RoleServices},
			mygo.Separator(),
			{Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers},
			{Role: mygo.RoleUnhide},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleWindowMenu},
	}))
}
