package main

import (
	"path/filepath"
	"reflect"
	"testing"

	"color.edit/spec"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestDeclarativePreviewScalesAndShowsCandidateColor(t *testing.T) {
	path := writeAssignments(t, `{"base":{"background":"#010203","text":"#f0f0f0"}}`)
	document, err := loadAssignments(path, testSpecification())
	if err != nil {
		t.Fatal(err)
	}
	preview := newDeclarativePreview(document)
	preview.setCandidate("accent", tcell.NewHexColor(0x123456), true)
	for _, size := range [][2]int{{60, 20}, {40, 12}} {
		screen := tcell.NewSimulationScreen("UTF-8")
		if err := screen.Init(); err != nil {
			t.Fatal(err)
		}
		screen.SetSize(size[0], size[1])
		preview.SetRect(0, 0, size[0], size[1])
		preview.Draw(screen)
		x, y, width, height := preview.GetInnerRect()
		accentX := x + 10*width/20
		accentY := y + 4*height/8
		character, _, cellStyle, _ := screen.GetContent(accentX, accentY)
		foreground, _, _ := cellStyle.Decompose()
		if character != '█' || foreground.Hex() != 0x123456 {
			t.Fatalf("preview %dx%d candidate cell = %q/%#x", size[0], size[1], character, foreground.Hex())
		}
		screen.Fini()
	}
}

func TestSamplePreviewRepresentsTableHeaderColors(t *testing.T) {
	document, err := loadAssignments(filepath.Join("examples", "themes", "dark.json"), sampleSpecification(t))
	if err != nil {
		t.Fatal(err)
	}
	preview := newDeclarativePreview(document)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(122, 43) // Border plus the exact 120x41 logical canvas.
	preview.SetRect(0, 0, 122, 43)

	const candidateHex = 0x123456
	preview.setCandidate("table.header.background", tcell.NewHexColor(candidateHex), true)
	preview.Draw(screen)
	x, y, _, _ := preview.GetInnerRect()
	character, _, style, _ := screen.GetContent(x+2, y+17) // The "id" header label.
	foreground, background, _ := style.Decompose()
	if character != 'i' || background.Hex() != candidateHex {
		t.Fatalf("header background candidate at label = %q on %#x, want i on %#x", character, background.Hex(), candidateHex)
	}
	if foreground.Hex() == candidateHex {
		t.Fatal("header background candidate incorrectly replaced header text")
	}

	const textHex = 0xabcdef
	preview.setCandidate("table.header.text", tcell.NewHexColor(textHex), true)
	preview.Draw(screen)
	character, _, style, _ = screen.GetContent(x+52, y+17) // The "created_at" label.
	foreground, background, _ = style.Decompose()
	if character != 'c' || foreground.Hex() != textHex {
		t.Fatalf("header text candidate = %q/%#x, want c/%#x", character, foreground.Hex(), textHex)
	}
	if background.Hex() == textHex {
		t.Fatal("header text candidate incorrectly replaced header background")
	}
}

func TestScaledSingletonDimensionsRemainOneCell(t *testing.T) {
	path := writeAssignments(t, `{"base":{"background":"#010203","text":"#f0f0f0"}}`)
	document, err := loadAssignments(path, testSpecification())
	if err != nil {
		t.Fatal(err)
	}
	preview := newDeclarativePreview(document)
	x, y, width, height := 3, 4, 80, 32

	horizontal := spec.Element{X: 2, Y: 3, Width: 10, Height: 1}
	_, _, _, scaledHeight := preview.scaled(horizontal, x, y, width, height)
	if scaledHeight != 1 {
		t.Fatalf("one-cell row scaled to height %d", scaledHeight)
	}
	vertical := spec.Element{X: 18, Y: 1, Width: 1, Height: 6}
	_, _, scaledWidth, _ := preview.scaled(vertical, x, y, width, height)
	if scaledWidth != 1 {
		t.Fatalf("one-cell column scaled to width %d", scaledWidth)
	}
	rightEdge := spec.Element{X: 19, Y: 1, Width: 1, Height: 6}
	scaledX, _, scaledWidth, _ := preview.scaled(rightEdge, x, y, width, height)
	if scaledX != x+width-1 || scaledWidth != 1 {
		t.Fatalf("right-edge cell scaled to x=%d width=%d, want x=%d width=1", scaledX, scaledWidth, x+width-1)
	}
}

func TestPreviewClickCyclesVisibleForegroundAndBackground(t *testing.T) {
	path := writeAssignments(t, `{"base":{"background":"#010203","text":"#f0f0f0"}}`)
	document, err := loadAssignments(path, testSpecification())
	if err != nil {
		t.Fatal(err)
	}
	preview := newDeclarativePreview(document)
	preview.SetRect(0, 0, 22, 10) // Border plus the exact 20x8 logical canvas.
	x, y, _, _ := preview.GetInnerRect()
	labelX, labelY := x+2, y+2
	if got := preview.colorPathsAt(labelX, labelY); !reflect.DeepEqual(got, []string{"base.text", "base.background"}) {
		t.Fatalf("label color paths = %v, want foreground then background", got)
	}
	for index, want := range []string{"base.text", "base.background", "base.text"} {
		got, ok := preview.nextColorPathAt(labelX, labelY)
		if !ok || got != want {
			t.Fatalf("click %d selected %q, %t; want %q", index+1, got, ok, want)
		}
	}
	if got := preview.colorPathsAt(x+10, y+4); !reflect.DeepEqual(got, []string{"accent", "base.background"}) {
		t.Fatalf("accent block color paths = %v, want foreground then background", got)
	}
	if got := preview.colorPathsAt(x+1, y+1); !reflect.DeepEqual(got, []string{"base.background"}) {
		t.Fatalf("blank canvas color paths = %v, want background only", got)
	}
}

func TestPreviewMouseClickReportsCycledColorPathWithoutTakingFocus(t *testing.T) {
	path := writeAssignments(t, `{"base":{"background":"#010203","text":"#f0f0f0"}}`)
	document, err := loadAssignments(path, testSpecification())
	if err != nil {
		t.Fatal(err)
	}
	preview := newDeclarativePreview(document)
	preview.SetRect(0, 0, 22, 10)
	x, y, _, _ := preview.GetInnerRect()
	var selected []string
	preview.setColorSelectionHandler(func(path string) { selected = append(selected, path) })
	focused := false
	setFocus := func(tview.Primitive) { focused = true }
	event := tcell.NewEventMouse(x+2, y+2, tcell.ButtonNone, tcell.ModNone)
	handler := preview.MouseHandler()
	if consumed, _ := handler(tview.MouseLeftClick, event, setFocus); !consumed {
		t.Fatal("preview click was not consumed")
	}
	if consumed, _ := handler(tview.MouseLeftDoubleClick, event, setFocus); !consumed {
		t.Fatal("repeated preview click was not consumed")
	}
	if !reflect.DeepEqual(selected, []string{"base.text", "base.background"}) {
		t.Fatalf("mouse-selected paths = %v, want foreground then background", selected)
	}
	if focused {
		t.Fatal("preview click moved keyboard focus away from the color list")
	}
}

func TestSamplePreviewClickSelectsAndCyclesColorList(t *testing.T) {
	document, err := loadAssignments(filepath.Join("examples", "themes", "dark.json"), sampleSpecification(t))
	if err != nil {
		t.Fatal(err)
	}
	application, err := newEditorApp(document, nil)
	if err != nil {
		t.Fatal(err)
	}
	application.preview.SetRect(0, 0, 122, 43) // Border plus exact generated canvas.
	x, y, _, _ := application.preview.GetInnerRect()
	event := tcell.NewEventMouse(x+2, y+17, tcell.ButtonNone, tcell.ModNone) // "id" header text.
	handler := application.preview.MouseHandler()
	handler(tview.MouseLeftClick, event, func(tview.Primitive) {})
	selected := application.options.CurrentDataItem()
	selectedPath := ""
	if selected >= 0 && selected < len(application.colors) {
		selectedPath = application.colors[selected].Path
	}
	if selectedPath != "table.header.text" {
		t.Fatalf("first header click selected index %d (%q), want table.header.text", selected, selectedPath)
	}
	handler(tview.MouseLeftDoubleClick, event, func(tview.Primitive) {})
	selected = application.options.CurrentDataItem()
	selectedPath = ""
	if selected >= 0 && selected < len(application.colors) {
		selectedPath = application.colors[selected].Path
	}
	if selectedPath != "table.header.background" {
		t.Fatalf("second header click selected index %d (%q), want table.header.background", selected, selectedPath)
	}
	application.flickerSeq.Add(1)
}
