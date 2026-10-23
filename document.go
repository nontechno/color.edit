package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"color.edit/spec"

	"github.com/gdamore/tcell/v2"
)

const (
	// maxAssignmentBytes is the largest color-assignment file the editor will
	// load.
	maxAssignmentBytes = 4 << 20

	// metaKey is the reserved top-level member that holds file metadata and the
	// alias table. Layout color paths cannot use it: path segments are limited
	// to letters, digits, '_' and '-'.
	metaKey = "$"
	// formatVersion is the color-file format version this editor reads and
	// writes. A file whose "$.version" is present must match it.
	formatVersion = "3"
	// aliasMarker prefixes an alias reference in an assignment, as in
	// "@background".
	aliasMarker = "@"
	// aliasPathPrefix prefixes the display path of an alias row in the editor.
	aliasPathPrefix = metaKey + ".aliases."
)

type colorOption struct {
	spec.ColorDefinition
	Color    tcell.Color
	Explicit bool
	// Alias is the alias that supplies Color, or "" when the value is a plain
	// color. For an alias row (IsAlias) it is the alias's own name.
	Alias string
	// IsAlias marks a row that edits an alias definition rather than a color
	// declared by the layout.
	IsAlias bool
	// UsedBy counts the layout colors that resolve through this alias. It is
	// only set for alias rows.
	UsedBy int
}

// section is the group heading the option is listed under.
func (o colorOption) section() string {
	if o.IsAlias {
		return "aliases"
	}
	section, _, _ := strings.Cut(o.Path, ".")
	if section == "" {
		return "other"
	}
	return section
}

// displayLabel is the label shown in the color list.
func (o colorOption) displayLabel() string {
	if o.IsAlias {
		return o.Label
	}
	return compactColorLabel(o.Label, o.Path)
}

// aliasEntry is one alias definition and its resolved color.
type aliasEntry struct {
	Name  string
	Color tcell.Color
}

// resolution is the outcome of resolving a declared color path.
type resolution struct {
	color    tcell.Color
	explicit bool
	// alias names the alias that supplied the color, or "" for a plain value.
	alias string
}

type assignmentDocument struct {
	path          string
	specification spec.Specification
	values        *orderedObject
	// indent and crlf record the loaded file's layout so a save matches it.
	indent string
	crlf   bool
	// loadedSum identifies the file contents the in-memory document came from;
	// save refuses to overwrite a file that no longer matches it.
	loadedSum [sha256.Size]byte
}

func loadAssignments(path string, specification spec.Specification) (*assignmentDocument, error) {
	data, err := readLimited(path)
	if err != nil {
		return nil, fmt.Errorf("open color assignments: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := decodeOrdered(decoder)
	if err != nil {
		return nil, fmt.Errorf("decode color assignments: %w", err)
	}
	values, ok := value.(*orderedObject)
	if !ok {
		return nil, fmt.Errorf("decode color assignments: %w", errTopLevelNotObject)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return nil, fmt.Errorf("decode color assignments: %w", err)
	}
	indent, crlf := detectLayout(data)
	document := &assignmentDocument{
		path: path, specification: specification, values: values,
		indent: indent, crlf: crlf, loadedSum: sha256.Sum256(data),
	}
	if err := document.validateMeta(); err != nil {
		return nil, err
	}
	for _, definition := range specification.Colors {
		if _, err := document.resolve(definition.Path, make(map[string]bool)); err != nil {
			return nil, fmt.Errorf("resolve %s: %w", definition.Path, err)
		}
	}
	return document, nil
}

// readLimited reads a whole file, failing if it is larger than
// maxAssignmentBytes.
func readLimited(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxAssignmentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxAssignmentBytes {
		return nil, fmt.Errorf("file is larger than %d MiB", maxAssignmentBytes>>20)
	}
	return data, nil
}

// validateMeta checks the optional "$" member: its version and alias table. A
// file without "$" is valid and simply has no aliases.
func (d *assignmentDocument) validateMeta() error {
	value, exists := d.values.get(metaKey)
	if !exists {
		return nil
	}
	meta, ok := value.(*orderedObject)
	if !ok {
		return fmt.Errorf("%s must be an object, got %T", metaKey, value)
	}
	if version, exists := meta.get("version"); exists {
		if text, ok := version.(string); !ok || text != formatVersion {
			return fmt.Errorf("unsupported color file version %v (expected %q)", version, formatVersion)
		}
	}
	aliases, exists := meta.get("aliases")
	if !exists {
		return nil
	}
	table, ok := aliases.(*orderedObject)
	if !ok {
		return fmt.Errorf("%s.aliases must be an object, got %T", metaKey, aliases)
	}
	for _, name := range table.keys {
		if !validAliasName(name) {
			return fmt.Errorf("invalid alias name %q: use letters, digits, '_' and '-'", name)
		}
		if _, err := d.aliasColor(name); err != nil {
			return err
		}
	}
	return nil
}

// validAliasName reports whether name can follow aliasMarker in a reference.
func validAliasName(name string) bool {
	if name == "" {
		return false
	}
	for _, character := range name {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

// aliasTable returns the alias definitions, or nil when the file has none.
func (d *assignmentDocument) aliasTable() *orderedObject {
	meta := d.values.object(metaKey)
	if meta == nil {
		return nil
	}
	return meta.object("aliases")
}

// aliasColor resolves the alias called name (without the marker). An alias holds
// a plain color, never another alias.
func (d *assignmentDocument) aliasColor(name string) (tcell.Color, error) {
	if !validAliasName(name) {
		return tcell.ColorDefault, fmt.Errorf("invalid alias reference %q", aliasMarker+name)
	}
	var value any
	found := false
	if table := d.aliasTable(); table != nil {
		value, found = table.get(name)
	}
	if !found {
		return tcell.ColorDefault, fmt.Errorf("unknown alias %q", aliasMarker+name)
	}
	text, ok := value.(string)
	if !ok {
		return tcell.ColorDefault, fmt.Errorf("alias %q must be a color string, got %T", aliasMarker+name, value)
	}
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, aliasMarker) {
		return tcell.ColorDefault, fmt.Errorf("alias %q must be a color, not another alias", aliasMarker+name)
	}
	color := tcell.GetColor(strings.ToLower(text))
	if !color.Valid() {
		return tcell.ColorDefault, fmt.Errorf("alias %q has invalid color %q", aliasMarker+name, text)
	}
	return color, nil
}

// aliasEntries lists the defined aliases in name order.
func (d *assignmentDocument) aliasEntries() []aliasEntry {
	table := d.aliasTable()
	if table == nil {
		return nil
	}
	names := append([]string(nil), table.keys...)
	sort.Strings(names)
	entries := make([]aliasEntry, 0, len(names))
	for _, name := range names {
		color, err := d.aliasColor(name)
		if err != nil {
			continue
		}
		entries = append(entries, aliasEntry{Name: name, Color: color})
	}
	return entries
}

func (d *assignmentDocument) options() ([]colorOption, error) {
	options := make([]colorOption, 0, len(d.specification.Colors))
	for _, definition := range d.specification.Colors {
		resolved, err := d.resolve(definition.Path, make(map[string]bool))
		if err != nil {
			return nil, err
		}
		options = append(options, colorOption{
			ColorDefinition: definition, Color: resolved.color, Explicit: resolved.explicit, Alias: resolved.alias,
		})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].Path < options[j].Path })
	usage := make(map[string]int)
	for _, option := range options {
		if option.Alias != "" {
			usage[option.Alias]++
		}
	}
	for _, entry := range d.aliasEntries() {
		options = append(options, colorOption{
			ColorDefinition: spec.ColorDefinition{
				Path:        aliasPathPrefix + entry.Name,
				Label:       aliasMarker + entry.Name,
				Description: "Alias " + aliasMarker + entry.Name,
			},
			Color: entry.Color, Explicit: true, Alias: entry.Name, IsAlias: true, UsedBy: usage[entry.Name],
		})
	}
	return options, nil
}

func (d *assignmentDocument) color(path string, fallback tcell.Color) tcell.Color {
	if strings.TrimSpace(path) == "" {
		return fallback
	}
	resolved, err := d.resolve(path, make(map[string]bool))
	if err != nil {
		return fallback
	}
	return resolved.color
}

// colorAndAlias returns the resolved color of path and the alias that supplies
// it. Unresolvable paths yield tcell.ColorDefault and no alias.
func (d *assignmentDocument) colorAndAlias(path string) (tcell.Color, string) {
	if strings.TrimSpace(path) == "" {
		return tcell.ColorDefault, ""
	}
	resolved, err := d.resolve(path, make(map[string]bool))
	if err != nil {
		return tcell.ColorDefault, ""
	}
	return resolved.color, resolved.alias
}

func (d *assignmentDocument) resolve(path string, visiting map[string]bool) (resolution, error) {
	if visiting[path] {
		return resolution{}, fmt.Errorf("fallback cycle at %s", path)
	}
	visiting[path] = true
	defer delete(visiting, path)
	if value, ok := getPath(d.values, path); ok {
		text, ok := value.(string)
		if !ok {
			return resolution{}, fmt.Errorf("assignment must be a color string, got %T", value)
		}
		text = strings.TrimSpace(text)
		if name, isAlias := strings.CutPrefix(text, aliasMarker); isAlias {
			color, err := d.aliasColor(name)
			if err != nil {
				return resolution{}, err
			}
			return resolution{color: color, explicit: true, alias: name}, nil
		}
		color := tcell.GetColor(strings.ToLower(text))
		if !color.Valid() {
			return resolution{}, fmt.Errorf("invalid color %q", text)
		}
		return resolution{color: color, explicit: true}, nil
	}
	for _, definition := range d.specification.Colors {
		if definition.Path == path {
			if definition.FallbackPath == "" {
				return resolution{}, errors.New("assignment is missing and has no fallback")
			}
			inherited, err := d.resolve(definition.FallbackPath, visiting)
			inherited.explicit = false
			return inherited, err
		}
	}
	return resolution{}, errors.New("unknown color path")
}

// setColor assigns a plain #rrggbb color to a declared path.
func (d *assignmentDocument) setColor(path string, color tcell.Color) error {
	if !color.Valid() || color.Hex() < 0 {
		return fmt.Errorf("%s cannot be set to an invisible/default color", path)
	}
	return d.setString(path, fmt.Sprintf("#%06x", color.Hex()))
}

// setAlias assigns an existing alias to a declared path.
func (d *assignmentDocument) setAlias(path, name string) error {
	if _, err := d.aliasColor(name); err != nil {
		return err
	}
	return d.setString(path, aliasMarker+name)
}

// setAliasColor changes the color of an existing alias. Every path that
// references the alias follows immediately.
func (d *assignmentDocument) setAliasColor(name string, color tcell.Color) error {
	if _, err := d.aliasColor(name); err != nil {
		return err
	}
	if !color.Valid() || color.Hex() < 0 {
		return fmt.Errorf("alias %q cannot be set to an invisible/default color", aliasMarker+name)
	}
	d.aliasTable().set(name, fmt.Sprintf("#%06x", color.Hex()))
	return nil
}

// setString writes text at a dotted path, creating missing intermediate
// objects.
func (d *assignmentDocument) setString(path, text string) error {
	parts := strings.Split(path, ".")
	cursor := d.values
	for _, part := range parts[:len(parts)-1] {
		value, exists := cursor.get(part)
		if !exists || value == nil {
			nested := newOrderedObject()
			cursor.set(part, nested)
			cursor = nested
			continue
		}
		nested, ok := value.(*orderedObject)
		if !ok {
			return fmt.Errorf("cannot create %s below non-object field %s", path, part)
		}
		cursor = nested
	}
	cursor.set(parts[len(parts)-1], text)
	return nil
}

// errChangedOnDisk is returned by save when the destination no longer holds the
// contents that were loaded.
var errChangedOnDisk = errors.New("color assignments changed on disk since they were loaded; save cancelled so those changes are not overwritten")

// checkUnchanged verifies the destination still matches what was loaded. A
// destination that has been removed is not a conflict.
func (d *assignmentDocument) checkUnchanged() error {
	current, err := readLimited(d.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect color assignments: %w", err)
	}
	if sha256.Sum256(current) != d.loadedSum {
		return errChangedOnDisk
	}
	return nil
}

func (d *assignmentDocument) save() error {
	if err := d.checkUnchanged(); err != nil {
		return err
	}
	data, err := encodeOrdered(d.values, d.indent)
	if err != nil {
		return fmt.Errorf("encode color assignments: %w", err)
	}
	data = append(data, '\n')
	if d.crlf {
		data = bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
	}
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(d.path); statErr == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect color assignments: %w", statErr)
	}
	directory := filepath.Dir(d.path)
	temporary, err := os.CreateTemp(directory, ".color-edit-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary color assignments: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return fmt.Errorf("set temporary assignment permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write color assignments: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync color assignments: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close color assignments: %w", err)
	}
	if err := os.Rename(temporaryPath, d.path); err != nil {
		return fmt.Errorf("replace color assignments: %w", err)
	}
	committed = true
	d.loadedSum = sha256.Sum256(data)
	return nil
}

func getPath(root *orderedObject, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var current any = root
	for _, part := range parts {
		object, ok := current.(*orderedObject)
		if !ok {
			return nil, false
		}
		current, ok = object.get(part)
		if !ok {
			return nil, false
		}
	}
	return current, true
}
