package cohub

import (
	"encoding/json"
	"testing"
)

func TestPatchReducer(t *testing.T) {
	var s PatchState
	ev := func(js string) PatchEvent {
		var e PatchEvent
		if err := json.Unmarshal([]byte(js), &e); err != nil {
			t.Fatal(err)
		}
		return e
	}
	steps := []string{
		`{"turnId":"t1","seq":1,"baseSeq":0,"ops":[{"o":"add","p":"/message/content/blocks/0","v":{"type":"thinking","thinking":"Hm"}}]}`,
		`{"turnId":"t1","seq":2,"baseSeq":1,"ops":[{"o":"append","p":"/message/content/blocks/0/thinking","v":"m."},{"o":"append","p":"/message/content/blocks/1/text","v":"Hel"}]}`,
		`{"turnId":"t1","seq":3,"baseSeq":2,"ops":[{"v":"lo"}]}`,
	}
	for _, js := range steps {
		if !s.Apply(ev(js)) {
			t.Fatalf("rejected %s", js)
		}
	}
	got := s.Content()
	if len(got) != 2 || got[0].Thinking != "Hmm." || got[1].Text != "Hello" {
		t.Fatalf("content %+v", got)
	}
	if s.Apply(ev(`{"turnId":"t1","seq":5,"baseSeq":4,"ops":[]}`)) {
		t.Fatal("a gap applied")
	}
	if !s.Apply(ev(`{"turnId":"t1","seq":3,"baseSeq":2,"ops":[]}`)) {
		t.Fatal("a duplicate asked for the snapshot")
	}
}

// A turn's second model call streams its own message with patches numbered
// from 1 again; the reducer took them for duplicates of the first message's
// and never showed the reply until the turn ended.
func TestStreamFollowsEachModelCall(t *testing.T) {
	var s Stream
	ev := func(js string) PatchEvent {
		var e PatchEvent
		if err := json.Unmarshal([]byte(js), &e); err != nil {
			t.Fatal(err)
		}
		return e
	}
	for _, js := range []string{
		`{"turnId":"t1","messageOrdinal":0,"seq":1,"baseSeq":0,"ops":[{"o":"replace","p":"/message/content/blocks/0","v":{"type":"tool_use","id":"c1","name":"bash","input":{}}}]}`,
		`{"turnId":"t1","messageOrdinal":0,"seq":2,"baseSeq":1,"ops":[{"o":"replace","p":"/message/content/blocks/0/input/command","v":"ls"}]}`,
		`{"turnId":"t1","messageOrdinal":0,"seq":3,"baseSeq":2,"ops":[{"o":"replace","p":"/message/content/blocks/0/_meta/toolStatus","v":"done"}]}`,
		`{"turnId":"t1","messageOrdinal":1,"seq":1,"baseSeq":0,"ops":[{"o":"replace","p":"/message/content/blocks/0","v":{"type":"text","text":"Do"}}]}`,
		`{"turnId":"t1","messageOrdinal":1,"seq":2,"baseSeq":1,"ops":[{"o":"append","p":"/message/content/blocks/0/text","v":"ne"}]}`,
	} {
		if !s.Patch(ev(js)) {
			t.Fatalf("rejected %s", js)
		}
	}
	if got := JoinText(s.Current()); got != "Done" {
		t.Fatalf("reply %q", got)
	}
	if done := s.Done(); len(done) != 1 || len(done[0].Content) != 1 || done[0].Content[0].Name != "bash" {
		t.Fatalf("steps %+v", done)
	}
	// The API saves the first call with its tool's result: it takes the
	// streamed one's place.
	saved := &StepMessage{ID: "m1", Content: []Block{{Type: "tool_use", Name: "bash"}, {Type: "tool_result"}}}
	saved.Meta.TurnID, saved.Meta.MessageOrdinal = "t1", new(int)
	s.Persisted(saved)
	if done := s.Done(); len(done) != 1 || len(done[0].Content) != 2 {
		t.Fatalf("steps after saving %+v", done)
	}
}
