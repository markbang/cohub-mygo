// Cohub for the desktop in native UI: the web app's layout and look, drawn
// by MyGo on the GPU, with no web view. It signs in with the `cohub` CLI's
// account, as the desktop shells do, and shows demo data without one.
package app

import (
	"errors"
	"log"
	"os"
	"strings"
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

// probeWebRTC logs whether a web view of the window can take a display's
// stream, an offer that receives H.264: COHUB_PROBE_WEBRTC=1 checks the
// system's web engine, as CI does on each platform.
func probeWebRTC(w *mygo.Window) {
	web, err := w.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{Untrusted: true}})
	sdp := ""
	if err == nil {
		web.Page().LoadHTML(displayHTML, "")
		for i := 0; i < 100 && sdp == "" && err == nil; i++ {
			var v any
			v, err = web.Page().Eval("return typeof cohubDisplay === 'object' ? cohubDisplay.offer([]) : ''")
			sdp, _ = v.(string)
			if sdp == "" {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
	if err == nil && !strings.Contains(sdp, "H264/90000") {
		err = errors.New("its offer receives no H.264")
	}
	if err != nil {
		log.Printf("webrtc probe: %v", err)
		return
	}
	log.Print("webrtc probe: ok, receives H.264")
}

// Run starts the app and returns when it quits.
func Run() error {
	a := newApp()
	a.startAt = time.Now()
	a.perf = os.Getenv("COHUB_PERF") != ""
	useUpdates(a)
	mygo.App.WhenReady(func() {
		setMenu(a)
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
		if os.Getenv("COHUB_PROBE_WEBRTC") != "" {
			go probeWebRTC(a.win)
		}
	})
	// cohub:// links and cohub.live links handed to the app open their
	// page, as the web's routes.
	mygo.App.OnOpenURL(func(raw string) {
		a.update(func() {
			if a.openLink(raw) && a.win != nil {
				a.win.Focus()
			}
		})
	})
	return mygo.App.Run()
}
