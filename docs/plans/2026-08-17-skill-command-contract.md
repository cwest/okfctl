# Plan — test(agentplugin): skill-command contract in CI

Base `main` @ `764bf94` · Branch `topic/skill-command-contract` · TDD throughout.
Spec: `docs/specs/2026-08-17-skill-command-contract.md`.

## Task 1 — RED: negative-control self-test fails on a bogus command

Write `scripts/skill-command-contract.sh` with only the extractor + existence
sweep + `--self-test` mode implemented. The self-test plants `okfctl node lst`
into a scratch skill copy and asserts the sweep exits non-zero.

RED signal: run `--self-test` while the sweep is a stub that always passes →
self-test must FAIL ("sweep did not go red on a bogus command"). This is the
genuine RED: it proves the harness detects a broken sweep before the sweep
itself works.

## Task 2 — GREEN: implement the existence sweep

Extractor: scan fenced code blocks in `skills/*/SKILL.md`; for each line whose
first token (after an optional `$ `) is `okfctl` or `okfctl-search`, capture the
`(command, subcommand)` pair; dedupe. For each pair, run `<bin> <command>
[<subcommand>] --help` and require exit 0. Any non-zero → sweep exits 1.

GREEN: `--self-test` now passes (bogus command → sweep red → self-test green;
revert → sweep green → self-test green). Existence sweep against the real
released binary passes.

## Task 3 — GREEN: workflow runs

Add the four workflow functions, each asserting output + exit code:
- authoring: full lifecycle, `bundle info` shows a node count.
- curation-health: `lint --strict` exit 0 on the indexed bundle; `index check`
  exit 0.
- migrate-plan: build a scratch v0.1 bundle, `migrate --plan`, assert plan file
  exists and contains `"target_version": "0.2"`.
- semantic-search: `config list` exit 0; `lint --semantic` (no index) exit 1 and
  stderr contains `okfctl-search index build`.

## Task 4 — CI job

Add `skill-contract` job to `.github/workflows/ci.yml`: resolve latest release,
install via `install.sh`, run the script. Trigger on `skills/**`, `cmd/**`, main.

## Task 5 — Witness RED + GREEN for the PR body

Plant a bogus command in one skill, run the script, capture the RED output.
Revert, run again, capture GREEN. Paste both in the PR body.

## Verification

`gofmt -l .` (no Go changed, but run anyway), `go vet ./...`, `go build ./...`,
`go test ./... -race`, `shellcheck scripts/skill-command-contract.sh`, and the
script itself against the released binary.
