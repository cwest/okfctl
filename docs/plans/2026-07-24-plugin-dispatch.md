# TDD Plan: Increment 5a - Core plugin dispatch

Spec: docs/specs/2026-07-24-plugin-dispatch.md
Branch: topic/plugin-dispatch  Base: main @ f0d9d4e
Execution: sequential TDD (tightly specced on an already-reviewed model), per-task
ground-truth verification + one whole-increment E2E binary review that gates the PR.

Each task: RED (failing test) -> GREEN (minimal impl) -> REFACTOR -> commit
(-S signed, Conventional, Copyright 2026 Google LLC header pre-written into new .go
files so addlicense is a no-op). Verify by real `go test` + SHA + signature.

## Task 0 - docs
Commit the spec + this plan. `docs(5a): spec + TDD plan for core plugin dispatch`.

## Task 1 - internal/plugin pure model (stdlib-only, no cobra)
RED: internal/plugin/plugin_test.go
  - TestDiscover_FindsExecutablesNamedOkfctlPrefix: lay a temp dir with
    `okfctl-alpha` (0755), `okfctl-beta` (0755), `okfctl-nonexec` (0644),
    `notaplugin` (0755), `other-okfctl-x` (0755, wrong prefix); Discover returns
    [alpha, beta] sorted, abs paths, nonexec + wrong-prefix excluded.
  - TestDiscover_DedupesFirstOnPathWins: two temp dirs both with `okfctl-alpha`;
    the one earlier in PATH wins; single entry returned.
  - TestLookup_ResolvesAndMisses: Lookup("alpha", path) -> abs path,true;
    Lookup("ghost", path) -> "",false.
GREEN: internal/plugin/plugin.go
  - Plugin{Name,Path}; Discover(pathenv) []Plugin; Lookup(name,pathenv)(string,bool).
  - Split pathenv on os.PathListSeparator; os.ReadDir each dir; keep entries with
    prefix "okfctl-"; stat for a regular-file + any-exec-bit; strip prefix -> Name;
    first-wins dedupe by Name; sort by Name.
Commit: `feat(plugin): PATH discovery of okfctl-* executables`.

## Task 2 - plugin list command
RED: cmd/plugin_test.go
  - TestPluginList_PrintsDiscoveredSorted: build a temp PATH with two fake
    okfctl-* execs (tiny shell stubs via t.TempDir + os.WriteFile 0755); run
    `plugin list --path <temp>`; stdout has both lines sorted `okfctl-<name>\t<path>`.
  - TestPluginList_EmptyIsFriendly: empty --path -> exit 0, friendly stderr note,
    empty stdout.
GREEN: cmd/plugin.go (newPluginCmd with `list` subcommand + --path override
  defaulting to os.Getenv("PATH")); register newPluginCmd() in cmd/root.go after
  newTemplateCmd(). Uses internal/plugin.Discover.
Commit: `feat(cmd): plugin list`.

## Task 3 - PATH dispatch on unknown subcommand + did-you-mean + README + full gate
RED: cmd/dispatch_test.go
  - TestDispatch_ExecsPluginWithArgsAndEnv: a fake `okfctl-demo` stub that echoes
    its args and asserts OKFCTL is set (writes a marker file / echoes env); run the
    root with args [demo hello --flag] and a temp PATH; assert the plugin saw
    exactly `hello --flag` and OKFCTL was present; stdout passed through.
  - TestDispatch_ExitCodeFidelity: stub `okfctl-boom` exits 7; dispatching `boom`
    yields exit 7 (inspect returned error / a testable dispatch func returning the
    code, not os.Exit in-process).
  - TestDispatch_UnknownNoPluginSuggests: unknown `valdate` with no plugin ->
    error mentions unknown command + suggests `validate`; non-zero.
  - TestDispatch_BuiltinNotShadowed: `validate <dir>` still runs the built-in even
    if an `okfctl-validate` sits on PATH.
  GREEN: wire cobra so an unknown subcommand routes to a dispatch func. Use a root
  RunE + `Args: cobra.ArbitraryArgs` guard OR cobra's command-not-found path;
  factor the exec into a testable `dispatch(name string, args []string, path string)
  (exitCode int, err error)` that internal tests call WITHOUT os.Exit, and have the
  cobra layer translate its return into the process exit. exec.Command with
  Stdin/Stdout/Stderr inherited, Env = append(os.Environ(), "OKFCTL="+self); read
  child code from exec.ExitError. Keep built-in precedence.
  Also: README `plugin` + dispatch docs (git/kubectl analogy; Unix-only note);
  full gate (gofmt -l, go vet, go build, go mod tidy -diff, internal/plugin has no
  cobra import, full -race suite).
Commit: `feat(cmd): PATH dispatch for okfctl-<name> plugins + README + full gate`.

## Whole-increment review (gates the PR)
Build the binary; place a real `okfctl-demo` stub on a temp PATH; exercise all 5
done-criteria on the built binary (list, dispatch+args+env, exit-code fidelity,
did-you-mean, built-in-not-shadowed). Confirm determinism of `plugin list`.
Then push -> file card -> draft PR -> wired review lane -> Casey acceptance.
