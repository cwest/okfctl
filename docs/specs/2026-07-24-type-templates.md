# Spec — Increment 6: Type templates + `validate --templates` overlay

**Status:** Approved · **Owner:** Casey West · **License:** Apache-2.0
**Increment:** 6 (depends only on increments 1 + 3, both merged)
**Branch:** `topic/type-templates` off `main` @ 8a22307

## Goal

Deliver the type-template system (PRD §9) as its OWN OKF bundle convention — no
taxonomy lives in the tool (§7.4 stays intact). A template is an ordinary node
whose `type` is `Type Template`; it governs a `target_type` and declares
`required_fields` / `recommended_fields` / `body_sections`. Add the read-only
`template list`/`template show` verbs, teach `node new --type` to scaffold from a
governing template when one exists, and add an opt-in `validate --templates`
overlay that reports **template drift** as warning-class findings — never a
spec-floor failure.

## Design (from PRD §9)

### Template node shape (§9.2)
```yaml
---
type: Type Template
target_type: Playbook              # the node.type this template governs
required_fields: [title, description, owner]
recommended_fields: [tags, timestamp]
body_sections: [Trigger, Steps, Rollback, Verification]
---
```
Because a template is just a node, the template bundle `validate`s / `lint`s /
`index`es like any bundle. Self-hosting: `okfctl` reads templates from the same
`Load(root)` surface — no separate store, no tool config.

### Two-tier validation (§9.4) — THE boundary that must not blur
- **Spec floor (core, always on):** unchanged `Validate` — `type` present +
  non-empty (§7 rule 2). Unknown type VALUES still pass (§7.4). This increment
  does NOT touch `Validate`.
- **Template overlay (opt-in, `validate --templates`):** a NEW pure function
  `TemplateDrift(b, opts)` layered on top. For each concept node whose `type`
  matches a template's `target_type`, report drift when:
  - a `required_field` is missing or empty in the node's frontmatter, or
  - a `body_section` heading (`## <Section>`) is absent from the node's body.
  `recommended_fields` are NOT drift (advisory only, never reported as a finding
  in this increment — they drive scaffolding, §9.3).
  Drift findings are **warning-class**, never spec violations. `validate` without
  `--templates` is pure spec conformance.

### Scaffolding (§9.3)
`node new --type Playbook` — when a governing template exists — stubs the
template's `required_fields` as empty frontmatter keys, stubs
`recommended_fields`, and lays down `body_sections` as empty `##` headings, so a
new node starts conformant to both floor and convention. With NO governing
template, `node new --type` behaves exactly as today (unchanged path).

## Model surface (pure `internal/okf`, cobra-free)

New file `internal/okf/template.go`:
- `type Template struct { TargetType string; RequiredFields, RecommendedFields, BodySections []string; Path string }`
- `func Templates(b *Bundle) map[string]Template` — fold nodes with
  `type == "Type Template"` keyed by `target_type` (last-writer-wins is fine;
  a bundle should not ship two templates for one target). Reuses `n.Frontmatter`.
- `type DriftFinding struct { Path, TargetType, Message string }`
- `func TemplateDrift(b *Bundle) []DriftFinding` — deterministic (sorted paths),
  reuses the same section/field scanning helpers.
- `func TemplateScaffold(t Template) (fields []string, sections []string)` — the
  pieces `NewNode` consumes to stub a conformant node.

`TemplateDrift` MUST NOT import cobra (pure model). `node new` scaffolding wires
through the existing `authoring.NewNode` path in `cmd/node.go`.

## CLI surface

- `okfctl template list [bundle-dir]` — table of `target_type → required/sections`
  from the linked template bundle. Read-only. Positional bundle-dir (like
  `validate`/`lint`).
- `okfctl template show <target-type> [bundle-dir]` — full template detail.
- `okfctl node new --type <T> [--bundle <dir>]` — scaffolds from a governing
  template when one exists (unchanged when none).
- `okfctl validate [bundle-dir] --templates [--strict]` — floor always runs; with
  `--templates` the overlay runs and prints drift as `warning:` lines. Advisory
  by default (exit 0 even with drift); `--strict` exits non-zero on drift. Mirrors
  `lint`'s severity contract exactly. Floor violations ALWAYS fail regardless.

## Non-goals (deferred)
- Semantic/similarity drift (that's increment 5's `lint` wiring).
- `template new`/authoring verbs — templates are authored as ordinary nodes.
- Enforcing a fixed set of templates — the tool ships none (§9.5).

## Done criteria (verified E2E on the built binary)
1. `template list`/`show` read a template bundle correctly.
2. `node new --type Playbook` with a governing template stubs its required
   fields + body sections; with no template, behaves as today.
3. `validate --templates` reports drift (missing required field, missing section)
   as warnings, exit 0; `--strict` exits nonzero; floor-only `validate` unchanged.
4. Floor purity: a node with an unknown type + no template still passes plain
   `validate` (§7.4 intact).
5. Full `-race` suite green; gofmt/vet/`go mod tidy -diff` clean; `internal/okf`
   cobra-free; stdlib + existing deps only (no new deps).
