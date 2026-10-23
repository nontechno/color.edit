package spec

import "testing"

func TestValidateRequiresEveryColorToBeVisible(t *testing.T) {
	valid := Specification{
		Version: Version, Name: "sample", Canvas: Canvas{Width: 20, Height: 8, Resize: "scale", Background: "base"},
		Colors:   []ColorDefinition{{Path: "base", Label: "Base"}, {Path: "text", Label: "Text"}},
		Elements: []Element{{ID: "background", Kind: "fill", Width: 20, Height: 8, Foreground: "text", Background: "base"}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.Colors = append(invalid.Colors, ColorDefinition{Path: "unused", Label: "Unused"})
	if err := invalid.Validate(); err == nil {
		t.Fatal("unused color was accepted")
	}
}

func TestValidateRejectsUnknownVisualBindings(t *testing.T) {
	specification := Specification{
		Version: Version, Name: "sample", Canvas: Canvas{Width: 20, Height: 8, Resize: "scale"},
		Colors:   []ColorDefinition{{Path: "base", Label: "Base"}},
		Elements: []Element{{ID: "bad", Kind: "text", Width: 10, Height: 1, Foreground: "missing"}},
	}
	if err := specification.Validate(); err == nil {
		t.Fatal("unknown visual color binding was accepted")
	}
}

func TestValidateAcceptsColorShownByEditorChrome(t *testing.T) {
	specification := Specification{
		Version: Version, Name: "sample", Canvas: Canvas{Width: 20, Height: 8, Resize: "scale", Background: "base"},
		Colors: []ColorDefinition{
			{Path: "base", Label: "Base"},
			{Path: "section", Label: "Section"},
		},
		Editor:   EditorStyle{SectionBackground: "section"},
		Elements: []Element{{ID: "background", Kind: "fill", Width: 20, Height: 8, Background: "base"}},
	}
	if err := specification.Validate(); err != nil {
		t.Fatal(err)
	}
}
