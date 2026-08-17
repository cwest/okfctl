# test(agentplugin) — prove the SKILLS work, not just the manifest

**Status:** Approved (design) · **Owner:** Casey West · **License:** Apache-2.0
**Source of truth:** the four shipped skills under `skills/*/SKILL.md`. No new
spec behavior — this adds coverage of the *documented CLI contract*, not rules.
**Base:** `main` @ `764bf94`  **Branch:** `topic/skill-command-contract`

## Problem

The plugin **manifest** (`plugin.json`) is gated twice over — an offline and a
pinned-remote schema check with a real negative control. But the four **skills**
the plugin ships (`okf-authoring`, `okf-curation-health`, `okf-migrate-plan`,
`okf-semantic-search`) are tested nowhere: no test executes a single command
they instruct an agent to run.

That is the failure mode most likely to hit a real plugin user. A CLI flag or
subcommand is renamed; the SKILL.md still documents the old form; the manifest
gate stays green because the manifest did not change; the plugin silently rots.
A skill drifting out of sync with the shipped CLI is invisible to every gate we
have today.

This was verified manually once (2026-08-17) against the released binary. That
run proved the system works — and evaporated when the session ended. It needs to
be a job.

## Design

Add a CI job that **installs the released binary from the public path** and runs
each skill's documented commands end to end. The thing under test is what a
plugin user actually installs, so a local `go build` is explicitly excluded — it
would hide a packaging break (a missing `okfctl-search` in the archive, a broken
install script, a bad release name template).

Two artifacts:

1. **`scripts/skill-command-contract.sh`** — the contract runner. It takes
   `okfctl` and `okfctl-search` off PATH (the CI job puts the released binaries
   there) and does three things:

   - **Existence sweep.** Extract every `okfctl <cmd>` / `okfctl-search <cmd>`
     invocation from fenced code blocks in `skills/*/SKILL.md`, dedupe to
     `(command, subcommand)` pairs, and assert each resolves — `<bin> <cmd>
     [<sub>] --help` must exit 0. A renamed or removed subcommand fails here.
     Only fenced code blocks are scanned; prose mentions ("okfctl migrate
     refuses to guess") are not commands and are excluded by construction.

   - **Workflow runs.** Execute the happy path each skill documents, in a temp
     dir, asserting on **output and exit code**, not merely "it ran":

     | skill | workflow |
     |---|---|
     | `okf-authoring` | `bundle init` → `node new` → `node list` → `index build` → `validate` → `lint --strict` → `bundle info` |
     | `okf-curation-health` | `lint --strict` (exit 0 on a clean, indexed bundle), `index check` (exit 0 when current) |
     | `okf-migrate-plan` | `migrate <v0.1-bundle> --plan <file>`; assert the plan file is written and `target_version` is `0.2` |
     | `okf-semantic-search` | `config list` (exit 0); `lint --semantic` WITHOUT an index → assert exit 1 AND the message names `okfctl-search index build` |

   - **Invocation form is part of the contract.** `node list` / `node new` take
     `--bundle <name>`; `index build` / `index check` / `lint` / `validate` /
     `bundle info` take a POSITIONAL path. The runner exercises the exact form
     each SKILL documents, so if either side changes without the other, CI goes
     red.

   - **Negative-control self-test (`--self-test`).** Plant a bogus command
     (`okfctl node lst`) into a scratch copy of a skill, run the existence sweep
     against it, and assert the sweep goes RED; revert, assert GREEN. This proves
     the job *can* fail — a job that cannot fail is decoration. The self-test
     runs in CI on every invocation, so the guard can never silently rot into a
     no-op.

2. **`.github/workflows/ci.yml`** — a new `skill-contract` job:
   - Resolves the **latest** release at job time (no hardcoded version) so the
     job keeps testing what users actually install.
   - Installs via `install.sh` (the same public path the docs advertise).
   - Runs `scripts/skill-command-contract.sh` (which runs `--self-test` first,
     then the sweep + workflows).
   - Triggers on PRs touching `skills/**` or `cmd/**`, and on `main`.

## Why a shell integration test, not a Go test

`committed_test.go` and the existing suite prove the *contract shape* against the
in-tree source. This card's gap is different: it is the released **packaging**
plus the shipped CLI vs. the SKILL prose. Neither is reachable from `go test`,
which builds from source and would mask exactly the packaging/release breaks the
card names. The runner is therefore a shell script exercising installed binaries,
gated in CI, with its own negative control baked in.

## Non-goals

- **Not** a third manifest re-validation. `plugin.json` conformance is already
  covered by `committed_test.go` and the manifest CI job; the gap is the skills.
- **Not** tested against a locally built binary. The release path is the SUT.
- **Not** version-pinned. Latest release is resolved at job time.
- **Not** a model-dependent path. The semantic workflow deliberately exercises
  only the model-free surfaces (`config list`, the missing-index error), so the
  job needs no embedding model.

## Done when

- A CI job runs the released binary against every documented skill command.
- The PR body carries a witnessed RED (bogus command planted) and the matching
  GREEN (reverted), as pasted run output.
- The job runs on PRs touching `skills/**` or `cmd/**`, and on main.
- A skill documenting a command the shipped CLI does not have fails the build.
