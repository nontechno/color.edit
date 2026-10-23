# Declarative Preview Specification

This document defines version 1 of the `color.edit` declarative preview format.
It is intended to be sufficiently precise for a coding agent to implement a
compatible specification generator, validator, renderer, or color editor
without depending on any particular application's internals.

The normative Go data model and validator are in
[`spec.go`](../spec/spec.go). If this document and the implementation differ,
the versioned data model and its tests are the executable source of truth.

## 1. Purpose and separation of concerns

A color-editing session uses two independent JSON documents:

1. A **preview specification** describes editable color names, the preview
   canvas, the visual elements drawn on it, resize behavior, fallbacks, and the
   colors used by the editor's own interface.
2. A **color-assignment document** contains actual color values at the
   dot-separated paths declared by the specification. It may also contain
   arbitrary application settings which the editor must preserve.

The preview specification contains no database-specific behavior and does not
execute application code. The assignment document contains no preview layout.
This separation lets the same `color.edit` executable edit themes for unrelated
terminal applications.

Color aliases (`@name` references resolved through a `$.aliases` table) belong
to the assignment document, whose format version is `3`; see
[`color-scheme-file-format.md`](color-scheme-file-format.md). They do
not change this preview format, which remains version 1.

The format is designed for representative visual states, not for recreating a
running application. A generator should include enough representative elements
that every editable color visibly affects the preview.

## 2. Terminology

The words **must**, **must not**, **should**, and **may** describe normative
requirements.

- **Logical cell**: one coordinate unit in the specification's canvas.
- **Terminal cell**: one cell in the currently available preview rectangle.
- **Color path**: a dot-separated path into the assignment JSON, such as
  `dialog.button.selected_text`.
- **Element**: one ordered drawing instruction in the preview.
- **Explicit value**: a color present directly at its declared assignment path.
- **Inherited value**: a missing color resolved through `fallback_path`.
- **Editor chrome**: the color editor's list, status line, dialogs, and buttons.

## 3. Encoding and loading

- A specification must be a single UTF-8 JSON object.
- The current loader reads at most 4 MiB.
- Unknown JSON fields are rejected.
- A second JSON value after the top-level object is rejected.
- All coordinates and dimensions are integers.
- Version 1 uses the top-level shape below:

```json
{
  "version": 1,
  "name": "application-name",
  "canvas": {},
  "colors": [],
  "editor": {},
  "elements": []
}
```

## 4. Top-level fields

### `version`

- Type: integer
- Required value for this specification: `1`
- Any other value must be rejected. A future incompatible format must use a
  different version and explicit migration or dispatch logic.

### `name`

- Type: string
- Must contain at least one non-whitespace character.
- Identifies the application or preview to users and incident reports.
- It is descriptive and does not select executable behavior.

### `canvas`

Defines the logical coordinate system and resize behavior. See section 5.

### `colors`

Defines every editable color. At least one entry is required. See section 6.

### `editor`

Maps the generic editor's own chrome to declared color paths. See section 7.

### `elements`

Contains ordered drawing instructions. Later elements overlay earlier ones.
See sections 8 and 9.

## 5. Canvas

```json
{
  "width": 120,
  "height": 38,
  "resize": "scale",
  "background": "pane.background"
}
```

### Fields

- `width`: logical width, inclusive range 20 through 500.
- `height`: logical height, inclusive range 8 through 200.
- `resize`: must be `scale` or `clip`.
- `background`: optional color path used to clear the preview before drawing
  elements. When present, it must refer to a declared color.

### `scale` resize policy

`scale` proportionally maps each logical rectangle to the complete available
preview rectangle. Horizontal and vertical axes are scaled independently; this
is cell-coordinate scaling, not image interpolation.

For an element `(ex, ey, ew, eh)`, canvas `(cw, ch)`, and terminal preview
rectangle `(px, py, pw, ph)`, implementations must calculate:

```text
left   = px + floor(ex        * pw / cw)
top    = py + floor(ey        * ph / ch)
right  = px + floor((ex + ew) * pw / cw)
bottom = py + floor((ey + eh) * ph / ch)
```

If `right <= left` or `bottom <= top`, the corresponding end must be increased
by one terminal cell before clipping to the preview bounds. Thus, a valid
logical element remains visible whenever at least one terminal cell is
available for it.

A logical dimension of exactly one cell is a terminal stroke and remains one
terminal cell after scaling. Its position and the element's other dimension
still scale normally. This keeps headers, status lines, separators, and
scrollbar thumbs faithful instead of making them two or more cells thick in a
large preview. A one-cell-wide element whose logical right edge equals the
canvas width is anchored to the scaled rightmost cell; this prevents a
right-frame scrollbar from drifting into the pane when the preview grows.

Text glyphs themselves are not geometrically scaled. Only their containing
rectangle and origin are scaled. Text may therefore be clipped in a small
terminal.

### Preview hit testing

The editor supports selecting assignments directly from the rendered preview.
A click resolves the colors that visibly paint that terminal cell after all
ordered elements have been composited:

- the topmost visible glyph supplies the foreground binding;
- the last fill or frame that paints the cell supplies the background binding;
- a blank fill contributes only its background;
- bindings hidden by a later element are excluded.

The foreground is selected first. Repeated clicks at the same cell cycle
through the distinct applicable bindings, including the background, and then
wrap to the first binding. Clicking the preview must not take keyboard focus
away from the color list.

### `clip` resize policy

`clip` maps one logical cell to one terminal cell, anchored at the preview's
top-left corner:

```text
left = px + ex
top  = py + ey
```

Element width and height are unchanged, then clipped at the preview's right and
bottom boundaries. Elements beginning entirely outside the available preview
are not drawn. Extra terminal space beyond the logical canvas remains filled
with the canvas background.

## 6. Color definitions

Each member of `colors` has this form:

```json
{
  "path": "dialog.background",
  "label": "Dialog background",
  "description": "Background of prompt and progress dialogs",
  "fallback_path": "pane.background",
  "uses": [
    "confirm-dialog-fill",
    "progress-dialog-fill"
  ]
}
```

### `path`

- Required and unique.
- A dot-separated sequence of non-empty segments.
- Each segment may contain ASCII letters, digits, `_`, or `-`.
- Whitespace, empty segments, and other punctuation are invalid.
- A path is traversed through nested JSON objects. For example,
  `dialog.button.text` resolves as:

```json
{
  "dialog": {
    "button": {
      "text": "#f0f0f0"
    }
  }
}
```

### `label`

- Required.
- Must contain at least one non-whitespace character.
- Intended for the human-facing color list.
- It should describe the semantic role rather than repeat a raw RGB value.

### `description`

- Optional.
- May provide more detailed application-specific guidance.
- It has no rendering or resolution semantics.

### `fallback_path`

- Optional.
- When the assignment document omits `path`, its value is resolved recursively
  from `fallback_path`.
- It must refer to another declared color.
- A color must not fall back to itself.
- Direct or indirect fallback cycles are invalid.
- Fallbacks allow older assignment files to remain compatible when new theme
  fields are introduced.
- A missing color without a fallback is a fatal assignment-loading error.

### `uses`

- Optional but recommended for generated specifications.
- Each entry must name an existing element ID.
- The named element must bind this color as its `foreground` or `background`.
- `uses` is explicit documentation for tools and coding agents. The renderer
  still derives actual behavior from element bindings.
- Independently of `uses`, every declared color must appear as a foreground or
  background binding on at least one preview element or editor-chrome field. A
  color with no visible use in either place makes the specification invalid.

## 7. Editor chrome bindings

The `editor` object allows the standalone editor to visually fit the supplied
assignment file without knowing application-specific path names.

```json
{
  "background": "pane.background",
  "text": "pane.text",
  "border": "pane.border",
  "title": "pane.title",
  "selected_background": "browser.selected_background",
  "selected_text": "browser.selected_text",
  "section_background": "browser.section.background",
  "section_text": "browser.section.text",
  "status_background": "status.background",
  "status_text": "status.info",
  "status_help": "status.help",
  "dialog_background": "dialog.background",
  "dialog_text": "dialog.text",
  "dialog_border": "dialog.border",
  "dialog_title": "dialog.title",
  "button_background": "dialog.button.background",
  "button_text": "dialog.button.text",
  "button_selected_background": "dialog.button.selected_background",
  "button_selected_text": "dialog.button.selected_text"
}
```

All editor fields are optional. Every non-empty value must refer to a declared
color. A compatible editor may use its own readable defaults for omitted
bindings. Version 1 defines these fields:

| Field | Use |
|---|---|
| `background` | Color-list and general editor background |
| `text` | Ordinary editor text |
| `border` | Editor pane borders |
| `title` | Editor pane titles |
| `selected_background` | Selected color-list row background |
| `selected_text` | Selected color-list row text |
| `section_background` | Non-selectable color-list section background |
| `section_text` | Non-selectable color-list section text |
| `status_background` | Bottom status background |
| `status_text` | Current color name, path, and value |
| `status_help` | Key usage and other supplemental editor text |
| `dialog_background` | Save/error dialog background |
| `dialog_text` | Save/error dialog message text |
| `dialog_border` | Save/error dialog border |
| `dialog_title` | Save/error dialog title |
| `button_background` | Unfocused dialog button background |
| `button_text` | Unfocused dialog button text |
| `button_selected_background` | Focused dialog button background |
| `button_selected_text` | Focused dialog button text |

## 8. Elements

Every element supports the following fields:

```json
{
  "id": "confirm-dialog-message",
  "kind": "text",
  "x": 24,
  "y": 8,
  "width": 32,
  "height": 1,
  "text": "Save changes?",
  "character": "",
  "align": "center",
  "fill": false,
  "foreground": "dialog.text"
}
```

### Common validation

- `id` is required, non-whitespace, and unique within `elements`.
- `kind` must be `fill`, `frame`, or `text`.
- `x` and `y` must be zero or positive.
- `width` and `height` must be at least 1.
- The complete logical rectangle must fit inside the logical canvas.
- `foreground` and `background` are optional. When non-empty, each must refer
  to a declared color.
- `align`, when non-empty, must be `left`, `center`, or `right`.
- Elements are drawn in array order. Later elements may intentionally replace
  characters, foreground colors, or backgrounds from earlier elements.

### Omitted colors

- An omitted `background` uses the resolved canvas background for operations
  that paint a background. Non-filling text preserves the background already
  present beneath it; generators should omit redundant background bindings
  from such text so `uses` and affected-area highlighting stay accurate.
- An omitted `foreground` uses `editor.text` when configured, otherwise the
  editor implementation's readable text default.

### `fill`

`fill` paints the entire scaled or clipped rectangle.

- `character` supplies the fill character.
- Only the first Unicode code point is used.
- An empty `character` means a space.
- Both foreground and background bindings affect the resulting terminal cells.

Example:

```json
{
  "id": "selected-row",
  "kind": "fill",
  "x": 1,
  "y": 2,
  "width": 38,
  "height": 1,
  "character": " ",
  "background": "browser.selected_background"
}
```

When `character` is blank or a space, its foreground is not visible. Generators
should omit that foreground binding and use a following `text` element for the
visible label. This keeps candidate-color previews and `uses` metadata from
highlighting areas the foreground does not actually paint.

### `frame`

`frame` draws a single-line Unicode rectangle using `┌─┐│└─┘`.

- The rectangle must scale or clip to at least 2 by 2 terminal cells to be
  visible.
- The interior is not filled.
- Use a preceding `fill` element when a framed region needs an independent
  interior background.

### `text`

`text` draws literal display text within the element rectangle.

- `text` may contain newlines. Each line consumes one row, up to `height`.
- Markup-like text must be treated literally; it must not inject editor style
  tags.
- `align` controls each line within the scaled width. The default is `left`.
- When `fill` is true, the entire rectangle is first filled with spaces using
  the element colors.
- When `fill` is false, text is drawn over the previously rendered background.
- Text is clipped to the element and terminal boundaries.

## 9. Rendering algorithm

A compatible renderer should perform these steps for every frame:

1. Determine the terminal rectangle allocated to the preview widget.
2. Remove the widget's own border/padding to obtain the drawable inner
   rectangle.
3. Resolve and fill the inner rectangle with `canvas.background`.
4. Iterate through `elements` in array order.
5. Map each logical rectangle using the selected resize policy.
6. Resolve foreground and background paths from the assignment document.
7. Apply any active candidate-color or flicker override for the exact bound
   path.
8. Draw according to `kind`.
9. Clip all writes to the drawable preview rectangle and terminal screen.

The renderer must not mutate the assignment document while previewing a
candidate swatch.

### Candidate-color preview

While a user moves through the swatch selector, all element properties bound to
the selected color path should resolve to the candidate color. Other paths are
unchanged. Canceling the selector removes the candidate without changing the
document; applying it writes the assignment. When the selector is editing an
alias, every binding whose color resolves through that alias, directly or by
fallback, should resolve to the candidate.

### Affected-area flicker

When the selected color option changes, implementations should temporarily
alternate every element binding for that exact path between its configured
color and a visibly contrasting color. The current `color.edit` implementation
uses the RGB inverse and toggles for one second. Fallback dependents are not
implicitly considered exact bindings; a generator should bind a color directly
where the editor is expected to highlight its use. When the selected option is an
alias, every binding that resolves through the alias is flickered.

## 10. Color-assignment document

The assignment document is deliberately not schema-locked by `color.edit`.
Applications may keep non-color settings alongside colors.

### Loading

- The top level must be a JSON object.
- The current implementation reads at most 4 MiB.
- Unrelated fields are retained.
- Each explicit declared path must resolve to a JSON string.
- Strings are parsed using tcell-compatible color syntax, including named
  colors and six-digit values such as `#d6e4f0`.
- A string beginning with `@` is an alias reference, resolved through the
  document's `$.aliases` table; an undefined alias is an error. An optional
  `$.version` must be `"3"`. See `color-scheme-file-format.md`.
- Invalid color strings and non-string values at declared paths are errors.
- Missing paths are resolved through `fallback_path` as described above.

### Displaying the current option

The editor should show:

- `label`;
- `path`;
- the resolved hexadecimal value, when available;
- whether the value is explicit or inherited;
- the alias that supplies the value, when there is one;
- the fallback source for inherited values;
- whether unsaved changes exist.

Options should be sorted alphabetically by `path`, regardless of their order in
the specification.

### Setting a value

Selecting a swatch writes a lowercase six-digit string such as `#123456` at the
declared path.

- Missing intermediate objects are created.
- If an intermediate component already exists but is not an object, the edit
  must fail rather than overwrite unrelated data.
- Choosing the same resolved color for an inherited option still creates an
  explicit value.
- Non-color fields must not be removed.
- Choosing an alias in the selector writes `@name` instead; choosing a palette
  color replaces any alias reference at the path.
- Editing an alias writes the six-digit string at `$.aliases.name`; every path
  that references the alias follows immediately.

### Saving

The current editor performs an atomic replacement:

1. Encode the complete assignment object as indented JSON.
2. Create a temporary file in the assignment file's directory.
3. Apply the existing file's permission bits, or `0644` for a new file.
4. Write, synchronize, and close the temporary file.
5. Rename it over the destination.
6. Remove the temporary file if any earlier step fails.

The editor prompts to Save, Discard, or Cancel when exiting with changes.

## 11. Complete minimal example

This example is valid because all three declared colors are visibly used.

```json
{
  "version": 1,
  "name": "sample-tui",
  "canvas": {
    "width": 80,
    "height": 24,
    "resize": "scale",
    "background": "pane.background"
  },
  "colors": [
    {
      "path": "dialog.background",
      "label": "Dialog background",
      "fallback_path": "pane.background",
      "uses": ["dialog-fill", "dialog-frame", "dialog-message"]
    },
    {
      "path": "pane.background",
      "label": "Pane background",
      "uses": ["canvas-fill"]
    },
    {
      "path": "pane.text",
      "label": "Pane text",
      "uses": ["canvas-fill", "dialog-message"]
    }
  ],
  "editor": {
    "background": "pane.background",
    "text": "pane.text",
    "dialog_background": "dialog.background",
    "dialog_text": "pane.text"
  },
  "elements": [
    {
      "id": "canvas-fill",
      "kind": "fill",
      "x": 0,
      "y": 0,
      "width": 80,
      "height": 24,
      "character": " ",
      "foreground": "pane.text",
      "background": "pane.background"
    },
    {
      "id": "dialog-fill",
      "kind": "fill",
      "x": 20,
      "y": 6,
      "width": 40,
      "height": 10,
      "character": " ",
      "foreground": "pane.text",
      "background": "dialog.background"
    },
    {
      "id": "dialog-frame",
      "kind": "frame",
      "x": 20,
      "y": 6,
      "width": 40,
      "height": 10,
      "foreground": "dialog.background",
      "background": "dialog.background"
    },
    {
      "id": "dialog-message",
      "kind": "text",
      "x": 23,
      "y": 10,
      "width": 34,
      "height": 1,
      "text": "Save changes?",
      "align": "center",
      "foreground": "pane.text",
      "background": "dialog.background"
    }
  ]
}
```

Example assignment document:

```json
{
  "name": "sample-light",
  "pane": {
    "background": "#eeeeee",
    "text": "#101010"
  },
  "application_setting_not_owned_by_color_edit": true
}
```

Here `dialog.background` is inherited from `pane.background` until the user
chooses an explicit dialog color.

## 12. Generator requirements

An application-specific generator should:

1. Emit version `1` explicitly.
2. Use a stable logical canvas and resize policy.
3. Emit stable, descriptive, unique element IDs.
4. Declare every editable assignment path exactly once.
5. Give every color a meaningful label.
6. Add fallbacks for fields introduced after older assignment files existed.
7. Bind every color to at least one representative element.
8. Populate `uses` from actual element bindings.
9. Map editor chrome only to declared paths.
10. Validate the completed specification before writing it.
11. Write the destination atomically when replacing an existing file.
12. Avoid embedding credentials, database values, or other sensitive content in
    the preview.

## 13. Compatibility and versioning

- Adding elements, labels, descriptions, or `uses` entries is compatible as
  long as existing semantics do not change.
- Adding new optional editor bindings is compatible within version 1 when old
  implementations can safely ignore them. The current strict decoder rejects
  unknown fields, so the implementation and versioned schema must be updated
  together before emitting such fields.
- Removing or renaming color paths can make assignment files incompatible and
  should be avoided. Prefer a new path with a fallback.
- Changing coordinate semantics, draw-order semantics, or the meaning of an
  existing element kind requires a new specification version.

## 14. Error handling and diagnostics

Load, validation, rendering initialization, mutation, and save errors should be
reported with enough operation context to diagnose the failure. Incident
reports must not include assignment contents because unrelated fields may be
sensitive. Safe context includes the specification name, file basename,
operation, runtime version, OS, architecture, and a sanitized stack trace for
unexpected panics.

## 15. Recommended conformance tests

A compatible implementation should test at least:

- rejection of unknown fields and unsupported versions;
- canvas dimension and resize-policy validation;
- duplicate color paths and element IDs;
- missing, self-referential, unknown, and cyclic fallbacks;
- unknown element color bindings;
- rejection of colors with no visible elements;
- verification of declared `uses` entries;
- `scale` rendering at two or more terminal sizes;
- `clip` rendering at smaller and larger sizes;
- element draw order and overlay behavior;
- candidate preview without assignment mutation;
- exact-path affected-area flicker;
- named and hexadecimal assignment colors;
- preservation of unrelated assignment fields;
- explicit creation from an inherited value;
- failure when an intermediate assignment path is not an object;
- atomic save and permission preservation;
- sanitized incident reporting for malformed files and unexpected crashes.
