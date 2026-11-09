package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// The web app's Paraglide messages (apps/web/messages), so the copy is
// the web's word for word, in both its languages.
//
//go:embed i18n/*.json
var messageFiles embed.FS

var messages = map[string]map[string]string{}

// locale is the language the window shows: "zh-CN" or "en".
var locale = "zh-CN"

// extraMessages are the app's words the web's catalogs lack, as
// zh-CN and en.
var extraMessages = map[string][2]string{
	"x_ready":          {"就绪", "Ready"},
	"x_created":        {"创建", "Created"},
	"x_mine":           {"我的", "Mine"},
	"x_no_spaces":      {"没有 Space", "No Spaces"},
	"x_no_results":     {"没有结果", "No results"},
	"x_no_chats":       {"还没有对话", "No chats yet"},
	"x_choose_space":   {"先选择一个 Space", "Choose a Space first"},
	"x_demo_no_send":   {"演示模式不能发送", "The demo cannot send"},
	"x_voice":          {"语音输入", "Voice input"},
	"x_side_panel":     {"侧边面板", "Side panel"},
	"x_menu":           {"菜单", "Menu"},
	"x_filter":         {"筛选", "Filter"},
	"x_new_window":     {"新窗口", "New window"},
	"x_error":          {"出错了", "Something went wrong"},
	"x_loading_chats":  {"加载对话…", "Loading chats…"},
	"x_toggle_sidebar": {"切换侧栏", "Toggle sidebar"},
	"x_message":        {"消息", "Message"},
	"x_untitled":       {"新对话", "New chat"},
	"x_provider":       {"提供方", "Provider"},
	"x_auto_destroy":   {"自动销毁", "Auto destroy"},
	"x_spec":           {"规格", "Spec"},
	"x_search_k":       {"搜索 (⌘K)", "Search (⌘K)"},
	"x_new_chat_k":     {"新建对话 (⌘O)", "New chat (⌘O)"},
	"x_running_dot":    {"运行中 · ", "Working · "},
	"x_offline":        {"离线：", "Offline: "},
	"x_type_to_filter": {" · 输入以筛选", " · type to filter"},
	"x_palette_keys":   {"↑↓ C-n/p 导航  ↵ 打开  esc 关闭", "↑↓ C-n/p navigate  ↵ open  esc close"},
	"x_apps":           {"应用", "Apps"},
	"x_account":        {"账户", "Account"},
	"x_to_latest":      {"回到最新", "Jump to latest"},
	"x_not_signed_in":  {"未登录", "Signed out"},
	"x_schedules":      {"定时任务", "Scheduled"},
	"x_no_schedules":   {"暂无定时任务", "No scheduled prompts"},
	"x_files":          {"文件", "Files"},
	"x_back":           {"返回", "Back"},
	"x_select_file":    {"选择一个文件", "Choose a file"},
	"x_media_empty":    {"这个 Space 还没有生成的媒体。", "This Space has no generated media yet."},
	"x_media_browse":   {"在 Task Browser 中浏览全部生成", "Browse all generations in Task Browser"},
	"x_apps_empty":     {"这个 Space 还没有发布的应用。", "This Space has no published apps yet."},
	"x_panel_session":  {"本对话中 Agent 生成的媒体和改动的文件会显示在这里。", "Media the agent makes and files it changes in this chat show here."},
	"x_panel_new":      {"开始对话后，这里会显示它生成的媒体和改动的文件。", "Once a chat starts, the media it makes and files it changes show here."},
	"x_new_chat":       {"新建对话", "New chat"},
	"x_select_or_new":  {"选择一个对话，或新建一个", "Choose a chat, or start one"},
	"x_no_messages":    {"还没有消息", "No messages yet"},
	"x_working":        {"运行中", "Working"},
	"x_done":           {"完成", "Done"},
	"x_failed":         {"失败", "Failed"},
	"x_steps":          {"{count} 步", "{count} steps"},
	"x_tools":          {"{count} 个工具", "{count} tools"},
	"x_failed_count":   {"{count} 个失败", "{count} failed"},
	"x_space_usage":    {"最近 7 天，此 Space 使用了 {tokens} 个 Token、{requests} 次 LLM 请求，合计 {cost}。", "In the last 7 days this Space used {tokens} tokens over {requests} LLM requests, {cost} in all."},
}

func init() {
	for _, l := range []string{"zh-CN", "en"} {
		b, _ := messageFiles.ReadFile("i18n/" + l + ".json")
		m := map[string]string{}
		json.Unmarshal(b, &m)
		messages[l] = m
	}
	for k, v := range extraMessages {
		messages["zh-CN"][k], messages["en"][k] = v[0], v[1]
	}
}

// systemLocale is the desktop's language, as the browser's navigator.language.
func systemLocale() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			if strings.HasPrefix(v, "zh") {
				return "zh-CN"
			}
			return "en"
		}
	}
	return "zh-CN"
}

func setLocale(pref string) {
	switch pref {
	case "en", "zh-CN":
		locale = pref
	default:
		locale = systemLocale()
	}
}

// m returns the message key in the window's language, its {params} filled
// from pairs of name and value.
func m(key string, params ...any) string {
	s, ok := messages[locale][key]
	if !ok {
		s, ok = messages["en"][key]
	}
	if !ok {
		return key
	}
	for i := 0; i+1 < len(params); i += 2 {
		s = strings.ReplaceAll(s, "{"+fmt.Sprint(params[i])+"}", fmt.Sprint(params[i+1]))
	}
	return s
}

// plural picks key_one or key_many by n.
func plural(key string, n int) string {
	if n == 1 {
		return m(key+"_one", "count", n)
	}
	return m(key+"_many", "count", n)
}

// longDate is a date and time as the web's billing and rules pages write them.
func longDate(t time.Time) string {
	if locale == "zh-CN" {
		return t.Local().Format("2006年1月2日 15:04")
	}
	return t.Local().Format("Jan 2, 2006, 15:04")
}
