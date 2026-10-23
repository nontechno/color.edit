package main

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

const aliasedAssignments = `{
  "$": {"version": "3", "aliases": {"paper": "#010203", "ink": "#f0f0f0"}},
  "base": {"background": "@paper", "text": "@ink"}
}`

func loadAliased(t *testing.T) *assignmentDocument {
	t.Helper()
	document, err := loadAssignments(writeAssignments(t, aliasedAssignments), testSpecification())
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func TestAliasesResolveAndAreListedAfterColors(t *testing.T) {
	options, err := loadAliased(t).options()
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 5 {
		t.Fatalf("got %d options, want 3 colors + 2 aliases: %+v", len(options), options)
	}
	accent, background, text := options[0], options[1], options[2]
	// accent is not assigned; it inherits base.text, which is the "ink" alias.
	if accent.Path != "accent" || accent.Explicit || accent.Alias != "ink" || accent.Color.Hex() != 0xf0f0f0 {
		t.Fatalf("accent = %+v", accent)
	}
	if !background.Explicit || background.Alias != "paper" || background.Color.Hex() != 0x010203 {
		t.Fatalf("base.background = %+v", background)
	}
	if !text.Explicit || text.Alias != "ink" || text.Color.Hex() != 0xf0f0f0 {
		t.Fatalf("base.text = %+v", text)
	}
	ink, paper := options[3], options[4]
	if !ink.IsAlias || ink.Path != "$.aliases.ink" || ink.Label != "@ink" || ink.UsedBy != 2 || ink.section() != "aliases" {
		t.Fatalf("ink alias row = %+v", ink)
	}
	if !paper.IsAlias || paper.Label != "@paper" || paper.UsedBy != 1 || paper.Color.Hex() != 0x010203 {
		t.Fatalf("paper alias row = %+v", paper)
	}
}

func TestSetAliasColorChangesEveryColorUsingIt(t *testing.T) {
	path := writeAssignments(t, aliasedAssignments)
	document, err := loadAssignments(path, testSpecification())
	if err != nil {
		t.Fatal(err)
	}
	if err := document.setAliasColor("ink", tcell.NewHexColor(0x123456)); err != nil {
		t.Fatal(err)
	}
	for _, colorPath := range []string{"base.text", "accent"} {
		if got := document.color(colorPath, tcell.ColorDefault).Hex(); got != 0x123456 {
			t.Fatalf("%s = %#x after changing @ink, want 0x123456", colorPath, got)
		}
	}
	if got := document.color("base.background", tcell.ColorDefault).Hex(); got != 0x010203 {
		t.Fatalf("base.background changed to %#x", got)
	}
	if err := document.save(); err != nil {
		t.Fatal(err)
	}
	saved := readFile(t, path)
	// The reference stays a reference; only the alias definition changes.
	for _, want := range []string{`"ink": "#123456"`, `"text": "@ink"`, `"paper": "#010203"`} {
		if !strings.Contains(saved, want) {
			t.Fatalf("saved file does not contain %s:\n%s", want, saved)
		}
	}
	if strings.Index(saved, `"$"`) > strings.Index(saved, `"base"`) {
		t.Fatalf("$ member moved:\n%s", saved)
	}
	if err := document.setAliasColor("missing", tcell.NewHexColor(1)); err == nil {
		t.Fatal("setAliasColor accepted an undefined alias")
	}
	if err := document.setAliasColor("ink", tcell.ColorDefault); err == nil {
		t.Fatal("setAliasColor accepted the default color")
	}
}

func TestSetAliasAssignsReferenceAndLiteralReplacesIt(t *testing.T) {
	path := writeAssignments(t, aliasedAssignments)
	document, err := loadAssignments(path, testSpecification())
	if err != nil {
		t.Fatal(err)
	}
	if err := document.setAlias("base.text", "paper"); err != nil {
		t.Fatal(err)
	}
	if _, alias := document.colorAndAlias("base.text"); alias != "paper" {
		t.Fatalf("base.text alias = %q, want paper", alias)
	}
	if err := document.setAlias("base.text", "missing"); err == nil {
		t.Fatal("setAlias accepted an undefined alias")
	}
	if err := document.setColor("base.text", tcell.NewHexColor(0xabcdef)); err != nil {
		t.Fatal(err)
	}
	color, alias := document.colorAndAlias("base.text")
	if alias != "" || color.Hex() != 0xabcdef {
		t.Fatalf("base.text = %#x alias %q after literal assignment", color.Hex(), alias)
	}
	if err := document.save(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFile(t, path), `"text": "#abcdef"`) {
		t.Fatalf("literal was not saved:\n%s", readFile(t, path))
	}
}

func TestInvalidAliasDefinitionsAreRejected(t *testing.T) {
	const base = `"base": {"background": "#010203", "text": "#f0f0f0"}`
	for name, contents := range map[string]string{
		"unknown reference":     `{"$": {"aliases": {"a": "#111111"}}, "base": {"background": "@missing", "text": "#f0f0f0"}}`,
		"empty reference":       `{"base": {"background": "@", "text": "#f0f0f0"}}`,
		"alias of alias":        `{"$": {"aliases": {"a": "#111111", "b": "@a"}}, ` + base + `}`,
		"alias bad name":        `{"$": {"aliases": {"bad name": "#111111"}}, ` + base + `}`,
		"alias bad color":       `{"$": {"aliases": {"a": "notacolor"}}, ` + base + `}`,
		"alias not a string":    `{"$": {"aliases": {"a": 7}}, ` + base + `}`,
		"meta not object":       `{"$": "x", ` + base + `}`,
		"aliases not object":    `{"$": {"aliases": []}, ` + base + `}`,
		"wrong version":         `{"$": {"version": "2"}, ` + base + `}`,
		"numeric version":       `{"$": {"version": 3}, ` + base + `}`,
		"reference without $":   `{"base": {"background": "@paper", "text": "#f0f0f0"}}`,
		"alias with whitespace": `{"$": {"aliases": {"a": "#111111"}}, "base": {"background": "@ a", "text": "#f0f0f0"}}`,
	} {
		if _, err := loadAssignments(writeAssignments(t, contents), testSpecification()); err == nil {
			t.Errorf("%s: invalid document was accepted", name)
		}
	}
}

func TestFileWithoutMetaHasNoAliases(t *testing.T) {
	document, err := loadAssignments(writeAssignments(t, `{"base":{"background":"#010203","text":"#f0f0f0"}}`), testSpecification())
	if err != nil {
		t.Fatal(err)
	}
	options, err := document.options()
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range options {
		if option.IsAlias {
			t.Fatalf("unexpected alias row %+v", option)
		}
	}
	if len(document.aliasEntries()) != 0 {
		t.Fatal("aliasEntries is not empty")
	}
	if err := document.setAlias("base.text", "paper"); err == nil {
		t.Fatal("setAlias succeeded without any aliases")
	}
}

func TestMetaVersionIsOptionalButChecked(t *testing.T) {
	const base = `"base": {"background": "@paper", "text": "#f0f0f0"}`
	if _, err := loadAssignments(writeAssignments(t, `{"$": {"aliases": {"paper": "#010203"}}, `+base+`}`), testSpecification()); err != nil {
		t.Fatalf("aliases without a version: %v", err)
	}
	if _, err := loadAssignments(writeAssignments(t, `{"$": {"version": "3", "note": "kept", "aliases": {"paper": "#010203"}}, `+base+`}`), testSpecification()); err != nil {
		t.Fatalf("version 3 with unrelated member: %v", err)
	}
}

func TestAliasNamesAcceptLettersDigitsUnderscoreAndDash(t *testing.T) {
	for name, want := range map[string]bool{
		"background": true, "on-accent": true, "Text_2": true,
		"": false, "a b": false, "a.b": false, "@a": false, "é": false,
	} {
		if got := validAliasName(name); got != want {
			t.Errorf("validAliasName(%q) = %v, want %v", name, got, want)
		}
	}
}
