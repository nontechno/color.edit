package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func TestParseArgumentsClassifiesUsageErrors(t *testing.T) {
	var output bytes.Buffer
	for _, test := range []struct {
		name     string
		args     []string
		reported bool
	}{
		{"unknown flag", []string{"--bogus"}, true},
		{"missing colors", []string{"--spec", "s.json"}, false},
		{"extra argument", []string{"--spec", "s.json", "--colors", "c.json", "extra"}, false},
	} {
		_, err := parseArguments(test.args, &output)
		var usage *usageError
		if !errors.As(err, &usage) || usage.reported != test.reported {
			t.Errorf("%s: err = %#v, want usageError with reported=%v", test.name, err, test.reported)
		}
	}
	if _, err := parseArguments([]string{"--help"}, &output); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("--help error = %v, want flag.ErrHelp", err)
	}
}

func TestUsageErrorsDoNotWriteIncidentReports(t *testing.T) {
	reports := filepath.Join(t.TempDir(), "reports")
	if code := execute([]string{"--report-dir", reports}); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if code := execute([]string{"--report-dir", reports, "--bogus"}); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	entries, err := os.ReadDir(reports)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("usage errors wrote %d incident report(s)", len(entries))
	}
}
