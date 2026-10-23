package main

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func newAliasedApp(t *testing.T) *editorApp {
	t.Helper()
	application, err := newEditorApp(loadAliased(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	return application
}

// pressKey delivers a key to whichever widget currently has focus.
func pressKey(a *editorApp, key tcell.Key) {
	a.application.GetFocus().InputHandler()(tcell.NewEventKey(key, 0, tcell.ModNone), func(tview.Primitive) {})
}

func TestAliasRowsAreListedAndDescribedInStatus(t *testing.T) {
	a := newAliasedApp(t)
	// Colors first (accent, base.background, base.text), then the aliases.
	if len(a.colors) != 5 || !a.colors[3].IsAlias || a.colors[3].Label != "@ink" || a.colors[4].Label != "@paper" {
		t.Fatalf("options = %+v", a.colors)
	}
	// List rows: Accent, accent, Base, background, text, Aliases, @ink, @paper.
	if main, _ := a.options.GetItemText(6); main != "[#f0f0f0]██[-] @ink" {
		t.Fatalf("alias row text = %q", main)
	}
	a.updateStatus(3)
	status := a.status.GetText(true)
	for _, want := range []string{"@ink", "$.aliases.ink", "#f0f0f0", "used by 2 colors"} {
		if !strings.Contains(status, want) {
			t.Fatalf("alias status %q does not contain %q", status, want)
		}
	}
	a.updateStatus(2)
	if status := a.status.GetText(true); !strings.Contains(status, "#f0f0f0 (@ink)") {
		t.Fatalf("color status %q does not name its alias", status)
	}
}

func TestChangingAliasUpdatesListAndPreviewImmediately(t *testing.T) {
	a := newAliasedApp(t)
	if err := a.document.setAliasColor("ink", tcell.NewHexColor(0x123456)); err != nil {
		t.Fatal(err)
	}
	a.refresh(3)
	for _, row := range []int{1, 4, 6} { // accent, base.text, @ink all follow @ink
		if main, _ := a.options.GetItemText(row); !strings.HasPrefix(main, "[#123456]") {
			t.Fatalf("row %d = %q, want the new alias color", row, main)
		}
	}
	if main, _ := a.options.GetItemText(3); !strings.HasPrefix(main, "[#010203]") {
		t.Fatalf("base.background row = %q, want it unchanged", main)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(60, 20)
	a.preview.SetRect(0, 0, 60, 20)
	a.preview.Draw(screen)
	x, y, width, height := a.preview.GetInnerRect()
	_, _, style, _ := screen.GetContent(x+10*width/20, y+4*height/8) // accent swatch
	if foreground, _, _ := style.Decompose(); foreground.Hex() != 0x123456 {
		t.Fatalf("preview accent = %#x, want 0x123456", foreground.Hex())
	}
}

func TestPreviewShowsAliasCandidateAndFlickerOnEveryUser(t *testing.T) {
	a := newAliasedApp(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(60, 20)
	a.preview.SetRect(0, 0, 60, 20)
	x, y, width, height := a.preview.GetInnerRect()
	accent := func() int32 {
		a.preview.Draw(screen)
		_, _, style, _ := screen.GetContent(x+10*width/20, y+4*height/8)
		foreground, _, _ := style.Decompose()
		return foreground.Hex()
	}
	if got := accent(); got != 0xf0f0f0 {
		t.Fatalf("accent = %#x, want 0xf0f0f0", got)
	}
	a.preview.setAliasCandidate("ink", tcell.NewHexColor(0xabcdef), true)
	if got := accent(); got != 0xabcdef {
		t.Fatalf("accent with @ink candidate = %#x, want 0xabcdef", got)
	}
	a.preview.setAliasCandidate("paper", tcell.NewHexColor(0xabcdef), true)
	if got := accent(); got != 0xf0f0f0 {
		t.Fatalf("accent with unrelated alias candidate = %#x, want 0xf0f0f0", got)
	}
	a.preview.setCandidate("", tcell.ColorDefault, false)
	a.preview.setAliasFlicker("ink", true)
	if got := accent(); got != 0xf0f0f0^0xffffff {
		t.Fatalf("accent while @ink flickers = %#x, want the inverse", got)
	}
}

func TestPickerStartsOnCurrentAliasAndPreviewsIt(t *testing.T) {
	a := newAliasedApp(t)
	a.showPicker(2) // base.text, explicitly @ink
	if front, _ := a.pages.GetFrontPage(); front != "picker" {
		t.Fatalf("front page = %q, want picker", front)
	}
	if !a.preview.candidateOn || a.preview.candidatePath != "base.text" || a.preview.candidateColor.Hex() != 0xf0f0f0 {
		t.Fatalf("candidate = %q %#x on=%v", a.preview.candidatePath, a.preview.candidateColor.Hex(), a.preview.candidateOn)
	}
	if _, onPalette := a.application.GetFocus().(*colorPicker); onPalette {
		t.Fatal("picker should start on the alias list for a color that uses an alias")
	}
	pressKey(a, tcell.KeyTab)
	if _, onPalette := a.application.GetFocus().(*colorPicker); !onPalette {
		t.Fatal("Tab did not move focus to the palette")
	}
	pressKey(a, tcell.KeyTab)
	if _, onPalette := a.application.GetFocus().(*colorPicker); onPalette {
		t.Fatal("Tab did not move focus back to the aliases")
	}
}

func TestChoosingAnAliasAssignsReference(t *testing.T) {
	a := newAliasedApp(t)
	a.showPicker(1) // base.background, currently @paper
	pressKey(a, tcell.KeyUp)
	pressKey(a, tcell.KeyEnter) // @ink
	if _, alias := a.document.colorAndAlias("base.background"); alias != "ink" {
		t.Fatalf("base.background alias = %q, want ink", alias)
	}
	if !a.dirty {
		t.Fatal("assigning an alias did not mark the document modified")
	}
	if front, _ := a.pages.GetFrontPage(); front != "main" {
		t.Fatalf("front page = %q, want main", front)
	}
}

func TestEditingAnAliasUsesThePaletteOnly(t *testing.T) {
	a := newAliasedApp(t)
	a.showPicker(4) // @paper
	if !a.preview.candidateOn || a.preview.candidateAlias != "paper" || a.preview.candidatePath != "" {
		t.Fatalf("candidate = alias %q path %q on=%v", a.preview.candidateAlias, a.preview.candidatePath, a.preview.candidateOn)
	}
	if _, onPalette := a.application.GetFocus().(*colorPicker); !onPalette {
		t.Fatal("alias editing should open on the palette")
	}
	palette := terminalPalette()
	want := palette[closestPaletteColor(a.colors[4].Color, palette)].Hex()
	pressKey(a, tcell.KeyEnter)
	if got := a.document.color("base.background", tcell.ColorDefault).Hex(); got != want {
		t.Fatalf("base.background = %#x after editing @paper, want %#x", got, want)
	}
	if !a.dirty || a.preview.candidateOn {
		t.Fatalf("dirty=%v candidateOn=%v after applying", a.dirty, a.preview.candidateOn)
	}
}
