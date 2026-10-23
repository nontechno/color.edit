# Declarative Layout File Format Specification

Status: normative description of version 1 of the application-independent
terminal color-preview format implemented by `color.edit`.

Version described: `1`, implementation state on 2026-10-23. Color aliases
(color-file format version 3, see `color-scheme-file-format.md`) do not change
the layout format, so the layout version remains `1`.

## 1. Purpose

A declarative layout tells a generic color editor:

- which color-assignment paths are editable;
- their user-facing names, descriptions, and fallback relationships;
- which assignment paths style the editor itself;
- the logical size and resize policy of a representative application preview;
- which ordered fill, frame, and text elements compose that preview; and
- which preview elements are expected to demonstrate each color.

It contains no executable code, database connection information, real database
content, or application-specific event handling. It is a representative visual
model, not a serialized live UI.

Actual color values are stored in a separate assignment document described in
`color-scheme-file-format.md`.

## 2. Normative implementation sources

The current executable contract is represented by:

- the Go data model and validator in [`spec.go`](../spec/spec.go);
- rendering and hit testing in `color.edit/preview.go`; and
- assignment resolution and saving in `color.edit/document.go`.

If a future implementation differs from this document, it must either correct
the implementation as a version-1 bug or introduce a new format version when
the semantic change is incompatible.

## 3. Terminology

- **Logical canvas**: the fixed coordinate system declared in `canvas`.
- **Logical cell**: one integer coordinate unit in that canvas.
- **Preview rectangle**: terminal cells allocated to the preview, excluding the
  editor widget's own border.
- **Color path**: a dot-separated lookup path into the assignment document.
- **Element**: one ordered drawing instruction.
- **Binding**: an element or editor property referring to a color path.
- **Editor chrome**: the generic editor's color list, frames, status line,
  dialogs, and buttons.
- **Candidate color**: the temporary swatch shown while a picker is open.

The words **must**, **must not**, **should**, and **may** are normative.

## 4. Encoding and loading

- A layout must contain exactly one UTF-8 JSON object.
- A second JSON value is invalid.
- The current loader reads at most 4 MiB.
- Unknown fields are rejected at every modeled object level.
- JSON comments and trailing commas are unsupported.
- Coordinates and dimensions are JSON integers.
- Version 1 has this top-level form:

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

## 5. Top-level object

| Field | Type | Requirement |
|---|---|---|
| `version` | integer | Required; must equal `1` |
| `name` | string | Required; must contain non-whitespace text |
| `canvas` | object | Required and validated as described in section 6 |
| `colors` | array | Required; must contain at least one color definition |
| `editor` | object | May contain any version-1 chrome bindings from section 8 |
| `elements` | array | Ordered drawing list; may be empty only if every color is visibly used by editor bindings |

`name` is descriptive. It is used in UI and diagnostics but does not dispatch
application code.

## 6. Canvas object

```json
{
  "width": 120,
  "height": 41,
  "resize": "scale",
  "background": "pane.background"
}
```

| Field | Type | Requirement |
|---|---|---|
| `width` | integer | Inclusive range 20–500 |
| `height` | integer | Inclusive range 8–200 |
| `resize` | string | Exactly `scale` or `clip` |
| `background` | string | Optional color path; if non-empty, it must be declared in `colors` |

The renderer clears the complete preview rectangle to `canvas.background`
before drawing elements. If the path is empty, the renderer uses the terminal's
default background.

Merely naming a color in `canvas.background` does **not** satisfy the validator's
visible-use requirement. That color must also appear in an element foreground,
an element background, or an `editor` binding. Generators normally include a
full-canvas fill bound to the same background.

## 7. Color definitions

Each `colors` entry has this shape:

```json
{
  "path": "dialog.background",
  "label": "Dialog Background",
  "description": "Background of prompt and progress dialogs",
  "fallback_path": "pane.background",
  "uses": ["dialog-fill", "dialog-frame"]
}
```

### 7.1 `path`

- Required and unique.
- Consists of dot-separated, non-empty segments.
- Each segment may contain only ASCII letters, digits, `_`, or `-`.
- Leading/trailing whitespace, whitespace inside a segment, empty segments, and
  other punctuation are invalid.
- Case is significant during assignment-object traversal.

Examples of valid paths are `accent`, `pane.background`, and
`dialog.button.selected_text`. Examples of invalid paths are `.accent`,
`pane..text`, `pane text`, and `pane/background`.

### 7.2 `label`

- Required and must contain non-whitespace text.
- Intended for display in the color list.
- Should use semantic words and conventional acronym capitalization, such as
  `JSON Key`, `DDL Keyword`, and `PEM Boundary`.

The current editor groups paths by their first segment and removes a matching
first word from the displayed label. For example, `Browser Metadata Text` for
`browser.metadata_text` is displayed as `Metadata Text` under the right-aligned
`Browser` section. This display compaction does not alter the stored label.

### 7.3 `description`

- Optional string.
- Provides application-specific guidance.
- Has no rendering, validation, fallback, or assignment semantics beyond being
  retained in the parsed specification.

### 7.4 `fallback_path`

- Optional color path.
- If non-empty, it must name another declared color.
- A color cannot fall back to itself.
- Direct and indirect fallback cycles are invalid.
- When an assignment omits `path`, the editor recursively resolves the value at
  `fallback_path`.
- A missing assignment without a fallback is a fatal assignment-loading error.

Fallbacks are intended to keep older color files usable when a new semantic
color is introduced. They do not cause candidate preview or flicker for the
fallback's dependent paths: those effects apply only to exact bindings.

### 7.5 `uses`

- Optional array of element IDs.
- Every ID must exist in `elements`.
- The named element must bind this exact color path as `foreground` or
  `background`.
- Duplicate entries are not explicitly rejected, but generators should not
  emit them.
- `uses` documents and audits intended placement. Rendering and hit testing use
  actual element bindings, not this list.

### 7.6 Visible-use invariant

Every declared color must be referenced by at least one element foreground,
element background, or non-empty editor-chrome binding. Otherwise validation
fails. A fallback relationship and `canvas.background` alone do not count as a
visible use.

## 8. Editor chrome object

`editor` maps the generic editor's own visuals to declared color paths. Every
field is optional. Every non-empty field must name a declared color.

```json
{
  "background": "pane.background",
  "text": "pane.text",
  "border": "pane.border",
  "title": "pane.title",
  "selected_background": "browser.selected_background",
  "selected_text": "browser.selected_text",
  "section_background": "pane.background",
  "section_text": "pane.title",
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

| Field | Semantic use |
|---|---|
| `background` | Color-list and general editor background |
| `text` | Ordinary color-list text and omitted element foreground fallback |
| `border` | Editor pane and color-picker borders |
| `title` | Editor pane and picker titles |
| `selected_background` | Complete width of selected color-list row |
| `selected_text` | Selected color-list item text |
| `section_background` | Full-width, non-selectable color-list section row |
| `section_text` | Right-aligned, capitalized section label |
| `status_background` | Bottom editor status-line background |
| `status_text` | Current color label, path, value, and modified state |
| `status_help` | Key usage and secondary status information |
| `dialog_background` | Save/error dialog background |
| `dialog_text` | Save/error dialog message text |
| `dialog_border` | Save/error dialog frame |
| `dialog_title` | Save/error dialog title |
| `button_background` | Unfocused dialog button background |
| `button_text` | Unfocused dialog button text |
| `button_selected_background` | Focused dialog button background |
| `button_selected_text` | Focused dialog button text |

When a binding is omitted, the current implementation uses readable built-in
fallbacks appropriate to that editor component. A generator should bind all
relevant fields so the editor accurately demonstrates the assignment document.

Editor-chrome bindings count as visible uses. A generator should not add fake
preview elements merely to repeat colors already shown in the editor chrome.

## 9. Element object

```json
{
  "id": "dialog-message",
  "kind": "text",
  "x": 24,
  "y": 8,
  "width": 32,
  "height": 1,
  "text": "Save changes?",
  "character": "",
  "align": "center",
  "fill": false,
  "foreground": "dialog.text",
  "background": "dialog.background"
}
```

### 9.1 Common fields and validation

| Field | Type | Requirement |
|---|---|---|
| `id` | string | Required, non-whitespace, unique across all elements |
| `kind` | string | Required; exactly `fill`, `frame`, or `text` |
| `x` | integer | Zero or greater |
| `y` | integer | Zero or greater |
| `width` | integer | At least 1 |
| `height` | integer | At least 1 |
| `text` | string | Used by `text`; optional otherwise |
| `character` | string | Used by `fill`; only its first Unicode code point is used |
| `align` | string | Empty/`left`, `center`, or `right` |
| `fill` | boolean | For `text`, whether to paint its complete rectangle first |
| `foreground` | string | Optional declared color path |
| `background` | string | Optional declared color path |

The full logical rectangle must fit within the canvas:

```text
0 <= x
0 <= y
1 <= width
1 <= height
x + width  <= canvas.width
y + height <= canvas.height
```

Elements are composited strictly in array order. Later elements may overwrite
glyphs, foreground, and backgrounds produced by earlier elements.

### 9.2 Omitted bindings

- An omitted element background resolves to the canvas background for an
  operation that paints a background.
- An omitted foreground resolves to `editor.text`; if that binding is also
  absent, the renderer uses its ordinary readable text default.
- A non-filling text element preserves the background already painted beneath
  its glyphs. It should normally omit `background`; declaring a redundant
  background inaccurately documents a visible use even though drawing does not
  paint it.

### 9.3 `fill`

A fill paints every cell in its mapped rectangle with one glyph and one style.

- The first Unicode code point of `character` is used.
- Empty `character` means a space.
- A blank or whitespace glyph has no visible foreground for hit-testing
  purposes; only its background is selectable.
- Generators should omit `foreground` for blank fills and add a separate text
  element where visible text is required.

### 9.4 `frame`

A frame draws a single-cell Unicode rectangle using `┌`, `─`, `┐`, `│`, `└`,
and `┘`.

- It draws only when its mapped rectangle is at least 2 cells wide and 2 cells
  high.
- It does not fill the interior.
- Its border cells use both foreground and background bindings.
- A region needing an independent interior background should have a preceding
  fill element.
- Titles must be separate text elements layered after the frame. A list section
  or selected item must be placed inside the list body, never in the top frame;
  the top frame should contain only the pane title.

### 9.5 `text`

- `text` is literal. Renderer-specific color/style markup must be escaped and
  must not execute.
- Newlines split the value into rows. At most `height` lines are drawn.
- Every line is clipped to `width` terminal cells.
- Empty or omitted `align` means left alignment.
- `center` and `right` align each line independently within the mapped width.
- If `fill` is true, the entire rectangle is first filled with spaces using the
  element's foreground and background, then text is drawn.
- If `fill` is false, only glyph cells are drawn and the existing background is
  preserved.

## 10. Resize semantics

The preview widget supplies an inner terminal rectangle `(px, py, pw, ph)`.
Scaling excludes the editor widget's own border.

### 10.1 `scale`

Horizontal and vertical axes are mapped independently. This is integer terminal
cell mapping, not image interpolation and not aspect-ratio preservation.

For logical element `(ex, ey, ew, eh)` and canvas `(cw, ch)`:

```text
left   = px + floor(ex        * pw / cw)
top    = py + floor(ey        * ph / ch)
right  = px + floor((ex + ew) * pw / cw)
bottom = py + floor((ey + eh) * ph / ch)
```

Then apply these rules in order:

1. If logical `width == 1`, keep the mapped width at exactly one terminal cell.
2. If that one-cell-wide element's logical right edge equals `canvas.width`,
   anchor it at the preview's rightmost cell. This is required for right-frame
   vertical scrollbars.
3. If logical `height == 1`, keep mapped height at exactly one terminal cell.
4. If either mapped end is not greater than its start, expand that dimension to
   one terminal cell.
5. Clip right and bottom edges to the preview rectangle.

One-cell rows and columns represent terminal chrome such as headers, status
lines, separators, and scrollbar tracks; they must not become thick bands when
the preview grows. Text glyphs are never geometrically scaled. Only their
origin and containing rectangle are mapped.

### 10.2 `clip`

Clip mode preserves logical size and anchors the canvas at the preview's
top-left corner:

```text
left = px + ex
top  = py + ey
```

The element retains its logical width and height, clipped at the preview's right
and bottom boundaries. An element beginning entirely outside the available
preview is not drawn. Extra terminal space beyond the logical canvas remains
filled with the canvas background.

## 11. Rendering algorithm

A compatible renderer must perform the following for every frame:

1. Apply editor-chrome colors to the preview widget border and title.
2. Obtain the widget's inner preview rectangle.
3. If width or height is non-positive, stop rendering the preview.
4. Resolve `canvas.background` and clear the complete inner rectangle with
   spaces using that background.
5. Iterate through `elements` in array order.
6. Map and clip each rectangle according to `canvas.resize`.
7. Skip an element whose mapped width or height is non-positive.
8. Resolve its color bindings from the assignment document.
9. Apply candidate or flicker substitution for an exact matching path or, when
   the substitution targets an alias, for every path whose color resolves
   through that alias.
10. Draw according to its kind while clipping every write to the preview and
    terminal screen.

The renderer must not mutate either input document.

## 12. Preview interactions

### 12.1 Candidate-color preview

While a swatch picker is open, every foreground or background bound to the
selected path is rendered with the candidate color. Other paths are unchanged.
Canceling removes the candidate. Applying it updates the assignment document.

The picker itself should be size-bounded and positioned away from the main
preview, currently at the bottom right. Its swatches show colors, not names or
RGB labels. Duplicate RGB values should be removed. The current palette has 247
distinct terminal colors.

When the assignment document defines aliases, the picker for a layout color
also lists them, as swatches labelled `@name`, above the palette; Tab switches
between the two. The candidate is the color of the highlighted alias or palette
entry, and applying an alias assigns `@name`.

When the selected row is itself an alias, the picker offers only the palette and
the candidate replaces that alias's color: every foreground or background whose
color resolves through the alias, directly or by fallback, is rendered with the
candidate, and other paths are unchanged.

### 12.2 Affected-area flicker

When the selected color-list item changes, the current editor flickers every
exact binding for that path for one second. It alternates the resolved color
with its 24-bit RGB inverse at 200 ms intervals and ends in the normal state.
Starting a new flicker cancels the visual effect of the prior sequence.

Fallback dependents are not automatically included because their element
bindings use different paths. When the selected row is an alias, every binding
whose color resolves through that alias flickers.

### 12.3 Mouse hit testing

Clicking the rendered layout selects the color that visibly owns the clicked
cell after ordered compositing:

1. Start with no foreground and `canvas.background` as the background.
2. Traverse elements in draw order and consider only mapped rectangles
   containing the cell.
3. A fill replaces the background. A non-whitespace fill glyph also replaces
   the foreground; a whitespace glyph clears the selectable foreground.
4. A frame affects the cell only when it lies on the mapped border. It replaces
   both foreground and background there.
5. Filling text replaces the background across its rectangle before glyph hit
   testing.
6. Text replaces the foreground only where a literal glyph is actually drawn,
   accounting for line, clipping, and alignment.
7. Return unique applicable paths in foreground-then-background order.

Repeated clicks at the same terminal coordinate cycle through those paths and
wrap. A different coordinate or changed applicable-path list resets the cycle.
Mouse-down and click events inside the preview are consumed, but preview clicks
must not steal keyboard focus from the color list.

## 13. Color-list behavior derived from the layout

- Sort entries lexicographically by `colors[].path`, not declaration order.
- Group entries by the first path segment.
- Insert a full-width, non-selectable section row only for a non-empty group.
- Capitalize the first character of the section name and align it right.
- Display one line per color. There is no secondary description line.
- Remove a case-insensitive leading section word from the label only when it is
  followed by a space. Thus `Browser Metadata Text` becomes `Metadata Text`
  inside `Browser`, while an unrelated custom label remains unchanged.
- Size the list to the widest compact entry or section label, including borders
  and the two-cell color swatch.
- Highlight the selected row across the complete list width.

Descriptions remain available to other compatible clients even though the
current one-line list does not show them.

## 14. Complete minimal example

This example is valid. All declared colors have a visible element or editor
binding, all element IDs are unique, and all rectangles fit the canvas.

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
      "label": "Dialog Background",
      "fallback_path": "pane.background",
      "uses": ["dialog-fill", "dialog-frame"]
    },
    {
      "path": "pane.background",
      "label": "Pane Background",
      "uses": ["canvas-fill"]
    },
    {
      "path": "pane.text",
      "label": "Pane Text",
      "uses": ["canvas-label", "dialog-message"]
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
      "background": "pane.background"
    },
    {
      "id": "canvas-label",
      "kind": "text",
      "x": 2,
      "y": 1,
      "width": 30,
      "height": 1,
      "text": "Sample application",
      "foreground": "pane.text"
    },
    {
      "id": "dialog-fill",
      "kind": "fill",
      "x": 20,
      "y": 6,
      "width": 40,
      "height": 10,
      "character": " ",
      "background": "dialog.background"
    },
    {
      "id": "dialog-frame",
      "kind": "frame",
      "x": 20,
      "y": 6,
      "width": 40,
      "height": 10,
      "foreground": "pane.text",
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
      "foreground": "pane.text"
    }
  ]
}
```

Compatible assignment document:

```json
{
  "name": "sample-light",
  "pane": {
    "background": "#eeeeee",
    "text": "#101010"
  }
}
```

`dialog.background` inherits `pane.background` until explicitly assigned.

## 15. Generator requirements

An application-specific generator must:

1. Emit `version: 1` explicitly.
2. Use a stable logical canvas and choose `scale` or `clip` deliberately.
3. Declare each editable color path exactly once.
4. Give each color a semantic, correctly capitalized label.
5. Add a fallback for a newly introduced path when a sensible older color can
   preserve compatibility.
6. Emit stable, descriptive, unique element IDs.
7. Bind every color visibly through an element or editor property.
8. Populate `uses` from actual exact element bindings.
9. Omit phantom bindings: do not attach foreground to blank fills or background
   to non-filling text merely for bookkeeping.
10. Represent each semantic visual in the correct location. Pane titles belong
    on top frames; list sections and selected rows belong inside list bodies;
    table headers belong below the top frame; vertical scrollbars belong in the
    right frame and must not extend beyond it.
11. Avoid unnecessary repeated uses. When a path is intentionally used more
    than once, tests or generator comments should justify each occurrence.
12. Use representative synthetic content only. Never embed credentials,
    connection strings, real database values, certificates, or other secrets.
13. Validate the completed structure before writing it.
14. Write atomically when replacing an existing destination.

## 16. Validation failures

A version-1 validator must reject:

- any version other than 1;
- blank `name`;
- canvas dimensions outside 20–500 by 8–200;
- resize modes other than `scale` and `clip`;
- zero color definitions;
- invalid or duplicate color paths;
- blank labels;
- unknown, self-referential, or cyclic fallbacks;
- an unknown `canvas.background`;
- blank or duplicate element IDs;
- unsupported element kinds;
- rectangles outside the logical canvas;
- alignment other than empty/left, center, or right;
- unknown element foreground or background paths;
- unknown non-empty editor binding paths;
- a declared color with no element or editor use;
- a `uses` ID that does not exist; and
- a `uses` ID whose element does not bind the declaring color.

## 17. Compatibility and versioning

- Adding or rearranging representative elements is version-1 compatible if it
  preserves coordinate and draw semantics.
- Adding labels, descriptions, or accurate `uses` entries is compatible.
- Adding a new color with a fallback can preserve assignment compatibility, but
  every consumer must also understand the new layout field before it is emitted
  because the current decoder rejects unknown JSON fields.
- Color aliases belong to the assignment document, not the layout: layouts
  neither declare nor reference aliases, so adding them does not change the
  layout version.
- Removing or renaming assignment paths is breaking for existing schemes.
- Changing scaling, clipping, compositing, hit-testing, or existing element-kind
  semantics requires a new layout version.
- A new optional JSON field still requires updating the strict decoder before a
  generator can safely emit it.

## 18. Error reporting and safety

- Loading errors should distinguish open, decode, validation, assignment
  resolution, mutation, and save phases.
- Diagnostic context may include the specification name, file basename,
  element ID, color path, runtime version, OS, and architecture.
- Reports must not contain assignment-file contents or sensitive source data.
- Unexpected panics should produce a sanitized stack trace and a nonzero exit.
- Invalid input must not partially overwrite either source file.

## 19. Required conformance tests

A compatible implementation should test at least:

- strict decoding, unknown-field rejection, and the 4 MiB limit;
- unsupported versions and blank names;
- every canvas boundary and both resize modes;
- duplicate paths and element IDs;
- path grammar;
- missing, unknown, self, and cyclic fallbacks;
- every editor binding and omitted-binding fallback;
- rejection of colors without visible element/editor uses;
- validation of `uses` against actual bindings;
- ordered overlays of fills, frames, and text;
- blank versus nonblank fill hit testing;
- filled and non-filling text behavior;
- multiline text, clipping, and each alignment;
- scale rendering at smaller, equal, and larger sizes;
- one-cell row/column preservation and right-edge anchoring;
- clip rendering when elements are partly or completely outside the preview;
- foreground-then-background click cycling without focus transfer;
- candidate preview without assignment mutation;
- exact-path flicker and cancellation by a newer selection;
- one-line sectioned color list behavior and full-row selection highlight;
- preservation of unrelated assignment fields;
- atomic save and permission preservation; and
- sanitized incident reports for malformed input and unexpected crashes.
