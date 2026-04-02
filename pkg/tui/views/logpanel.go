package views

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/textfuel/lazyjira/pkg/tui/components"
	"github.com/textfuel/lazyjira/pkg/tui/theme"
)

// LogEntry is a single log line
type LogEntry struct {
	Time    time.Time
	Method  string
	Path    string
	Status  int
	Elapsed time.Duration
}

// LogPanel shows API request logs under the main panel
type LogPanel struct {
	entries []LogEntry
	mu      sync.Mutex
	width   int
	height  int
	theme   *theme.Theme
}

func NewLogPanel() *LogPanel {
	return &LogPanel{
		theme: theme.Default,
	}
}

func (l *LogPanel) SetSize(w, h int) { l.width = w; l.height = h }

func (l *LogPanel) AddEntry(entry LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
	// Keep last 100.
	if len(l.entries) > 100 {
		l.entries = l.entries[len(l.entries)-100:]
	}
}

func (l *LogPanel) Init() tea.Cmd { return nil }

func (l *LogPanel) Update(msg tea.Msg) (*LogPanel, tea.Cmd) {
	return l, nil
}

func (l *LogPanel) View() string {
	contentWidth, innerHeight := components.PanelDimensions(l.width, l.height)

	l.mu.Lock()
	entries := l.entries
	l.mu.Unlock()

	var lines []string

	// Show last N entries that fit.
	start := max(len(entries)-innerHeight, 0)
	for _, e := range entries[start:] {
		statusStyle := l.theme.SuccessText
		if e.Status >= 400 {
			statusStyle = l.theme.ErrorText
		}
		line := fmt.Sprintf("%s %s %s %s %s",
			l.theme.Subtitle.Render(e.Time.Format("15:04:05")),
			l.theme.KeyStyle.Render(e.Method),
			l.theme.ValueStyle.Render(components.Truncate(e.Path, contentWidth-30)),
			statusStyle.Render(strconv.Itoa(e.Status)),
			l.theme.Subtitle.Render(e.Elapsed.Round(time.Millisecond).String()),
		)
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		lines = append(lines, l.theme.Subtitle.Render("No requests yet"))
	}

	// Pad
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	if len(lines) > innerHeight {
		lines = lines[len(lines)-innerHeight:]
	}

	content := strings.Join(lines, "\n")
	return components.RenderPanel("Command log", content, l.width, innerHeight, false)
}
