# nontechno/color.edit

`color.edit` is an application-independent terminal color-assignment editor. It
loads a declarative preview specification and a separate JSON assignment file:

```sh
./color.edit --spec cmap --colors light.json
```

The included `cmap` layout and `light.json` assignment provide a runnable
example. An application may supply any compatible version-1
layout and assignment pair.

Build it from this project root with `make build`, or directly without CGO:

```sh
CGO_ENABLED=0 go build -buildvcs=false -o color.edit .
```

Run the complete validation suite with `make test` and `make vet`. The project
is self-contained at the source level.

## Interaction

- Arrow keys select an alphabetically ordered color option.
- The selected option's affected preview elements blink for one second.
- Clicking a rendered preview element selects its applicable color in the
  Colors list. Repeated clicks on the same cell cycle through visible
  foreground and background settings.
- `Enter` opens the color selector at the bottom-right: the file's aliases (if
  it defines any) above a palette of distinct-color swatches. `Tab` switches
  between the two.
- Arrow keys move through the aliases or swatches; the candidate is shown live
  in the preview.
- `Enter` applies the highlighted alias or swatch and `Escape` cancels the
  selector.
- Aliases are listed in an `Aliases` group at the end of the Colors list. Select
  one and press `Enter` to change its color; every color that uses it changes
  immediately.
- `q`, `Escape`, `Ctrl-Q`, or `Ctrl-C` exits.
- Modified files prompt for Save, Discard, or Cancel. Saves are atomic and
  preserve existing permissions, member order, indentation, line endings, and
  unrelated JSON fields. A save is refused, and the file left untouched, if the
  file changed on disk after it was loaded.

The editable colors are grouped by the first component of their assignment
path (for example, `dialog`, `pane`, and `status`). These group names are
full-width, non-selectable section rows and are not color options themselves.
Their colors come from the optional `editor.section_background` and
`editor.section_text` bindings. The preview and the two editor panes
automatically resize with the terminal.
Each color occupies one line. Because the section heading already supplies the
top-level path component, that component is omitted from labels in the list
(`Browser Metadata Text` is displayed as `Metadata Text`). The status line
retains the complete friendly name, JSON path, resolved value, inheritance
source, and modification state. Section headings are capitalized and aligned
to the right. The Colors pane is sized from the widest rendered section or
color entry.

## Color aliases

A color file may define reusable named colors in a reserved top-level `$`
member (color-file format version 3). A color is then either a literal such as
`#d9d9d9` or a reference such as `@text`:

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

The editor uses the aliases already present in the file; it does not create,
rename, or delete them. Choosing a palette color for a path replaces its alias
reference with a literal. Alias values must be plain colors (aliases do not
refer to other aliases), and a reference to an undefined alias is an error. A
file without `$` is still valid. `$.version`, when present, must be `"3"`. The
full rules are in [`specs/color-scheme-file-format.md`](specs/color-scheme-file-format.md).

## Specification version 1

The normative, implementation-level contract is documented in
[`specs/declarative-preview.md`](specs/declarative-preview.md). The summary below is a quick
reference.

The standalone format specifications are also available in
[`specs/color-scheme-file-format.md`](specs/color-scheme-file-format.md) and
[`specs/declarative-layout-file-format.md`](specs/declarative-layout-file-format.md).

The top-level document contains:

- `version`: currently `1`.
- `name`: the application or preview name.
- `canvas`: logical `width`, `height`, a `resize` policy, and a color path for
  `background`. `scale` proportionally fits the canvas to the preview, while
  `clip` keeps one logical cell per terminal cell and clips overflow.
- `colors`: editable JSON paths, labels, optional descriptions, explicit
  preview-element `uses`, and optional `fallback_path` values for
  backward-compatible assignment files.
- `editor`: color-path bindings for list selections and sections, status,
  dialog, and button chrome.
- `elements`: ordered drawing instructions. Later elements overlay earlier
  elements.

Every color path is a dot-separated path into the assignment JSON. Every
declared color must be referenced by at least one preview element. Unknown
fields, duplicate paths or element IDs, invalid rectangles, unknown color
references, and unsupported specification versions are rejected.

### Elements

Element positions and multi-cell dimensions are proportionally scaled to the
current preview size. A one-cell dimension remains one terminal cell so TUI
headers, separators, status lines, and scrollbars retain their intended
thickness. A one-cell element on the logical right edge remains anchored to
the scaled right edge, keeping frame scrollbars inside their pane. Supported
`kind` values are:

- `fill`: fills the rectangle with `character`, or a space when omitted.
- `frame`: draws a single-line frame around the rectangle.
- `text`: draws `text`, optionally filling its rectangle first with `fill`.

Common fields are `id`, `x`, `y`, `width`, `height`, `foreground`, and
`background`. Text also accepts `align` values `left`, `center`, or `right`.
The foreground and background values are color paths declared in `colors`.

Example:

```json
{
  "version": 1,
  "name": "sample-app",
  "canvas": {
    "width": 80,
    "height": 24,
    "resize": "scale",
    "background": "pane.background"
  },
  "colors": [
    {"path": "pane.background", "label": "Pane background"},
    {"path": "pane.text", "label": "Pane text"},
    {
      "path": "dialog.background",
      "label": "Dialog background",
      "fallback_path": "pane.background"
    }
  ],
  "editor": {
    "background": "pane.background",
    "text": "pane.text"
  },
  "elements": [
    {
      "id": "canvas",
      "kind": "fill",
      "x": 0,
      "y": 0,
      "width": 80,
      "height": 24,
      "foreground": "pane.text",
      "background": "pane.background"
    },
    {
      "id": "dialog",
      "kind": "fill",
      "x": 20,
      "y": 6,
      "width": 40,
      "height": 10,
      "foreground": "pane.text",
      "background": "dialog.background"
    }
  ]
}
```

Startup failures and unexpected crashes produce sanitized incident reports in
`~/.local/state/color.edit/reports` by default. Override this with
`--report-dir`. Command-line usage errors are printed to standard error only and
do not create reports.
