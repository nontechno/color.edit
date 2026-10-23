package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// orderedObject is a JSON object that remembers the order of its members, so a
// document can be edited and written back without reshuffling the user's file.
type orderedObject struct {
	keys   []string
	values map[string]any
}

func newOrderedObject() *orderedObject {
	return &orderedObject{values: make(map[string]any)}
}

func (o *orderedObject) get(key string) (any, bool) {
	value, ok := o.values[key]
	return value, ok
}

// object returns the member as an object, or nil when it is absent or not an
// object.
func (o *orderedObject) object(key string) *orderedObject {
	value, _ := o.get(key)
	object, _ := value.(*orderedObject)
	return object
}

// set replaces an existing member in place, or appends a new one.
func (o *orderedObject) set(key string, value any) {
	if _, exists := o.values[key]; !exists {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
}

// decodeOrdered reads one JSON value. Objects become *orderedObject, arrays
// []any, and numbers json.Number when the decoder has UseNumber set, so number
// text round-trips exactly.
func decodeOrdered(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return token, nil
	}
	switch delimiter {
	case '{':
		object := newOrderedObject()
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("unexpected object key %v", keyToken)
			}
			value, err := decodeOrdered(decoder)
			if err != nil {
				return nil, err
			}
			object.set(key, value)
		}
		if _, err := decoder.Token(); err != nil { // Closing brace.
			return nil, err
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			value, err := decodeOrdered(decoder)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if _, err := decoder.Token(); err != nil { // Closing bracket.
			return nil, err
		}
		return array, nil
	}
	return nil, fmt.Errorf("unexpected delimiter %q", delimiter)
}

// encodeOrdered writes value as indented JSON in the style of
// json.MarshalIndent, but keeps object member order and does not HTML-escape
// strings. It does not append a trailing newline.
func encodeOrdered(value any, indent string) ([]byte, error) {
	var buffer bytes.Buffer
	if err := writeOrdered(&buffer, value, indent, 0); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeOrdered(buffer *bytes.Buffer, value any, indent string, depth int) error {
	switch typed := value.(type) {
	case *orderedObject:
		if len(typed.keys) == 0 {
			buffer.WriteString("{}")
			return nil
		}
		buffer.WriteByte('{')
		for index, key := range typed.keys {
			if index > 0 {
				buffer.WriteByte(',')
			}
			buffer.WriteByte('\n')
			buffer.WriteString(strings.Repeat(indent, depth+1))
			if err := writeJSONString(buffer, key); err != nil {
				return err
			}
			buffer.WriteString(": ")
			if err := writeOrdered(buffer, typed.values[key], indent, depth+1); err != nil {
				return err
			}
		}
		buffer.WriteByte('\n')
		buffer.WriteString(strings.Repeat(indent, depth))
		buffer.WriteByte('}')
	case []any:
		if len(typed) == 0 {
			buffer.WriteString("[]")
			return nil
		}
		buffer.WriteByte('[')
		for index, element := range typed {
			if index > 0 {
				buffer.WriteByte(',')
			}
			buffer.WriteByte('\n')
			buffer.WriteString(strings.Repeat(indent, depth+1))
			if err := writeOrdered(buffer, element, indent, depth+1); err != nil {
				return err
			}
		}
		buffer.WriteByte('\n')
		buffer.WriteString(strings.Repeat(indent, depth))
		buffer.WriteByte(']')
	case json.Number:
		buffer.WriteString(typed.String())
	case string:
		return writeJSONString(buffer, typed)
	case bool:
		if typed {
			buffer.WriteString("true")
		} else {
			buffer.WriteString("false")
		}
	case nil:
		buffer.WriteString("null")
	default:
		return fmt.Errorf("unsupported JSON value of type %T", value)
	}
	return nil
}

func writeJSONString(buffer *bytes.Buffer, value string) error {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return err
	}
	buffer.Write(bytes.TrimRight(encoded.Bytes(), "\n"))
	return nil
}

// detectLayout reports the indentation and line-ending style of an existing
// JSON file so a save can match it. Files without indentation get two spaces.
func detectLayout(data []byte) (indent string, crlf bool) {
	indent = "  "
	newline := bytes.IndexByte(data, '\n')
	if newline < 0 {
		return indent, false
	}
	crlf = newline > 0 && data[newline-1] == '\r'
	end := newline + 1
	for end < len(data) && (data[end] == ' ' || data[end] == '\t') {
		end++
	}
	if end > newline+1 {
		indent = string(data[newline+1 : end])
	}
	return indent, crlf
}

var errTopLevelNotObject = errors.New("top-level value must be an object")
