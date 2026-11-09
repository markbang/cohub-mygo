package main

import (
	"context"
	"encoding/base64"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// The account's channels: adding one, as /settings/channels/new (Discord
// and QQ bots, Feishu apps, WeChat by QR code), and deleting one.

type channelForm struct {
	open                          bool
	provider                      string
	name, token, appID, secret    string
	brand                         string // feishu, lark
	saving                        bool
	err                           string
	wechatKey, wechatMsg, verify  string
	wechatQR                      *ui.Bitmap
	wechatExpires                 time.Time
	wechatNeedCode, wechatPolling bool
}

var channelProviders = []struct{ id, name, desc string }{
	{"discord", "Discord", "channel_new_discord_desc"},
	{"feishu", "Feishu", "channel_new_feishu_desc"},
	{"wechat", "WeChat", "channel_new_wechat_desc"},
	{"qq", "QQ", "channel_new_qq_desc"},
}

func (a *app) deleteChannel(id string) {
	a.confirm(m("channels_delete_title"), m("channels_delete_confirm"), m("common_delete"), func() {
		a.act(func(ctx context.Context) error {
			return a.api.do(ctx, "DELETE", "/api/channels/"+url.PathEscape(id), nil, nil)
		}, "channels")
	})
}

// channelDialog is the new channel's form.
func (a *app) channelDialog(c *ui.Context) {
	f := &a.channelNew
	if !f.open {
		return
	}
	p := a.pal
	a.dialog(c, &f.open, m("channel_new_header"), 480, func() {
		ui.Column(c.Key("new-channel")).Padding(16).Gap(14).Children(func() {
			if f.provider == "" {
				ui.Text(c, m("channel_new_select_hint")).FontSize(12).TextColor(p.textTertiary)
				for _, pr := range channelProviders {
					pr := pr
					card := ui.ButtonBase(c.Key(pr.id)).Row().Gap(12).AlignItems(ui.Center).Padding(12).Radius(8).Border(1, p.borderSubtle).FocusRing(false).Children(func() {
						ui.Box(c).Size(32, 32).Radius(8).Center().Background(providerColor(pr.id, p).Alpha(0.12)).Children(func() {
							a.dot(c, providerColor(pr.id, p), 10)
						})
						ui.Column(c).Grow(1).Gap(2).Children(func() {
							ui.Text(c, pr.name).FontSize(13).FontWeight(500)
							ui.Text(c, m(pr.desc)).FontSize(11).LineHeight(1.5).TextColor(p.textTertiary)
						})
						ui.Icon(c, icon("chevron-right")).Size(14, 14).TextColor(p.placeholder)
					})
					if card.Hovered() {
						card.Background(p.hover)
					}
					if card.Clicked() {
						*f = channelForm{open: true, provider: pr.id, brand: "feishu"}
					}
				}
				return
			}
			back := ui.Row(c.Key("back")).Gap(4).AlignItems(ui.Center).Cursor(ui.CursorPointer).Children(func() {
				ui.Icon(c, icon("chevron-left")).Size(13, 13).TextColor(p.textSecond)
				ui.Text(c, m("channel_new_back")).FontSize(12).TextColor(p.textSecond)
			})
			if back.Clicked() {
				*f = channelForm{open: true}
				return
			}
			label := func(t string) {
				ui.Text(c, strings.ToUpper(t)).FontSize(10).FontWeight(500).LetterSpacing(0.6).TextColor(p.textTertiary)
			}
			ui.Column(c).Gap(6).Children(func() {
				label(m("channel_new_label_channel_name"))
				a.field(c, "name", &f.name, m("channel_new_placeholder_bot_name"))
			})
			switch f.provider {
			case "discord":
				ui.Column(c).Gap(6).Children(func() {
					label(m("channel_new_label_bot_token"))
					a.field(c, "token", &f.token, "Bot token").Font(fontMono)
					ui.Text(c, m("channel_new_discord_help_2")).FontSize(11).LineHeight(1.5).TextColor(p.textTertiary)
				})
			case "feishu", "qq":
				if f.provider == "feishu" {
					ui.Row(c.Key("brand")).Gap(8).Children(func() {
						for _, b := range []string{"feishu", "lark"} {
							b := b
							on := f.brand == b
							chip := ui.ButtonBase(c.Key(b)).Height(28).Padding(0, 12).Radius(6).Center().Border(1, p.borderSubtle).FocusRing(false).Children(func() {
								ui.Text(c, map[string]string{"feishu": "飞书", "lark": "Lark"}[b]).FontSize(12)
							})
							if on {
								chip.Border(1, p.brand).Background(p.brandMuted)
							}
							if chip.Clicked() {
								f.brand = b
							}
						}
					})
				}
				ui.Column(c).Gap(6).Children(func() {
					label(m("channel_new_label_app_id"))
					a.field(c, "app-id", &f.appID, "App ID").Font(fontMono)
				})
				ui.Column(c).Gap(6).Children(func() {
					k := "channel_new_label_app_secret"
					if f.provider == "qq" {
						k = "channel_new_label_client_secret"
					}
					label(m(k))
					a.field(c, "secret", &f.secret, m(k)).Font(fontMono)
				})
			case "wechat":
				a.wechatBinding(c, f)
				return
			}
			if f.err != "" {
				ui.Text(c.Key("err"), f.err).FontSize(12).TextColor(p.errorC)
			}
			ui.Row(c.Key("foot")).Gap(8).Children(func() {
				ui.Spacer(c)
				if a.button(c, "", m("channel_new_cancel"), false).Clicked() {
					*f = channelForm{}
				}
				label := m("channel_new_save")
				if f.saving {
					label = m("channel_new_saving")
				}
				if a.button(c, "check", label, true).Clicked() && !f.saving {
					a.createChannel(f)
				}
			})
		})
	})
	if !f.open {
		*f = channelForm{}
	}
}

func (a *app) createChannel(f *channelForm) {
	creds := map[string]any{}
	switch f.provider {
	case "discord":
		creds["token"] = strings.TrimSpace(f.token)
	case "feishu":
		creds["appId"], creds["appSecret"], creds["brand"] = strings.TrimSpace(f.appID), strings.TrimSpace(f.secret), f.brand
	case "qq":
		creds["appId"], creds["clientSecret"] = strings.TrimSpace(f.appID), strings.TrimSpace(f.secret)
	}
	f.saving, f.err = true, ""
	in := map[string]any{"provider": f.provider, "name": strings.TrimSpace(f.name), "credentials": creds}
	a.act(func(ctx context.Context) error {
		err := a.api.do(ctx, "POST", "/api/channels", in, nil)
		a.update(func() {
			f.saving = false
			if err != nil {
				f.err = err.Error()
				return
			}
			*f = channelForm{}
		})
		return nil
	}, "channels")
}

// wechatBinding starts a WeChat login, shows its QR code, and waits for
// the scan, as the web's WeChat binding.
func (a *app) wechatBinding(c *ui.Context, f *channelForm) {
	p := a.pal
	ui.Text(c, m("channel_new_wechat_intro")).FontSize(12).LineHeight(1.5).TextColor(p.textTertiary)
	if f.wechatKey == "" {
		if f.err != "" {
			ui.Text(c.Key("err"), f.err).FontSize(12).TextColor(p.errorC)
		}
		label := m("channel_new_start_wechat")
		if f.saving {
			label = m("channel_new_starting_binding")
		}
		if a.button(c, "qr-code", label, true).Clicked() && !f.saving {
			f.saving, f.err = true, ""
			name := strings.TrimSpace(f.name)
			a.act(func(ctx context.Context) error {
				var out struct {
					SessionKey       string `json:"sessionKey"`
					QRDataURL        string `json:"qrDataUrl"`
					Message          string `json:"message"`
					ExpiresInSeconds int    `json:"expiresInSeconds"`
				}
				err := a.api.do(ctx, "POST", "/api/channels/wechat/login/start", map[string]any{"name": name}, &out)
				a.update(func() {
					f.saving = false
					if err != nil {
						f.err = err.Error()
						return
					}
					f.wechatKey, f.wechatMsg = out.SessionKey, out.Message
					f.wechatExpires = time.Now().Add(time.Duration(out.ExpiresInSeconds) * time.Second)
					if _, data, ok := strings.Cut(out.QRDataURL, "base64,"); ok {
						if b, err := base64.StdEncoding.DecodeString(data); err == nil {
							f.wechatQR, _ = ui.DecodeBitmap(b)
						}
					}
					a.pollWeChat(f)
				})
				return nil
			})
		}
		return
	}
	ui.Column(c.Key("qr")).AlignItems(ui.Center).Gap(8).Children(func() {
		if f.wechatQR != nil {
			ui.Image(c, f.wechatQR).Size(200, 200).Radius(8)
		}
		left := time.Until(f.wechatExpires).Round(time.Second)
		if left > 0 {
			ui.Text(c, m("channel_new_expires_in", "time", left.String())).FontSize(11).TextColor(p.textTertiary)
			c.After(time.Second)
		} else {
			ui.Text(c, m("channel_new_scan_expired")).FontSize(11).TextColor(p.errorC)
		}
		if f.wechatMsg != "" {
			ui.Text(c, f.wechatMsg).FontSize(12).TextColor(p.textSecond)
		}
	})
	if f.wechatNeedCode {
		ui.Row(c.Key("code")).Gap(8).Children(func() {
			a.field(c, "verify", &f.verify, m("channel_new_placeholder_code")).Grow(1)
			if a.button(c, "", m("channel_new_continue"), true).Clicked() {
				f.wechatNeedCode = false
				a.pollWeChat(f)
			}
		})
	}
}

func (a *app) pollWeChat(f *channelForm) {
	if f.wechatPolling || a.api == nil {
		return
	}
	f.wechatPolling = true
	key, code := f.wechatKey, strings.TrimSpace(f.verify)
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
		defer cancel()
		in := map[string]any{"sessionKey": key}
		if code != "" {
			in["verifyCode"] = code
		}
		var out struct {
			Connected      bool   `json:"connected"`
			Expired        bool   `json:"expired"`
			NeedVerifyCode bool   `json:"needVerifyCode"`
			Message        string `json:"message"`
		}
		err := a.api.do(ctx, "POST", "/api/channels/wechat/login/wait", in, &out)
		a.update(func() {
			f.wechatPolling = false
			if f.wechatKey != key {
				return
			}
			switch {
			case err != nil:
				f.err = err.Error()
			case out.Connected:
				*f = channelForm{}
				a.forget("channels")
			case out.Expired:
				f.wechatMsg = m("channel_new_scan_expired")
			case out.NeedVerifyCode:
				f.wechatNeedCode, f.wechatMsg = true, m("channel_new_verify_intro")
			default:
				if out.Message != "" {
					f.wechatMsg = out.Message
				}
				a.pollWeChat(f)
			}
		})
	})
}
