# okfctl Walking Skeleton—Design Spec

**Status:** Approved · **Owner:** Casey West · **License:** Apache-2.0
**Source of truth:** [`docs/PRD.md`](../PRD.md)—this spec implements the first
increment of that PRD's v1.

## Purpose

The PRD specifies the entire v1 tool. That is far too large for a single branch
or PR. This spec defines the **walking skeleton**: the smallest end-to-end-useful
increment, on which every later capability (`lint`, `serve`, the search plugin,
type templates) is a function over the same in-memory model.

The walking skeleton delivers the loop *init → author → validate*: a user can
scaffold a conformant OKF bundle, author nodes into it, and validate structural
conformance including the managed-`type` floor.

## Scope of THIS increment (PR #1)

1. **Project foundation**—Go module, `cobra` root command, layered config
   (`config get/set/list`), generated shell `completion`, CI (build + test +
   `go vet` + `gofmt` gate).
2. **Core bundle model**—the loader: walk a bundle directory, parse each file's
   YAML frontmatter + Markdown body, build an in-memory typed graph (nodes +
   edges from links). *Everything downstream operates on this model.*
3. **`bundle init`**—scaffold a minimal conformant bundle: root `index.md`,
   `log.md`, spec-version pin, directory shape.
4. **`validate`**—enforce the spec floor: parseable frontmatter, a non-empty
   `type` on every node (§7.1 managed-`type`), well-formed reserved files.
   Pass/fail with a non-zero exit on failure. NO `--templates` overlay yet
   (that ships with the type-template increment).
5. **`node new` / `node show` / `node list`**—create a node with a required,
   non-empty `type` (§7.2); print a node surfacing `type` (§7.3); list the bundle
   tree with a `type` column (§7.3).

## Explicitly OUT of scope for this increment

Deferred to later branches, each its own worktree → plan → TDD → PR:

- `lint` (the curation differentiator)—needs the model this slice builds.
- `graph` export + `serve` web visualizer.
- `okfctl-search` PATH-dispatch plugin + `sqlite-vec` vector index (§8)—and the
  §13.2 shell-out-vs-native-Go fork, deferred until that slice.
- Type templates + the `validate --templates` overlay (§9).
- `node edit` / `node mv` / `node rm`, `index build/check`, `log append/show`,
  `registry`/`connect`, `plugin install`.

Later commands are stubbed only where cobra needs the noun to exist for the tree
to be coherent; unimplemented verbs return a clear "not yet implemented" error,
never a silent no-op.

## Architecture (this increment)

Layered, per PRD §11. Decided stack (PRD §13.1):

- **CLI layer**—`spf13/cobra` (Apache-2.0) noun-verb tree; `spf13/viper` (MIT)
  only if config-file layering needs it beyond flags+env (defer until proven).
- **Core bundle model (stdlib-only where possible)**—`goldmark` +
  `goldmark-meta` (MIT) for Markdown + frontmatter, `gopkg.in/yaml.v3` for
  frontmatter typing. In-memory graph via `dominikbraun/graph` (Apache-2.0)—confirm health bar at adoption; if it fails the gate, fall back to a tiny
  internal adjacency structure (this slice only needs nodes + edges, not heavy
  analytics).
- **No plugins, no backends** in this slice. Core stays `CGO_ENABLED=0` static.

### Package layout

```
okfctl/
  main.go                    # thin entrypoint → cmd.Execute()
  cmd/                       # cobra commands (one file per noun)
    root.go                  # root command, global flags, config wiring
    bundle.go                # bundle init/info
    node.go                  # node new/show/list
    validate.go              # validate (spec floor)
    completion.go            # generated shell completions
    config.go                # config get/set/list
  internal/
    okf/                     # the core model — no cobra imports here
      node.go                # Node type: path, frontmatter (incl. type), body
      bundle.go              # Bundle: load a dir → typed graph
      frontmatter.go         # parse/serialize YAML frontmatter via goldmark-meta
      reserved.go            # index.md / log.md structure + scaffold
      validate.go            # spec-floor checks (returns findings, not exit codes)
    config/
      config.go              # layered config: flags > env > file > defaults
  testdata/                  # golden bundles: known-good + known-bad fixtures
```

**Boundary rule:** `internal/okf/` never imports `cobra`. Commands are thin
adapters that call model functions and format output. This keeps the model
testable in isolation and reusable by every later capability.

## Testing strategy (PRD §12)

- **Conformance fixtures** in `testdata/`: a known-good bundle and known-bad
  bundles (missing `type`, empty `type`, malformed frontmatter, broken reserved
  file). `validate` tests assert exact findings + exit code.
- **Golden-file tests** for `bundle init` scaffold output and `node new`
  frontmatter.
- **Unit tests** for the loader (frontmatter parse, edge extraction) and each
  validate check.
- **Model-free / deterministic**: this slice has no embedder, so no model
  download is needed in CI.

## Success criteria for this increment

1. `okfctl bundle init` produces a directory that `okfctl validate` passes.
2. `okfctl validate` FAILS (non-zero exit) a bundle with a node missing/empty
   `type`, and PASSES a bundle whose nodes carry unfamiliar `type` values
   (presence, not taxonomy—PRD §7.4).
3. `okfctl node new --type X` creates a conformant node; `node show`/`node list`
   surface `type`.
4. Clean `go build`, `go vet`, `gofmt`, and full `go test ./...` green in CI from
   a clean checkout.
