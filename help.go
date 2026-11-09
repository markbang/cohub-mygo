package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The account menu's help, feedback and sign out, and the keys the help
// lists, as HelpPanel, FeedbackDialog and the session page's shortcuts.

type helpItem struct {
	label string
	keys  [][]string
}

func (a *app) helpSections() []struct {
	title, desc string
	items       []helpItem
} {
	mod := "Ctrl"
	if runtime.GOOS == "darwin" {
		mod = "⌘"
	}
	return []struct {
		title, desc string
		items       []helpItem
	}{
		{m("help_global"), m("help_global_desc"), []helpItem{
			{m("help_search_everywhere"), [][]string{{mod, "K"}}},
			{m("help_new_chat"), [][]string{{mod, "O"}}},
			{m("help_focus_chats"), [][]string{{mod, "⇧", "U"}}},
			{m("help_toggle_left"), [][]string{{"Ctrl", "Alt", "←"}}},
			{m("help_toggle_right"), [][]string{{"Ctrl", "Alt", "→"}}},
			{m("help_open_help"), [][]string{{"?"}}},
		}},
		{m("help_session_page"), m("help_session_desc"), []helpItem{
			{m("help_open_model_selector"), [][]string{{mod, "⇧", "M"}}},
			{m("help_focus_composer"), [][]string{{"i"}}},
			{m("help_scroll_down"), [][]string{{"j"}}},
			{m("help_scroll_up"), [][]string{{"k"}}},
			{m("help_next_turn"), [][]string{{"Shift", "J"}}},
			{m("help_prev_turn"), [][]string{{"Shift", "K"}}},
			{m("help_top"), [][]string{{"g", "g"}}},
			{m("help_bottom"), [][]string{{"G"}}},
		}},
		{m("help_model_selector"), m("help_model_desc"), []helpItem{
			{m("help_next_model"), [][]string{{"↓"}, {"Ctrl", "N"}}},
			{m("help_prev_model"), [][]string{{"↑"}, {"Ctrl", "P"}}},
			{m("help_select"), [][]string{{"Enter"}}},
			{m("help_close"), [][]string{{"Esc"}}},
		}},
		{m("help_composer"), m("help_composer_desc"), []helpItem{
			{m("help_send"), [][]string{{"Enter"}}},
			{m("help_force_send"), [][]string{{mod, "↵"}}},
			{m("help_new_line"), [][]string{{"Shift", "↵"}}},
			{m("help_blur"), [][]string{{"Esc"}}},
		}},
		{m("help_files"), m("help_files_desc"), []helpItem{
			{m("help_save_file"), [][]string{{mod, "S"}}},
		}},
	}
}

// helpView is the help panel: the shortcuts, and where to read more.
func (a *app) helpView(c *ui.Context) {
	if !a.helpOpen {
		return
	}
	p := a.pal
	a.dialog(c, &a.helpOpen, m("help_title"), 560, func() {
		ui.Scroll(c.Key("help")).MaxHeight(600).Children(func() {
			ui.Column(c).Padding(16).Gap(18).Children(func() {
				for _, s := range a.helpSections() {
					ui.Column(c.Key(s.title)).Gap(6).Children(func() {
						ui.Text(c, s.title).FontSize(13).FontWeight(600)
						ui.Text(c, s.desc).FontSize(11).TextColor(p.textTertiary)
						for _, it := range s.items {
							ui.Row(c.Key(it.label)).Gap(8).AlignItems(ui.Center).Height(26).Children(func() {
								ui.Text(c, it.label).FontSize(12).TextColor(p.textSecond).Grow(1)
								for i, combo := range it.keys {
									if i > 0 {
										ui.Text(c, "/").FontSize(11).TextColor(p.placeholder)
									}
									for _, k := range combo {
										ui.Text(c.Key(fmt.Sprint(i, k)), k).FontSize(11).Font(fontMono).TextColor(p.textSecond).
											Padding(1, 6).Radius(4).Border(1, p.borderSubtle).Background(p.bgElevated)
									}
								}
							})
						}
					})
				}
				ui.Row(c.Key("links")).Gap(8).Wrap().Children(func() {
					for _, l := range []struct{ label, url string }{
						{m("help_docs"), "https://cohub.live/docs"},
						{m("help_changelog"), "https://cohub.live/changelog"},
					} {
						l := l
						if a.button(c, "external-link", l.label, false).Clicked() {
							c.OpenURL(l.url)
						}
					}
				})
			})
		})
	})
}

type feedbackForm struct {
	open    bool
	text    string
	files   []string
	sending bool
	sent    bool
	err     string
}

// feedbackView is FeedbackDialog: what went wrong, files, Send.
func (a *app) feedbackView(c *ui.Context) {
	f := &a.feedback
	if !f.open {
		return
	}
	p := a.pal
	a.dialog(c, &f.open, m("feedback_title"), 480, func() {
		ui.Column(c.Key("feedback")).Padding(16).Gap(12).Children(func() {
			if f.sent {
				ui.Text(c, m("feedback_sent_title")).FontSize(15).FontWeight(600)
				if a.button(c, "check", m("feedback_done"), true).Clicked() {
					*f = feedbackForm{}
				}
				return
			}
			in := ui.TextAreaBase(c.Key("text"), &f.text).Lines(5, 12).Placeholder(m("feedback_placeholder")).PlaceholderColor(p.placeholder).
				FontSize(13).Padding(8, 12).Radius(6).Border(1, p.borderSubtle).Background(p.bgInput).AutoFocus().Label(m("feedback_title"))
			if files := in.DroppedFiles(); files != nil {
				f.files = append(f.files, files...)
			}
			for i, path := range f.files {
				i := i
				ui.Row(c.Key(path)).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icon("file")).Size(13, 13).TextColor(p.textTertiary)
					ui.Text(c, filepath.Base(path)).FontSize(12).Grow(1).SingleLine()
					if a.iconButton(c.Key("rm"), "x", m("feedback_remove_file", "name", filepath.Base(path)), false).Clicked() {
						f.files = append(f.files[:i:i], f.files[i+1:]...)
					}
				})
			}
			ui.Row(c.Key("foot")).Gap(8).AlignItems(ui.Center).Children(func() {
				if a.button(c, "paperclip", m("feedback_add_files"), false).Clicked() && a.win != nil {
					go func() {
						paths, _ := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Multiple: true})
						a.update(func() { f.files = append(f.files, paths...) })
					}()
				}
				ui.Text(c, m("feedback_attachment_hint")).FontSize(11).TextColor(p.textTertiary).Grow(1)
				label := m("feedback_send")
				if f.sending {
					label = m("feedback_sending")
				}
				if a.button(c, "send", label, true).Clicked() && !f.sending && strings.TrimSpace(f.text) != "" {
					a.sendFeedback(f)
				}
			})
			if f.err != "" {
				ui.Text(c.Key("err"), f.err).FontSize(12).TextColor(p.errorC)
			}
		})
	})
	if !f.open {
		*f = feedbackForm{}
	}
}

func (a *app) sendFeedback(f *feedbackForm) {
	f.sending, f.err = true, ""
	text, files := f.text, append([]string(nil), f.files...)
	extra := map[string]any{"client": "cohub-mygo", "os": runtime.GOOS, "route": a.route.String()}
	if a.me != nil {
		extra["userUuid"] = a.me.UUID
	}
	go func() {
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		w.WriteField("app", "cohub")
		w.WriteField("content", text)
		ex, _ := json.Marshal(extra)
		w.WriteField("extra", string(ex))
		for _, p := range files {
			b, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			part, _ := w.CreateFormFile("files", filepath.Base(p))
			part.Write(b)
		}
		w.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, "POST", "https://cfeed.neta.art/api/v1/feedback", &body)
		req.Header.Set("Content-Type", w.FormDataContentType())
		res, err := http.DefaultClient.Do(req)
		if err == nil {
			defer res.Body.Close()
			if res.StatusCode/100 != 2 {
				msg, _ := io.ReadAll(io.LimitReader(res.Body, 300))
				err = fmt.Errorf("%s: %s", res.Status, strings.TrimSpace(string(msg)))
			}
		}
		a.update(func() {
			f.sending = false
			if err != nil {
				f.err = m("feedback_error_send")
				return
			}
			f.sent = true
		})
	}()
}

// signOut clears the CLI's session, which the app shares, and shows the
// sign-in.
func (a *app) signOut() {
	a.confirm(m("sidebar_sign_out"), m("x_sign_out_detail"), m("sidebar_sign_out"), func() {
		go func() {
			exec.Command("cohub", "auth", "logout").Run()
			a.update(func() {
				a.api, a.me, a.spaces, a.sessions = nil, nil, nil, nil
				a.timelines = map[string]*sessionTurns{}
				a.pages.init()
				a.loading = map[string]bool{}
				a.login = loginState{needed: true}
			})
		}()
	})
}

// pageKeys are the session page's single keys, as the web's: j and k
// scroll, J and K step through turns, gg and G go to the ends, i focuses
// the composer, ? opens the help.
func (a *app) pageKeys(c *ui.Context) {
	c.OnShortcut(0, ui.KeySlash, func() {})
	c.OnWindowShortcut(ui.Shift, ui.KeySlash, func() { a.helpOpen = true })
	if a.route.sessionID() == "" {
		return
	}
	c.OnWindowShortcut(0, ui.KeyJ, func() { a.scrollTimeline(1) })
	c.OnWindowShortcut(0, ui.KeyK, func() { a.scrollTimeline(-1) })
	c.OnWindowShortcut(ui.Shift, ui.KeyJ, func() { a.stepTurn(1) })
	c.OnWindowShortcut(ui.Shift, ui.KeyK, func() { a.stepTurn(-1) })
	c.OnWindowShortcut(ui.Shift, ui.KeyG, func() { a.timeline.ScrollToEnd() })
	c.OnWindowShortcut(0, ui.KeyG, func() {
		if time.Since(a.lastG) < 600*time.Millisecond {
			a.timeline.ScrollTo(0, ui.Start)
		}
		a.lastG = time.Now()
	})
}

func (a *app) scrollTimeline(dir int) {
	first, last := a.timeline.Visible()
	if dir > 0 {
		a.timeline.ScrollIntoView(last + 1)
	} else {
		a.timeline.ScrollIntoView(max(first-1, 0))
	}
}

func (a *app) stepTurn(dir int) {
	first, _ := a.timeline.Visible()
	a.timeline.ScrollTo(max(first+dir, 0), ui.Start)
}
