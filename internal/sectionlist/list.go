// Package sectionlist provides a tview list with full-width, unselectable
// section headings. Data callbacks and selection use indices that exclude the
// headings, so adding visual grouping does not leak into callers' data model.
package sectionlist

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type row struct {
	section    bool
	label      string
	dataIndex  int
	text       tcell.Color
	background tcell.Color
	alignment  int
}

type List struct {
	*tview.List
	rows          []row
	dataCount     int
	showSecondary bool
	changed       func(int, string, string, rune)
	selected      func(int, string, string, rune)
}

func (l *List) ShowSecondaryText(show bool) *List {
	l.showSecondary = show
	l.List.ShowSecondaryText(show)
	return l
}

func New() *List {
	l := &List{List: tview.NewList().SetWrapAround(false), showSecondary: true}
	l.List.SetChangedFunc(func(index int, main, secondary string, shortcut rune) {
		if index < 0 || index >= len(l.rows) || l.rows[index].section {
			return
		}
		if l.changed != nil {
			l.changed(l.rows[index].dataIndex, main, secondary, shortcut)
		}
	})
	l.List.SetSelectedFunc(func(index int, main, secondary string, shortcut rune) {
		if index < 0 || index >= len(l.rows) || l.rows[index].section || l.selected == nil {
			return
		}
		l.selected(l.rows[index].dataIndex, main, secondary, shortcut)
	})
	return l
}

func (l *List) Clear() *List {
	l.rows = nil
	l.dataCount = 0
	l.List.Clear()
	return l
}

func (l *List) AddSection(label string, text, background tcell.Color) *List {
	return l.AddSectionAligned(label, text, background, tview.AlignLeft)
}

// AddSectionAligned adds a non-selectable full-width heading with independent
// text alignment. Ordinary section headings remain left-aligned by default.
func (l *List) AddSectionAligned(label string, text, background tcell.Color, alignment int) *List {
	l.rows = append(l.rows, row{section: true, label: label, dataIndex: -1, text: text, background: background, alignment: alignment})
	l.List.AddItem("", "", 0, nil)
	return l
}

func (l *List) AddDataItem(main, secondary string, shortcut rune) *List {
	l.rows = append(l.rows, row{dataIndex: l.dataCount})
	l.dataCount++
	l.List.AddItem(main, secondary, shortcut, nil)
	return l
}

func (l *List) SetChangedFunc(handler func(int, string, string, rune)) *List {
	l.changed = handler
	return l
}

func (l *List) SetSelectedFunc(handler func(int, string, string, rune)) *List {
	l.selected = handler
	return l
}

func (l *List) SetCurrentDataItem(index int) *List {
	for rowIndex, row := range l.rows {
		if !row.section && row.dataIndex == index {
			l.List.SetCurrentItem(rowIndex)
			l.revealProgrammaticSelection(rowIndex)
			l.ensureSectionHeadingVisible()
			break
		}
	}
	return l
}

func (l *List) revealProgrammaticSelection(rowIndex int) {
	vertical, horizontal := l.List.GetOffset()
	_, _, _, height := l.GetInnerRect()
	if height <= 0 || (rowIndex >= vertical && rowIndex < vertical+height) {
		return
	}
	start := rowIndex
	for start > 0 && !l.rows[start-1].section {
		start--
	}
	if start > 0 && l.rows[start-1].section {
		start--
	}
	l.List.SetOffset(start, horizontal)
}

func (l *List) CurrentDataItem() int {
	index := l.List.GetCurrentItem()
	if index < 0 || index >= len(l.rows) || l.rows[index].section {
		return -1
	}
	return l.rows[index].dataIndex
}

func (l *List) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	handler := l.List.InputHandler()
	return func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
		previous := l.List.GetCurrentItem()
		handler(event, func(primitive tview.Primitive) {
			if primitive == l.List {
				setFocus(l)
				return
			}
			setFocus(primitive)
		})
		direction := 1
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyBacktab || event.Key() == tcell.KeyPgUp {
			direction = -1
		}
		if current := l.List.GetCurrentItem(); current < previous && event.Key() != tcell.KeyHome {
			direction = -1
		}
		l.ensureSelectable(direction)
		l.ensureSectionHeadingVisible()
	}
}

func (l *List) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	handler := l.List.MouseHandler()
	return func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		previous := l.List.GetCurrentItem()
		consumed, capture := handler(action, event, func(primitive tview.Primitive) {
			if primitive == l.List {
				setFocus(l)
				return
			}
			setFocus(primitive)
		})
		direction := 1
		if l.List.GetCurrentItem() < previous {
			direction = -1
		}
		l.ensureSelectable(direction)
		l.ensureSectionHeadingVisible()
		if capture == l.List {
			capture = l
		}
		return consumed, capture
	}
}

// ensureSectionHeadingVisible keeps the heading immediately above the current
// data row in the viewport. tview adjusts a List's offset only far enough to
// reveal the selected row, which otherwise leaves the first section heading
// hidden after Page Down followed by Page Up.
func (l *List) ensureSectionHeadingVisible() {
	current := l.List.GetCurrentItem()
	if current <= 0 || current >= len(l.rows) || !l.rows[current-1].section {
		return
	}
	vertical, horizontal := l.List.GetOffset()
	if vertical > current-1 {
		l.List.SetOffset(current-1, horizontal)
	}
}

func (l *List) Draw(screen tcell.Screen) {
	_, _, width, _ := l.GetInnerRect()
	for index, row := range l.rows {
		if !row.section {
			continue
		}
		label := " " + row.label
		if row.alignment == tview.AlignRight {
			label = row.label + " "
		}
		if taggedWidth := tview.TaggedStringWidth(tview.Escape(label)); taggedWidth > width {
			label = truncate(label, width)
		}
		padding := max(0, width-tview.TaggedStringWidth(tview.Escape(label)))
		if row.alignment == tview.AlignRight {
			label = strings.Repeat(" ", padding) + label
			padding = 0
		}
		style := styleTag(row.text, row.background)
		secondary := ""
		if l.showSecondary {
			secondary = style + strings.Repeat(" ", width) + "[-:-]"
		}
		l.List.SetItemText(index, style+tview.Escape(label)+strings.Repeat(" ", padding)+"[-:-]", secondary)
	}
	l.List.Draw(screen)
}

// styleTag returns a tview dynamic-color tag with both foreground and
// background colors, used for full-width section rows.
func styleTag(foreground, background tcell.Color) string {
	colorName := func(color tcell.Color) string {
		if !color.Valid() || color.Hex() < 0 {
			return "default"
		}
		return fmt.Sprintf("#%06x", color.Hex())
	}
	return "[" + colorName(foreground) + ":" + colorName(background) + "]"
}

func (l *List) ensureSelectable(direction int) {
	index := l.List.GetCurrentItem()
	if index < 0 || index >= len(l.rows) || !l.rows[index].section {
		return
	}
	next := l.selectableFrom(index, direction)
	if next < 0 {
		next = l.selectableFrom(index, -direction)
	}
	if next < 0 {
		return
	}
	l.List.SetCurrentItem(next)
}

func (l *List) selectableFrom(index, direction int) int {
	for index += direction; index >= 0 && index < len(l.rows); index += direction {
		if !l.rows[index].section {
			return index
		}
	}
	return -1
}

func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}
