package main

import (
	"bytes"
	"image"
	"image/color"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/webp"
)

func isWebP(b []byte) bool {
	return len(b) > 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP"
}

// decodeWebP decodes a WebP picture as browsers show it. Lossy WebP is VP8,
// whose YCbCr is BT.601 limited range (Y in 16…235), but x/image/webp hands
// it back as image.YCbCr, which Go converts as full range JPEG: blacks come
// out gray and whites dim. Lossless pictures decode to RGBA already.
func decodeWebP(b []byte) (*ui.Bitmap, error) {
	img, err := webp.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	switch src := img.(type) {
	case *image.YCbCr:
		return ui.NewBitmap(limitedRange(src, nil)), nil
	case *image.NYCbCrA:
		return ui.NewBitmap(limitedRange(&src.YCbCr, src.A)), nil
	}
	return ui.NewBitmap(img), nil
}

func limitedRange(src *image.YCbCr, alpha []uint8) *image.NRGBA {
	r := src.Rect
	out := image.NewNRGBA(r)
	clamp := func(v int32) uint8 {
		v = (v + 1<<15) >> 16
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return uint8(v)
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			yi, ci := src.YOffset(x, y), src.COffset(x, y)
			yy := (int32(src.Y[yi]) - 16) * 76284 // 1.164 * 65536
			cb, cr := int32(src.Cb[ci])-128, int32(src.Cr[ci])-128
			c := color.NRGBA{
				R: clamp(yy + 104595*cr),           // 1.596
				G: clamp(yy - 25690*cb - 53281*cr), // 0.392, 0.813
				B: clamp(yy + 132186*cb),           // 2.017
				A: 255,
			}
			if alpha != nil {
				c.A = alpha[(y-r.Min.Y)*r.Dx()+(x-r.Min.X)]
			}
			out.SetNRGBA(x, y, c)
		}
	}
	return out
}
