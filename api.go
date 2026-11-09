package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Profile is a user's public profile.
type Profile struct {
	UserUUID    string `json:"userUuid"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
}

func (p *Profile) name() string {
	if p == nil {
		return ""
	}
	if p.DisplayName != "" {
		return p.DisplayName
	}
	return p.Username
}

type publicProfile struct {
	AvatarURL string `json:"avatarUrl"`
}

// Space is a row of GET /api/spaces.
type Space struct {
	ID                 string         `json:"id"`
	Name               string         `json:"name"`
	Slug               string         `json:"slug"`
	Description        string         `json:"description"`
	CreatedAt          time.Time      `json:"createdAt"`
	LastActivityAt     time.Time      `json:"lastActivityAt"`
	PersonalActivityAt time.Time      `json:"personalActivityAt"`
	JoinedAt           time.Time      `json:"joinedAt"`
	PublicProfile      *publicProfile `json:"publicProfile"`
	OwnerProfile       *Profile       `json:"ownerProfile"`
	SandboxStatus      string         `json:"sandboxStatus"`
	IsPinned           bool           `json:"isPinned"`
	IsArchived         bool           `json:"isArchived"`
	Relation           string         `json:"relation"`
}

func (s *Space) avatar() string {
	if s == nil || s.PublicProfile == nil {
		return ""
	}
	return s.PublicProfile.AvatarURL
}

// activity is when the user last used the space, as the web's "recent" sort.
func (s *Space) activity() time.Time {
	if !s.PersonalActivityAt.IsZero() {
		return s.PersonalActivityAt
	}
	return s.LastActivityAt
}

type ActiveTurn struct {
	ID        string    `json:"id"`
	Sequence  int       `json:"sequence"`
	Status    string    `json:"status"`
	Model     string    `json:"model"`
	StartedAt time.Time `json:"startedAt"`
}

type sessionSpace struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Slug          string         `json:"slug"`
	PublicProfile *publicProfile `json:"publicProfile"`
}

// Session is a row of GET /api/me/sessions or /api/spaces/:id/sessions.
type Session struct {
	ID                string        `json:"id"`
	SpaceID           string        `json:"spaceId"`
	Title             string        `json:"title"`
	Source            string        `json:"source"`
	Status            string        `json:"status"`
	LatestMessageText string        `json:"latestMessageText"`
	LastMessageAt     time.Time     `json:"lastMessageAt"`
	CreatedAt         time.Time     `json:"createdAt"`
	UpdatedAt         time.Time     `json:"updatedAt"`
	ActiveTurn        *ActiveTurn   `json:"activeTurn"`
	Space             *sessionSpace `json:"space"`
	UserProfile       *Profile      `json:"userProfile"`
	ArchivedAt        *time.Time    `json:"archivedAt"`
}

func (s *Session) running() bool {
	return s.ActiveTurn != nil && (s.ActiveTurn.Status == "running" || s.ActiveTurn.Status == "queued" || s.ActiveTurn.Status == "pending")
}

func (s *Session) when() time.Time {
	if !s.LastMessageAt.IsZero() {
		return s.LastMessageAt
	}
	return s.UpdatedAt
}

func (s *Session) displayTitle() string {
	if t := strings.TrimSpace(s.Title); t != "" {
		return t
	}
	return m("x_untitled")
}

type Block struct {
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	Thinking string          `json:"thinking"`
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Args     json.RawMessage `json:"arguments"`
	Input    json.RawMessage `json:"input"`
	IsError  bool            `json:"is_error"`
	ToolUse  string          `json:"tool_use_id"`
	Meta     *blockMeta      `json:"_meta"`
}

type blockMeta struct {
	Summary    string `json:"summary"`
	ToolStatus string `json:"toolStatus"`
	Timing     *struct {
		DurationMs int64 `json:"durationMs"`
	} `json:"timing"`
}

// StepMessage is one model call of a turn's work, from its intermediate file.
type StepMessage struct {
	ID         string    `json:"id"`
	Sequence   int       `json:"sequence"`
	Text       string    `json:"text"`
	Model      string    `json:"model"`
	StopReason string    `json:"stopReason"`
	Error      string    `json:"errorMessage"`
	Usage      *Usage    `json:"usage"`
	DurationMs int64     `json:"durationMs"`
	CreatedAt  time.Time `json:"createdAt"`
	Content    []Block   `json:"content"`
	Meta       struct {
		MessageKind string `json:"messageKind"`
	} `json:"meta"`
}

func (m *StepMessage) hasText() bool {
	for _, b := range m.Content {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			return true
		}
	}
	return false
}

// Model is a language model a chat can use.
type Model struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	Model    struct {
		Name             string             `json:"name"`
		Reasoning        bool               `json:"reasoning"`
		ContextWindow    int64              `json:"contextWindow"`
		ThinkingLevelMap map[string]*string `json:"thinkingLevelMap"`
		Input            []string           `json:"input"`
		DefaultLevel     string             `json:"defaultThinkingLevel"`
		Cost             struct {
			Input, Output, CacheRead, CacheWrite float64
		} `json:"cost"`
	} `json:"model"`
}

// costLabel is formatModelCost: "US$4/M 输入 · US$20/M 输出 · …".
func (m_ *Model) costLabel() string {
	c := m_.Model.Cost
	var parts []string
	for _, x := range []struct {
		v   float64
		key string
	}{{c.Input, "model_cost_input"}, {c.Output, "model_cost_output"}, {c.CacheRead, "model_cost_cache_read"}, {c.CacheWrite, "model_cost_cache_write"}} {
		if x.v == 0 {
			continue
		}
		var s string
		switch {
		case x.v < 0.01:
			s = fmt.Sprintf("US$%.4f", x.v)
		case x.v < 1:
			s = fmt.Sprintf("US$%.2f", x.v)
		case x.v == float64(int64(x.v)):
			s = fmt.Sprintf("US$%d", int64(x.v))
		default:
			s = fmt.Sprintf("US$%.2f", x.v)
		}
		parts = append(parts, s+"/M "+m(x.key))
	}
	return strings.Join(parts, " · ")
}

func (m_ *Model) vision() bool {
	for _, in := range m_.Model.Input {
		if in == "image" {
			return true
		}
	}
	return false
}

// levels returns the thinking levels the model takes, in order.
func (m *Model) levels() []string {
	var out []string
	for _, l := range []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"} {
		if v, ok := m.Model.ThinkingLevelMap[l]; ok && v != nil {
			out = append(out, l)
		}
	}
	return out
}

type modelChoice struct{ provider, id, level string }

// Label is a Space's label, with the system's source, user and channel trees.
type Label struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Slug      string   `json:"slug"`
	Source    string   `json:"source"`
	SystemKey string   `json:"systemKey"`
	Depth     int      `json:"depth"`
	Children  []*Label `json:"children"`
}

type LabelItem struct {
	ResourceType string `json:"resourceType"`
	ResourceRef  string `json:"resourceRef"`
}

// SpaceUsage sums a Space's language model use over some days.
type SpaceUsage struct {
	Totals struct {
		TotalTokens  int64   `json:"totalTokens"`
		RequestCount int64   `json:"requestCount"`
		CostTotal    float64 `json:"costTotal"`
	} `json:"totals"`
	Days int `json:"days"`
}

type Usage struct {
	Input       int64 `json:"input"`
	Output      int64 `json:"output"`
	CacheRead   int64 `json:"cacheRead"`
	CacheWrite  int64 `json:"cacheWrite"`
	TotalTokens int64 `json:"totalTokens"`
	Cost        *struct {
		Total float64 `json:"total"`
	} `json:"cost"`
}

func (u *Usage) cost() float64 {
	if u == nil || u.Cost == nil {
		return 0
	}
	return u.Cost.Total
}

type intermediateSummary struct {
	MessageCount  int    `json:"messageCount"`
	ToolCallCount int    `json:"toolCallCount"`
	HasError      bool   `json:"hasError"`
	Usage         *Usage `json:"usage"`
	DurationMs    int64  `json:"durationMs"`
}

type turnMeta struct {
	Harness                string `json:"harness"`
	Source                 string `json:"source"`
	FinalMessageDurationMs int64  `json:"finalMessageDurationMs"`
	RequestedThinkingLevel string `json:"requestedThinkingLevel"`
	RuntimeArchiveStatus   string `json:"runtimeArchiveStatus"`
}

// Turn is one exchange of a session: the user's message and the reply.
type Turn struct {
	ID                  string               `json:"id"`
	Sequence            int                  `json:"sequence"`
	Status              string               `json:"status"`
	UserContent         []Block              `json:"userContent"`
	UserText            string               `json:"userText"`
	AssistantContent    []Block              `json:"assistantContent"`
	Provider            string               `json:"provider"`
	Model               string               `json:"model"`
	ThinkingLevel       string               `json:"thinkingLevel"`
	ErrorMessage        string               `json:"errorMessage"`
	StartedAt           time.Time            `json:"startedAt"`
	CompletedAt         *time.Time           `json:"completedAt"`
	DurationMs          int64                `json:"durationMs"`
	TotalUsage          *Usage               `json:"totalUsage"`
	IntermediateSummary *intermediateSummary `json:"intermediateSummary"`
	Meta                turnMeta             `json:"meta"`
	AuthorProfile       *Profile             `json:"authorProfile"`
	FinalUsage          *Usage               `json:"finalUsage"`
	IntermediateIndex   *struct {
		MessagesObjectKey string `json:"messagesObjectKey"`
	} `json:"intermediateIndex"`
}

func (t *Turn) running() bool {
	return t.CompletedAt == nil && t.Status != "completed" && t.Status != "failed" && t.Status != "aborted" && t.Status != "cancelled"
}

func joinText(blocks []Block) string {
	var b strings.Builder
	for _, x := range blocks {
		if x.Type == "text" && strings.TrimSpace(x.Text) != "" {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(x.Text)
		}
	}
	return b.String()
}

func (t *Turn) prompt() string {
	if s := joinText(t.UserContent); s != "" {
		return s
	}
	return t.UserText
}

func (t *Turn) reply() string { return joinText(t.AssistantContent) }

type turnsPage struct {
	Turns      []*Turn `json:"turns"`
	HasMore    bool    `json:"hasMore"`
	NextCursor *int    `json:"nextCursor"`
}

type streamSnapshot struct {
	Snapshot *struct {
		TurnID  string `json:"turnId"`
		Current *struct {
			Content []Block `json:"content"`
		} `json:"current"`
		IntermediateMessages []json.RawMessage `json:"intermediateMessages"`
	} `json:"snapshot"`
}

type Me struct {
	UUID    string   `json:"uuid"`
	Profile *Profile `json:"profile"`
	Email   string   `json:"email"`
}

// client talks to the Cohub API with the account of the `cohub` CLI, the
// one the desktop shells share.
type client struct {
	base string
	http *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

type cliAuth struct {
	BaseURL              string `json:"baseUrl"`
	AccessToken          string `json:"accessToken"`
	AccessTokenExpiresAt int64  `json:"accessTokenExpiresAt"`
}

var errNoAccount = errors.New("no Cohub account: run `cohub auth login`")

func newClient() (*client, error) {
	cl := &client{base: "https://api.cohub.live", http: &http.Client{Timeout: 30 * time.Second}}
	if v := os.Getenv("COHUB_API_URL"); v != "" {
		cl.base = strings.TrimRight(v, "/")
	}
	if v := os.Getenv("COHUB_TOKEN"); v != "" {
		cl.token, cl.expires = v, time.Now().Add(24*time.Hour)
		return cl, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, errNoAccount
	}
	b, err := os.ReadFile(filepath.Join(dir, "cohub", "auth.json"))
	if err != nil {
		return nil, errNoAccount
	}
	var a cliAuth
	if json.Unmarshal(b, &a) != nil || a.AccessToken == "" {
		return nil, errNoAccount
	}
	if a.BaseURL != "" && os.Getenv("COHUB_API_URL") == "" {
		cl.base = strings.TrimRight(a.BaseURL, "/")
	}
	cl.token = a.AccessToken
	exp := a.AccessTokenExpiresAt
	if exp > 1e12 {
		exp /= 1000
	}
	cl.expires = time.Unix(exp, 0)
	return cl, nil
}

// accessToken returns a token valid for another minute, refreshed by the CLI.
func (cl *client) accessToken(ctx context.Context) (string, error) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if cl.token != "" && time.Until(cl.expires) > time.Minute {
		return cl.token, nil
	}
	out, err := exec.CommandContext(ctx, "cohub", "auth", "token").Output()
	if err != nil {
		return "", errNoAccount
	}
	cl.token, cl.expires = strings.TrimSpace(string(out)), time.Now().Add(50*time.Minute)
	return cl.token, nil
}

func (cl *client) do(ctx context.Context, method, path string, body, out any) error {
	tok, err := cl.accessToken(ctx)
	if err != nil {
		return err
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, cl.base+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Cohub-Source-Via", "desktop-mygo")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := cl.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized {
		cl.mu.Lock()
		cl.expires = time.Time{}
		cl.mu.Unlock()
	}
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("%s %s: %d %s", method, path, res.StatusCode, strings.TrimSpace(string(b)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func (cl *client) me(ctx context.Context) (*Me, error) {
	var me Me
	return &me, cl.do(ctx, "GET", "/api/me", nil, &me)
}

func (cl *client) spaces(ctx context.Context) ([]*Space, error) {
	var page struct {
		Items []*Space `json:"items"`
	}
	return page.Items, cl.do(ctx, "GET", "/api/spaces?limit=100", nil, &page)
}

// mySessions lists the user's chats from a source ("" for every one), or
// the archived ones, as the web's chats inbox.
func (cl *client) mySessions(ctx context.Context, f chatsFilter) ([]*Session, error) {
	var page struct {
		Sessions []*Session `json:"sessions"`
	}
	q := url.Values{"limit": {"60"}}
	if f.archived {
		q.Set("archived", "only")
	} else if f.source != "" {
		q.Set("source", f.source)
	}
	return page.Sessions, cl.do(ctx, "GET", "/api/me/sessions?"+q.Encode(), nil, &page)
}

// spaceChats lists a Space's chats from a source, through the Space's
// source label, as the inbox does once a chip is chosen.
func (cl *client) spaceChats(ctx context.Context, spaceID string, f chatsFilter) ([]*Session, error) {
	if f.source == "" || f.archived {
		path := "/api/spaces/" + url.PathEscape(spaceID) + "/sessions?limit=60"
		if f.archived {
			path += "&archived=only"
		}
		var page struct {
			Sessions []*Session `json:"sessions"`
		}
		return page.Sessions, cl.do(ctx, "GET", path, nil, &page)
	}
	var out struct {
		Sessions []*Session `json:"sessions"`
	}
	q := url.Values{"ref": {"Source/" + sourceName(f.source)}, "limit": {"60"}}
	return out.Sessions, cl.do(ctx, "GET", "/api/spaces/"+url.PathEscape(spaceID)+"/labels/items?"+q.Encode(), nil, &out)
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

func (cl *client) spaceSessions(ctx context.Context, spaceID string) ([]*Session, error) {
	var page struct {
		Sessions []*Session `json:"sessions"`
	}
	return page.Sessions, cl.do(ctx, "GET", "/api/spaces/"+url.PathEscape(spaceID)+"/sessions?limit=60", nil, &page)
}

// turns returns the latest turns of a session, oldest first.
func (cl *client) turns(ctx context.Context, sessionID string, limit int) (*turnsPage, error) {
	var page turnsPage
	return &page, cl.do(ctx, "GET", fmt.Sprintf("/api/sessions/%s/turns?limit=%d", url.PathEscape(sessionID), limit), nil, &page)
}

func (cl *client) snapshot(ctx context.Context, sessionID string) (*streamSnapshot, error) {
	var s streamSnapshot
	return &s, cl.do(ctx, "GET", "/api/sessions/"+url.PathEscape(sessionID)+"/turns/stream-snapshot", nil, &s)
}

// prompt sends text to a session of a space, or to a new one without sessionID.
func (cl *client) prompt(ctx context.Context, spaceID, sessionID string, content []map[string]any, model modelChoice) (string, error) {
	in := map[string]any{"content": content, "source": "web", "clientMessageId": newID()}
	if sessionID != "" {
		in["sessionId"] = sessionID
	}
	if model.id != "" {
		in["model"], in["provider"] = model.id, model.provider
	}
	if model.level != "" {
		in["thinkingLevel"] = model.level
	}
	var out struct {
		SessionID string `json:"sessionId"`
		Session   *struct {
			ID string `json:"id"`
		} `json:"session"`
	}
	if err := cl.do(ctx, "POST", "/api/spaces/"+url.PathEscape(spaceID)+"/prompt", in, &out); err != nil {
		return "", err
	}
	if out.SessionID == "" && out.Session != nil {
		out.SessionID = out.Session.ID
	}
	return out.SessionID, nil
}

func (cl *client) models(ctx context.Context) ([]*Model, error) {
	var out map[string][]*Model
	if err := cl.do(ctx, "GET", "/api/models", nil, &out); err != nil {
		return nil, err
	}
	return out["cohub"], nil
}

func (cl *client) labels(ctx context.Context, spaceID string) ([]*Label, error) {
	var out struct {
		Labels []*Label `json:"labels"`
	}
	return out.Labels, cl.do(ctx, "GET", "/api/spaces/"+url.PathEscape(spaceID)+"/labels", nil, &out)
}

func (cl *client) labelItems(ctx context.Context, spaceID, ref string) ([]LabelItem, error) {
	var out struct {
		Items []LabelItem `json:"items"`
	}
	q := url.Values{"ref": {ref}, "limit": {"100"}}
	return out.Items, cl.do(ctx, "GET", "/api/spaces/"+url.PathEscape(spaceID)+"/labels/items?"+q.Encode(), nil, &out)
}

func (cl *client) spaceUsage(ctx context.Context, spaceID string, days int) (*SpaceUsage, error) {
	var out SpaceUsage
	return &out, cl.do(ctx, "GET", fmt.Sprintf("/api/spaces/%s/usage?days=%d", url.PathEscape(spaceID), days), nil, &out)
}

// steps loads the intermediate messages of a turn, through a signed URL.
func (cl *client) steps(ctx context.Context, sessionID, turnID, objectKey string) ([]*StepMessage, error) {
	var signed struct {
		URLs map[string]string `json:"urls"`
	}
	if err := cl.do(ctx, "POST", fmt.Sprintf("/api/sessions/%s/turns/%s/signed-urls", url.PathEscape(sessionID), url.PathEscape(turnID)),
		map[string]any{"objectKeys": []string{objectKey}}, &signed); err != nil {
		return nil, err
	}
	u := signed.URLs[objectKey]
	if u == "" {
		return nil, errors.New("no signed URL for the steps")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	res, err := cl.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var file struct {
		Messages []*StepMessage `json:"messages"`
	}
	return file.Messages, json.NewDecoder(res.Body).Decode(&file)
}

// get reads any JSON endpoint, for pages that show what they read.
func (cl *client) get(ctx context.Context, path string, out any) error {
	return cl.do(ctx, "GET", path, nil, out)
}

func (cl *client) abort(ctx context.Context, sessionID string) error {
	return cl.do(ctx, "POST", "/api/sessions/"+url.PathEscape(sessionID)+"/abort", map[string]any{}, nil)
}
