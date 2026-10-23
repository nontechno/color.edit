package main

import (
	"os"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSavePreservesMemberOrderIndentAndHTMLCharacters(t *testing.T) {
	path := writeAssignments(t, "{\n\t\"z\": 1,\n\t\"name\": \"a<b>&\",\n\t\"base\": {\n\t\t\"text\": \"#f0f0f0\",\n\t\t\"background\": \"#010203\"\n\t}\n}\n")
	document, err := loadAssignments(path, testSpecification())
	if err != nil {
		t.Fatal(err)
	}
	if err := document.setColor("base.background", tcell.NewHexColor(0x123456)); err != nil {
		t.Fatal(err)
	}
	if err := document.setColor("accent", tcell.NewHexColor(0xabcdef)); err != nil {
		t.Fatal(err)
	}
	if err := document.save(); err != nil {
		t.Fatal(err)
	}
	want := "{\n\t\"z\": 1,\n\t\"name\": \"a<b>&\",\n\t\"base\": {\n\t\t\"text\": \"#f0f0f0\",\n\t\t\"background\": \"#123456\"\n\t},\n\t\"accent\": \"#abcdef\"\n}\n"
	if got := readFile(t, path); got != want {
		t.Fatalf("saved file =\n%q\nwant\n%q", got, want)
	}
}

func TestSavePreservesWindowsLineEndings(t *testing.T) {
	path := writeAssignments(t, "{\r\n  \"base\": {\r\n    \"background\": \"#010203\",\r\n    \"text\": \"#f0f0f0\"\r\n  }\r\n}\r\n")
	document, err := loadAssignments(path, testSpecification())
	if err != nil {
		t.Fatal(err)
	}
	if err := document.setColor("accent", tcell.NewHexColor(0x123456)); err != nil {
		t.Fatal(err)
	}
	if err := document.save(); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)
	if strings.Count(got, "\n") != strings.Count(got, "\r\n") || !strings.HasSuffix(got, "}\r\n") {
		t.Fatalf("line endings were not preserved: %q", got)
	}
}

func TestSaveRefusesToOverwriteFileChangedOnDisk(t *testing.T) {
	path := writeAssignments(t, `{"base":{"background":"#010203","text":"#f0f0f0"}}`)
	document, err := loadAssignments(path, testSpecification())
	if err != nil {
		t.Fatal(err)
	}
	if err := document.setColor("accent", tcell.NewHexColor(0x123456)); err != nil {
		t.Fatal(err)
	}
	external := `{"base":{"background":"#111111","text":"#f0f0f0"},"edited_elsewhere":true}`
	if err := os.WriteFile(path, []byte(external), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := document.save(); err != errChangedOnDisk {
		t.Fatalf("save error = %v, want errChangedOnDisk", err)
	}
	if got := readFile(t, path); got != external {
		t.Fatalf("external edit was overwritten: %q", got)
	}
	entries, err := os.ReadDir(strings.TrimSuffix(path, "colors.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".color-edit-") {
			t.Fatalf("temporary file %s was left behind", entry.Name())
		}
	}
}

func TestSecondSaveSucceedsAfterFirst(t *testing.T) {
	path := writeAssignments(t, `{"base":{"background":"#010203","text":"#f0f0f0"}}`)
	document, err := loadAssignments(path, testSpecification())
	if err != nil {
		t.Fatal(err)
	}
	for _, color := range []int32{0x111111, 0x222222} {
		if err := document.setColor("accent", tcell.NewHexColor(color)); err != nil {
			t.Fatal(err)
		}
		if err := document.save(); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(readFile(t, path), "#222222") {
		t.Fatal("second save was not written")
	}
}

func TestSaveRecreatesRemovedFile(t *testing.T) {
	path := writeAssignments(t, `{"base":{"background":"#010203","text":"#f0f0f0"}}`)
	document, err := loadAssignments(path, testSpecification())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := document.save(); err != nil {
		t.Fatalf("save after removal: %v", err)
	}
}

func TestLoadRejectsNonObjectAndTrailingJSON(t *testing.T) {
	for name, contents := range map[string]string{
		"array":    `[]`,
		"null":     `null`,
		"trailing": `{"base":{"background":"#010203","text":"#f0f0f0"}} {}`,
		"empty":    ``,
	} {
		if _, err := loadAssignments(writeAssignments(t, contents), testSpecification()); err == nil {
			t.Errorf("%s: invalid assignment file was accepted", name)
		}
	}
}
