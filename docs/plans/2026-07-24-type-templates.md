# TDD Plan — Increment 6: Type templates + `validate --templates` overlay

**Spec:** [`docs/specs/2026-07-24-type-templates.md`](../specs/2026-07-24-type-templates.md)
**Branch:** `topic/type-templates` off `main` @ 8a22307
**Execution:** sequential TDD (RED→GREEN→REFACTOR→commit), per-task ground-truth
verification + one whole-increment E2E review at the end. Same shape as 2a/3/4.

Pre-write the `// Copyright 2026 Google LLC` Apache header into every new `.go`
file so the addlicense pre-commit hook is a no-op.

## Task 1 — `Templates()` + `TemplateDrift()` pure model
**RED:** `internal/okf/template_test.go`
- `TestTemplates_FoldsByTargetType` — a bundle with two `type: Type Template`
  nodes folds into a map keyed by `target_type`; non-template nodes ignored.
- `TestTemplateDrift_MissingRequiredField` — a `Playbook` node missing an
  `owner` required field → one drift finding naming the field.
- `TestTemplateDrift_MissingBodySection` — a `Playbook` node whose body lacks a
  `## Rollback` heading → one drift finding naming the section.
- `TestTemplateDrift_Conformant_NoFindings` — a node satisfying all required
  fields + sections → zero findings.
- `TestTemplateDrift_NoGoverningTemplate_NoFindings` — a node whose type has no
  template → zero findings (unknown types are fine, §7.4).
- `TestTemplateDrift_Deterministic` — repeated calls return byte-identical order.
**GREEN:** `internal/okf/template.go` — `Template` struct, `Templates(b)`,
`TemplateDrift(b)`, `TemplateScaffold(t)`; helpers `hasSection(body, name)` (scan
`## <name>`) and `stringSlice(fm, key)` (tolerant `[]any`→`[]string`). Sorted
paths for determinism.
**COMMIT:** `feat(okf): type-template model + drift overlay`

## Task 2 — `template list` / `template show` commands
**RED:** `cmd/template_test.go`
- `TestTemplateList` — lists each `target_type` with its required-field +
  section counts.
- `TestTemplateShow` — `template show Playbook` prints required/recommended
  fields + body sections.
- `TestTemplateShow_Unknown` — `template show Nope` exits nonzero with a clear
  "no template governs type" message.
**GREEN:** `cmd/template.go` — `newTemplateCmd()` with `list`/`show`
subcommands, positional `[bundle-dir]` (like `validate`/`lint`); register in
`cmd/root.go`.
**COMMIT:** `feat(cmd): template list and show`

## Task 3 — `node new --type` scaffolds from a governing template
**RED:** extend `cmd/node_test.go` (or a focused `cmd/node_scaffold_test.go`)
- `TestNodeNew_ScaffoldsFromTemplate` — with a governing `Playbook` template,
  `node new --type Playbook --title X` writes a node stubbing the required
  fields + `##` body-section headings; the result passes `validate --templates`.
- `TestNodeNew_NoTemplate_Unchanged` — with no governing template, output is
  byte-identical to today's `node new` (regression guard on the existing path).
**GREEN:** thread `authoring.NewNode` (or a new `NewNodeFromTemplate`) so `node
new` reads `Templates(b)` and, when a match exists, stubs required fields +
sections. NO change to the no-template path.
**COMMIT:** `feat(cmd): node new scaffolds from governing template`

## Task 4 — `validate --templates` overlay + README + full gate
**RED:** extend `cmd/validate_test.go`
- `TestValidateTemplates_ReportsDrift` — `validate --templates` on a bundle with
  a drifting node prints a `warning:` line, exit 0.
- `TestValidateTemplates_Strict` — same bundle with `--strict` exits nonzero.
- `TestValidate_FloorUnchangedWithoutFlag` — plain `validate` on a bundle whose
  nodes drift (but have valid types) still exits 0 (floor purity, §7.4).
- `TestValidateTemplates_FloorStillFails` — a node with empty `type` fails plain
  `validate` AND `validate --templates` (floor is non-negotiable).
**GREEN:** add `--templates` + `--strict` flags to `cmd/validate.go`; run
`okf.Validate` (floor, always) then, when `--templates`, `okf.TemplateDrift`
printed as warnings. Floor findings always fail; drift fails only under
`--strict`. README `template` + `validate --templates` docs.
**FULL GATE:** gofmt -l (empty) · `go vet ./...` · `go build ./...` ·
`go test ./... -race -count=1` · `go mod tidy -diff` (no delta) · confirm
`internal/okf` imports no cobra · no new deps.
**COMMIT:** `feat(cmd): validate --templates overlay + README + full gate`

## Whole-increment review (E2E on the built binary)
Build `/tmp/okfctl-templates`; author a real template bundle + a knowledge bundle
referencing it, then exercise: `template list`/`show`; `node new --type Playbook`
(scaffolds conformant); a hand-drifted node → `validate --templates` warns (exit
0), `--strict` exits 1; plain `validate` on the drifted-but-typed node still
exits 0 (floor purity); an empty-type node fails both. Confirm READY_TO_MERGE.
