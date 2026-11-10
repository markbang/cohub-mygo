// Package mp4 reads an MP4 file's duration from its movie header, walking
// the top-level boxes so that a file read over HTTP ranges is fetched only
// where its boxes start and where the header is.
package mp4

import (
	"encoding/binary"
	"errors"
	"io"
	"time"
)

// maxMoov bounds the movie box read whole, where the header is.
const maxMoov = 8 << 20

var errNoHeader = errors.New("mp4: no movie header")

// Duration returns the movie's duration, from the mvhd box in its moov box.
func Duration(r io.ReaderAt) (time.Duration, error) {
	var off int64
	for {
		size, typ, hdr, err := box(r, off)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, errNoHeader
			}
			return 0, err
		}
		if typ == "moov" {
			if size == 0 || size-hdr > maxMoov {
				size = hdr + maxMoov
			}
			b := make([]byte, size-hdr)
			n, err := r.ReadAt(b, off+hdr)
			if n == 0 && err != nil {
				return 0, err
			}
			return mvhd(b[:n])
		}
		if size == 0 {
			return 0, errNoHeader
		}
		off += size
	}
}

// box reads the header of the box at off: its size (0 to the end of the
// file), type and header length.
func box(r io.ReaderAt, off int64) (size int64, typ string, hdr int64, err error) {
	var h [16]byte
	if _, err := r.ReadAt(h[:8], off); err != nil {
		return 0, "", 0, err
	}
	size, typ, hdr = int64(binary.BigEndian.Uint32(h[:4])), string(h[4:8]), 8
	if size == 1 {
		if _, err := r.ReadAt(h[8:16], off+8); err != nil {
			return 0, "", 0, err
		}
		size, hdr = int64(binary.BigEndian.Uint64(h[8:16])), 16
	}
	if size != 0 && size < hdr {
		return 0, "", 0, errors.New("mp4: bad box size")
	}
	return size, typ, hdr, nil
}

// mvhd finds the movie header among the moov box's children.
func mvhd(b []byte) (time.Duration, error) {
	for len(b) >= 8 {
		size, typ := int(binary.BigEndian.Uint32(b)), string(b[4:8])
		if size < 8 || size > len(b) {
			break
		}
		if typ == "mvhd" {
			return header(b[8:size])
		}
		b = b[size:]
	}
	return 0, errNoHeader
}

func header(b []byte) (time.Duration, error) {
	var scale, dur uint64
	switch {
	case len(b) >= 20 && b[0] == 0:
		scale, dur = uint64(binary.BigEndian.Uint32(b[12:])), uint64(binary.BigEndian.Uint32(b[16:]))
	case len(b) >= 32 && b[0] == 1:
		scale, dur = uint64(binary.BigEndian.Uint32(b[20:])), binary.BigEndian.Uint64(b[24:])
	default:
		return 0, errNoHeader
	}
	if scale == 0 {
		return 0, errNoHeader
	}
	return time.Duration(float64(dur) / float64(scale) * float64(time.Second)), nil
}
