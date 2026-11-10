package cohub

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The running turn's reply as it is written, as the SDK's
// session-patch-reducer.ts: the gateway sends session.turn.patch events,
// each a list of operations on the current message's content blocks
// since a sequence number; applied in order they give the reply token by
// token, and one out of order sends the window back to the snapshot.

type PatchOp struct {
	O string          `json:"o"`
	P string          `json:"p"`
	V json.RawMessage `json:"v"`
}

type PatchEvent struct {
	TurnID         string    `json:"turnId"`
	MessageID      string    `json:"messageId"`
	MessageOrdinal *int      `json:"messageOrdinal"`
	Seq            int       `json:"seq"`
	BaseSeq        int       `json:"baseSeq"`
	Ops            []PatchOp `json:"ops"`
}

// Message is the key of the message the patch writes, as the SDK's.
func (ev PatchEvent) Message() string { return messageKey(ev.MessageID, ev.TurnID, ev.MessageOrdinal) }

// messageKey names a message of a turn: by its ID, or by the turn and its
// place among the turn's model calls.
func messageKey(id, turnID string, ordinal *int) string {
	switch {
	case strings.TrimSpace(id) != "":
		return strings.TrimSpace(id)
	case ordinal != nil && turnID != "":
		return fmt.Sprintf("turn:%s:assistant:%d", turnID, *ordinal)
	}
	return ""
}

// ordinalOf is a message's place among its turn's model calls, from the
// ordinal sent or the key's end; -1 when neither tells.
func ordinalOf(key string, ordinal *int) int {
	if ordinal != nil {
		return *ordinal
	}
	if i := strings.LastIndexByte(key, ':'); i >= 0 {
		if n, err := strconv.Atoi(key[i+1:]); err == nil {
			return n
		}
	}
	return -1
}

// PatchState is the running turn's current message, block by block.
type PatchState struct {
	turnID     string
	message    string // the message's key, "" before the first patch that names it
	ordinal    int
	seq        int
	blocks     []map[string]any
	appendPath string
	terminal   bool
}

var (
	blockSubPath  = regexp.MustCompile(`^/message/content/blocks/(\d+)/(.+)$`)
	blockPath     = regexp.MustCompile(`^/message/content/blocks/(\d+)$`)
	blockMetaPath = regexp.MustCompile(`^/message/content/blocks/(\d+)/_meta$`)
)

func streamIndex(b map[string]any) (int, bool) {
	meta, _ := b["_meta"].(map[string]any)
	if v, ok := meta["streamIndex"].(float64); ok {
		return int(v), true
	}
	return 0, false
}

func findStream(blocks []map[string]any, idx int) int {
	for i, b := range blocks {
		if s, ok := streamIndex(b); ok && s == idx {
			return i
		}
	}
	return -1
}

func cloneJSON(v any) any {
	b, _ := json.Marshal(v)
	var out any
	json.Unmarshal(b, &out)
	return out
}

func pointer(tail string) []string {
	if tail == "" {
		return nil
	}
	segs := strings.Split(tail, "/")
	for i, s := range segs {
		segs[i] = strings.NewReplacer("~1", "/", "~0", "~").Replace(s)
	}
	return segs
}

// blockFor finds the block of a stream index, copied so the state before
// stays as it was, or starts a text or thinking block for it.
func blockFor(blocks *[]map[string]any, idx int, first string) map[string]any {
	if i := findStream(*blocks, idx); i >= 0 {
		b := cloneJSON((*blocks)[i]).(map[string]any)
		(*blocks)[i] = b
		return b
	}
	var b map[string]any
	switch first {
	case "text":
		b = map[string]any{"type": "text", "text": "", "_meta": map[string]any{"streamIndex": float64(idx)}}
	case "thinking":
		b = map[string]any{"type": "thinking", "thinking": "", "_meta": map[string]any{"streamIndex": float64(idx)}}
	default:
		return nil
	}
	*blocks = append(*blocks, b)
	return b
}

// parentOf walks to the object holding the path's last segment.
func parentOf(root map[string]any, segs []string) map[string]any {
	cur := root
	for _, k := range segs[:len(segs)-1] {
		child, ok := cur[k].(map[string]any)
		if !ok {
			return nil
		}
		cur = child
	}
	return cur
}

func appendAt(blocks *[]map[string]any, path string, raw json.RawMessage) bool {
	m := blockSubPath.FindStringSubmatch(path)
	if m == nil {
		return false
	}
	var suffix string
	if json.Unmarshal(raw, &suffix) != nil {
		return false
	}
	idx, _ := strconv.Atoi(m[1])
	segs := pointer(m[2])
	if len(segs) == 0 {
		return false
	}
	b := blockFor(blocks, idx, segs[0])
	if b == nil {
		return false
	}
	parent := parentOf(b, segs)
	if parent == nil {
		return false
	}
	leaf, ok := parent[segs[len(segs)-1]].(string)
	if !ok {
		return false
	}
	parent[segs[len(segs)-1]] = leaf + suffix
	return true
}

// Apply applies a patch; false asks for the snapshot again.
func (s *PatchState) Apply(ev PatchEvent) bool {
	if s.terminal && s.turnID == ev.TurnID {
		return true // a late patch of a finished turn
	}
	different := s.turnID != "" && ev.TurnID != "" && s.turnID != ev.TurnID
	fresh := different && ev.BaseSeq == 0
	cur := s.seq
	if fresh {
		cur = 0
	}
	keyframe := s.turnID != "" && s.turnID == ev.TurnID && ev.BaseSeq == 0 && ev.Seq >= cur
	switch {
	case different && !fresh:
		return false
	case !keyframe && ev.Seq <= cur:
		return true // a duplicate
	case !keyframe && ev.BaseSeq != cur:
		return false
	}
	start := ev.BaseSeq == 0 || fresh || keyframe
	next := []map[string]any{}
	appendPath := ""
	if !start {
		next = append(next, s.blocks...)
		appendPath = s.appendPath
	}
	for _, op := range ev.Ops {
		switch {
		case op.O == "" && op.P == "":
			if appendPath == "" || !appendAt(&next, appendPath, op.V) {
				return false
			}
		case op.O == "merge" && op.P == "/message/metadata":
		case op.O == "append":
			if !appendAt(&next, op.P, op.V) {
				return false
			}
			appendPath = op.P
		case op.O == "merge":
			m := blockMetaPath.FindStringSubmatch(op.P)
			if m == nil {
				continue
			}
			idx, _ := strconv.Atoi(m[1])
			i := findStream(next, idx)
			if i < 0 {
				continue
			}
			var v map[string]any
			json.Unmarshal(op.V, &v)
			b := cloneJSON(next[i]).(map[string]any)
			meta, _ := b["_meta"].(map[string]any)
			if meta == nil {
				meta = map[string]any{}
			}
			for k, x := range v {
				meta[k] = x
			}
			b["_meta"] = meta
			next[i] = b
		case op.O == "replace" && blockSubPath.MatchString(op.P):
			m := blockSubPath.FindStringSubmatch(op.P)
			idx, _ := strconv.Atoi(m[1])
			segs := pointer(m[2])
			if len(segs) == 0 {
				return false
			}
			b := blockFor(&next, idx, segs[0])
			if b == nil {
				return false
			}
			parent := parentOf(b, segs)
			if parent == nil {
				return false
			}
			var v any
			json.Unmarshal(op.V, &v)
			parent[segs[len(segs)-1]] = v
		case op.O == "replace" || op.O == "add":
			m := blockPath.FindStringSubmatch(op.P)
			if m == nil {
				continue
			}
			var b map[string]any
			if json.Unmarshal(op.V, &b) != nil || b["type"] == nil {
				continue
			}
			idx, _ := strconv.Atoi(m[1])
			meta, _ := b["_meta"].(map[string]any)
			if meta == nil {
				meta = map[string]any{}
			}
			meta["streamIndex"] = float64(idx)
			b["_meta"] = meta
			if i := findStream(next, idx); i >= 0 && compatible(next[i], b) {
				next[i] = b
			} else {
				next = append(next, b)
			}
		case op.O == "remove":
			m := blockPath.FindStringSubmatch(op.P)
			if m == nil {
				continue
			}
			idx, _ := strconv.Atoi(m[1])
			if i := findStream(next, idx); i >= 0 {
				next = append(next[:i], next[i+1:]...)
			}
		}
	}
	sort.SliceStable(next, func(i, j int) bool {
		a, aok := streamIndex(next[i])
		b, bok := streamIndex(next[j])
		switch {
		case !aok:
			return false
		case !bok:
			return true
		}
		return a < b
	})
	s.blocks, s.seq, s.appendPath, s.terminal = next, ev.Seq, appendPath, false
	if ev.TurnID != "" {
		s.turnID = ev.TurnID
	}
	if m := ev.Message(); m != "" {
		s.message, s.ordinal = m, ordinalOf(m, ev.MessageOrdinal)
	}
	return true
}

func compatible(a, b map[string]any) bool {
	if a["type"] != b["type"] {
		return false
	}
	switch a["type"] {
	case "tool_use":
		return a["id"] == b["id"] && a["name"] == b["name"]
	case "tool_result":
		return a["tool_use_id"] == b["tool_use_id"]
	}
	return true
}

// Content is the state as the app's blocks.
func (s *PatchState) Content() []Block {
	b, _ := json.Marshal(s.blocks)
	var out []Block
	json.Unmarshal(b, &out)
	return out
}

// PatchState is the snapshot's running message, for patches to continue
// from.
func (snap *StreamSnapshot) PatchState() *PatchState {
	cur := snap.Snapshot.Current
	ps := &PatchState{turnID: snap.Snapshot.TurnID, seq: snap.Snapshot.Seq, blocks: cur.Content}
	ps.message = messageKey(cur.MessageID, snap.Snapshot.TurnID, cur.MessageOrdinal)
	ps.ordinal = ordinalOf(ps.message, cur.MessageOrdinal)
	if cur.AppendPath != nil {
		ps.appendPath = *cur.AppendPath
	}
	return ps
}

// ahead tells whether the patches got further than a snapshot read before
// them: a later message of the turn, or more of the same one.
func (s *PatchState) ahead(snap *StreamSnapshot) bool {
	sn := snap.Snapshot
	if s.turnID == "" || s.turnID != sn.TurnID {
		return false
	}
	key := messageKey(sn.Current.MessageID, sn.TurnID, sn.Current.MessageOrdinal)
	if s.message != "" && key != "" && s.message != key {
		return s.ordinal > ordinalOf(key, sn.Current.MessageOrdinal)
	}
	return sn.Seq < s.seq
}

// TurnID is the running turn's.
func (s *PatchState) TurnID() string { return s.turnID }

// Seq is the sequence number of the last patch applied.
func (s *PatchState) Seq() int { return s.seq }

// String describes the state for diagnostics.
func (s *PatchState) String() string {
	if s == nil {
		return `state=-1 append=""`
	}
	return fmt.Sprintf("state=%d append=%q", s.seq, s.appendPath)
}
