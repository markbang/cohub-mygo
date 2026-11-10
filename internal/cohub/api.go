// Package cohub is the Cohub API: its types, a client with the cohub CLI's
// account, the gateway's realtime connection, and the reducer of streamed
// replies.
package cohub

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
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

func (p *Profile) Name() string {
	if p == nil {
		return ""
	}
	if p.DisplayName != "" {
		return p.DisplayName
	}
	return p.Username
}

type PublicProfile struct {
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
	PublicProfile      *PublicProfile `json:"publicProfile"`
	OwnerProfile       *Profile       `json:"ownerProfile"`
	SandboxStatus      string         `json:"sandboxStatus"`
	IsPinned           bool           `json:"isPinned"`
	IsArchived         bool           `json:"isArchived"`
	Relation           string         `json:"relation"`
}

func (s *Space) Avatar() string {
	if s == nil || s.PublicProfile == nil {
		return ""
	}
	return s.PublicProfile.AvatarURL
}

// Activity is when the user last used the space, as the web's "recent" sort.
func (s *Space) Activity() time.Time {
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

type SessionSpace struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Slug          string         `json:"slug"`
	PublicProfile *PublicProfile `json:"publicProfile"`
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
	Space             *SessionSpace `json:"space"`
	UserProfile       *Profile      `json:"userProfile"`
	ArchivedAt        *time.Time    `json:"archivedAt"`
}

func (s *Session) Running() bool {
	return s.ActiveTurn != nil && (s.ActiveTurn.Status == "running" || s.ActiveTurn.Status == "queued" || s.ActiveTurn.Status == "pending")
}

func (s *Session) When() time.Time {
	if !s.LastMessageAt.IsZero() {
		return s.LastMessageAt
	}
	return s.UpdatedAt
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
	Meta     *BlockMeta      `json:"_meta"`
}

type BlockMeta struct {
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

func (m *StepMessage) HasText() bool {
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

func (m_ *Model) Vision() bool {
	for _, in := range m_.Model.Input {
		if in == "image" {
			return true
		}
	}
	return false
}

// Levels returns the thinking levels the model takes, in order.
func (m *Model) Levels() []string {
	var out []string
	for _, l := range []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"} {
		if v, ok := m.Model.ThinkingLevelMap[l]; ok && v != nil {
			out = append(out, l)
		}
	}
	return out
}

type ModelChoice struct{ Provider, ID, Level, Harness, Approval string }

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

func (u *Usage) TotalCost() float64 {
	if u == nil || u.Cost == nil {
		return 0
	}
	return u.Cost.Total
}

type IntermediateSummary struct {
	MessageCount  int    `json:"messageCount"`
	ToolCallCount int    `json:"toolCallCount"`
	HasError      bool   `json:"hasError"`
	Usage         *Usage `json:"usage"`
	DurationMs    int64  `json:"durationMs"`
}

type TurnMeta struct {
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
	UserUUID            string               `json:"userUuid"`
	AssistantContent    []Block              `json:"assistantContent"`
	Provider            string               `json:"provider"`
	Model               string               `json:"model"`
	ThinkingLevel       string               `json:"thinkingLevel"`
	ErrorMessage        string               `json:"errorMessage"`
	StartedAt           time.Time            `json:"startedAt"`
	CompletedAt         *time.Time           `json:"completedAt"`
	DurationMs          int64                `json:"durationMs"`
	TotalUsage          *Usage               `json:"totalUsage"`
	IntermediateSummary *IntermediateSummary `json:"intermediateSummary"`
	Meta                TurnMeta             `json:"meta"`
	AuthorProfile       *Profile             `json:"authorProfile"`
	FinalUsage          *Usage               `json:"finalUsage"`
	IntermediateIndex   *struct {
		MessagesObjectKey string `json:"messagesObjectKey"`
	} `json:"intermediateIndex"`
}

func (t *Turn) Running() bool {
	return t.CompletedAt == nil && t.Status != "completed" && t.Status != "failed" && t.Status != "aborted" && t.Status != "cancelled"
}

func JoinText(blocks []Block) string {
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

func (t *Turn) Prompt() string {
	if s := JoinText(t.UserContent); s != "" {
		return s
	}
	return t.UserText
}

func (t *Turn) Reply() string { return JoinText(t.AssistantContent) }

type TurnsPage struct {
	Turns      []*Turn `json:"turns"`
	HasMore    bool    `json:"hasMore"`
	NextCursor *int    `json:"nextCursor"`
}

type StreamSnapshot struct {
	Snapshot *struct {
		TurnID  string `json:"turnId"`
		Seq     int    `json:"seq"`
		Current *struct {
			Content    []map[string]any `json:"content"`
			AppendPath *string          `json:"appendPath"`
		} `json:"current"`
		IntermediateMessages []struct {
			MessageID string  `json:"messageId"`
			Content   []Block `json:"content"`
		} `json:"intermediateMessages"`
	} `json:"snapshot"`
}

type Me struct {
	UUID    string   `json:"uuid"`
	Profile *Profile `json:"profile"`
	Email   string   `json:"email"`
}

// Client talks to the Cohub API with the account of the `cohub` CLI, the
// one the desktop shells share.
type Client struct {
	base     string
	http     *http.Client
	clientID string // this install's, as X-Cohub-Source-Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

type cliAuth struct {
	BaseURL              string `json:"baseUrl"`
	AccessToken          string `json:"accessToken"`
	AccessTokenExpiresAt int64  `json:"accessTokenExpiresAt"`
}

var ErrNoAccount = errors.New("no Cohub account: run `cohub auth login`")

// New signs in with the cohub CLI's account, or $COHUB_TOKEN, against
// $COHUB_API_URL or the account's API. clientID is this install's, sent as
// X-Cohub-Source-Client.
func New(clientID string) (*Client, error) {
	cl := &Client{base: "https://api.cohub.live", http: &http.Client{Timeout: 30 * time.Second}, clientID: clientID}
	if v := os.Getenv("COHUB_API_URL"); v != "" {
		cl.base = strings.TrimRight(v, "/")
	}
	if v := os.Getenv("COHUB_TOKEN"); v != "" {
		cl.token, cl.expires = v, time.Now().Add(24*time.Hour)
		return cl, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, ErrNoAccount
	}
	b, err := os.ReadFile(filepath.Join(dir, "cohub", "auth.json"))
	if err != nil {
		return nil, ErrNoAccount
	}
	var a cliAuth
	if json.Unmarshal(b, &a) != nil || a.AccessToken == "" {
		return nil, ErrNoAccount
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

// NewWithToken is a client of base with a fixed token, as for a test server.
func NewWithToken(base, token, clientID string, hc *http.Client) *Client {
	return &Client{base: base, http: hc, clientID: clientID, token: token, expires: time.Now().Add(24 * time.Hour)}
}

// Base is the API's URL, like https://api.cohub.live.
func (cl *Client) Base() string { return cl.base }

// ClientID is this install's, as the gateway addresses desktop commands.
func (cl *Client) ClientID() string { return cl.clientID }

// AccessToken returns a token valid for another minute, refreshed by the CLI.
func (cl *Client) AccessToken(ctx context.Context) (string, error) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if cl.token != "" && time.Until(cl.expires) > time.Minute {
		return cl.token, nil
	}
	out, err := exec.CommandContext(ctx, "cohub", "auth", "token").Output()
	if err != nil {
		return "", ErrNoAccount
	}
	cl.token, cl.expires = strings.TrimSpace(string(out)), time.Now().Add(50*time.Minute)
	return cl.token, nil
}

func (cl *Client) Do(ctx context.Context, method, path string, body, out any) error {
	tok, err := cl.AccessToken(ctx)
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
	if cl.clientID != "" {
		req.Header.Set("X-Cohub-Source-Client", cl.clientID)
	}
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

func (cl *Client) Me(ctx context.Context) (*Me, error) {
	var me Me
	return &me, cl.Do(ctx, "GET", "/api/me", nil, &me)
}

func (cl *Client) Spaces(ctx context.Context) ([]*Space, error) {
	var page struct {
		Items []*Space `json:"items"`
	}
	return page.Items, cl.Do(ctx, "GET", "/api/spaces?limit=100", nil, &page)
}

func (cl *Client) SpaceSessions(ctx context.Context, spaceID string) ([]*Session, []SessionFork, error) {
	var page struct {
		Sessions []*Session    `json:"sessions"`
		Forks    []SessionFork `json:"forks"`
	}
	err := cl.Do(ctx, "GET", "/api/spaces/"+url.PathEscape(spaceID)+"/sessions?limit=60&includeForks=1", nil, &page)
	return page.Sessions, page.Forks, err
}

// Turns returns the latest turns of a session, oldest first.
func (cl *Client) Turns(ctx context.Context, sessionID string, limit int) (*TurnsPage, error) {
	var page TurnsPage
	return &page, cl.Do(ctx, "GET", fmt.Sprintf("/api/sessions/%s/turns?limit=%d", url.PathEscape(sessionID), limit), nil, &page)
}

func (cl *Client) Snapshot(ctx context.Context, sessionID string) (*StreamSnapshot, error) {
	var s StreamSnapshot
	return &s, cl.Do(ctx, "GET", "/api/sessions/"+url.PathEscape(sessionID)+"/turns/stream-snapshot", nil, &s)
}

// Prompt sends text to a session of a space, or to a new one without sessionID.
func (cl *Client) Prompt(ctx context.Context, spaceID, sessionID string, content []map[string]any, model ModelChoice) (string, error) {
	in := map[string]any{"content": content, "source": "web", "clientMessageId": NewID()}
	if sessionID != "" {
		in["sessionId"] = sessionID
	}
	if model.ID != "" {
		in["model"], in["provider"] = model.ID, model.Provider
	}
	if model.Level != "" {
		in["thinkingLevel"] = model.Level
	}
	if model.Harness != "" && model.Harness != "cohub" {
		in["harness"] = model.Harness
		if model.Approval != "" {
			in["approvalMode"] = model.Approval
		}
	}
	var out struct {
		SessionID string `json:"sessionId"`
		Session   *struct {
			ID string `json:"id"`
		} `json:"session"`
	}
	if err := cl.Do(ctx, "POST", "/api/spaces/"+url.PathEscape(spaceID)+"/prompt", in, &out); err != nil {
		return "", err
	}
	if out.SessionID == "" && out.Session != nil {
		out.SessionID = out.Session.ID
	}
	return out.SessionID, nil
}

func (cl *Client) Models(ctx context.Context) ([]*Model, error) {
	var out map[string][]*Model
	if err := cl.Do(ctx, "GET", "/api/models", nil, &out); err != nil {
		return nil, err
	}
	return out["cohub"], nil
}

func (cl *Client) Labels(ctx context.Context, spaceID string) ([]*Label, error) {
	var out struct {
		Labels []*Label `json:"labels"`
	}
	return out.Labels, cl.Do(ctx, "GET", "/api/spaces/"+url.PathEscape(spaceID)+"/labels", nil, &out)
}

func (cl *Client) LabelItems(ctx context.Context, spaceID, ref string) ([]LabelItem, error) {
	var out struct {
		Items []LabelItem `json:"items"`
	}
	q := url.Values{"ref": {ref}, "limit": {"100"}}
	return out.Items, cl.Do(ctx, "GET", "/api/spaces/"+url.PathEscape(spaceID)+"/labels/items?"+q.Encode(), nil, &out)
}

func (cl *Client) SpaceUsage(ctx context.Context, spaceID string, days int) (*SpaceUsage, error) {
	var out SpaceUsage
	return &out, cl.Do(ctx, "GET", fmt.Sprintf("/api/spaces/%s/usage?days=%d", url.PathEscape(spaceID), days), nil, &out)
}

// Steps loads the intermediate messages of a turn, through a signed URL.
func (cl *Client) Steps(ctx context.Context, sessionID, turnID, objectKey string) ([]*StepMessage, error) {
	var signed struct {
		URLs map[string]string `json:"urls"`
	}
	if err := cl.Do(ctx, "POST", fmt.Sprintf("/api/sessions/%s/turns/%s/signed-urls", url.PathEscape(sessionID), url.PathEscape(turnID)),
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

// Get reads any JSON endpoint, for pages that show what they read.
func (cl *Client) Get(ctx context.Context, path string, out any) error {
	return cl.Do(ctx, "GET", path, nil, out)
}

func (cl *Client) Abort(ctx context.Context, sessionID string) error {
	return cl.Do(ctx, "POST", "/api/sessions/"+url.PathEscape(sessionID)+"/abort", map[string]any{}, nil)
}

type SessionFork struct {
	ParentSessionID        string  `json:"parentSessionId"`
	ChildSessionID         string  `json:"childSessionId"`
	Depth                  int     `json:"depth"`
	AnchorSequence         int     `json:"anchorSequence"`
	FirstUserTextAfterFork *string `json:"firstUserTextAfterFork"`
	ParentTitle            *string `json:"parentTitle"`
}

func (s *SessionSpace) AvatarURL() string {
	if s == nil || s.PublicProfile == nil {
		return ""
	}
	return s.PublicProfile.AvatarURL
}

// Download reads a file's bytes from the Space, waiting while it's prepared.
func (cl *Client) Download(ctx context.Context, spaceID, p string) ([]byte, string, error) {
	token, err := cl.AccessToken(ctx)
	if err != nil {
		return nil, "", err
	}
	for i := 0; i < 20; i++ {
		req, _ := http.NewRequestWithContext(ctx, "GET", cl.base+"/api/spaces/"+url.PathEscape(spaceID)+"/fs/download?path="+url.QueryEscape(p), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, "", err
		}
		if res.StatusCode == http.StatusAccepted {
			res.Body.Close()
			time.Sleep(time.Second)
			continue
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return nil, "", fmt.Errorf("download %s: HTTP %d", p, res.StatusCode)
		}
		b, err := io.ReadAll(io.LimitReader(res.Body, 256<<20))
		return b, res.Header.Get("Content-Type"), err
	}
	return nil, "", fmt.Errorf("download %s: still preparing", p)
}

// NewID is a random UUID.
func NewID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
