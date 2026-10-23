package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func decodeForTest(t *testing.T, text string) any {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	value, err := decodeOrdered(decoder)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestOrderedJSONRoundTripKeepsOrderNumbersAndStructure(t *testing.T) {
	input := `{"z":1.50,"a":[1,{"b":null,"a":true}],"m":{},"e":[],"s":"x<y>&é","big":12345678901234567890}`
	want := "{\n" +
		"  \"z\": 1.50,\n" +
		"  \"a\": [\n    1,\n    {\n      \"b\": null,\n      \"a\": true\n    }\n  ],\n" +
		"  \"m\": {},\n" +
		"  \"e\": [],\n" +
		"  \"s\": \"x<y>&é\",\n" +
		"  \"big\": 12345678901234567890\n" +
		"}"
	got, err := encodeOrdered(decodeForTest(t, input), "  ")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("encoded =\n%s\nwant\n%s", got, want)
	}
}

func TestOrderedObjectDuplicateKeyKeepsFirstPositionAndLastValue(t *testing.T) {
	object := decodeForTest(t, `{"a":1,"b":2,"a":3}`).(*orderedObject)
	if strings.Join(object.keys, ",") != "a,b" {
		t.Fatalf("keys = %v", object.keys)
	}
	if value, _ := object.get("a"); value != json.Number("3") {
		t.Fatalf("a = %v, want 3", value)
	}
}

func TestDetectLayout(t *testing.T) {
	for _, test := range []struct {
		name   string
		data   string
		indent string
		crlf   bool
	}{
		{"two spaces", "{\n  \"a\": 1\n}\n", "  ", false},
		{"tabs", "{\n\t\"a\": 1\n}\n", "\t", false},
		{"four spaces crlf", "{\r\n    \"a\": 1\r\n}\r\n", "    ", true},
		{"minified", `{"a":1}`, "  ", false},
		{"no indentation", "{\n\"a\": 1\n}\n", "  ", false},
	} {
		indent, crlf := detectLayout([]byte(test.data))
		if indent != test.indent || crlf != test.crlf {
			t.Errorf("%s: detectLayout = %q, %v; want %q, %v", test.name, indent, crlf, test.indent, test.crlf)
		}
	}
}
