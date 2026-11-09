// Cohub for the desktop in native UI: the web app's layout and look, drawn
// by MyGo on the GPU, with no web view. It signs in with the `cohub` CLI's
// account, as the desktop shells do, and shows demo data without one.
package main

import (
	"log"
	"net/url"
	"os"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// capture saves what the window shows once its data and pictures have
// come, then quits: COHUB_CAPTURE=shot.png checks a build end to end.
func capture(w *mygo.Window, path string) {
	time.Sleep(12 * time.Second)
	png, err := w.CapturePage()
	if err == nil {
		err = os.WriteFile(path, png, 0o644)
	}
	if err != nil {
		log.Print(err)
	}
	mygo.App.Quit()
}

func main() {
	a := newApp()
	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Role: mygo.RoleEditMenu},
			{Role: mygo.RoleWindowMenu},
		}))
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:         "Cohub",
			Width:         1440,
			Height:        900,
			MinWidth:      360,
			MinHeight:     480,
			StateKey:      "main",
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(a.view),
		})
		// Work on goroutines reaches the view through the window.
		if os.Getenv("COHUB_DEMO") != "" {
			loadDemo(a)
		} else {
			a.start()
		}
		if path := os.Getenv("COHUB_CAPTURE"); path != "" {
			go capture(a.win, path)
		}
	})
	// cohub:// links and cohub.live links handed to the app open their
	// page, as the web's routes.
	mygo.App.OnOpenURL(func(raw string) {
		u, err := url.Parse(raw)
		if err != nil {
			return
		}
		path := u.Path
		if u.Scheme == "cohub" && u.Host != "" {
			path = "/" + u.Host + u.Path
		}
		if u.RawQuery != "" {
			path += "?" + u.RawQuery
		}
		a.update(func() {
			a.navigate(path)
			if a.win != nil {
				a.win.Focus()
			}
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
