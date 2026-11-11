package app

// The window's zoom, as a browser's: the view lays out in a window 1/z its
// size and draws at the screen's resolution, so text and edges stay sharp.
const (
	zoomMin = 0.5
	zoomMax = 3
)

func clampZoom(z float32) float32 {
	if z < zoomMin {
		return zoomMin
	}
	if z > zoomMax {
		return zoomMax
	}
	return z
}

// zoomStep is how much one press changes the zoom: a tenth up to 2, then
// quarters, as native apps step it.
func zoomStep(z float32) float32 {
	if z >= 2 {
		return 0.25
	}
	return 0.1
}

func (a *app) setZoom(z float32) {
	a.zoom = clampZoom(z)
	a.prefs.Zoom = a.zoom
	a.prefs.save()
}
