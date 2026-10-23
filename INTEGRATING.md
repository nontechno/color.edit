# Integrating applications with color.edit

A short guide for application authors and coding agents. The full specification
index is [`specs/README.md`](specs/README.md). This guide only says how the
pieces fit together and where to look. The rules themselves live in the
documents linked below and are not repeated here.

## The two files

| File | Owned by | Defines | Spec |
| --- | --- | --- | --- |
| Layout (`--spec`, e.g. [`cmap`](cmap)) | the application | which colors exist, how to preview them | [Declarative layout format](specs/declarative-layout-file-format.md), [Declarative preview](specs/declarative-preview.md), code in [`spec/spec.go`](spec/spec.go) |
| Colors (`--colors`, e.g. [`light.json`](light.json)) | the user | the color assigned to each declared path, plus optional `$` aliases | [Color scheme file format](specs/color-scheme-file-format.md) |

The editor is application-independent. Run it with
`color.edit --spec <layout> --colors <colors>` ([README](README.md)).

## Using it from your application

1. Ship a layout file that declares every color path your UI reads. Give
   optional paths a `fallback_path`
   ([layout §7.4](specs/declarative-layout-file-format.md),
   [compatibility rules](specs/color-scheme-file-format.md#8-compatibility-rules)).
2. Load the user's colors file with your own loader. Treat the `$` member as
   reserved metadata, not a color path
   ([§3.1](specs/color-scheme-file-format.md#31-the--member-and-aliases)).
3. Read each color through the resolution algorithm
   ([§6](specs/color-scheme-file-format.md#6-resolution-algorithm)): explicit
   value, then fallback, otherwise fail. Accept `#RRGGBB` and `@alias`
   ([§5](specs/color-scheme-file-format.md#5-color-literals)).
4. Launch `color.edit` to let the user change colors, then reload the file.
   The editor preserves member order, indentation and unrelated fields, and
   refuses to save over a file that changed on disk
   ([§7.3](specs/color-scheme-file-format.md#73-save-behavior)).

Minimal Go resolver for a stored value (no fallbacks; add them per §6):

```go
type Theme struct {
	Meta struct {
		Version string            `json:"version"`
		Aliases map[string]string `json:"aliases"`
	} `json:"$"`
}

func (t *Theme) Resolve(value string) (string, error) {
	value = strings.TrimSpace(value)
	if name, ok := strings.CutPrefix(value, "@"); ok {
		hex, found := t.Meta.Aliases[name]
		if !found {
			return "", fmt.Errorf("undefined alias %q", name)
		}
		return hex, nil
	}
	return value, nil // a literal such as "#ff8700"
}
```

Aliases hold plain colors only and never chain, so a single lookup is enough.
Validate the rest (version, alias names, reserved `$` paths) as described in
[§3.1](specs/color-scheme-file-format.md#31-the--member-and-aliases).

## Upgrading existing files to version 3 (`$` and aliases)

Existing files without `$` keep working unchanged. Adopt aliases only when you
want shared colors.

1. Make sure your own loader tolerates a top-level `$` member. A strict loader
   that rejects unknown members must be updated first
   ([§8](specs/color-scheme-file-format.md#8-compatibility-rules)).
2. Add `"$": {"version": "3", "aliases": {...}}` to the colors file. Version
   `"3"` is the only accepted value.
3. Name aliases by role (`body_text`, `frame`), not by appearance, so a theme
   change does not make the name wrong. Names follow
   [§3.1](specs/color-scheme-file-format.md#31-the--member-and-aliases).
4. Replace repeated literals with `@name`. The result must resolve to the same
   colors; compare before and after. A worked example is
   [`test/colors.json`](test/colors.json), and the format of the `$` object is
   shown in [examples/themes](examples/themes).
5. Keep the layout file at version 1. Aliases are not part of the layout
   ([layout §17](specs/declarative-layout-file-format.md)).

How aliases show up in the editor (alias rows, picker, live preview of an
alias change) is described in the [README](README.md#color-aliases) and
[declarative preview §10](specs/declarative-preview.md).

## Checking your integration

- Conformance expectations for loaders and editors:
  [color scheme §10](specs/color-scheme-file-format.md#10-required-conformance-tests)
  and [layout §19](specs/declarative-layout-file-format.md).
- Layout validity: run the validator in [`spec/spec.go`](spec/spec.go).
- Sample inputs: [`cmap`](cmap) with [`light.json`](light.json) and the
  [example themes](examples/themes).
