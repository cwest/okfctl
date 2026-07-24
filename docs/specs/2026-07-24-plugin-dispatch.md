# Spec: Increment 5a - Core plugin dispatch (git/kubectl style)

Status: Approved  Owner: Casey West  License: Apache-2.0
Increment: 5a (substrate for 5b okfctl-search; also required by increment 7 registry/install)
PRD: section 8 (plugin model), section 5.1 (dependency-free core)

## Goal

Give okfctl a git/kubectl-style PATH-dispatch plugin mechanism so an unknown
subcommand `okfctl foo bar` execs an `okfctl-foo` binary found on PATH, passing
through the remaining args, flags, and environment, and propagating its exit code.
Built-in subcommands always win; the core stays dependency-free and oblivious to
which plugins exist. Ship `plugin list` for discovery. Defer `plugin install`
(a convenience installer) to a later slice to keep this increment proportionate.

## Model surface

New pure-ish package `internal/plugin` (stdlib only; no cobra import):

- `Discover(pathenv string) []Plugin` - scan every dir on PATH for executables
  named `okfctl-<name>`; return sorted, de-duplicated by name (first on PATH wins),
  each carrying `{Name, Path}`. Deterministic order (sorted by Name).
- `Lookup(name, pathenv string) (string, bool)` - resolve `okfctl-<name>` to an
  absolute path via the same PATH scan (thin wrapper used by dispatch).
- `Plugin` struct: `{Name string; Path string}`.

Executable test on Unix: regular file (or symlink to one) with any owner/group/
other execute bit set. Keep it dependency-free using `os.Stat` + mode bits.

## Command surface

1. `plugin list [--path <PATH override>]` - print discovered plugins, one per
   line as `okfctl-<name>\t<abs-path>`, sorted by name. Empty discovery prints a
   friendly "no okfctl plugins found on PATH" to stderr, exit 0.

2. PATH dispatch on unknown subcommand. When `okfctl <foo> [args...]` names no
   built-in subcommand:
   - resolve `okfctl-<foo>` on PATH;
   - found -> exec it with `[args...]` (all tokens after `<foo>`), inheriting
     stdin/stdout/stderr and the parent environment plus `OKFCTL=<abs path to
     okfctl>` so a plugin can call back into core; propagate the child's exit code
     verbatim (0 stays 0, non-zero passes through).
   - not found -> the existing cobra "unknown command" error PLUS a did-you-mean
     suggestion drawn from built-ins AND discovered plugins; exit non-zero.

## Boundaries / decisions

- `internal/plugin` is stdlib-only, NO cobra import (mirrors internal/okf purity).
  The cobra wiring (RunE / dispatch interception) lives in cmd/.
- Built-ins ALWAYS take precedence: dispatch fires only for genuinely unknown
  subcommands (cobra found no matching command).
- Exit-code fidelity is a hard done-criterion: `okfctl foo` where `okfctl-foo`
  exits 7 must make okfctl exit 7. Use exec error inspection (exec.ExitError) to
  read the child code; never collapse to 1.
- Windows executable-bit detection differs; scope 5a to Unix semantics (darwin/
  linux), matching the dev+CI platform. Note the Windows gap in README, defer.
- `plugin install` deferred (own slice). `plugin list` only for 5a.

## Done criteria (exercised on the built binary end to end)

1. `plugin list` discovers a real `okfctl-demo` placed on a temp PATH, prints it
   sorted with its abs path, exit 0; empty PATH -> friendly stderr note, exit 0.
2. `okfctl demo hello --flag` with `okfctl-demo` on PATH execs the plugin, which
   receives exactly `hello --flag`, sees `OKFCTL` in its env, and its stdout is
   passed through.
3. Exit-code fidelity: a plugin that exits 7 makes `okfctl` exit 7.
4. Unknown subcommand with NO matching plugin prints the unknown-command error +
   a did-you-mean suggestion and exits non-zero.
5. A built-in (e.g. `validate`) still runs as a built-in - dispatch never shadows
   it. Full -race suite green; gofmt/vet clean; internal/plugin cobra-free;
   stdlib-only (no new module deps).
