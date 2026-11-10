package mp4

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func mkbox(typ string, body ...[]byte) []byte {
	b := bytes.Join(body, nil)
	h := make([]byte, 8)
	binary.BigEndian.PutUint32(h, uint32(8+len(b)))
	copy(h[4:], typ)
	return append(h, b...)
}

func mvhdV0(scale, dur uint32) []byte {
	b := make([]byte, 100)
	binary.BigEndian.PutUint32(b[12:], scale)
	binary.BigEndian.PutUint32(b[16:], dur)
	return mkbox("mvhd", b)
}

// The duration is read from the movie header whether the moov box comes
// before the media data or after it, as in files that aren't fast-start.
func TestDuration(t *testing.T) {
	moov := mkbox("moov", mkbox("trak"), mvhdV0(1000, 5042))
	mdat := mkbox("mdat", make([]byte, 4096))
	for name, file := range map[string][]byte{
		"moov first": bytes.Join([][]byte{mkbox("ftyp", []byte("isom")), moov, mdat}, nil),
		"moov last":  bytes.Join([][]byte{mkbox("ftyp", []byte("isom")), mkbox("free"), mdat, moov}, nil),
	} {
		got, err := Duration(bytes.NewReader(file))
		if err != nil || got != 5042*time.Millisecond {
			t.Errorf("%s: %v, %v", name, got, err)
		}
	}
	if _, err := Duration(bytes.NewReader(mkbox("ftyp"))); err == nil {
		t.Error("a file without moov has a duration")
	}
}
