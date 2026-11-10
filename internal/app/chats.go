package app

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"cohub-mygo/internal/cohub"
)

// mySessions lists the user's chats from a source ("" for every one), or
// the archived ones, as the web's chats inbox.
func mySessions(ctx context.Context, cl *cohub.Client, f chatsFilter) ([]*cohub.Session, error) {
	var page struct {
		Sessions []*cohub.Session `json:"sessions"`
	}
	q := url.Values{"limit": {"60"}}
	if f.archived {
		q.Set("archived", "only")
	} else if f.source != "" {
		q.Set("source", f.source)
	}
	return page.Sessions, cl.Do(ctx, "GET", "/api/me/sessions?"+q.Encode(), nil, &page)
}

// spaceChats lists a Space's chats from a source, through the Space's
// source label, as the inbox does once a chip is chosen.
func spaceChats(ctx context.Context, cl *cohub.Client, spaceID string, f chatsFilter) ([]*cohub.Session, error) {
	if f.source == "" || f.archived {
		path := "/api/spaces/" + url.PathEscape(spaceID) + "/sessions?limit=60"
		if f.archived {
			path += "&archived=only"
		}
		var page struct {
			Sessions []*cohub.Session `json:"sessions"`
		}
		return page.Sessions, cl.Do(ctx, "GET", path, nil, &page)
	}
	var out struct {
		Sessions []*cohub.Session `json:"sessions"`
	}
	q := url.Values{"ref": {"Source/" + sourceName(f.source)}, "limit": {"60"}}
	return out.Sessions, cl.Do(ctx, "GET", "/api/spaces/"+url.PathEscape(spaceID)+"/labels/items?"+q.Encode(), nil, &out)
}

// chatsFilter is stores/chats-filter.ts: which chats the inbox lists.
type chatsFilter struct {
	source   string // "web", "cli", …; "" for every source
	archived bool
}

func (f chatsFilter) key() string { return fmt.Sprint(f.source, f.archived) }

// chatSourceGroups and sourceName are CHATS_SOURCE_GROUPS and
// CHATS_SOURCE_NAMES.
var chatSourceGroups = []struct {
	id      string
	sources []string
}{
	{"clients", []string{"web", "cli", "websocket"}},
	{"channels", []string{"feishu", "wechat", "discord", "qq"}},
	{"automations", []string{"scheduled_task", "space_hook", "public_api"}},
	{"other", []string{"other"}},
}

func sourceName(source string) string {
	switch source {
	case "":
		return m("chats_source_all")
	case "other":
		return m("chats_source_other")
	}
	return map[string]string{
		"web": "Web App", "cli": "CLI", "websocket": "Websocket", "feishu": "Feishu", "wechat": "WeChat",
		"discord": "Discord", "qq": "QQ", "scheduled_task": "Scheduled Task", "space_hook": "Hook", "public_api": "Public API",
	}[source]
}

// chatTitle is a chat's title, or "Untitled".
func chatTitle(s *cohub.Session) string {
	if t := strings.TrimSpace(s.Title); t != "" {
		return t
	}
	return m("x_untitled")
}
