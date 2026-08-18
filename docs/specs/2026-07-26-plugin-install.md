# Spec: plugin install (convenience installer over PATH dispatch)

Status: Implemented  Owner: Casey West  License: Apache-2.0
Increment: 7 (partial)—`plugin install`, the slice deferred from 5a
PRD: section 6.4 (extension model), section 8 (plugin model)
Predecessor: docs/specs/2026-07-24-plugin-dispatch.md (5a—Discover/Lookup/dispatch/`plugin list`)

## Goal

Ship `okfctl plugin install`, the convenience installer 5a deferred. `plugin list`
and PATH dispatch already exist; install is the missing half of the pair. It copies
an `okfctl-<name>` executable into an okfctl-managed plugins directory so that the
existing discovery/dispatch contract (unchanged) subsequently finds and runs it.
No second, parallel plugin mechanism is introduced.

## Model surface (internal/plugin, stdlib-only, no cobra)

- `InstallDir() string`—the managed plugins dir, the default install destination.
  Mirrors the okfconfig home resolution so there is a single config-home
  convention: `$OKFCTL_CONFIG_HOME/plugins`, else `<user config dir>/okfctl/plugins`,
  else `./.okfctl/plugins`. This directory must be on PATH for `Discover`/`Lookup`
  (and thus `plugin list` and dispatch) to find installed plugins.
- `Install(source, destDir string) (Plugin, error)`—copy the executable at
  `source` into `destDir`, returning the installed `Plugin`. `source` must be an
  existing regular file whose base name follows `okfctl-<name>`. The copy keeps that
  name, is written atomically (temp file + rename in `destDir`) with `0o755` so
  `Discover` finds it, and overwrites an existing same-named plugin. `destDir` is
  created if missing.

## Command surface

`plugin install <source> [--dir <dir>]`:
- Default destination is `InstallDir()`; `--dir` overrides it.
- On success prints `installed okfctl-<name> -> <abs path>` to stdout.
- If the destination is not on `$PATH`, prints a note to stderr so the plugin is
  not silently undiscoverable.
- A source not named `okfctl-<name>`, a missing source, or a non-regular source is
  a clear error (non-zero exit), not a silent skip.

## Boundaries / decisions

- `internal/plugin` stays stdlib-only, NO cobra import (mirrors 5a and internal/okf
  purity). Cobra wiring lives in `cmd/plugin.go`.
- Reuses the existing PATH-dispatch contract verbatim—install only places a file
  where `Discover` already looks. No registry, no network fetch, no manifest: those
  are the rest of increment 7 and are out of scope here.
- Atomic install (temp + rename) so a partially-copied binary is never left on PATH.
- Unix executable semantics (macOS/Linux), consistent with 5a; Windows deferred.

## Done criteria (exercised on the built binary end to end)

1. `plugin install <okfctl-demo> --dir <d>` copies the executable to
   `<d>/okfctl-demo`, exit 0, and reports the path.
2. Round-trip with the real in-repo `okfctl-search`: build it, install via the
   command, `plugin list` discovers it, dispatch (`okfctl search --help`) runs it
   exit 0.
3. Default (no `--dir`) install lands under `InstallDir()`; `OKFCTL_CONFIG_HOME`
   redirects it.
4. Bad source (wrong name / missing / non-regular) errors non-zero.
5. `go build ./...` and `go test -race ./...` green; gofmt/vet clean;
   internal/plugin cobra-free.
