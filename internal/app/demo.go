package app

import (
	"fmt"
	"time"

	"cohub-mygo/internal/cohub"
)

// loadDemo fills the app with data shaped as the API's, for tests and for
// running without a Cohub account.
func loadDemo(a *app) {
	a.spaces, a.sessions = nil, nil
	now := a.now()
	at := func(d time.Duration) time.Time { return now.Add(-d) }
	me := &cohub.Profile{UserUUID: "u1", Username: "bangwu", DisplayName: "bangwu"}
	a.me = &cohub.Me{UUID: "u1", Profile: me, Email: "bangwu@example.com"}
	spaces := []struct{ id, name, desc string }{
		{"s1", "All you need", ""},
		{"s2", "cohub-mobile", "cohub mobile: native for all"},
		{"s3", "context", "Context here"},
		{"s4", "bangwu router", ""},
		{"s5", "Home", "Personal Home Space"},
		{"s6", "Worldloom", ""},
	}
	for i, s := range spaces {
		a.spaces = append(a.spaces, &cohub.Space{
			ID: s.id, Name: s.name, Slug: s.id, Description: s.desc, OwnerProfile: me, Relation: "owner",
			CreatedAt: at(time.Duration(30+i) * 24 * time.Hour), LastActivityAt: at(time.Duration(i*3+1) * time.Minute),
			IsPinned: i < 4, SandboxStatus: "running",
		})
	}
	titles := []struct {
		space, title, preview string
		ago                   time.Duration
		running               bool
	}{
		{"s1", "看看这个 https://nsh.dev/", "", 2 * time.Minute, true},
		{"s2", "安卓端menu可替换为对应GitHub项目", "", 4 * time.Minute, true},
		{"s2", "看看这两个 cohub 的 PR", "原因是同一个上限在两层各写了一遍。", 30 * time.Minute, false},
		{"s2", "执行 npx skills@latest update", "已提交并 push 到 origin/main。", 35 * time.Minute, false},
		{"s3", "在当前工作区 .agents/skills 里补齐", "impeccable 其实没丢，不需要重装。", 40 * time.Minute, false},
		{"s4", "检查PR方案是否符合工程原则", "main 已经合进 PR 分支，没有冲突。", 50 * time.Minute, false},
		{"s2", "修复详情页按钮放大", "The fix is pushed to main as 73a1c0e.", 26 * time.Hour, false},
		{"s5", "Find all implicit state", "action-ui-compare 已经加进 PR #412。", 27 * time.Hour, false},
		{"s6", "修改landing并重新设计", "README 的拼图推到 main 了，提交是 5508bf1。", 28 * time.Hour, false},
	}
	for i, t := range titles {
		sp := a.space(t.space)
		s := &cohub.Session{
			ID: fmt.Sprintf("demo-%d", i), SpaceID: t.space, Title: t.title, Source: "web", Status: "active",
			LatestMessageText: t.preview, LastMessageAt: at(t.ago), CreatedAt: at(t.ago + time.Hour), UpdatedAt: at(t.ago),
			Space: &cohub.SessionSpace{ID: sp.ID, Name: sp.Name}, UserProfile: me,
		}
		if t.running {
			s.ActiveTurn = &cohub.ActiveTurn{ID: "run", Status: "running", Model: "claude-opus-5-5", StartedAt: at(t.ago + time.Minute)}
		}
		a.sessions = append(a.sessions, s)
		a.spaceSessions[t.space] = append(a.spaceSessions[t.space], s)
		if i >= 3 {
			a.seen[s.ID] = s.When()
		}
	}
	done := at(9 * time.Minute)
	tl := a.timelineOf("demo-0")
	tl.loaded = true
	tl.turns = []*cohub.Turn{
		{
			ID: "t1", Sequence: 1, Status: "completed", AuthorProfile: me,
			UserContent:      []cohub.Block{{Type: "text", Text: "看看这个 https://nsh.dev/"}},
			AssistantContent: []cohub.Block{{Type: "text", Text: demoReply}},
			Model:            "claude-opus-5-5", ThinkingLevel: "high", StartedAt: at(12 * time.Minute), CompletedAt: &done,
			TotalUsage:          &cohub.Usage{Input: 1100, CacheRead: 137100, Output: 1300},
			IntermediateSummary: &cohub.IntermediateSummary{MessageCount: 19, ToolCallCount: 19},
			Meta:                cohub.TurnMeta{Harness: "pi", Source: "web", FinalMessageDurationMs: 19181, RequestedThinkingLevel: "high"},
		},
		{
			ID: "t2", Sequence: 2, Status: "running", AuthorProfile: me,
			UserContent: []cohub.Block{{Type: "text", Text: "就用 mygo 写一个吧，纯 GPU 的🤔 复刻当前 web 一样的 ui 和布局"}},
			Model:       "claude-opus-5-5", ThinkingLevel: "high", StartedAt: at(3 * time.Minute),
			IntermediateSummary: &cohub.IntermediateSummary{MessageCount: 7, ToolCallCount: 7},
		},
	}
}

const demoReply = "**Native Shell (NSH)** 是 Longbridge 出的跨平台原生 UI 框架，用 Rust 写。\n\n" +
	"### 几个方案放在一起看\n\n" +
	"| 方案 | 解决什么 | 代价 |\n|---|---|---|\n" +
	"| 维持现状 | 已经能用，界面只有一份 web | 外壳要维护 3 种语言 |\n" +
	"| MyGo 统一桌面外壳 | 桌面 3 套外壳变成 1 套 Go | 新框架有风险 |\n" +
	"| NSH 嵌入外壳逐屏原生化 | 原生性能和手感 | 要用 TSX 重写界面 |\n\n" +
	"我的看法：现在的架构本来就是“界面只写一份 web，外壳越薄越好”，方向是对的。\n\n" +
	"```go\nui.Row(c).Gap(8).Children(func() {\n\tui.Text(c, \"Cohub\")\n})\n```\n\n" +
	"wiki 已更新：新增 `wiki/sources/mygo-egoist.md`，`index.md` 和 `log.md` 也补了记录。"
