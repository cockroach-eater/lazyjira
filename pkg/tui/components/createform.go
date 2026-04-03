package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/textfuel/lazyjira/pkg/tui/theme"
)

const (
	keyEsc    = "esc"
	keyEnter  = "enter"
	keyDown   = "down"
	keyCtrlD  = "ctrl+d"
	keyCtrlU  = "ctrl+u"
)

// CreatePanel identifies which sub-panel of the create form is focused
type CreatePanel int

const (
	CreatePanelSummary     CreatePanel = iota
	CreatePanelDescription
	CreatePanelFields
	createPanelCount = 3
)

// CreateFormField holds one field in the create issue form
type CreateFormField struct {
	Name          string
	FieldID       string
	Type          int
	Value         any
	DisplayValue  string
	Required      bool
	AllowedValues []ModalItem
	HasError      bool
	SchemaItems   string
}

const (
	CFFieldSingleSelect = iota
	CFFieldMultiSelect
	CFFieldPerson
	CFFieldSingleText
	CFFieldMultiText
)

type CreateFormTypeSelectedMsg struct {
	TypeID   string
	TypeName string
}
type CreateFormEditTextMsg struct{ FieldIndex int }
type CreateFormEditExternalMsg struct{ FieldIndex int }
type CreateFormPickerMsg struct {
	FieldIndex int
	Items      []ModalItem
}
type CreateFormChecklistMsg struct {
	FieldIndex int
	Items      []ModalItem
}
type CreateFormSubmitMsg struct{ Fields map[string]any }
type CreateFormCancelMsg struct{}

// DescRenderFunc renders description text to styled terminal lines for preview
type DescRenderFunc func(text string, width int) []string

// DescADFRenderFunc renders raw ADF data to styled terminal lines for preview
type DescADFRenderFunc func(adf any, width int) []string

// CreateForm is a 3-panel accordion overlay for issue creation
type CreateForm struct {
	visible bool
	width   int
	height  int

	issueTypeName string
	projectKey    string
	focusedPanel  CreatePanel

	allFields []CreateFormField

	summaryText   []rune
	summaryCursor int
	summaryIdx    int

	descIdx         int
	descOffset      int
	descRenderer    DescRenderFunc
	descADFRenderer DescADFRenderFunc

	fieldIndices  []int
	fieldCursor   int
	fieldOffset   int
	fieldDblClick DblClickDetector

	paused      bool
	errorMsg    string
	loading     bool
	filterInput TextInput
	filtering   bool
}

func NewCreateForm() CreateForm {
	return CreateForm{summaryIdx: -1, descIdx: -1}
}

// Pause stops intercepting keys so sub-overlays can receive input
func (f *CreateForm) Pause() { f.paused = true }

// Resume resumes key interception after sub-overlay closes
func (f *CreateForm) Resume() { f.paused = false }

// FocusedPanel returns which sub-panel is currently focused
func (f *CreateForm) FocusedPanel() CreatePanel { return f.focusedPanel }

// IsFiltering returns true when the fields filter input is active
func (f *CreateForm) IsFiltering() bool { return f.filtering }

// FilterQuery returns the current filter text
func (f *CreateForm) FilterQuery() string { return f.filterInput.Value() }

// FilterBarView renders the filter bar with cursor positioning
func (f *CreateForm) FilterBarView() string { return RenderFilterBarInput(&f.filterInput) }

// SetDescRenderer sets an optional rich renderer for description preview
func (f *CreateForm) SetDescRenderer(r DescRenderFunc) { f.descRenderer = r }

// SetDescADFRenderer sets an optional renderer for raw ADF description values
func (f *CreateForm) SetDescADFRenderer(r DescADFRenderFunc) { f.descADFRenderer = r }


func (f *CreateForm) ShowForm(fields []CreateFormField, issueTypeName, projectKey string) {
	f.visible = true
	f.issueTypeName = issueTypeName
	f.projectKey = projectKey
	f.allFields = fields
	f.summaryIdx = -1
	f.descIdx = -1
	f.fieldIndices = nil
	f.fieldCursor = 0
	f.fieldOffset = 0
	f.descOffset = 0
	f.errorMsg = ""
	f.loading = false
	f.filterInput.SetValue("")
	f.filtering = false
	f.focusedPanel = CreatePanelSummary

	for i, fld := range fields {
		switch fld.FieldID {
		case "summary":
			f.summaryIdx = i
			f.summaryText = []rune(fld.DisplayValue)
			f.summaryCursor = len(f.summaryText)
		case "description":
			f.descIdx = i
		default:
			f.fieldIndices = append(f.fieldIndices, i)
		}
	}
}

func (f *CreateForm) Hide() {
	f.visible = false
	f.allFields = nil
	f.fieldIndices = nil
	f.summaryText = nil
	f.errorMsg = ""
	f.loading = false
	f.filterInput.SetValue("")
	f.filtering = false
}

func (f *CreateForm) SetFieldValue(index int, value any, display string) {
	if index < 0 || index >= len(f.allFields) {
		return
	}
	f.allFields[index].Value = value
	if display == "" && !f.allFields[index].Required {
		display = "None"
	}
	f.allFields[index].DisplayValue = display
	f.allFields[index].HasError = false

	if index == f.summaryIdx {
		f.summaryText = []rune(display)
		f.summaryCursor = len(f.summaryText)
	}
}

func (f *CreateForm) SetError(msg string) {
	f.errorMsg = msg
	f.loading = false
}

func (f *CreateForm) SetLoading(loading bool) {
	f.loading = loading
	if loading {
		f.visible = true
	}
}

func (f *CreateForm) FieldAt(index int) *CreateFormField {
	if index < 0 || index >= len(f.allFields) {
		return nil
	}
	return &f.allFields[index]
}

func (f *CreateForm) IsVisible() bool { return f.visible }

func (f *CreateForm) SetSize(w, h int) {
	f.width = w
	f.height = h
}

// Intercept handles keyboard and mouse input for the 3-panel form
func (f *CreateForm) Intercept(msg tea.Msg) (tea.Cmd, bool) {
	if !f.visible || f.paused {
		return nil, false
	}

	if mm, isMouse := msg.(tea.MouseMsg); isMouse {
		return f.interceptMouse(mm)
	}

	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, false
	}
	if f.loading {
		if km.String() == keyEsc {
			f.Hide()
			return func() tea.Msg { return CreateFormCancelMsg{} }, true
		}
		return nil, true
	}
	if f.filtering {
		return f.interceptFilter(km)
	}

	switch km.Type { //nolint:exhaustive
	case tea.KeyTab:
		f.focusedPanel = CreatePanel((int(f.focusedPanel) + 1) % createPanelCount)
		return nil, true
	case tea.KeyShiftTab:
		f.focusedPanel = CreatePanel((int(f.focusedPanel) + createPanelCount - 1) % createPanelCount)
		return nil, true
	}

	switch f.focusedPanel {
	case CreatePanelSummary:
		return f.interceptSummary(km)
	case CreatePanelDescription:
		return f.interceptDescription(km)
	case CreatePanelFields:
		return f.interceptFields(km)
	}
	return nil, true
}

func (f *CreateForm) interceptMouse(mm tea.MouseMsg) (tea.Cmd, bool) {
	switch mm.Button { //nolint:exhaustive
	case tea.MouseButtonWheelUp:
		f.scrollFocused(-3)
		return nil, true
	case tea.MouseButtonWheelDown:
		f.scrollFocused(3)
		return nil, true
	case tea.MouseButtonLeft:
		if mm.Action != tea.MouseActionPress {
			return nil, true
		}
	default:
		return nil, true
	}

	availH := f.height - 1
	if f.errorMsg != "" {
		availH--
	}
	summaryH, descH, fieldsH := f.layoutSubPanels(availH)
	formW := min(max(f.width*6/10, 40), f.width-2)
	totalH := summaryH + descH + fieldsH
	if f.errorMsg != "" {
		totalH++
	}
	formX := (f.width - formW) / 2
	formY := (f.height - totalH) / 2

	relX := mm.X - formX
	relY := mm.Y - formY

	if relX < 0 || relX >= formW || relY < 0 || relY >= totalH {
		return nil, true
	}

	switch {
	case relY < summaryH:
		f.focusedPanel = CreatePanelSummary
	case relY < summaryH+descH:
		f.focusedPanel = CreatePanelDescription
	case relY < summaryH+descH+fieldsH:
		f.focusedPanel = CreatePanelFields
		rowInPanel := relY - summaryH - descH - 1
		innerH := max(fieldsH-2, 1)
		if rowInPanel >= 0 && rowInPanel < innerH {
			filtered := f.filteredFields()
			idx := f.fieldOffset + rowInPanel
			if idx >= 0 && idx < len(filtered) {
				f.fieldCursor = idx
				if f.fieldDblClick.Click(idx) {
					return f.editCurrentField(filtered)
				}
			}
		}
	}

	return nil, true
}

// scrollFocused scrolls the currently focused panel by delta lines
func (f *CreateForm) scrollFocused(delta int) {
	switch f.focusedPanel {
	case CreatePanelSummary:
		// move cursor by delta chars so the view follows
		f.summaryCursor += delta * 10
		if f.summaryCursor < 0 {
			f.summaryCursor = 0
		}
		if f.summaryCursor > len(f.summaryText) {
			f.summaryCursor = len(f.summaryText)
		}
	case CreatePanelDescription:
		f.scrollDesc(delta)
	case CreatePanelFields:
		filtered := f.filteredFields()
		f.fieldCursor += delta
		if f.fieldCursor < 0 {
			f.fieldCursor = 0
		}
		if f.fieldCursor >= len(filtered) {
			f.fieldCursor = max(len(filtered)-1, 0)
		}
		f.ensureFieldVisible()
	}
}

func (f *CreateForm) scrollDesc(delta int) {
	totalLines := f.descLineCount()
	innerH := f.descInnerH()
	maxOff := max(totalLines-innerH, 0)
	f.descOffset += delta
	if f.descOffset < 0 {
		f.descOffset = 0
	}
	if f.descOffset > maxOff {
		f.descOffset = maxOff
	}
}

func (f *CreateForm) descLineCount() int {
	text := ""
	if f.descIdx >= 0 {
		text = f.allFields[f.descIdx].DisplayValue
	}
	formW := min(max(f.width*6/10, 40), f.width-2)
	innerW := max(formW-2, 1)
	return len(f.renderDescLines(text, innerW))
}

// renderDescLines converts description text to display lines using the rich
// renderer if set, falling back to plain text wrapping
func (f *CreateForm) renderDescLines(text string, innerW int) []string {
	if text == "" {
		return []string{""}
	}
	// try ADF renderer with raw Value if available and not a plain string
	if f.descADFRenderer != nil && f.descIdx >= 0 {
		if val := f.allFields[f.descIdx].Value; val != nil {
			if _, isStr := val.(string); !isStr {
				if lines := f.descADFRenderer(val, innerW-1); len(lines) > 0 {
					result := make([]string, len(lines))
					for i, l := range lines {
						result[i] = " " + l
					}
					return result
				}
			}
		}
	}
	if f.descRenderer != nil {
		if lines := f.descRenderer(text, innerW-1); len(lines) > 0 {
			// add leading space to each line
			result := make([]string, len(lines))
			for i, l := range lines {
				result[i] = " " + l
			}
			return result
		}
	}
	// fallback: plain text wrapping
	var lines []string
	for _, l := range wrapTextLines(text, innerW-1) {
		lines = append(lines, " "+l)
	}
	return lines
}

func (f *CreateForm) descInnerH() int {
	availH := f.height - 1
	if f.errorMsg != "" {
		availH--
	}
	_, descH, _ := f.layoutSubPanels(availH)
	return max(descH-2, 1)
}

func (f *CreateForm) interceptFilter(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case keyEsc:
		f.filtering = false
		f.filterInput.SetValue("")
		f.fieldCursor = 0
		f.fieldOffset = 0
	case keyEnter:
		f.confirmFilter()
	case keyDown, KeyCtrlJ:
		filtered := f.filteredFields()
		if f.fieldCursor < len(filtered)-1 {
			f.fieldCursor++
			f.ensureFieldVisible()
		}
	case "up", KeyCtrlK:
		if f.fieldCursor > 0 {
			f.fieldCursor--
			f.ensureFieldVisible()
		}
	default:
		updated, changed := f.filterInput.Update(msg)
		f.filterInput = updated
		if changed {
			f.fieldCursor = 0
			f.fieldOffset = 0
		}
	}
	return nil, true
}

func (f *CreateForm) interceptSummary(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.Type { //nolint:exhaustive
	case tea.KeyEnter:
		return f.submitForm()
	case tea.KeyEsc:
		f.Hide()
		return func() tea.Msg { return CreateFormCancelMsg{} }, true
	case tea.KeyBackspace:
		if f.summaryCursor > 0 {
			f.summaryText = append(f.summaryText[:f.summaryCursor-1], f.summaryText[f.summaryCursor:]...)
			f.summaryCursor--
		}
	case tea.KeyDelete:
		if f.summaryCursor < len(f.summaryText) {
			f.summaryText = append(f.summaryText[:f.summaryCursor], f.summaryText[f.summaryCursor+1:]...)
		}
	case tea.KeyLeft:
		if f.summaryCursor > 0 {
			f.summaryCursor--
		}
	case tea.KeyRight:
		if f.summaryCursor < len(f.summaryText) {
			f.summaryCursor++
		}
	case tea.KeyHome, tea.KeyCtrlA:
		f.summaryCursor = 0
	case tea.KeyEnd, tea.KeyCtrlE:
		f.summaryCursor = len(f.summaryText)
	case tea.KeyCtrlU:
		f.summaryText = f.summaryText[f.summaryCursor:]
		f.summaryCursor = 0
	case tea.KeyCtrlK:
		f.summaryText = f.summaryText[:f.summaryCursor]
	case tea.KeySpace:
		f.insertSummaryRunes([]rune{' '})
	case tea.KeyRunes:
		f.insertSummaryRunes(msg.Runes)
	}
	return nil, true
}

func (f *CreateForm) insertSummaryRunes(runes []rune) {
	newText := make([]rune, 0, len(f.summaryText)+len(runes))
	newText = append(newText, f.summaryText[:f.summaryCursor]...)
	newText = append(newText, runes...)
	newText = append(newText, f.summaryText[f.summaryCursor:]...)
	f.summaryText = newText
	f.summaryCursor += len(runes)
}

func (f *CreateForm) interceptDescription(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case keyEnter:
		return f.submitForm()
	case keyEsc, "q":
		f.Hide()
		return func() tea.Msg { return CreateFormCancelMsg{} }, true
	case "e":
		if f.descIdx >= 0 {
			idx := f.descIdx
			return func() tea.Msg { return CreateFormEditExternalMsg{FieldIndex: idx} }, true
		}
	case "j", keyDown, KeyCtrlJ:
		f.scrollDesc(1)
	case "k", "up", KeyCtrlK:
		f.scrollDesc(-1)
	case "g":
		f.descOffset = 0
	case "G":
		f.scrollDesc(f.descLineCount())
	case keyCtrlD:
		f.scrollDesc(f.descInnerH() / 2)
	case keyCtrlU:
		f.scrollDesc(-f.descInnerH() / 2)
	}
	return nil, true
}

func (f *CreateForm) interceptFields(msg tea.KeyMsg) (tea.Cmd, bool) {
	filtered := f.filteredFields()
	switch msg.String() {
	case "j", keyDown, KeyCtrlJ:
		if f.fieldCursor < len(filtered)-1 {
			f.fieldCursor++
			f.ensureFieldVisible()
		}
	case "k", "up", KeyCtrlK:
		if f.fieldCursor > 0 {
			f.fieldCursor--
			f.ensureFieldVisible()
		}
	case "g":
		f.fieldCursor = 0
		f.fieldOffset = 0
	case "G":
		if len(filtered) > 0 {
			f.fieldCursor = len(filtered) - 1
			f.ensureFieldVisible()
		}
	case keyCtrlD:
		half := f.fieldsInnerH() / 2
		f.fieldCursor = min(f.fieldCursor+half, max(len(filtered)-1, 0))
		f.ensureFieldVisible()
	case keyCtrlU:
		half := f.fieldsInnerH() / 2
		f.fieldCursor = max(f.fieldCursor-half, 0)
		f.ensureFieldVisible()
	case "/":
		f.filtering = true
		f.filterInput.SetValue("")
	case "e", " ":
		return f.editCurrentField(filtered)
	case keyEnter:
		return f.submitForm()
	case keyEsc, "q":
		f.Hide()
		return func() tea.Msg { return CreateFormCancelMsg{} }, true
	}
	return nil, true
}

func (f *CreateForm) ensureFieldVisible() {
	vh := f.fieldsInnerH()
	if f.fieldCursor < f.fieldOffset {
		f.fieldOffset = f.fieldCursor
	}
	if f.fieldCursor >= f.fieldOffset+vh {
		f.fieldOffset = f.fieldCursor - vh + 1
	}
}

func (f *CreateForm) fieldsInnerH() int {
	availH := f.height - 1
	if f.errorMsg != "" {
		availH--
	}
	_, _, fieldsH := f.layoutSubPanels(availH)
	return max(fieldsH-2, 1)
}

func (f *CreateForm) filteredFields() []int {
	if f.filterInput.Value() == "" {
		return f.fieldIndices
	}
	lower := strings.ToLower(f.filterInput.Value())
	var indices []int
	for _, idx := range f.fieldIndices {
		fld := f.allFields[idx]
		if strings.Contains(strings.ToLower(fld.Name), lower) ||
			strings.Contains(strings.ToLower(fld.DisplayValue), lower) {
			indices = append(indices, idx)
		}
	}
	return indices
}

// confirmFilter restores full field list and places cursor on the matched field
func (f *CreateForm) confirmFilter() {
	filtered := f.filteredFields()
	var matchedIdx int
	if f.fieldCursor >= 0 && f.fieldCursor < len(filtered) {
		matchedIdx = filtered[f.fieldCursor]
	}
	f.filtering = false
	f.filterInput.SetValue("")
	f.fieldCursor = 0
	for i, idx := range f.fieldIndices {
		if idx == matchedIdx {
			f.fieldCursor = i
			break
		}
	}
	f.fieldOffset = 0
	f.ensureFieldVisible()
}

func (f *CreateForm) editCurrentField(filtered []int) (tea.Cmd, bool) {
	if f.fieldCursor < 0 || f.fieldCursor >= len(filtered) {
		return nil, true
	}
	idx := filtered[f.fieldCursor]
	field := f.allFields[idx]

	switch field.Type {
	case CFFieldSingleText:
		return func() tea.Msg { return CreateFormEditTextMsg{FieldIndex: idx} }, true
	case CFFieldMultiText:
		return func() tea.Msg { return CreateFormEditExternalMsg{FieldIndex: idx} }, true
	case CFFieldSingleSelect, CFFieldPerson:
		if len(field.AllowedValues) > 0 {
			return func() tea.Msg {
				return CreateFormPickerMsg{FieldIndex: idx, Items: field.AllowedValues}
			}, true
		}
		return func() tea.Msg {
			return CreateFormPickerMsg{FieldIndex: idx, Items: nil}
		}, true
	case CFFieldMultiSelect:
		return func() tea.Msg {
			return CreateFormChecklistMsg{FieldIndex: idx, Items: field.AllowedValues}
		}, true
	}
	return nil, true
}

func (f *CreateForm) submitForm() (tea.Cmd, bool) {
	// sync summary text back to allFields
	if f.summaryIdx >= 0 {
		text := strings.TrimSpace(string(f.summaryText))
		f.allFields[f.summaryIdx].Value = text
		f.allFields[f.summaryIdx].DisplayValue = text
	}

	// validate required fields
	hasErrors := false
	for i := range f.allFields {
		if f.allFields[i].Required && f.allFields[i].DisplayValue == "" && f.allFields[i].Value == nil {
			f.allFields[i].HasError = true
			hasErrors = true
		}
	}
	if hasErrors {
		count := 0
		for _, fld := range f.allFields {
			if fld.HasError {
				count++
			}
		}
		f.errorMsg = fmt.Sprintf("%d required field(s) empty", count)
		return nil, true
	}

	fieldsMap := make(map[string]any)
	for _, fld := range f.allFields {
		if fld.Value != nil {
			fieldsMap[fld.FieldID] = fld.Value
		}
	}
	return func() tea.Msg { return CreateFormSubmitMsg{Fields: fieldsMap} }, true
}

// Layout

const panelMinH = 3 // 1 content line + 2 borders

func (f *CreateForm) layoutSubPanels(availH int) (summaryH, descH, fieldsH int) {
	formW := min(max(f.width*6/10, 40), f.width-2)
	innerW := max(formW-2, 1)

	// summary always sized to its wrapped content
	summaryLines := f.summaryWrapCount(innerW)
	summaryH = max(summaryLines+2, panelMinH)

	fieldCount := len(f.filteredFields())
	fieldsNat := max(fieldCount+2, panelMinH)

	descLines := 1
	if f.descIdx >= 0 && f.allFields[f.descIdx].DisplayValue != "" {
		descLines = strings.Count(f.allFields[f.descIdx].DisplayValue, "\n") + 1
	}
	descNat := max(descLines+2, panelMinH)

	// cap summary so desc+fields get at least panelMinH each
	if summaryH > availH-2*panelMinH {
		summaryH = max(availH-2*panelMinH, panelMinH)
	}
	remaining := availH - summaryH

	switch f.focusedPanel {
	case CreatePanelFields:
		// fields gets priority, desc gets leftovers
		fieldsH = min(fieldsNat, max(remaining-panelMinH, panelMinH))
		descH = min(descNat, max(remaining-fieldsH, panelMinH))
	default:
		// desc gets priority, fields gets leftovers
		descH = min(descNat, max(remaining-panelMinH, panelMinH))
		fieldsH = min(fieldsNat, max(remaining-descH, panelMinH))
	}

	return summaryH, descH, fieldsH
}

// summaryWrapCount returns how many display lines the summary text wraps to
func (f *CreateForm) summaryWrapCount(innerW int) int {
	if len(f.summaryText) == 0 || innerW <= 0 {
		return 1
	}
	// account for leading space
	allRunes := append([]rune{' '}, f.summaryText...)
	count := 1
	w := 0
	for _, r := range allRunes {
		rw := lipgloss.Width(string(r))
		if w+rw > innerW {
			count++
			w = rw
		} else {
			w += rw
		}
	}
	// +1 for cursor at end of full line
	if w >= innerW {
		count++
	}
	return count
}

// Render

func (f *CreateForm) Render(bg string, w, h int) string {
	if !f.visible {
		return bg
	}
	if f.loading && len(f.allFields) == 0 {
		popup := RenderPanelFull("Create issue", "", "\n  Loading...\n", 40, 3, true, nil)
		return Overlay(bg, popup, w, h)
	}
	return f.renderForm(bg, w, h)
}

func (f *CreateForm) renderForm(bg string, w, h int) string {
	formW := min(max(w*6/10, 40), w-2)
	availH := h - 1 // leave 1 line for help bar
	if f.errorMsg != "" {
		availH-- // reserve 1 line for error
	}

	summaryH, descH, fieldsH := f.layoutSubPanels(availH)

	summaryPanel := f.renderSummary(formW, summaryH)
	descPanel := f.renderDescription(formW, descH)
	fieldsPanel := f.renderFields(formW, fieldsH)

	combined := lipgloss.JoinVertical(lipgloss.Left, summaryPanel, descPanel, fieldsPanel)

	if f.errorMsg != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
		errLine := errStyle.Render(" " + f.errorMsg)
		if lw := lipgloss.Width(errLine); lw < formW {
			errLine += strings.Repeat(" ", formW-lw)
		}
		combined = lipgloss.JoinVertical(lipgloss.Left, combined, errLine)
	}

	totalH := summaryH + descH + fieldsH
	if f.errorMsg != "" {
		totalH++
	}
	x := (w - formW) / 2
	y := (h - totalH) / 2
	return OverlayAt(bg, combined, x, y, w, h)
}

func (f *CreateForm) renderSummary(formW, panelH int) string {
	focused := f.focusedPanel == CreatePanelSummary
	innerW := max(formW-2, 1)
	innerH := max(panelH-2, 1)

	title := "Summary"
	if f.summaryIdx >= 0 && f.allFields[f.summaryIdx].Required {
		title = "*Summary"
	}

	if focused {
		content := f.renderSummaryWithCursor(innerW, innerH)
		return RenderPanelFull(title, "", content, formW, innerH, true, nil)
	}

	// not focused: render same layout as cursor mode but without cursor
	content := f.renderSummaryPlain(innerW, innerH)
	return RenderPanelFull(title, "", content, formW, innerH, false, nil)
}

func (f *CreateForm) renderSummaryWithCursor(innerW, innerH int) string {
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	allRunes := append([]rune{' '}, f.summaryText...)
	cursorPos := f.summaryCursor + 1 // +1 for leading space

	// wrap runes into display lines
	type wLine struct {
		runes []rune
		start int
	}
	var wrapped []wLine
	off := 0
	for off < len(allRunes) {
		cut := 0
		w := 0
		for i := off; i < len(allRunes); i++ {
			rw := lipgloss.Width(string(allRunes[i]))
			if w+rw > innerW {
				break
			}
			w += rw
			cut = i + 1
		}
		if cut <= off {
			cut = off + 1
		}
		wrapped = append(wrapped, wLine{runes: allRunes[off:cut], start: off})
		off = cut
	}
	if len(wrapped) == 0 {
		wrapped = append(wrapped, wLine{})
	}

	// find which line the cursor is on and auto-scroll to keep it visible
	cursorLine := 0
	for li, wl := range wrapped {
		lineEnd := wl.start + len(wl.runes)
		if cursorPos < lineEnd || (cursorPos == lineEnd && cursorPos >= len(allRunes)) {
			cursorLine = li
			break
		}
	}
	viewStart := 0
	if cursorLine >= innerH {
		viewStart = cursorLine - innerH + 1
	}

	var lines []string
	cursorPlaced := false
	for li := viewStart; li < len(wrapped) && len(lines) < innerH; li++ {
		wl := wrapped[li]
		lineEnd := wl.start + len(wl.runes)
		lineW := lipgloss.Width(string(wl.runes))

		if !cursorPlaced && (cursorPos < lineEnd || (cursorPos == lineEnd && cursorPos >= len(allRunes))) {
			col := cursorPos - wl.start
			switch {
			case col >= len(wl.runes) && lineW >= innerW:
				// cursor past end of a full line: put cursor on next line
				lines = append(lines, string(wl.runes))
				if len(lines) < innerH {
					lines = append(lines, cursorStyle.Render("█")+strings.Repeat(" ", max(innerW-1, 0)))
				}
			case col >= len(wl.runes):
				// cursor past end of a short line: append cursor block
				rendered := string(wl.runes) + cursorStyle.Render("█")
				lines = append(lines, rendered)
			default:
				before := string(wl.runes[:col])
				at := string(wl.runes[col : col+1])
				after := string(wl.runes[col+1:])
				lines = append(lines, before+cursorStyle.Render(at)+after)
			}
			cursorPlaced = true
		} else {
			lines = append(lines, string(wl.runes))
		}
	}

	return strings.Join(lines, "\n")
}

// renderSummaryPlain uses the same wrapping as cursor mode but without cursor styling
func (f *CreateForm) renderSummaryPlain(innerW, innerH int) string {
	allRunes := append([]rune{' '}, f.summaryText...)
	if len(allRunes) <= 1 {
		return ""
	}

	var lines []string
	off := 0
	for off < len(allRunes) {
		cut := 0
		w := 0
		for i := off; i < len(allRunes); i++ {
			rw := lipgloss.Width(string(allRunes[i]))
			if w+rw > innerW {
				break
			}
			w += rw
			cut = i + 1
		}
		if cut <= off {
			cut = off + 1
		}
		lines = append(lines, string(allRunes[off:cut]))
		off = cut
	}

	if len(lines) > innerH {
		lines = lines[:innerH]
	}
	return strings.Join(lines, "\n")
}

func (f *CreateForm) renderDescription(formW, panelH int) string {
	focused := f.focusedPanel == CreatePanelDescription
	innerW := max(formW-2, 1)
	innerH := max(panelH-2, 1)

	text := ""
	if f.descIdx >= 0 {
		text = f.allFields[f.descIdx].DisplayValue
	}

	allLines := f.renderDescLines(text, innerW)

	// clamp scroll offset
	maxOff := max(len(allLines)-innerH, 0)
	if f.descOffset > maxOff {
		f.descOffset = maxOff
	}
	if f.descOffset < 0 {
		f.descOffset = 0
	}

	// apply scroll
	end := min(f.descOffset+innerH, len(allLines))
	visible := allLines[f.descOffset:end]
	for len(visible) < innerH {
		visible = append(visible, "")
	}

	content := strings.Join(visible, "\n")

	var scroll *ScrollInfo
	if len(allLines) > innerH {
		scroll = &ScrollInfo{
			Total:   len(allLines),
			Visible: innerH,
			Offset:  f.descOffset,
		}
	}

	return RenderPanelFull("Description", "", content, formW, innerH, focused, scroll)
}

func (f *CreateForm) renderFields(formW, panelH int) string {
	focused := f.focusedPanel == CreatePanelFields
	innerW := max(formW-2, 1)
	innerH := max(panelH-2, 1)

	filtered := f.filteredFields()

	// clamp cursor
	if f.fieldCursor >= len(filtered) {
		f.fieldCursor = max(len(filtered)-1, 0)
	}

	// adjust scroll offset
	if f.fieldCursor < f.fieldOffset {
		f.fieldOffset = f.fieldCursor
	}
	if f.fieldCursor >= f.fieldOffset+innerH {
		f.fieldOffset = f.fieldCursor - innerH + 1
	}
	maxOffset := max(len(filtered)-innerH, 0)
	if f.fieldOffset > maxOffset {
		f.fieldOffset = maxOffset
	}

	selStyle := lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	reqMark := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true).Render("*")

	// label column = longest field name + 1 space gap (+ 1 for req mark / leading space)
	labelW := 0
	for _, idx := range filtered {
		w := lipgloss.Width(f.allFields[idx].Name)
		if w > labelW {
			labelW = w
		}
	}
	labelW += 2 // leading marker + trailing space

	// cap label column to half the inner width so values always have room
	maxLabelW := innerW / 2
	if labelW > maxLabelW {
		labelW = maxLabelW
	}

	end := min(f.fieldOffset+innerH, len(filtered))
	var lines []string
	for ci := f.fieldOffset; ci < end; ci++ {
		idx := filtered[ci]
		fld := f.allFields[idx]

		label := fld.Name
		if fld.Required {
			label = reqMark + label
		} else {
			label = " " + label
		}
		// truncate long labels
		if lipgloss.Width(label) > labelW {
			label = TruncateEnd(label, labelW)
		}
		for lipgloss.Width(label) < labelW {
			label += " "
		}

		val := fld.DisplayValue
		maxVal := innerW - labelW - 1
		if maxVal > 0 && lipgloss.Width(val) > maxVal {
			val = TruncateEnd(val, maxVal)
		}

		var line string
		if focused && ci == f.fieldCursor {
			plain := " " + label + val
			for lipgloss.Width(plain) < innerW {
				plain += " "
			}
			line = selStyle.Render(plain)
		} else {
			if val != "" {
				val = styleFieldValue(fld, val)
			}
			line = " " + label + val
			if fld.HasError {
				line = " " + errStyle.Render(label) + val
			}
		}

		lines = append(lines, line)
	}

	for len(lines) < innerH {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")

	footer := ""
	if len(filtered) > 0 {
		footer = fmt.Sprintf("%d of %d", f.fieldCursor+1, len(filtered))
	}

	var scroll *ScrollInfo
	if len(filtered) > innerH {
		scroll = &ScrollInfo{
			Total:   len(filtered),
			Visible: innerH,
			Offset:  f.fieldOffset,
		}
	}

	title := "Fields"
	return RenderPanelFull(title, footer, content, formW, innerH, focused, scroll)
}

var noneStyle = lipgloss.NewStyle().Foreground(theme.ColorGray)

func styleFieldValue(fld CreateFormField, val string) string {
	if val == "None" {
		return noneStyle.Render(val)
	}
	switch fld.FieldID {
	case "priority":
		return theme.PriorityStyled(val)
	default:
		if fld.Type == CFFieldPerson || fld.SchemaItems == "user" {
			return theme.AuthorRender(val)
		}
		return val
	}
}

// wrapTextLines wraps text to fit within maxWidth display columns
func wrapTextLines(s string, maxWidth int) []string {
	if s == "" {
		return []string{""}
	}
	if maxWidth <= 0 {
		return []string{s}
	}
	var lines []string
	for _, rawLine := range strings.Split(s, "\n") {
		if rawLine == "" {
			lines = append(lines, "")
			continue
		}
		runes := []rune(rawLine)
		off := 0
		for off < len(runes) {
			w := 0
			cut := off
			for i := off; i < len(runes); i++ {
				rw := lipgloss.Width(string(runes[i]))
				if w+rw > maxWidth {
					break
				}
				w += rw
				cut = i + 1
			}
			if cut <= off {
				cut = off + 1
			}
			lines = append(lines, string(runes[off:cut]))
			off = cut
		}
	}
	return lines
}
