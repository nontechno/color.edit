package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"color.edit/spec"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func testSpecification() spec.Specification {
	return spec.Specification{
		Version: spec.Version, Name: "generic-test",
		Canvas: spec.Canvas{Width: 20, Height: 8, Resize: "scale", Background: "base.background"},
		Colors: []spec.ColorDefinition{
			{Path: "accent", Label: "Accent", FallbackPath: "base.text"},
			{Path: "base.background", Label: "Background"},
			{Path: "base.text", Label: "Text"},
		},
		Editor: spec.EditorStyle{Background: "base.background", Text: "base.text", Border: "accent", Title: "accent"},
		Elements: []spec.Element{
			{ID: "background", Kind: "fill", Width: 20, Height: 8, Character: " ", Foreground: "base.text", Background: "base.background"},
			{ID: "label", Kind: "text", X: 2, Y: 2, Width: 10, Height: 1, Text: "preview", Foreground: "base.text", Background: "base.background"},
			{ID: "accent", Kind: "fill", X: 10, Y: 4, Width: 5, Height: 2, Character: "█", Foreground: "accent", Background: "base.background"},
		},
	}
}

// sampleSpecification loads the layout shipped with the project (cmap).
func sampleSpecification(t *testing.T) spec.Specification {
	t.Helper()
	specification, err := spec.Load("cmap")
	if err != nil {
		t.Fatal(err)
	}
	return specification
}

func writeAssignments(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "colors.json")
	if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAssignmentDocumentResolvesFallbackEditsAndPreservesOtherData(t *testing.T) {
	specification := testSpecification()
	if err := specification.Validate(); err != nil {
		t.Fatal(err)
	}
	path := writeAssignments(t, `{"name":"sample","base":{"background":"#010203","text":"#f0f0f0"},"non_color":{"kept":42}}`)
	document, err := loadAssignments(path, specification)
	if err != nil {
		t.Fatal(err)
	}
	options, err := document.options()
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 3 || options[0].Path != "accent" || options[0].Explicit || options[0].Color.Hex() != 0xf0f0f0 {
		t.Fatalf("resolved options = %+v", options)
	}
	chosen := tcell.NewHexColor(0x123456)
	if err := document.setColor("accent", chosen); err != nil {
		t.Fatal(err)
	}
	if err := document.save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("saved permissions = %o, want 640", info.Mode().Perm())
	}
	var saved map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["accent"] != "#123456" || saved["name"] != "sample" {
		t.Fatalf("saved assignments = %s", data)
	}
	if saved["non_color"].(map[string]any)["kept"] != float64(42) {
		t.Fatalf("non-color content was not preserved: %s", data)
	}
}

func TestSampleSpecificationLoadsBothExampleThemes(t *testing.T) {
	generated := sampleSpecification(t)
	for _, name := range []string{"dark", "light"} {
		t.Run(name, func(t *testing.T) {
			document, err := loadAssignments(filepath.Join("examples", "themes", name+".json"), generated)
			if err != nil {
				t.Fatal(err)
			}
			options, err := document.options()
			if err != nil {
				t.Fatal(err)
			}
			colors := 0
			for _, option := range options {
				if !option.IsAlias {
					colors++
				}
			}
			if colors != 60 {
				t.Fatalf("resolved color count = %d, want 60", colors)
			}
			if aliases := len(options) - colors; aliases == 0 || aliases != len(document.aliasEntries()) {
				t.Fatalf("alias rows = %d, want one per alias in the file (%d)", aliases, len(document.aliasEntries()))
			}
			application, err := newEditorApp(document, nil)
			if err != nil {
				t.Fatal(err)
			}
			main, secondary := application.options.GetItemText(2) // Header, Background, then Metadata Text.
			if secondary != "" {
				t.Fatalf("color row secondary text = %q, want one-line row", secondary)
			}
			if !strings.Contains(main, "Metadata Text") || strings.Contains(main, "Browser Metadata Text") {
				t.Fatalf("compact browser color label = %q, want Metadata Text", main)
			}
		})
	}
}

func TestSelectedColorRowHighlightsCompleteListWidth(t *testing.T) {
	document, err := loadAssignments(filepath.Join("examples", "themes", "dark.json"), sampleSpecification(t))
	if err != nil {
		t.Fatal(err)
	}
	application, err := newEditorApp(document, nil)
	if err != nil {
		t.Fatal(err)
	}
	application.options.SetCurrentDataItem(0)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(36, 8)
	application.options.SetRect(0, 0, 36, 8)
	application.options.Draw(screen)
	x, y, width, _ := application.options.GetInnerRect()
	_, _, style, _ := screen.GetContent(x+width-1, y+1) // Section is row 0; first color is row 1.
	_, background, _ := style.Decompose()
	want := document.color(document.specification.Editor.SelectedBackground, tcell.ColorWhite)
	if background.Hex() != want.Hex() {
		t.Fatalf("selected row trailing background = %#x, want %#x", background.Hex(), want.Hex())
	}
}

func TestCompactColorLabelRemovesOnlyMatchingSectionPrefix(t *testing.T) {
	for _, test := range []struct {
		label string
		path  string
		want  string
	}{
		{"Browser Metadata Text", "browser.metadata_text", "Metadata Text"},
		{"Dialog Button Text", "dialog.button.text", "Button Text"},
		{"Custom Metadata", "browser.metadata_text", "Custom Metadata"},
		{"Accent", "accent", "Accent"},
	} {
		if got := compactColorLabel(test.label, test.path); got != test.want {
			t.Errorf("compactColorLabel(%q, %q) = %q, want %q", test.label, test.path, got, test.want)
		}
	}
}

func TestColorListWidthUsesWidestCompactEntry(t *testing.T) {
	colors := []colorOption{
		{ColorDefinition: spec.ColorDefinition{Path: "browser.metadata_text", Label: "Browser Metadata Text"}},
		{ColorDefinition: spec.ColorDefinition{Path: "status.info", Label: "Status Info"}},
	}
	want := tview.TaggedStringWidth(tview.Escape("██ Metadata Text")) + 2
	if got := colorListWidth(colors); got != want {
		t.Fatalf("color list width = %d, want %d", got, want)
	}
	if got := capitalizeFirst("browser"); got != "Browser" {
		t.Fatalf("capitalized section = %q, want Browser", got)
	}
}

func TestArgumentParserRequiresSpecificationAndAssignments(t *testing.T) {
	var output bytes.Buffer
	options, err := parseArguments([]string{"--spec", "preview.json", "--colors", "colors.json", "--report-dir", "reports"}, &output)
	if err != nil {
		t.Fatal(err)
	}
	if options.specPath != "preview.json" || options.colorsPath != "colors.json" || options.reportDir != "reports" {
		t.Fatalf("parsed options = %+v", options)
	}
	if _, err := parseArguments([]string{"--spec", "preview.json"}, &output); err == nil {
		t.Fatal("missing assignment file was accepted")
	}
}

func TestEditorDisplaysCurrentColorNamePathAndValue(t *testing.T) {
	path := writeAssignments(t, `{"base":{"background":"#010203","text":"#f0f0f0"}}`)
	document, err := loadAssignments(path, testSpecification())
	if err != nil {
		t.Fatal(err)
	}
	application, err := newEditorApp(document, nil)
	if err != nil {
		t.Fatal(err)
	}
	status := application.status.GetText(true)
	for _, expected := range []string{"Accent", "accent", "#f0f0f0", "inherited from base.text"} {
		if !strings.Contains(status, expected) {
			t.Fatalf("status %q does not contain %q", status, expected)
		}
	}
}

func TestTerminalPaletteIsDistinct(t *testing.T) {
	palette := terminalPalette()
	seen := make(map[int32]bool, len(palette))
	for _, color := range palette {
		if seen[color.Hex()] {
			t.Fatalf("duplicate palette color %#x", color.Hex())
		}
		seen[color.Hex()] = true
	}
	if len(palette) != 247 {
		t.Fatalf("palette length = %d, want 247", len(palette))
	}
}
