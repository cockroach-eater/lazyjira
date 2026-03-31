package components

import "time"

const dblClickThreshold = 500 * time.Millisecond

// DblClickDetector tracks clicks to detect double-clicks on the same item
type DblClickDetector struct {
	lastIdx  int
	lastTime time.Time
}

// Click registers a click on idx and returns true if it is a double-click
// (same index clicked twice within 500ms)
func (d *DblClickDetector) Click(idx int) bool {
	now := time.Now()
	dbl := idx == d.lastIdx && now.Sub(d.lastTime) < dblClickThreshold
	if dbl {
		d.lastTime = time.Time{} // reset to prevent triple-click
	} else {
		d.lastIdx = idx
		d.lastTime = now
	}
	return dbl
}
