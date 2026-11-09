package main

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// The sandbox's ports, as the web's port windows: a port that starts
// listening says so in a toast, and opens as ?window=port:3000. The web
// shows the page inside the window; a native client without a web view
// shows the port's state and its address, and opens it in the browser.

type portEndpoint struct {
	URL        string `json:"url"`
	Status     string `json:"status"` // listening, closed, unknown
	ObservedAt int64  `json:"observedAt"`
}

type portToast struct {
	space, port, url string
	at               time.Time
}

func (r route) port() string {
	if p, ok := strings.CutPrefix(r.win, "port:"); ok {
		return p
	}
	return ""
}

func (a *app) ports(spaceID string) map[string]portEndpoint {
	res, _, _ := loadJSON[struct {
		Endpoints map[string]portEndpoint `json:"endpoints"`
	}](a, "ports:"+spaceID, sp(spaceID)+"/sandbox/ports")
	if res == nil {
		return nil
	}
	return res.Endpoints
}

// portsChanged takes space.ports.changed: the ports' new states, and a
// toast for one that starts listening.
func (a *app) portsChanged(spaceID string, payload json.RawMessage) {
	var ev struct {
		Ports []struct {
			Port       int    `json:"port"`
			Status     string `json:"status"`
			ObservedAt int64  `json:"observedAt"`
		} `json:"ports"`
	}
	if json.Unmarshal(payload, &ev) != nil {
		return
	}
	eps := a.ports(spaceID)
	for _, p := range ev.Ports {
		k := strconv.Itoa(p.Port)
		ep, ok := eps[k]
		if !ok || p.Status == "" {
			continue
		}
		ep.Status, ep.ObservedAt = p.Status, p.ObservedAt
		eps[k] = ep
		if p.Status == "listening" && ep.URL != "" && a.route.port() != k && spaceID == a.currentSpace() {
			a.portToast = &portToast{space: spaceID, port: k, url: ep.URL, at: time.Now()}
		}
	}
	if eps == nil {
		a.forget("ports:" + spaceID)
	}
}

func (a *app) openPort(spaceID, port string) {
	r := route{kind: routeSpace, space: spaceID, win: "port:" + port}
	if s := a.route.session; s != "" && a.route.space == spaceID {
		r.kind, r.session = routeSpaceSession, s
	}
	a.go_(r)
}

// portToastView is the toast at the bottom right, gone after 7 s.
func (a *app) portToastView(c *ui.Context) {
	t := a.portToast
	if t == nil {
		return
	}
	if time.Since(t.at) > 7*time.Second {
		a.portToast = nil
		return
	}
	p := a.pal
	ui.Row(c.Key("port-toast")).Attach(ui.AnchorBottomRight, ui.AnchorBottomRight).Margin(0, 20, 20, 0).Gap(10).AlignItems(ui.Center).
		Padding(10, 10, 10, 14).Radius(10).Border(1, p.borderSubtle).Background(p.bgElevated).Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, 0.12)).Children(func() {
		ui.Column(c).Gap(2).MaxWidth(280).Children(func() {
			ui.Text(c, "Port :"+t.port+" is ready").FontSize(12).FontWeight(500)
			ui.Text(c, t.url).FontSize(11).TextColor(p.textTertiary).SingleLine()
		})
		if a.button(c, "", "Open", true).Clicked() {
			a.portToast = nil
			a.openPort(t.space, t.port)
		}
		if a.button(c, "external-link", "Open externally", false).Clicked() {
			a.portToast = nil
			c.OpenURL(t.url)
		}
		if a.iconButton(c.Key("close"), "x", m("port_ready_dismiss"), false).Clicked() {
			a.portToast = nil
		}
	})
	c.After(7*time.Second - time.Since(t.at))
}

// portActions are the port window's header controls, as PortPreview's:
// its state, refresh, open in the browser, and copy and publish.
func (a *app) portActions(c *ui.Context, spaceID, port string) {
	p := a.pal
	ep := a.ports(spaceID)[port]
	label := map[string]string{"listening": m("port_listening"), "closed": m("port_closed")}[ep.Status]
	if label == "" {
		label = m("port_detecting")
	}
	dot := ui.Box(c.Key("dot")).Size(7, 7).Radius(4).Background(p.placeholder).Tooltip(label).Margin(0, 4)
	switch ep.Status {
	case "listening":
		dot.Background(p.success)
	case "closed":
		dot.Background(p.errorC)
	}
	if a.iconButton(c.Key("refresh"), "refresh-cw", m("port_refresh"), false).Clicked() {
		a.reload("ports:" + spaceID)
	}
	if ep.URL != "" && a.iconButton(c.Key("open"), "external-link", m("port_open_external"), false).Clicked() {
		c.OpenURL(ep.URL)
	}
	more := a.iconButton(c.Key("more"), "ellipsis", m("x_more"), false)
	if more.Clicked() {
		a.menus["port-more"] = !a.menus["port-more"]
	}
	open := a.menus["port-more"]
	items := []menuItem{{icon: "copy", label: m("port_copy_url"), run: func() { c.WriteClipboard(ep.URL); a.notify(m("copied")) }}}
	if a.can(spaceID, "app.manage") {
		items = append(items, menuItem{icon: "rocket", label: m("port_publish"), run: func() { a.publishPort(spaceID, port) }})
	}
	a.menuWidth(c, more, &open, 180, items)
	a.menus["port-more"] = open
}

// portView is a port's window: the page runs in the browser.
func (a *app) portView(c *ui.Context, spaceID, port string) {
	p := a.pal
	ep, ok := a.ports(spaceID)[port]
	ui.Column(c.Key("body")).Grow(1).Center().Padding(24).Background(p.bgContent).Children(func() {
		ui.Column(c).MaxWidth(360).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(40, 40).Radius(8).Border(1, p.borderSubtle).Background(p.bgSurface).Center().Children(func() {
				ui.Icon(c, icon("globe")).Size(20, 20).TextColor(p.textTertiary)
			})
			ui.Box(c).Height(6)
			switch {
			case !ok || ep.URL == "":
				ui.Text(c, m("port_no_url")).FontSize(12).TextColor(p.textTertiary)
				return
			case ep.Status == "closed":
				ui.Text(c, m("port_closed_title", "port", port)).FontSize(14).FontWeight(500)
				ui.Text(c, m("port_closed_hint")).FontSize(12).LineHeight(1.6).TextColor(p.textTertiary).TextAlign(ui.Center)
			default:
				ui.Text(c, m("port_preview_title", "port", port)).FontSize(14).FontWeight(500)
				ui.Text(c, ep.URL).FontSize(12).Font(fontMono).TextColor(p.textTertiary).Selectable()
			}
			ui.Box(c).Height(10)
			ui.Row(c).Gap(8).Children(func() {
				if ep.Status == "closed" && a.button(c, "", m("files_refresh"), false).Clicked() {
					a.reload("ports:" + spaceID)
				}
				if a.button(c, "external-link", m("port_open_external"), true).Clicked() {
					c.OpenURL(ep.URL)
				}
			})
		})
	})
}

// publishPort publishes the port as an app, as the web's Publish App.
func (a *app) publishPort(spaceID, port string) { a.openPublish(spaceID, "port", port) }

// webSurface stands for a window the web draws as a web page, an app's
// surface: a native client without a web view opens it in the browser.
func (a *app) webSurface(c *ui.Context, link string) {
	p := a.pal
	ui.Column(c.Key("surface")).Grow(1).Center().Padding(24).Background(p.bgContent).Children(func() {
		ui.Column(c).MaxWidth(360).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(40, 40).Radius(8).Border(1, p.borderSubtle).Background(p.bgSurface).Center().Children(func() {
				ui.Icon(c, icon("panels-top-left")).Size(20, 20).TextColor(p.textTertiary)
			})
			ui.Text(c, m("x_port_external")).FontSize(12).LineHeight(1.6).TextColor(p.textTertiary).TextAlign(ui.Center)
			if a.button(c, "external-link", m("port_open_external"), true).Clicked() {
				c.OpenURL(link)
			}
		})
	})
}

// portChips are the sandbox's ports above the files, as SpacePreviewPorts:
// a chip opens the port's window, its arrow the browser.
func (a *app) portChips(c *ui.Context, spaceID string) {
	eps := a.ports(spaceID)
	if len(eps) == 0 {
		return
	}
	p := a.pal
	var keys []string
	for k := range eps {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		x, _ := strconv.Atoi(keys[i])
		y, _ := strconv.Atoi(keys[j])
		return x < y
	})
	ui.Column(c.Key("ports")).Padding(8, 6).Gap(6).BorderWidth(0, 0, 1, 0).BorderColor(p.borderSubtle).Children(func() {
		ui.Text(c, strings.ToUpper(m("ports_title"))).FontSize(11).LetterSpacing(1.5).TextColor(p.textTertiary)
		ui.Row(c).Wrap().Gap(6).Children(func() {
			for _, k := range keys {
				k, ep := k, eps[k]
				on := a.route.port() == k
				fg, bd, bg, dot := p.textTertiary, p.borderSubtle, p.bgSurface, p.placeholder
				switch {
				case on:
					fg, bd, bg = p.brand, p.brand.Alpha(0.6), p.brand.Alpha(0.1)
				case ep.Status == "listening":
					fg, bd, bg, dot = p.success, p.success.Alpha(0.35), p.success.Alpha(0.1), p.success
				}
				ui.Row(c.Key(k)).Height(24).AlignItems(ui.Center).Radius(999).Border(1, bd).Background(bg).Clip().Children(func() {
					b := ui.ButtonBase(c.Key("open")).Row().Gap(6).AlignItems(ui.Center).Padding(0, 8).HeightPercent(100).FocusRing(false).Children(func() {
						ui.Box(c).Size(6, 6).Radius(3).Background(dot)
						ui.Text(c, ":"+k).FontSize(11).FontWeight(500).TextColor(fg)
					})
					if b.Clicked() {
						a.openPort(spaceID, k)
					}
					x := ui.ButtonBase(c.Key("ext")).Size(24, 24).Center().BorderWidth(0, 0, 0, 1).BorderColor(fg.Alpha(0.1)).Label(m("port_open_external")).FocusRing(false).Children(func() {
						ui.Icon(c, icon("external-link")).Size(12, 12).TextColor(fg.Alpha(0.55))
					})
					if x.Clicked() && ep.URL != "" {
						c.OpenURL(ep.URL)
					}
				})
			}
		})
	})
}
