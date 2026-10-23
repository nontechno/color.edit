package sectionlist

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestSectionsAreFullWidthAndExcludedFromSelection(t *testing.T) {
	list := New()
	sectionText := tcell.NewHexColor(0x102030)
	sectionBackground := tcell.NewHexColor(0xa0b0c0)
	list.AddSection("tables", sectionText, sectionBackground)
	list.AddDataItem("first", "", 0)
	list.AddSection("views", sectionText, sectionBackground)
	list.AddDataItem("second", "", 0)

	selected := -1
	list.SetChangedFunc(func(index int, _, _ string, _ rune) { selected = index })
	list.SetCurrentDataItem(0)
	list.InputHandler()(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), func(tview.Primitive) {})
	if selected != 1 || list.CurrentDataItem() != 1 {
		t.Fatalf("selection after section = callback %d, current %d; want data item 1", selected, list.CurrentDataItem())
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(24, 7)
	list.SetRect(0, 0, 24, 7)
	list.Draw(screen)
	// Secondary text is enabled by default, and keeping the final item visible
	// scrolls the first row away; the second section heading starts at y=2.
	for x := 0; x < 24; x++ {
		_, _, style, _ := screen.GetContent(x, 2)
		foreground, background, _ := style.Decompose()
		if foreground != sectionText || background != sectionBackground {
			t.Fatalf("section style at x=%d = %v on %v, want %v on %v", x, foreground, background, sectionText, sectionBackground)
		}
	}
}

func TestRightAlignedSectionHeading(t *testing.T) {
	list := New().ShowSecondaryText(false)
	list.AddSectionAligned("Browser", tcell.ColorWhite, tcell.ColorNavy, tview.AlignRight)
	list.AddDataItem("value", "", 0)
	list.SetCurrentDataItem(0)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(16, 3)
	list.SetRect(0, 0, 16, 3)
	list.Draw(screen)
	var heading strings.Builder
	for x := 0; x < 16; x++ {
		character, _, _, _ := screen.GetContent(x, 0)
		heading.WriteRune(character)
	}
	if got := heading.String(); got != "        Browser " {
		t.Fatalf("right-aligned heading = %q, want %q", got, "        Browser ")
	}
}

func TestPageUpRestoresSectionHeading(t *testing.T) {
	list := New().ShowSecondaryText(false)
	list.AddSection("Tables", tcell.ColorWhite, tcell.ColorNavy)
	for index := 0; index < 10; index++ {
		list.AddDataItem("row", "", 0)
	}
	list.SetCurrentDataItem(0)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(20, 4)
	list.SetRect(0, 0, 20, 4)
	list.Draw(screen)
	handler := list.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone), func(tview.Primitive) {})
	list.Draw(screen) // Let tview advance its viewport offset.
	handler(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone), func(tview.Primitive) {})
	list.Draw(screen)

	if vertical, _ := list.GetOffset(); vertical != 0 {
		t.Fatalf("offset after Page Down/Page Up = %d, want section heading at offset 0", vertical)
	}
	var firstLine strings.Builder
	for x := 0; x < 20; x++ {
		character, _, _, _ := screen.GetContent(x, 0)
		firstLine.WriteRune(character)
	}
	if !strings.Contains(firstLine.String(), "Tables") {
		t.Fatalf("first line after Page Down/Page Up = %q, want section heading", firstLine.String())
	}
}

func TestNavigationDoesNotWrapAfterLastDataItem(t *testing.T) {
	list := New().ShowSecondaryText(false)
	list.AddSection("Tables", tcell.ColorWhite, tcell.ColorNavy)
	list.AddDataItem("first", "", 0)
	list.AddDataItem("last", "", 0)
	list.SetRect(0, 0, 20, 4)
	list.SetCurrentDataItem(1)
	handler := list.InputHandler()

	for _, key := range []tcell.Key{tcell.KeyDown, tcell.KeyPgDn} {
		handler(tcell.NewEventKey(key, 0, tcell.ModNone), func(tview.Primitive) {})
		if got := list.CurrentDataItem(); got != 1 {
			t.Fatalf("%v at final item moved selection to data item %d, want it to remain at 1", key, got)
		}
	}
}

func TestProgrammaticSelectionRevealsItsSectionHeading(t *testing.T) {
	list := New().ShowSecondaryText(false)
	list.AddSection("First", tcell.ColorWhite, tcell.ColorNavy)
	for index := 0; index < 5; index++ {
		list.AddDataItem("first", "", 0)
	}
	list.AddSection("Second", tcell.ColorWhite, tcell.ColorNavy)
	for index := 0; index < 5; index++ {
		list.AddDataItem("second", "", 0)
	}
	list.SetRect(0, 0, 20, 4)
	list.SetCurrentDataItem(7)
	if vertical, _ := list.GetOffset(); vertical != 6 {
		t.Fatalf("programmatic selection offset = %d, want second section heading at 6", vertical)
	}
}
