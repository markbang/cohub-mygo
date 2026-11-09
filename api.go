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

// activity is when the space was last used, as the web's "recent" sort.
func (s *Space) activity() time.Time {
	if s.PersonalActivityAt.After(s.LastActivityAt) {
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
	return "新对话"
}

type Block struct {
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	Thinking string          `json:"thinking"`
	Name     string          `json:"name"`
	Args     json.RawMessage `json:"arguments"`
}

type Usage struct {
	Input       int64 `json:"input"`
	Output      int64 `json:"output"`
	CacheRead   int64 `json:"cacheRead"`
	CacheWrite  int64 `json:"cacheWrite"`
	TotalTokens int64 `json:"totalTokens"`
}

type intermediateSummary struct {
	MessageCount  int  `json:"messageCount"`
	ToolCallCount int  `json:"toolCallCount"`
	HasError      bool `json:"hasError"`
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
	Meta                struct {
		Harness string `json:"harness"`
		Source  string `json:"source"`
	} `json:"meta"`
	AuthorProfile *Profile `json:"authorProfile"`
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

func (cl *client) mySessions(ctx context.Context) ([]*Session, error) {
	var page struct {
		Sessions []*Session `json:"sessions"`
	}
	return page.Sessions, cl.do(ctx, "GET", "/api/me/sessions?limit=60", nil, &page)
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
func (cl *client) prompt(ctx context.Context, spaceID, sessionID, text string) (string, error) {
	in := map[string]any{"content": []map[string]string{{"type": "text", "text": text}}, "source": "web"}
	if sessionID != "" {
		in["sessionId"] = sessionID
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

func (cl *client) abort(ctx context.Context, sessionID string) error {
	return cl.do(ctx, "POST", "/api/sessions/"+url.PathEscape(sessionID)+"/abort", map[string]any{}, nil)
}
