package app

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// A display streams over WebRTC, as the web's DisplayWindow, in a web view
// whose engine decodes the video: the app does the signaling with its
// account, and the page plays the stream and sends what is pressed and
// typed over it straight to the display. Without a web view or WebRTC, as
// on Linux without WebKitGTK, or once a connection fails, the display shows
// as frames from /capture instead (displayView).

//go:embed display.html
var displayHTML string

// evaler is a page that runs scripts: *mygo.Page, or a test's.
type evaler interface {
	Eval(code string) (any, error)
}

type displayRTC struct {
	web  *mygo.WebView // nil until made
	page evaler
	// The connection shown: its display, the display's session for it,
	// and a generation that a newer connection or leaving the display
	// moves on, so a job of an older one drops its result.
	space, id string
	session   string
	gen       int
	state     string // connecting, connected
	since     time.Time
	checked   time.Time
	checking  bool
	// failed are the displays shown as frames instead, by space/id; off
	// tells that this system's web view has no WebRTC.
	failed map[string]bool
	off    bool
	making bool // the web view
}

const displayConnectTimeout = 20 * time.Second

// streams tells whether the display shows over WebRTC.
func (a *app) streams(spaceID, id string) bool {
	if a.api == nil || a.rtc == nil && a.win == nil {
		return false
	}
	if a.rtc == nil {
		a.rtc = &displayRTC{failed: map[string]bool{}, off: !webViewsAvailable()}
	}
	if a.rtc.off || a.rtc.failed[spaceID+"/"+id] {
		return false
	}
	list, _ := a.displays(spaceID)
	for _, d := range list {
		if d.ID == id {
			return d.Stream
		}
	}
	return false
}

// rtcView shows the display's stream, making the web view the first time.
func (a *app) rtcView(c *ui.Context, spaceID, id string) {
	p := a.pal
	r := a.rtc
	if r.page == nil && !r.making {
		r.making = true
		win := a.win
		go func() {
			web, err := win.NewWebView(mygo.WebViewOptions{BackgroundColor: "#000", Page: mygo.PageOptions{Untrusted: true}})
			if err == nil {
				web.Page().LoadHTML(displayHTML, "")
			}
			a.update(func() {
				if err != nil {
					r.off = true
					return
				}
				r.web, r.page = web, web.Page()
			})
		}()
	}
	if r.page == nil {
		ui.Column(c.Key("display-state")).Grow(1).Center().Background(ui.RGB(0, 0, 0)).Children(func() {
			a.spinner(c, 16, p.textTertiary)
		})
		return
	}
	if r.space != spaceID || r.id != id || r.state == "" {
		a.connectDisplay(spaceID, id)
	}
	now := a.now()
	if !r.checking && now.Sub(r.checked) >= time.Second {
		a.checkDisplay()
	}
	c.After(time.Second)
	if r.web == nil {
		return
	}
	ui.WebView(c.Key("display"), r.web).Grow(1).MinHeight(0).Label(m("display_live")).Children(func() {
		if r.state != "connected" {
			ui.Column(c).Absolute().Fill().Center().Gap(6).Background(ui.RGB(0, 0, 0)).Children(func() {
				a.spinner(c, 16, p.textTertiary)
				ui.Text(c, m("display_connecting")).FontSize(12).TextColor(p.textTertiary)
			})
		}
	})
	a.displayKeysBar(c, a.displayOf(spaceID, id))
}

// connectDisplay opens a session of the display for the page's offer.
func (a *app) connectDisplay(spaceID, id string) {
	r := a.rtc
	a.closeDisplay()
	r.gen++
	gen, page, api := r.gen, r.page, a.api
	r.space, r.id, r.state, r.since = spaceID, id, "connecting", a.now()
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), displayConnectTimeout)
		defer cancel()
		session, err := func() (string, error) {
			var ice struct {
				IceServers json.RawMessage `json:"iceServers"`
			}
			if err := api.Do(ctx, "GET", sp(spaceID)+"/rtc/ice-servers", nil, &ice); err != nil {
				return "", err
			}
			if len(ice.IceServers) == 0 {
				ice.IceServers = json.RawMessage("[]")
			}
			v, err := page.Eval(fmt.Sprintf("return cohubDisplay.offer(%s, %d)", ice.IceServers, gen))
			if err != nil {
				return "", err
			}
			offer, _ := v.(string)
			if offer == "" {
				return "", errors.New("the page made no offer")
			}
			var out struct {
				SessionID string `json:"sessionId"`
				Answer    string `json:"answer"`
			}
			path := sp(spaceID) + "/displays/" + url.PathEscape(id) + "/sessions"
			if err := api.Do(ctx, "POST", path, map[string]any{"offer": offer, "control": true}, &out); err != nil {
				return "", err
			}
			answer, _ := json.Marshal(out.Answer)
			_, err = page.Eval(fmt.Sprintf("return cohubDisplay.answer(%s, %d)", answer, gen))
			return out.SessionID, err
		}()
		a.update(func() {
			if r.gen != gen {
				if session != "" {
					a.deleteDisplaySession(spaceID, id, session)
				}
				return
			}
			r.session = session
			if err != nil {
				a.displayFailed(err)
			}
		})
	})
}

// checkDisplay reads the connection's state from the page.
func (a *app) checkDisplay() {
	r := a.rtc
	if r.state == "" || r.page == nil {
		return
	}
	r.checking, r.checked = true, a.now()
	gen, page := r.gen, r.page
	a.spawn(func() {
		var info struct {
			State  string `json:"state"`
			Reason string `json:"reason"`
		}
		v, err := page.Eval("return cohubDisplay.info()")
		if err == nil {
			b, _ := json.Marshal(v)
			err = json.Unmarshal(b, &info)
		}
		a.update(func() {
			r.checking = false
			if r.gen != gen {
				return
			}
			switch {
			case err != nil:
				a.displayFailed(err)
			case info.State == "connected":
				r.state = "connected"
			case info.State == "unsupported":
				r.off = true
				a.closeDisplay()
			case info.State == "closed":
				a.displayFailed(fmt.Errorf("display closed: %s", info.Reason))
			case r.state == "connecting" && a.now().Sub(r.since) > displayConnectTimeout:
				a.displayFailed(errors.New("display connect timeout"))
			}
		})
	})
}

// displayFailed shows the display as frames from now on.
func (a *app) displayFailed(err error) {
	r := a.rtc
	if strings.Contains(err.Error(), "unsupported") {
		r.off = true
	}
	r.failed[r.space+"/"+r.id] = true
	a.closeDisplay()
}

// closeDisplay ends the connection shown, if any.
func (a *app) closeDisplay() {
	r := a.rtc
	if r == nil || r.state == "" {
		return
	}
	r.gen++
	space, id, session, page := r.space, r.id, r.session, r.page
	r.state, r.session = "", ""
	a.spawn(func() { page.Eval("cohubDisplay.close()") })
	if session != "" {
		a.deleteDisplaySession(space, id, session)
	}
}

func (a *app) deleteDisplaySession(spaceID, id, session string) {
	api := a.api
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		api.Do(ctx, "DELETE", sp(spaceID)+"/displays/"+url.PathEscape(id)+"/sessions/"+url.PathEscape(session), nil, nil)
	})
}
