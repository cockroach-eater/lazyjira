package components

const (
	KeyCtrlJ = "ctrl+j"
	KeyCtrlK = "ctrl+k"
)

type NavAction int

const (
	NavNone     NavAction = iota
	NavDown
	NavUp
	NavTop
	NavBottom
	NavHalfDown
	NavHalfUp
)

type NavResolver func(key string) NavAction

type ListBase struct {
	Cursor      int
	Offset      int
	Width       int
	Height      int
	Focused     bool
	ResolveNav  NavResolver
	itemCount   int
	dblClick    DblClickDetector
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
	return max(l.Height-2, 1)
}

func (l *ListBase) ContentHeight(minH int) int {
	return max(l.itemCount+2, minH)
}

func (l *ListBase) AdjustOffset() {
	l.Offset = AdjustOffset(l.Cursor, l.Offset, l.VisibleRows(), l.itemCount)
}

func (l *ListBase) ScrollBy(delta int) {
	l.Cursor += delta
	l.clampCursor()
	l.AdjustOffset()
}

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
	nav := NavNone
	if l.ResolveNav != nil {
		nav = l.ResolveNav(key)
	}
	if nav == NavNone {
		return false
	}
	prev := l.Cursor
	switch nav {
	case NavDown:
		if l.Cursor < l.itemCount-1 {
			l.Cursor++
		} else if l.itemCount > 0 {
			l.Cursor = 0
		}
	case NavUp:
		if l.Cursor > 0 {
			l.Cursor--
		} else if l.itemCount > 0 {
			l.Cursor = l.itemCount - 1
		}
	case NavTop:
		l.Cursor = 0
	case NavBottom:
		if l.itemCount > 0 {
			l.Cursor = l.itemCount - 1
		}
	case NavHalfDown:
		l.Cursor += l.VisibleRows() / 2
		l.clampCursor()
	case NavHalfUp:
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

func (l *ListBase) clampCursor() {
	if l.Cursor >= l.itemCount {
		l.Cursor = l.itemCount - 1
	}
	if l.Cursor < 0 {
		l.Cursor = 0
	}
}
