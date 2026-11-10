package cohub

import "sort"

// Stream is a session's running turn as the gateway streams it, as the
// SDK's SessionGenerationStreamClient: the messages of the model calls done
// so far, and the one being written. Each model call streams a message of
// its own, its patches numbered from 1 again, so a patch of the next
// message finishes the one before.
type Stream struct {
	turn string
	cur  *PatchState
	done []doneMessage
	// shown is cur as blocks, kept until it changes (rev): views read it
	// every frame.
	rev, shownRev int
	shown         []Block
}

type doneMessage struct {
	key     string
	ordinal int
	m       *StepMessage
}

// Turn is the running turn's ID, "" until a patch or snapshot names it.
func (s *Stream) Turn() string { return s.turn }

// Current is the message being written.
func (s *Stream) Current() []Block {
	if s.cur == nil {
		return nil
	}
	if s.shownRev != s.rev || s.shown == nil {
		s.shown, s.shownRev = s.cur.Content(), s.rev
	}
	return s.shown
}

// Done are the turn's messages finished so far, in order, each with the
// key of its stream as ID.
func (s *Stream) Done() []*StepMessage {
	out := make([]*StepMessage, len(s.done))
	for i, d := range s.done {
		out[i] = d.m
	}
	return out
}

// Patch applies a patch; false asks for the snapshot again.
func (s *Stream) Patch(ev PatchEvent) bool {
	if s.turn != "" && ev.TurnID != "" && ev.TurnID != s.turn {
		if ev.BaseSeq != 0 {
			return false
		}
		*s = Stream{}
	}
	if key := ev.Message(); key != "" {
		if s.has(key) {
			return true // a late patch of a message saved since
		}
		if s.cur != nil && s.cur.message != "" && key != s.cur.message {
			if ordinalOf(key, ev.MessageOrdinal) < s.cur.ordinal {
				return true
			}
			s.finish()
		}
	}
	if s.cur == nil {
		if ev.BaseSeq != 0 {
			return false
		}
		s.cur = &PatchState{}
	}
	if !s.cur.Apply(ev) {
		return false
	}
	s.rev++
	if ev.TurnID != "" {
		s.turn = ev.TurnID
	}
	return true
}

// Persisted takes a message of the turn the API saved: it is done, as
// saved, with its tools' results.
func (s *Stream) Persisted(m *StepMessage) {
	if s.turn == "" {
		s.turn = m.Meta.TurnID
	}
	if m.Meta.TurnID != "" && m.Meta.TurnID != s.turn {
		return
	}
	key := messageKey(m.Meta.StreamMessageID, m.Meta.TurnID, m.Meta.MessageOrdinal)
	s.add(key, ordinalOf(key, m.Meta.MessageOrdinal), m)
	if s.cur != nil && s.cur.message == key {
		s.cur = nil
	}
}

// Seed takes a snapshot of the turn, unless the patches got further since
// it was read; the messages it lists as done are taken either way.
func (s *Stream) Seed(snap *StreamSnapshot) {
	sn := snap.Snapshot
	if sn.TurnID != s.turn {
		*s = Stream{}
	}
	if s.cur == nil || !s.cur.ahead(snap) {
		s.cur = snap.PatchState()
		s.rev++
	}
	s.turn = sn.TurnID
	for _, im := range sn.IntermediateMessages {
		if len(im.Content) == 0 {
			continue
		}
		m := im.StepMessage
		key := messageKey(im.MessageID, sn.TurnID, im.MessageOrdinal)
		s.add(key, ordinalOf(key, im.MessageOrdinal), &m)
	}
}

// finish moves the message being written to the done ones, unless it was
// saved already.
func (s *Stream) finish() {
	c := s.cur
	s.cur = nil
	if c == nil || s.has(c.message) {
		return
	}
	if blocks := c.Content(); len(blocks) > 0 {
		s.add(c.message, c.ordinal, &StepMessage{Content: blocks})
	}
}

func (s *Stream) has(key string) bool {
	for _, d := range s.done {
		if d.key == key {
			return true
		}
	}
	return false
}

// add keeps a done message, in the place of one of its key.
func (s *Stream) add(key string, ordinal int, m *StepMessage) {
	cp := *m
	if key == "" {
		key = cp.ID
	}
	cp.ID = key
	for i, d := range s.done {
		if d.key == key {
			s.done[i].m = &cp
			return
		}
	}
	s.done = append(s.done, doneMessage{key, ordinal, &cp})
	sort.SliceStable(s.done, func(i, j int) bool { return s.done[i].ordinal < s.done[j].ordinal })
}
