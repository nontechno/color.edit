package main

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"color.edit/internal/sectionlist"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type incidentRecorder interface {
	Record(operation string, err error, stack []byte, context map[string]string) string
	Redact(value string) string
}

type editorApp struct {
	application *tview.Application
	document    *assignmentDocument
	reporter    incidentRecorder
	pages       *tview.Pages
	body        *tview.Flex
	options     *sectionlist.List
	preview     *declarativePreview
	status      *tview.TextView
	colors      []colorOption
	dirty       bool
	flickerSeq  atomic.Uint64
}

func newEditorApp(document *assignmentDocument, reporter incidentRecorder) (*editorApp, error) {
	colors, err := document.options()
	if err != nil {
		return nil, err
	}
	a := &editorApp{
		application: tview.NewApplication().EnableMouse(true), document: document, reporter: reporter,
		pages: tview.NewPages(), options: sectionlist.New(),
		preview: newDeclarativePreview(document), status: tview.NewTextView().SetDynamicColors(true), colors: colors,
	}
	a.options.ShowSecondaryText(false).SetUseStyleTags(true, false)
	a.options.SetBorder(true).SetTitle(" Colors ")
	body := tview.NewFlex().
		AddItem(a.options, 0, 1, true).
		AddItem(a.preview, 0, 2, false)
	a.body = body
	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(body, 0, 1, true).
		AddItem(a.status, 1, 0, false)
	a.pages.AddPage("main", root, true, true)
	a.application.SetRoot(a.pages, true)
	a.refresh(0)
	a.options.SetChangedFunc(func(index int, _, _ string, _ rune) {
		a.updateStatus(index)
		if index >= 0 && index < len(a.colors) {
			a.flicker(a.colors[index])
		}
	})
	a.options.SetSelectedFunc(func(index int, _, _ string, _ rune) { a.showPicker(index) })
	a.preview.setColorSelectionHandler(a.selectColorPath)
	a.application.SetInputCapture(a.captureInput)
	return a, nil
}

func (a *editorApp) selectColorPath(path string) {
	for index, option := range a.colors {
		if option.Path != path {
			continue
		}
		previous := a.options.CurrentDataItem()
		a.options.SetCurrentDataItem(index)
		if previous == index {
			a.updateStatus(index)
			a.flicker(option)
		}
		return
	}
}

func (a *editorApp) run() error { return a.application.Run() }

func (a *editorApp) captureInput(event *tcell.EventKey) *tcell.EventKey {
	front, _ := a.pages.GetFrontPage()
	if front != "main" {
		return event
	}
	if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyCtrlC || event.Key() == tcell.KeyCtrlQ ||
		(event.Key() == tcell.KeyRune && event.Rune() == 'q' && event.Modifiers() == tcell.ModNone) {
		a.requestExit()
		return nil
	}
	return event
}

func (a *editorApp) refresh(selected int) {
	colors, err := a.document.options()
	if err != nil {
		a.showError("resolve colors", err)
		return
	}
	a.colors = colors
	chrome := a.document.specification.Editor
	background := a.document.color(chrome.Background, tcell.ColorBlack)
	text := a.document.color(chrome.Text, tcell.ColorWhite)
	border := a.document.color(chrome.Border, tcell.ColorWhite)
	title := a.document.color(chrome.Title, tcell.ColorAqua)
	selectedBackground := a.document.color(chrome.SelectedBackground, tcell.ColorWhite)
	selectedText := a.document.color(chrome.SelectedText, tcell.ColorBlack)
	sectionBackground := a.document.color(chrome.SectionBackground, background)
	sectionText := a.document.color(chrome.SectionText, title)
	a.options.Clear()
	section := ""
	for _, option := range colors {
		optionSection := option.section()
		if optionSection != section {
			a.options.AddSectionAligned(capitalizeFirst(optionSection), sectionText, sectionBackground, tview.AlignRight)
			section = optionSection
		}
		a.options.AddDataItem(colorTag(option.Color)+"██[-] "+tview.Escape(option.displayLabel()), "", 0)
	}
	a.body.ResizeItem(a.options, colorListWidth(colors), 0)
	a.options.SetBackgroundColor(background)
	a.options.SetMainTextStyle(tcell.StyleDefault.Foreground(text).Background(background)).
		SetSecondaryTextStyle(tcell.StyleDefault.Foreground(a.document.color(chrome.StatusHelp, tcell.ColorGray)).Background(background)).
		SetSelectedBackgroundColor(selectedBackground).
		SetSelectedTextColor(selectedText).
		SetHighlightFullLine(true).
		SetBorderColor(border).
		SetTitleColor(title)
	a.status.SetBackgroundColor(a.document.color(chrome.StatusBackground, background))
	a.preview.applyChrome()
	if len(colors) > 0 {
		selected = min(max(selected, 0), len(colors)-1)
		a.options.SetCurrentDataItem(selected)
	}
	a.updateStatus(selected)
}

func capitalizeFirst(value string) string {
	characters := []rune(value)
	if len(characters) == 0 {
		return value
	}
	characters[0] = unicode.ToUpper(characters[0])
	return string(characters)
}

func colorListWidth(colors []colorOption) int {
	maximum := tview.TaggedStringWidth(" Colors ")
	seenSections := make(map[string]bool)
	for _, option := range colors {
		section := option.section()
		if !seenSections[section] {
			// Section rendering includes one trailing padding cell.
			maximum = max(maximum, tview.TaggedStringWidth(tview.Escape(capitalizeFirst(section)))+1)
			seenSections[section] = true
		}
		label := "██ " + option.displayLabel()
		maximum = max(maximum, tview.TaggedStringWidth(tview.Escape(label)))
	}
	return maximum + 2 // Pane borders.
}

func compactColorLabel(label, path string) string {
	section, _, found := strings.Cut(path, ".")
	if !found || section == "" {
		return label
	}
	section = strings.NewReplacer("_", " ", "-", " ").Replace(section)
	prefixLength := len(section)
	if len(label) <= prefixLength || !strings.EqualFold(label[:prefixLength], section) || label[prefixLength] != ' ' {
		return label
	}
	compact := strings.TrimSpace(label[prefixLength+1:])
	if compact == "" {
		return label
	}
	return compact
}

func (a *editorApp) updateStatus(index int) {
	if index < 0 || index >= len(a.colors) {
		return
	}
	option := a.colors[index]
	chrome := a.document.specification.Editor
	dirty := ""
	if a.dirty {
		dirty = " · modified"
	}
	source := ""
	if !option.Explicit {
		source = " · inherited from " + option.FallbackPath
	}
	value := "default"
	if option.Color.Hex() >= 0 {
		value = fmt.Sprintf("#%06x", option.Color.Hex())
	}
	if option.Alias != "" && !option.IsAlias {
		value += " (" + aliasMarker + option.Alias + ")"
	}
	usage := ""
	if option.IsAlias {
		usage = fmt.Sprintf(" · used by %d %s", option.UsedBy, pluralize(option.UsedBy, "color", "colors"))
	}
	message := option.Label + " · " + option.Path + " · " + value + source + usage + dirty
	help := "Enter choose color · click preview select/cycle · arrows move · q/Esc exit"
	a.status.SetText(colorTag(a.document.color(chrome.StatusText, tcell.ColorAqua)) + tview.Escape(message) + "[-]" +
		colorTag(a.document.color(chrome.StatusHelp, tcell.ColorGray)) + " · " + tview.Escape(help) + "[-]")
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

// flicker blinks the preview elements the option affects: those bound to its
// path, or, for an alias row, every element whose color comes from the alias.
func (a *editorApp) flicker(option colorOption) {
	sequence := a.flickerSeq.Add(1)
	setFlicker := func(on bool) {
		if option.IsAlias {
			a.preview.setAliasFlicker(option.Alias, on)
		} else {
			a.preview.setFlicker(option.Path, on)
		}
	}
	setFlicker(true)
	for _, step := range []struct {
		delay time.Duration
		on    bool
	}{{200 * time.Millisecond, false}, {400 * time.Millisecond, true}, {600 * time.Millisecond, false}, {800 * time.Millisecond, true}, {time.Second, false}} {
		step := step
		time.AfterFunc(step.delay, func() {
			a.application.QueueUpdateDraw(func() {
				if a.flickerSeq.Load() == sequence {
					setFlicker(step.on)
				}
			})
		})
	}
}

func (a *editorApp) showPicker(optionIndex int) {
	if optionIndex < 0 || optionIndex >= len(a.colors) {
		return
	}
	option := a.colors[optionIndex]
	chrome := a.document.specification.Editor
	background := a.document.color(chrome.Background, tcell.ColorBlack)
	text := a.document.color(chrome.Text, tcell.ColorWhite)
	border := a.document.color(chrome.Border, tcell.ColorWhite)
	title := a.document.color(chrome.Title, tcell.ColorAqua)
	selectedBackground := a.document.color(chrome.SelectedBackground, tcell.ColorWhite)
	selectedText := a.document.color(chrome.SelectedText, tcell.ColorBlack)
	a.flickerSeq.Add(1)
	a.preview.setFlicker("", false)

	// A color can be given an alias; an alias itself always holds a plain color.
	var aliases []aliasEntry
	if !option.IsAlias {
		aliases = a.document.aliasEntries()
	}
	showCandidate := func(color tcell.Color) {
		if option.IsAlias {
			a.preview.setAliasCandidate(option.Alias, color, true)
		} else {
			a.preview.setCandidate(option.Path, color, true)
		}
	}
	paletteTitle := " Color · Enter apply · Esc cancel "
	if option.IsAlias {
		paletteTitle = " Alias color · Enter apply · Esc cancel "
	} else if len(aliases) > 0 {
		paletteTitle = " Palette · Tab: aliases "
	}

	palette := terminalPalette()
	table := tview.NewTable().SetSelectable(true, true).SetFixed(0, 0).SetBorders(true).SetBordersColor(background).SetSeparator(' ')
	table.SetBackgroundColor(background).SetBorder(true).SetBorderColor(border).SetTitleColor(title).
		SetTitle(paletteTitle)
	const columns = 10
	for index, color := range palette {
		table.SetCell(index/columns, index%columns, tview.NewTableCell("██").
			SetTextColor(color).SetBackgroundColor(background).SetAlign(tview.AlignCenter).SetMaxWidth(2).SetReference(color))
	}
	setSelection := func(row, column int) {
		cell := table.GetCell(row, column)
		if cell == nil {
			return
		}
		color, ok := cell.GetReference().(tcell.Color)
		if !ok {
			return
		}
		table.SetSelectedStyle(tcell.StyleDefault.Foreground(color).Background(background).Bold(true))
		showCandidate(color)
	}
	table.SetSelectionChangedFunc(setSelection)
	selected := closestPaletteColor(option.Color, palette)
	table.Select(selected/columns, selected%columns)
	setSelection(selected/columns, selected%columns)
	closePicker := func() {
		a.preview.setCandidate("", tcell.ColorDefault, false)
		a.pages.RemovePage("picker")
		a.application.SetFocus(a.options)
	}
	apply := func(assign func() error) {
		if err := assign(); err != nil {
			closePicker()
			a.showError("apply color", err)
			return
		}
		a.dirty = true
		a.refresh(optionIndex)
		closePicker()
	}
	table.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			closePicker()
		}
	})
	table.SetSelectedFunc(func(row, column int) {
		cell := table.GetCell(row, column)
		chosen, ok := cell.GetReference().(tcell.Color)
		if !ok {
			return
		}
		changed := chosen.Hex() != option.Color.Hex() || !option.Explicit || (!option.IsAlias && option.Alias != "")
		if !changed {
			closePicker()
			return
		}
		apply(func() error {
			if option.IsAlias {
				return a.document.setAliasColor(option.Alias, chosen)
			}
			return a.document.setColor(option.Path, chosen)
		})
	})
	picker := &colorPicker{Table: table, frameColor: contrastingColor(background), backgroundColor: background}

	var content tview.Primitive = picker
	preferredWidth, preferredHeight := max(34, tview.TaggedStringWidth(paletteTitle)+2), 13
	startOnAliases := false
	var focusAliases func()
	if len(aliases) > 0 {
		aliasList := tview.NewList().ShowSecondaryText(false).SetWrapAround(false)
		aliasList.SetBackgroundColor(background)
		aliasList.SetMainTextStyle(tcell.StyleDefault.Foreground(text).Background(background)).
			SetSelectedBackgroundColor(selectedBackground).
			SetSelectedTextColor(selectedText).
			SetHighlightFullLine(true).
			SetBorderColor(border).
			SetTitleColor(title)
		aliasList.SetBorder(true).SetTitle(" Aliases · Tab: palette ")
		widest := 0
		for index, entry := range aliases {
			label := aliasMarker + entry.Name
			widest = max(widest, tview.TaggedStringWidth(tview.Escape("██ "+label)))
			aliasList.AddItem(colorTag(entry.Color)+"██[-] "+tview.Escape(label), "", 0, nil)
			if option.Explicit && option.Alias == entry.Name {
				aliasList.SetCurrentItem(index)
				startOnAliases = true
			}
		}
		aliasList.SetChangedFunc(func(index int, _, _ string, _ rune) {
			if index >= 0 && index < len(aliases) {
				showCandidate(aliases[index].Color)
			}
		})
		aliasList.SetSelectedFunc(func(index int, _, _ string, _ rune) {
			if index < 0 || index >= len(aliases) {
				return
			}
			name := aliases[index].Name
			if option.Explicit && option.Alias == name {
				closePicker()
				return
			}
			apply(func() error { return a.document.setAlias(option.Path, name) })
		})
		aliasList.SetDoneFunc(closePicker)
		focusAliases = func() {
			a.application.SetFocus(aliasList)
			if index := aliasList.GetCurrentItem(); index >= 0 && index < len(aliases) {
				showCandidate(aliases[index].Color)
			}
		}
		focusPalette := func() {
			a.application.SetFocus(picker)
			row, column := table.GetSelection()
			setSelection(row, column)
		}
		switchOnTab := func(switchFocus func()) func(*tcell.EventKey) *tcell.EventKey {
			return func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab {
					switchFocus()
					return nil
				}
				return event
			}
		}
		aliasList.SetInputCapture(switchOnTab(focusPalette))
		table.SetInputCapture(switchOnTab(focusAliases))
		aliasRows := min(len(aliases), 6)
		content = tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(aliasList, aliasRows+2, 0, false).
			AddItem(picker, 0, 1, true)
		preferredHeight += aliasRows + 2
		preferredWidth = max(preferredWidth, widest+4)
	}
	modal := &centeredModal{content: content, preferredWidth: preferredWidth, preferredHeight: preferredHeight, bottomRight: true}
	a.pages.AddPage("picker", modal, true, true)
	if startOnAliases && focusAliases != nil {
		focusAliases()
	} else {
		a.application.SetFocus(picker)
	}
}

func (a *editorApp) requestExit() {
	if !a.dirty {
		a.application.Stop()
		return
	}
	modal := a.newModal().SetText("Save color changes before exiting?").AddButtons([]string{"Save", "Discard", "Cancel"})
	modal.SetDoneFunc(func(_ int, label string) {
		switch label {
		case "Save":
			if err := a.document.save(); err != nil {
				a.pages.RemovePage("save")
				a.showError("save color assignments", err)
				return
			}
			a.dirty = false
			a.application.Stop()
		case "Discard":
			a.application.Stop()
		default:
			a.pages.RemovePage("save")
			a.application.SetFocus(a.options)
		}
	})
	a.pages.AddPage("save", modal, true, true)
	a.application.SetFocus(modal)
}

func (a *editorApp) newModal() *tview.Modal {
	style := a.document.specification.Editor
	background := a.document.color(style.DialogBackground, tcell.ColorBlack)
	modal := tview.NewModal().
		SetBackgroundColor(background).
		SetTextColor(a.document.color(style.DialogText, tcell.ColorWhite)).
		SetButtonStyle(tcell.StyleDefault.
			Foreground(a.document.color(style.ButtonText, tcell.ColorWhite)).
			Background(a.document.color(style.ButtonBackground, tcell.ColorGray))).
		SetButtonActivatedStyle(tcell.StyleDefault.
			Foreground(a.document.color(style.ButtonSelectedText, tcell.ColorWhite)).
			Background(a.document.color(style.ButtonSelectedBG, tcell.ColorNavy)))
	modal.Box.SetBackgroundColor(background).
		SetBorderColor(a.document.color(style.DialogBorder, tcell.ColorWhite)).
		SetTitleColor(a.document.color(style.DialogTitle, tcell.ColorAqua))
	return modal
}

func (a *editorApp) showError(operation string, err error) {
	message := err.Error()
	report := ""
	if a.reporter != nil {
		report = a.reporter.Record(operation, err, nil, map[string]string{"component": "color.edit"})
		message = a.reporter.Redact(message)
	}
	if report != "" {
		message += "\n\nIncident report: " + report
	}
	modal := a.newModal().SetText(message).AddButtons([]string{"OK"})
	modal.SetDoneFunc(func(_ int, _ string) {
		a.pages.RemovePage("error")
		a.application.SetFocus(a.options)
	})
	a.pages.AddPage("error", modal, true, true)
	a.application.SetFocus(modal)
}

func colorTag(color tcell.Color) string {
	if !color.Valid() || color.Hex() < 0 {
		return "[default]"
	}
	return fmt.Sprintf("[#%06x]", color.Hex())
}

func terminalPalette() []tcell.Color {
	colors := make([]tcell.Color, 0, 256)
	seen := make(map[int32]bool, 256)
	for index := 0; index < 256; index++ {
		color := tcell.ColorBlack + tcell.Color(index)
		hex := color.Hex()
		if hex < 0 || seen[hex] {
			continue
		}
		seen[hex] = true
		colors = append(colors, color)
	}
	return colors
}

func closestPaletteColor(color tcell.Color, palette []tcell.Color) int {
	red, green, blue := color.RGB()
	closest, distance := 0, int64(^uint64(0)>>1)
	for index, candidate := range palette {
		candidateRed, candidateGreen, candidateBlue := candidate.RGB()
		dRed, dGreen, dBlue := int64(red-candidateRed), int64(green-candidateGreen), int64(blue-candidateBlue)
		candidateDistance := dRed*dRed + dGreen*dGreen + dBlue*dBlue
		if candidateDistance < distance {
			closest, distance = index, candidateDistance
		}
	}
	return closest
}

// colorPicker hides ordinary cell borders and frames only the active swatch.
type colorPicker struct {
	*tview.Table
	frameColor, backgroundColor tcell.Color
}

func (p *colorPicker) Draw(screen tcell.Screen) {
	p.Table.Draw(screen)
	selectedRow, selectedColumn := p.GetSelection()
	x, y, width, height := p.GetInnerRect()
	const tableBorders = "┌┬┐├┼┤└┴┘─│"
	for screenY := y; screenY < y+height; screenY++ {
		for screenX := x; screenX < x+width; screenX++ {
			character, _, cellStyle, _ := screen.GetContent(screenX, screenY)
			if strings.ContainsRune(tableBorders, character) {
				screen.SetContent(screenX, screenY, ' ', nil, cellStyle.Foreground(p.backgroundColor).Background(p.backgroundColor))
			}
		}
	}
	minX, minY, maxX, maxY := x+width, y+height, -1, -1
	for screenY := y; screenY < y+height; screenY++ {
		for screenX := x; screenX < x+width; screenX++ {
			row, column := p.CellAt(screenX, screenY)
			if row == selectedRow && column == selectedColumn {
				minX, minY = min(minX, screenX), min(minY, screenY)
				maxX, maxY = max(maxX, screenX), max(maxY, screenY)
			}
		}
	}
	if maxX < minX || maxY < minY {
		return
	}
	if minY > y {
		minY--
	}
	if minX > x {
		minX--
	}
	p.drawSelectionFrame(screen, minX, minY, maxX, maxY)
}

func (p *colorPicker) drawSelectionFrame(screen tcell.Screen, left, top, right, bottom int) {
	set := func(x, y int, character rune) {
		_, _, cellStyle, _ := screen.GetContent(x, y)
		screen.SetContent(x, y, character, nil, cellStyle.Foreground(p.frameColor))
	}
	set(left, top, '╔')
	set(right, top, '╗')
	set(left, bottom, '╚')
	set(right, bottom, '╝')
	for x := left + 1; x < right; x++ {
		set(x, top, '═')
		set(x, bottom, '═')
	}
	for y := top + 1; y < bottom; y++ {
		set(left, y, '║')
		set(right, y, '║')
	}
}

func contrastingColor(background tcell.Color) tcell.Color {
	red, green, blue := background.RGB()
	if 299*red+587*green+114*blue >= 128000 {
		return tcell.ColorBlack
	}
	return tcell.ColorWhite
}

type centeredModal struct {
	content                         tview.Primitive
	preferredWidth, preferredHeight int
	bottomRight                     bool
}

func (m *centeredModal) Draw(screen tcell.Screen) {
	width, height := screen.Size()
	modalWidth := min(m.preferredWidth, max(8, width-2))
	modalHeight := min(m.preferredHeight, max(5, height-2))
	x, y := max(0, (width-modalWidth)/2), max(0, (height-modalHeight)/2)
	if m.bottomRight {
		x, y = max(0, width-modalWidth-1), max(0, height-modalHeight-1)
	}
	m.content.SetRect(x, y, modalWidth, modalHeight)
	m.content.Draw(screen)
}

func (m *centeredModal) SetRect(int, int, int, int)    {}
func (m *centeredModal) GetRect() (int, int, int, int) { return 0, 0, 0, 0 }
func (m *centeredModal) HasFocus() bool                { return m.content.HasFocus() }
func (m *centeredModal) Blur()                         { m.content.Blur() }
func (m *centeredModal) Focus(delegate func(tview.Primitive)) {
	if delegate != nil {
		delegate(m.content)
	}
}
func (m *centeredModal) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return m.content.InputHandler()
}
func (m *centeredModal) PasteHandler() func(string, func(tview.Primitive)) {
	return m.content.PasteHandler()
}
func (m *centeredModal) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	handler := m.content.MouseHandler()
	return func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		if handler == nil {
			return true, nil
		}
		_, capture := handler(action, event, setFocus)
		return true, capture
	}
}
