# Plan: `okfctl registry` / `okfctl connect`

Spec: [docs/specs/2026-07-26-registry-connect.md](../specs/2026-07-26-registry-connect.md)

TDD, one behavior at a time. Files:

- `internal/okf/gitsource.go`—thin git-clone/pull helpers (shell out, like `gitmeta.go`).
- `internal/okf/gitsource_test.go`—clone/pull against a local bare-repo fixture.
- `cmd/registry.go`—`newRegistryCmd()`: add/list/show/remove over `okfconfig`.
- `cmd/connect.go`—`newConnectCmd()`: resolve name→url, clone or ff-pull.
- `cmd/registry_test.go`, `cmd/connect_test.go`—command-level tests via `runOKF`.
- `cmd/root.go`—wire `newRegistryCmd()` and `newConnectCmd()`.

## Task 1—registry config keys (RED→GREEN)

Registry entries are `registry.<name>` keys in the shared okfconfig map.
Helpers in `cmd/registry.go`: `registryKey(name)`, `validRegistryName(name)`.

- RED: `TestRegistry_AddListShowRemove_RoundTrips`—add two, list shows both
  sorted, show returns one url, remove drops it, show after remove errors.
- RED: `TestRegistry_RejectsBadName`—a name with `/` or space errors non-zero.
- GREEN: implement the four subcommands over `loadConfig`/`saveConfig`.

## Task 2—git source clone/pull helper (RED→GREEN)

`internal/okf/gitsource.go`:
- `Clone(url, dir string) error`—`git clone url dir`.
- `PullFastForward(dir string) error`—`git -C dir pull --ff-only`.
- `IsGitWorkTree(dir string) bool`—`git -C dir rev-parse --is-inside-work-tree`.
- `GitAvailable() bool`—`exec.LookPath("git")`.

- RED: `TestClone_FromLocalBareRepo`—seed a bare repo, clone into temp, assert
  a seeded file lands. Skip if git absent.
- RED: `TestPullFastForward_PicksUpNewCommit`.
- GREEN: implement the shell-outs; return wrapped errors with git's stderr.

## Task 3—`connect` command (RED→GREEN)

`cmd/connect.go`: resolve arg (registry name else raw url), compute default dir
from url leaf (strip `.git`), then clone or ff-pull, refusing a non-repo
non-empty dir.

- RED: `TestConnect_ClonesRegisteredSource`—register a bare-repo url, connect,
  assert the bundle `validate`s. Skip if git absent.
- RED: `TestConnect_SecondRunFastForwards`.
- RED: `TestConnect_RefusesNonRepoDir`.
- RED: `TestConnect_UnknownGitAbsent` (structure only; skip if git present).
- GREEN: implement.

## Task 4—wire + docs

- Add both to `cmd/root.go`.
- `docs/PRD.md`: the capability table already says "Registry / connect: Yes";
  add a short §6 sentence documenting the implemented verbs.
- `README.md`: add the command surface entries.

## Verify

`gofmt -w .`, `go vet ./...`, `go build ./...`, `go test ./... -count=1`.
