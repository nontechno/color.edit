# color.edit specifications

Everything an external application needs to produce, read or edit color.edit
files. The documents are normative; the Go package in [`../spec/`](../spec) is the
reference implementation of the layout format.

For a task-oriented walkthrough (including upgrading to version 3 with `$`
aliases), start with [`INTEGRATING.md`](../INTEGRATING.md).

## Which document do I need?

| If your application... | Read |
| --- | --- |
| reads a colors file and resolves `#RRGGBB` / `@alias` values | [Color scheme file format](color-scheme-file-format.md) (version 3) |
| ships a layout so users can edit its colors with color.edit | the above, plus [Declarative layout file format](declarative-layout-file-format.md) (version 1) |
| implements its own editor, renderer or preview | all of the above, plus [Declarative preview](declarative-preview.md) |

## Documents

- [`color-scheme-file-format.md`](color-scheme-file-format.md): the user-owned
  colors file: the `$` member and aliases (§3.1), color literals (§5),
  resolution (§6), editing and saving (§7), compatibility (§8), conformance
  tests (§10).
- [`declarative-layout-file-format.md`](declarative-layout-file-format.md): the
  application-owned layout: color definitions and `fallback_path` (§7),
  elements (§9), rendering (§11), versioning (§17), conformance tests (§19).
- [`declarative-preview.md`](declarative-preview.md): the preview and editor
  behavior built on the layout, including how the colors document is loaded,
  displayed, changed and saved (§10).
- [`spec.go`](../spec/spec.go): the Go data model and validator for layouts. The data
  model and its tests are the executable source of truth when a document is unclear
  ([`spec_test.go`](../spec/spec_test.go)).

## Samples

- Layout: [`../cmap`](../cmap)
- Colors files with aliases: [`../light.json`](../light.json),
  [`../examples/themes/`](../examples/themes)

## Versions

| Format | Version | Notes |
| --- | --- | --- |
| Colors file | 3 | `$` member with `version` `"3"` and `aliases`; files without `$` remain valid |
| Layout | 1 | unchanged by aliases |
