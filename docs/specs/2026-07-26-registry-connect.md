# Spec: `okfctl registry` / `okfctl connect` — git-backed remote bundle sources

## Problem

`registry` / `connect` is the one PRD-specified command family with zero
implementation. It appears in the capability table (§4, "Registry / connect:
Yes") and in both architecture diagrams (§6 command tree, §11 "git remotes /
registry / connect"). Today `okfctl` cannot reach a remote bundle at all —
every command operates only on a local directory.

## The hard boundary (narrow reading)

The PRD names the command but does not spell out its verbs, so this spec takes
the **narrowest reading** the surrounding constraints allow and states it here.

`okfctl` is explicitly **not** a hosted service, registry backend, or account
system (§5.2), and **must not** embed a schema registry (§9.1). "Registry" here
is therefore *not* a package/schema registry — it is a local, named directory of
**remote bundle sources**, each source being a plain **git URL**. This is the
same relationship `git remote` has to a repository: a name mapped to a URL, plus
the plumbing to fetch it. Nothing is hosted, no account exists, no schema is
registered.

- `registry` manages the *set of named remotes* (config-backed).
- `connect` *materializes* a remote (registered name or ad-hoc URL) into a local
  directory via git.

## Command surface

```
okfctl registry add <name> <git-url>     # register (or update) a named source
okfctl registry list                     # list name -> url, sorted by name
okfctl registry show <name>              # print one source's url
okfctl registry remove <name>           # unregister a named source (alias: rm)

okfctl connect <name|git-url> [dir]     # clone the source into dir (git clone),
                                         # or fast-forward it if dir already holds it
```

### `registry` — named remotes in the ONE config store

The registry lives in the existing `okfconfig` JSON store (§ shared config
mechanism, `~/.config/okfctl/config.json`), keyed `registry.<name> = <url>`.
There is deliberately no second config file: `okfctl` has one config mechanism,
and named remotes are config.

- `add` requires a non-empty name and a non-empty URL; a name that already
  exists is overwritten (idempotent re-point), reported as updated vs added.
- `list` prints `name\turl` for every `registry.*` key, sorted; empty registry
  prints a friendly "no remote sources registered".
- `show <name>` prints the URL, or errors non-zero on an unknown name.
- `remove <name>` deletes the key, or errors non-zero on an unknown name.

A name is restricted to a safe identifier (letters, digits, `-`, `_`, `.`) so
it can never collide with the `registry.` key prefix or smuggle characters.

### `connect` — clone/fetch over git

`connect` resolves its first argument to a URL: if it names a registered remote,
its URL is used; otherwise the argument is treated as an ad-hoc git URL. The
second, optional argument is the destination directory (default: a directory
named after the last path segment of the URL, minus any `.git`).

- If the destination does not exist (or is empty), `connect` runs
  `git clone <url> <dir>`.
- If the destination already exists and is a git work tree for that URL,
  `connect` runs `git -C <dir> pull --ff-only` — a safe update that never
  rewrites local history.
- If the destination exists, is non-empty, and is not a git repo, `connect`
  refuses rather than clobbering it.

git is shelled out to (matching `internal/okf/gitmeta.go`), so there is no new
dependency. When git is not installed, `connect` returns a clear, actionable
error (it cannot degrade to a no-op — the whole point is to fetch).

## Testing

- `registry` verbs round-trip through a temp `OKFCTL_CONFIG_HOME` (the existing
  config-test pattern): add → list → show → remove, plus unknown-name errors and
  name-validation rejection.
- `connect` is tested against a **local bare git repo fixture** (no network):
  init a bare repo, seed a bundle, `connect` it into a temp dir, assert the
  bundle materialized and `validate`s; then a second `connect` fast-forwards a
  new commit. git-absent and clobber-refusal paths are covered. All git-touching
  tests `t.Skip` when git is unavailable, matching `gitmeta_test.go`.

## Non-goals

- No hosted registry, no account/auth layer, no publish/push verb (a source is
  pushed to with plain `git`, outside `okfctl`).
- No schema/type registry — unchanged from §7.4/§9.1.
- No credential handling: authentication to a private git URL is git's own
  concern (ssh agent, credential helper), exactly as with any git remote.
