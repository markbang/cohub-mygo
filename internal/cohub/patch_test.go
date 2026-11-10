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
