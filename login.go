package main

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"time"

	"github.com/egoist/mygo/ui"
)

// Signing in. The app shares the cohub CLI's account (its auth.json), so
// signing in is the CLI's device login: a code to confirm in the browser,
// then the CLI stores the tokens and the app starts with them.

type loginState struct {
	needed   bool
	code     string
	link     string
	busy     bool
	err      string
	noCLI    bool
	attempts int
}

func (a *app) startLogin() {
	l := &a.login
	if l.busy {
		return
	}
	if _, err := exec.LookPath("cohub"); err != nil {
		l.noCLI, l.err = true, m("x_login_no_cli")
		return
	}
	l.busy, l.err, l.code = true, "", ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		out, err := exec.CommandContext(ctx, "cohub", "auth", "login", "--request-code", "--json").Output()
		cancel()
		var req struct {
			UserCode                string `json:"userCode"`
			VerificationURIComplete string `json:"verificationUriComplete"`
		}
		if err == nil {
			err = json.Unmarshal(out, &req)
		}
		if err == nil && req.VerificationURIComplete == "" {
			err = errors.New("no verification link")
		}
		a.update(func() {
			if err != nil {
				l.busy, l.err = false, err.Error()
				return
			}
			l.code, l.link = req.UserCode, req.VerificationURIComplete
		})
		if err != nil {
			return
		}
		a.openURL(req.VerificationURIComplete)
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		_, err = exec.CommandContext(ctx, "cohub", "auth", "login", "--verify-code", "--json").Output()
		a.update(func() {
			l.busy = false
			if err != nil {
				l.err = m("x_login_failed")
				return
			}
			l.needed = false
			a.start()
		})
	}()
}

// loginView is the window before an account: the logo, sign in, or the
// demo.
func (a *app) loginView(c *ui.Context) {
	p := a.pal
	l := &a.login
	ui.Column(c.Key("login")).Fill().Center().Gap(16).Background(p.bgPrimary).DragWindow().Children(func() {
		a.logo(c, 48)
		ui.Text(c, m("app_auth_sign_in")).FontSize(22).FontWeight(600)
		ui.Text(c, m("x_login_hint")).FontSize(13).TextColor(p.textTertiary).MaxWidth(360).TextAlign(ui.Center)
		if l.code != "" {
			ui.Column(c.Key("code")).AlignItems(ui.Center).Gap(6).Padding(14, 20).Radius(10).Border(1, p.borderSubtle).Background(p.bgElevated).Children(func() {
				ui.Text(c, m("x_login_code")).FontSize(11).TextColor(p.textTertiary)
				ui.Text(c, l.code).FontSize(24).FontWeight(600).Font(fontMono).LetterSpacing(2)
				link := ui.Text(c, l.link).FontSize(11).Font(fontMono).TextColor(p.brand).Cursor(ui.CursorPointer)
				if link.Clicked() {
					c.OpenURL(l.link)
				}
			})
		}
		b := ui.ButtonBase(c.Key("sign-in")).Row().Gap(8).AlignItems(ui.Center).Height(40).Padding(0, 18).Radius(8).Background(p.brand).FocusRing(false).Children(func() {
			if l.busy {
				a.spinner(c, 14, p.brandFg)
			}
			label := m("access_sign_in")
			if l.code != "" {
				label = m("x_login_waiting")
			}
			ui.Text(c, label).FontSize(14).FontWeight(500).TextColor(p.brandFg)
		})
		if b.Clicked() && !l.busy {
			a.startLogin()
		}
		if l.err != "" {
			ui.Text(c.Key("err"), l.err).FontSize(12).TextColor(p.errorC).MaxWidth(420).TextAlign(ui.Center)
		}
		demo := ui.Text(c.Key("demo"), m("x_login_demo")).FontSize(12).TextColor(p.textTertiary).Cursor(ui.CursorPointer)
		if demo.Clicked() {
			l.needed = false
			loadDemo(a)
		}
	})
}
