# Color Scheme File Format Specification

Status: normative description of the color-assignment format consumed by the
current `color.edit` implementation.

Format version described: `3` (the value of `$.version`), implementation state
on 2026-10-23. Version 3 adds the reserved `$` member and color aliases.
Documents without a `$` member remain valid and simply define no aliases.

## 1. Scope

A color-editing session has two separate inputs:

1. A **color scheme**, or assignment document, containing actual color values.
2. A **declarative layout specification** declaring which dot-separated paths
   are editable, how missing values inherit, and where each color appears in a
   representative preview.

This document specifies the first input. The companion
`declarative-layout-file-format.md` specifies the second.

The generic editor does not impose one fixed application schema on a color
scheme. The layout's `colors[].path` entries select values from an otherwise
ordinary JSON object. This permits an application to retain non-color settings
in the same file. An application may impose an additional strict schema of its
own when it loads a theme for runtime use. The one reserved member is the
top-level `$`, which carries format metadata and color aliases (section 3.1).

## 2. Terminology

- **Assignment path**: a dot-separated path declared by the layout, such as
  `table.header.background`.
- **Explicit value**: a color string physically present at its assignment path.
- **Inherited value**: a missing assignment resolved through the layout color's
  `fallback_path` chain.
- **Alias**: a named color defined once in `$.aliases` and referenced from
  assignments as `@name`.
- **Alias reference**: an assignment string of the form `@name`.
- **Unrelated value**: any JSON member not selected by a declared assignment
  path. The generic editor preserves these values.
- **Application profile**: additional validation imposed by the application
  that ultimately consumes the scheme.

The words **must**, **must not**, **should**, and **may** are normative.

## 3. Encoding and document shape

- The file must contain one UTF-8 JSON value.
- The top-level value must be an object.
- A second JSON value after the object is invalid.
- The current generic loader reads at most 4 MiB. Implementations should reject
  oversized input instead of silently parsing a prefix.
- The generic loader does not reject unknown members. It must preserve all
  unrelated members when saving.
- Object member order has no semantic meaning.
- JSON comments and trailing commas are not supported.

A minimal assignment document might be:

```json
{
  "name": "sample-light",
  "pane": {
    "background": "#eeeeee",
    "text": "#101010"
  },
  "application_setting_not_owned_by_the_editor": true
}
```

Whether this is complete depends on the color definitions and fallback chains
in the paired layout specification.

### 3.1 The `$` member and aliases

The top-level member named `$` is reserved for metadata. A layout cannot declare
a path that starts with `$`, because path segments are limited to letters,
digits, `_`, and `-`. Its shape is:

```json
{
  "$": {
    "version": "3",
    "aliases": {
      "background": "#ffff00",
      "text": "#d9d9d9"
    }
  },
  "banner": {
    "filter": {
      "background": "@background",
      "text": "@text"
    }
  }
}
```

- `$` is optional. When present it must be an object. Its members other than
  `version` and `aliases` are unrelated values and must be preserved.
- `$.version`, when present, must be the string `"3"`. Any other value,
  including the number `3`, is rejected. A missing version is accepted.
- `$.aliases`, when present, must be an object that maps alias names to colors.
- An alias name is one or more letters, digits, `_`, or `-`, and is
  case-sensitive.
- An alias value is a color literal (section 5). It must not itself be an alias
  reference: aliases do not chain.
- An assignment may be an alias reference, `@name`, where `name` is a defined
  alias. The `@` must be the first character after surrounding whitespace is
  trimmed, with no whitespace between it and the name.
- A reference to an undefined alias, including any reference in a document that
  defines no aliases, is an error.
- The editor does not create, rename, or delete aliases. The list of available
  aliases is expected to be in the file already; the editor changes an existing
  alias's color and assigns existing aliases to colors.

## 4. Assignment-path traversal

A path is split on `.` and each segment addresses a nested JSON object. For
example, `dialog.button.selected_text` addresses:

```json
{
  "dialog": {
    "button": {
      "selected_text": "#ffffff"
    }
  }
}
```

For every declared path:

- If the terminal member exists, its value must be a JSON string containing a
  valid color literal or an alias reference (section 3.1).
- If an intermediate member exists but is not an object, resolution fails.
- If the terminal member is absent, resolution follows the corresponding
  layout `fallback_path` recursively.
- If the member is absent and there is no fallback, loading fails.
- Fallback cycles are invalid in the layout and must also be guarded against at
  resolution time.

An explicit JSON `null` is not equivalent to an omitted member. At a declared
path it is a type error because assignments must be strings.

## 5. Color literals

The current implementation trims surrounding whitespace, converts the input to
lowercase, and passes it to `tcell.GetColor`.

Portable color schemes should use exactly one of:

- A six-digit RGB literal `#RRGGBB`, for example `#d6e4f0`. Hexadecimal digits
  are case-insensitive. The editor writes lowercase `#rrggbb`.
- A name recognized by tcell's W3C color-name table, for example `black`,
  `white`, `navy`, `aqua`, `orange`, `darkgray`, or `grey`.

The generic editor requires the resolved tcell color to be valid and visible.
It therefore does not accept `default`, `none`, or `reset` as portable editable
assignments. An application's own loader may separately accept the exact word
`default`, but the generic editor cannot load or select that value. Schemes
intended for both consumers must use named or six-digit RGB colors.

The following are invalid:

- three-, four-, or eight-digit hex forms such as `#fff` or `#112233ff`;
- CSS functions such as `rgb(1, 2, 3)`;
- numeric JSON values;
- arrays or objects at a declared path;
- unrecognized names.

Alpha/transparency is not represented.

Instead of a literal, an assignment may be an alias reference such as
`@background` (section 3.1). The alias supplies the literal.

## 6. Resolution algorithm

For declared color `C`, a compatible implementation must:

1. Look up `C.path` in the assignment object.
2. If present, require a string and trim surrounding whitespace. If it begins
   with `@`, look the remainder up in `$.aliases`; return that alias's color
   with `explicit = true` and the alias name as the supplying alias. An
   undefined alias is a fatal loading error. Otherwise parse the string as a
   color literal and return the parsed color with `explicit = true` and no
   supplying alias.
3. If absent and `C.fallback_path` is non-empty, resolve the fallback color by
   the same algorithm and return its color, and its supplying alias if any,
   with `explicit = false`.
4. If absent and no fallback exists, return a fatal loading error.
5. Track the active recursion chain and reject a fallback cycle even if layout
   validation was skipped.

The explicit/inherited state belongs to the originally requested path. If
`dialog.background` inherits from an explicit `pane.background`, the resolved
dialog value is still reported as inherited.

The supplying alias belongs to the value that ultimately provides the color. If
`dialog.background` inherits from `pane.background` and that is `@paper`, the
dialog value is inherited and reports `paper` as its alias.

Changing an alias's color changes every path that references it, directly or
through a fallback chain.

Changing a fallback source changes all descendants that remain omitted. Setting
an inherited path creates an explicit value and disconnects that path from
future changes to its fallback.

## 7. Editing and persistence

### 7.1 Listing

- Editable entries come from the paired layout, not from recursively discovering
  strings in the assignment document.
- Entries are sorted lexicographically by complete assignment path.
- The editor groups entries by the first path segment for display. Group labels
  are not assignment paths and must not be added to the scheme.
- A displayed entry should show its human label, path, resolved value, and
  whether it is explicit or inherited. An entry whose color comes from an alias
  also names that alias.
- Aliases are listed after the layout's colors, in a separate group headed
  `Aliases` and sorted by name. Each alias row is labelled `@name`, has the
  display path `$.aliases.name`, and reports how many layout colors resolve
  through it.

### 7.2 Mutation

When the user applies a color:

- The editor writes a lowercase six-digit RGB string at the declared path.
- Missing intermediate objects are created.
- Existing non-object intermediate values must not be overwritten; the edit
  fails with the conflicting path identified.
- Selecting the same resolved value for an inherited color still creates an
  explicit assignment.
- Unrelated JSON members must remain unchanged semantically.

When the file defines aliases, the color picker offers them together with the
palette:

- Choosing an alias writes `@name` at the declared path.
- Choosing a palette color writes a lowercase six-digit RGB string, replacing
  any alias reference at that path. Doing so for a path that currently holds an
  alias reference always changes the assignment, even when the palette color
  equals the alias's color.
- Choosing the alias that is already assigned changes nothing.

Editing an alias row opens the palette only; aliases cannot refer to aliases.
Choosing a color writes a lowercase six-digit RGB string at `$.aliases.name`.
The new color takes effect immediately everywhere the alias is used, in the
color list and the preview, and every reference stays a reference.

### 7.3 Save behavior

The current editor saves by atomic replacement:

1. If the destination exists, verify that its contents are still those that were
   loaded; otherwise cancel the save and leave the destination untouched. A
   destination that has been removed is not a conflict.
2. Encode the complete top-level object as JSON, preserving the order of its
   members, without HTML escaping, indented like the loaded file (two spaces
   when it had no indentation) and using its line-ending style, and append a
   newline.
3. Create a temporary file in the destination directory.
4. Reuse the destination's permission bits; use `0644` only when implementing
   creation of a previously absent destination.
5. Write, synchronize, and close the temporary file.
6. Rename it over the destination.
7. Remove the temporary file after any unsuccessful operation.

Members created by an edit are appended to the end of their parent object.

The UI must ask whether to Save, Discard, or Cancel when exiting with unsaved
changes. Previewing a candidate swatch must not mutate the document.

## 8. Compatibility rules

- Adding a declared path with a fallback is backward-compatible with older
  assignment documents.
- Adding a required path without a fallback makes older files fail to load.
- Renaming or deleting paths is breaking unless migration is provided.
- Changing a path from a color string to an object, or the reverse, is breaking
  for nested traversal.
- Adding a `$` member with aliases is compatible with this editor. A strict
  application loader that rejects unknown members may reject `$`, so generators
  should start from a file valid for the target application.
- A document that uses `$` should state `"version": "3"`. A different version
  value is rejected, so an incompatible future revision must use a new value.
- Layout and application fallbacks should agree.
- A standalone editor must preserve unrelated fields even if it does not
  understand the application profile.
- A strict application may reject unrelated fields after an editor preserves
  them; generators should therefore start from a file valid for the target
  application.

## 9. Security and diagnostics

- Do not include the full assignment document in incident reports because
  unrelated fields may contain sensitive application settings.
- Safe diagnostic context includes the operation, file basename, declared path,
  layout name, runtime version, OS, architecture, and a sanitized stack trace.
- Error messages should identify the failing path without echoing neighboring
  JSON values.
- Save failures must leave the original destination intact whenever atomic
  rename is supported.

## 10. Required conformance tests

A compatible implementation should cover at least:

- valid named and `#RRGGBB` colors;
- invalid names, malformed hex values, and non-string assignments;
- missing values with and without fallback;
- multi-hop fallback and cycle rejection;
- explicit versus inherited reporting;
- creation of missing intermediate objects;
- rejection of a non-object intermediate value;
- preservation of unrelated nested values and JSON numbers;
- creation of an explicit value equal to its inherited value;
- alphabetical path ordering and grouping by first segment;
- atomic save, newline termination, and permission preservation;
- preservation of member order, indentation, line endings, and HTML characters;
- refusal to save over a file that changed after it was loaded;
- alias resolution, including through fallback chains, and reporting of the
  supplying alias;
- rejection of undefined aliases, alias chains, invalid alias names, invalid
  alias colors, and unsupported `$.version` values;
- assigning an alias, assigning a literal over an alias reference, and editing
  an alias color so that every reference follows;
- preservation of `$` members unrelated to the editor;
- the 4 MiB input limit.
