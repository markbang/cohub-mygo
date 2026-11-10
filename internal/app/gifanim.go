package app

import (
	"image"
	"io"
	"time"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/draw"
)

// An animated GIF as an avatar plays it, as the web's <img> does: its
// frames drawn ahead at most gifAnimSide on a side, which its bitmap shows
// in turn. Every avatar of one GIF shows the same frame at a time.
type gifAnim struct {
	bmp    *ui.Bitmap
	frames []*image.RGBA
	ends   []time.Duration // when each frame ends, from the animation's start
	shown  int
}

const (
	gifAnimSide      = 128
	gifAnimMaxFrames = 240
)

var gifEpoch = time.Now()

// decodeGIFAnim draws the frames of an animated GIF, or returns nil for a
// GIF of one frame or one that doesn't decode.
func decodeGIFAnim(data []byte) *gifAnim {
	dec, err := ui.NewGIFDecoder(data)
	if err != nil {
		return nil
	}
	a := &gifAnim{}
	var end time.Duration
	for len(a.frames) < gifAnimMaxFrames {
		f, err := dec.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}
		b := f.Image.Bounds()
		w, h := b.Dx(), b.Dy()
		if s := max(w, h); s > gifAnimSide {
			w, h = max(1, w*gifAnimSide/s), max(1, h*gifAnimSide/s)
		}
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.ApproxBiLinear.Scale(img, img.Bounds(), f.Image, b, draw.Src, nil)
		d := f.Duration
		if d <= 0 {
			d = 100 * time.Millisecond
		}
		end += d
		a.frames, a.ends = append(a.frames, img), append(a.ends, end)
	}
	if len(a.frames) < 2 {
		return nil
	}
	a.bmp = ui.NewBitmap(a.frames[0])
	return a
}

// frame shows the frame due now, and asks for a frame when the next is.
func (a *gifAnim) frame(c *ui.Context) *ui.Bitmap {
	total := a.ends[len(a.ends)-1]
	at := time.Since(gifEpoch) % total
	i := 0
	for i < len(a.ends)-1 && a.ends[i] <= at {
		i++
	}
	if i != a.shown {
		a.bmp.Update(a.frames[i])
		a.shown = i
	}
	c.After(a.ends[i] - at)
	return a.bmp
}
