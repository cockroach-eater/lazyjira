package components

const (
	KeyCtrlJ = "ctrl+j"
	KeyCtrlK = "ctrl+k"
)

// ListBase provides shared cursor, offset, and scroll logic for list panels.
// Embed it in a view struct and call SetItemCount when the data changes.
type ListBase struct {
	Cursor    int
	Offset    int
	Width     int
	Height    int
	Focused   bool
	itemCount int
	dblClick  DblClickDetector
}

func (l *ListBase) SetSize(w, h int)       { l.Width = w; l.Height = h }
func (l *ListBase) SetFocused(focused bool) { l.Focused = focused }
func (l *ListBase) SetItemCount(n int) {
	l.itemCount = n
	if l.Cursor >= n {
		l.Cursor = n - 1
	}
	if l.Cursor < 0 {
		l.Cursor = 0
	}
	l.AdjustOffset()
}

func (l *ListBase) ItemCount() int { return l.itemCount }

func (l *ListBase) VisibleRows() int {
	return max(l.Height-2, 1) // minus top + bottom border
}

// ContentHeight returns natural height: items + 2 borders, with a minimum.
func (l *ListBase) ContentHeight(minH int) int {
	return max(l.itemCount+2, minH)
}

func (l *ListBase) AdjustOffset() {
	l.Offset = AdjustOffset(l.Cursor, l.Offset, l.VisibleRows(), l.itemCount)
}

// ScrollBy moves the cursor by delta, clamping to valid range.
func (l *ListBase) ScrollBy(delta int) {
	l.Cursor += delta
	l.clampCursor()
	l.AdjustOffset()
}

// ClickAt selects an item by relative Y coordinate (relY=0 is top border).
// Returns true on double-click (same item clicked twice within 500ms).
func (l *ListBase) ClickAt(relY int) bool {
	idx := l.Offset + relY - 1 // -1 for top border
	if idx >= 0 && idx < l.itemCount {
		l.Cursor = idx
		l.AdjustOffset()
		return l.dblClick.Click(idx)
	}
	return false
}

func (l *ListBase) KeyNav(key string) bool {
	prev := l.Cursor
	switch key {
	case "j", "down", KeyCtrlJ:
		if l.Cursor < l.itemCount-1 {
			l.Cursor++
		} else if l.itemCount > 0 {
			l.Cursor = 0
		}
	case "k", "up", KeyCtrlK:
		if l.Cursor > 0 {
			l.Cursor--
		} else if l.itemCount > 0 {
			l.Cursor = l.itemCount - 1
		}
	case "g", "home":
		l.Cursor = 0
	case "G", "end":
		if l.itemCount > 0 {
			l.Cursor = l.itemCount - 1
		}
	case "ctrl+d":
		l.Cursor += l.VisibleRows() / 2
		l.clampCursor()
	case "ctrl+u":
		l.Cursor -= l.VisibleRows() / 2
		if l.Cursor < 0 {
			l.Cursor = 0
		}
	default:
		return false
	}
	l.AdjustOffset()
	return l.Cursor != prev
}

func IsNavKey(key string) bool {
	switch key {
	case "j", "down", KeyCtrlJ, "k", "up", KeyCtrlK,
		"g", "home", "G", "end", "ctrl+d", "ctrl+u":
		return true
	}
	return false
}

func (l *ListBase) clampCursor() {
	if l.Cursor >= l.itemCount {
		l.Cursor = l.itemCount - 1
	}
	if l.Cursor < 0 {
		l.Cursor = 0
	}
}
