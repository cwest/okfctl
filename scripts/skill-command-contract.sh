#!/usr/bin/env bash
# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# skill-command-contract.sh — prove the SHIPPED skills work, not just the
# manifest. It runs every command the four shipped skills document against the
# okfctl / okfctl-search binaries already on PATH (the CI job installs the
# RELEASED binaries there, which is the whole point: a `go build` would hide a
# packaging break). The manifest gate proves plugin.json is well-formed; this
# proves an agent following the SKILLs gets working commands.
#
# Three parts, each described where it is defined below:
#   1. existence sweep  — every documented subcommand resolves (--help exits 0)
#   2. workflow runs     — each skill's happy path, asserting output AND exit code
#   3. --self-test       — the negative control: plant a bogus command, prove
#                          the sweep goes RED (a job that cannot fail is
#                          decoration)
#
# Usage:
#   skill-command-contract.sh              run self-test, then sweep + workflows
#   skill-command-contract.sh --self-test  run only the negative control

set -euo pipefail

OKFCTL="${OKFCTL:-okfctl}"
OKFCTL_SEARCH="${OKFCTL_SEARCH:-okfctl-search}"

info() { printf '==> %s\n' "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; return 1; }

# ---------------------------------------------------------------------------
# Extraction
# ---------------------------------------------------------------------------
# extract_commands SKILLS_DIR: print one "binary<TAB>command<TAB>subcommand" row
# per DISTINCT documented invocation found in fenced code blocks under
# SKILLS_DIR. Only fenced code blocks are scanned, so prose that merely names a
# verb ("okfctl migrate refuses to guess") is not mistaken for a command. A line
# is an invocation when, after stripping a leading "$ " prompt, its first token
# is `okfctl` or `okfctl-search`. The next token is the command; the token after
# that is the subcommand IF it looks like one (lowercase, may contain a dash) and
# is not a flag or a path — group commands (node, bundle, index, ...) always take
# a real subcommand, so this is what makes `node list` distinct from a bare
# positional bundle arg.
extract_commands() {
  local skills_dir="$1"
  local f
  for f in "$skills_dir"/*/SKILL.md; do
    [ -f "$f" ] || continue
    awk '
      /^```/ { infence = !infence; next }
      !infence { next }
      {
        line = $0
        sub(/^[[:space:]]*\$[[:space:]]+/, "", line)   # drop a "$ " prompt
        sub(/^[[:space:]]+/, "", line)
        n = split(line, tok, /[[:space:]]+/)
        if (n < 2) next
        bin = tok[1]
        if (bin != "okfctl" && bin != "okfctl-search") next
        cmd = tok[2]
        # command must be a bare word (a subcommand name), not a flag/path/redir
        if (cmd !~ /^[a-z][a-z-]*$/) next
        subcmd = ""
        if (n >= 3 && tok[3] ~ /^[a-z][a-z-]*$/) subcmd = tok[3]
        print bin "\t" cmd "\t" subcmd
      }
    ' "$f"
  done | sort -u
}

# ---------------------------------------------------------------------------
# 1. Existence sweep
# ---------------------------------------------------------------------------
# sweep_skills SKILLS_DIR: assert every documented command resolves against the
# installed binary. Returns 0 iff every documented command resolves; prints each
# unresolved command.
#
# Two subtleties, both grounded in what the binary actually reports (never a
# hardcoded command list that could itself drift):
#
#   1. `--help` exit code alone is NOT a valid existence probe: cobra treats an
#      unknown token after a GROUP command as an argument, prints the group's
#      help, and exits 0 (`okfctl node lst --help` -> exit 0). The reliable
#      discriminator is the `Usage:` line, which names the FULL command path for
#      a real command (`okfctl node list`) but only the group
#      (`okfctl node [command]`) for a bogus subcommand.
#
#   2. A GROUP command's 2nd token is a real subcommand (`node list`); a LEAF
#      command's 2nd token is a POSITIONAL argument (`lint mykb`, `validate
#      mykb`). We must probe the subcommand for a group but NOT mistake a leaf's
#      bundle-path arg for one. Group-ness is read from the binary: a group's
#      own `--help` Usage line ends in `[command]`.
sweep_skills() {
  local skills_dir="$1"
  local rc=0 bin cmd sub prog cmdhelp want subhelp
  while IFS=$'\t' read -r bin cmd sub; do
    [ -n "$bin" ] || continue
    prog="$OKFCTL"
    [ "$bin" = "okfctl-search" ] && prog="$OKFCTL_SEARCH"

    # Probe the command itself first. Its Usage line must name it.
    cmdhelp="$("$prog" "$cmd" --help 2>&1)" || { echo "  unresolved (non-zero): $bin $cmd"; rc=1; continue; }
    case "$cmdhelp" in
      *"$bin $cmd"*) : ;;
      *) echo "  unresolved: $bin $cmd (no Usage line names it)"; rc=1; continue ;;
    esac

    # Only descend into a subcommand when $cmd is a GROUP (its Usage line ends in
    # "[command]"). For a leaf command the captured 2nd token is a positional
    # arg, not a subcommand — probing it would be a false positive.
    [ -n "$sub" ] || continue
    case "$cmdhelp" in
      *"$bin $cmd [command]"*) : ;;   # group — descend
      *) continue ;;                   # leaf — 2nd token is a positional arg
    esac
    want="$bin $cmd $sub"
    subhelp="$("$prog" "$cmd" "$sub" --help 2>&1)" || { echo "  unresolved (non-zero): $want"; rc=1; continue; }
    case "$subhelp" in
      *"$want"*) : ;;
      *) echo "  unresolved: $want (no Usage line names it)"; rc=1 ;;
    esac
  done < <(extract_commands "$skills_dir")
  return "$rc"
}

# ---------------------------------------------------------------------------
# 2. Workflow runs — each asserts OUTPUT and EXIT CODE, not just "it ran".
# ---------------------------------------------------------------------------
# Invocation form is part of the contract the skills document: `node new` /
# `node list` take `--bundle <name>`; `index build` / `index check` / `lint` /
# `validate` / `bundle info` take a POSITIONAL path. Each workflow below uses the
# exact form its SKILL documents, so a one-sided change (skill OR CLI) fails CI.

assert_contains() { # HAYSTACK NEEDLE LABEL
  case "$1" in
    *"$2"*) : ;;
    *) fail "$3: expected output to contain '$2', got: $1" ;;
  esac
}

workflow_authoring() {
  info "okf-authoring: bundle init -> node new -> node list -> index build -> validate -> lint --strict -> bundle info"
  local out
  "$OKFCTL" bundle init mykb >/dev/null 2>&1 || fail "authoring: bundle init"
  # `node new` / `node list` take --bundle; the rest take a positional path.
  "$OKFCTL" node new concepts/tannin.md --type Reference --title "Tannin" --bundle mykb >/dev/null 2>&1 \
    || fail "authoring: node new"
  out="$("$OKFCTL" node list --bundle mykb)"
  assert_contains "$out" "concepts/tannin.md" "authoring: node list"
  "$OKFCTL" index build mykb >/dev/null 2>&1 || fail "authoring: index build"
  out="$("$OKFCTL" validate mykb)"
  assert_contains "$out" "conforms to the OKF spec floor" "authoring: validate"
  # After index build the node is reachable, so --strict must exit 0.
  "$OKFCTL" lint mykb --strict >/dev/null 2>&1 || fail "authoring: lint --strict exited non-zero on a clean indexed bundle"
  out="$("$OKFCTL" bundle info mykb)"
  assert_contains "$out" "nodes: 1" "authoring: bundle info node count"
  # bundle init created ./mykb in CWD; clean it up.
  rm -rf mykb
  echo "  authoring workflow OK"
}

workflow_curation_health() {
  info "okf-curation-health: lint --strict (exit 0 clean), index check (exit 0 current)"
  local out
  "$OKFCTL" bundle init mykb >/dev/null 2>&1 || fail "curation: bundle init"
  "$OKFCTL" node new concepts/oak.md --type Concept --title "Oak" --bundle mykb >/dev/null 2>&1 \
    || fail "curation: node new"
  "$OKFCTL" index build mykb >/dev/null 2>&1 || fail "curation: index build"
  # lint --strict on a clean, indexed bundle exits 0.
  "$OKFCTL" lint mykb --strict >/dev/null 2>&1 || fail "curation: lint --strict exited non-zero on a clean indexed bundle"
  # index check exits 0 when the index is current.
  out="$("$OKFCTL" index check mykb)"
  assert_contains "$out" "index.md is current" "curation: index check"
  rm -rf mykb
  echo "  curation-health workflow OK"
}

workflow_migrate_plan() {
  info "okf-migrate-plan: migrate <v0.1 bundle> --plan <file>; assert the plan file is written (target_version 0.2)"
  local w plan out
  w="$(mktemp -d)"
  mkdir -p "$w/v01/concepts"
  printf 'okf_version: 0.1\n' > "$w/v01/.okf"
  printf -- '---\ntype: Index\ntitle: Index\nokf_version: 0.1\n---\n\n# Index\n\n- [Tannin](concepts/tannin.md)\n' > "$w/v01/index.md"
  printf '# Change Log\n' > "$w/v01/log.md"
  printf -- '---\ntype: Concept\ntitle: Tannin\ntimestamp: 2026-01-15T09:30:00Z\n---\n\n# Tannin\n\nSee https://en.wikipedia.org/wiki/Tannin\n\n# Citations\n\n- https://en.wikipedia.org/wiki/Tannin\n' > "$w/v01/concepts/tannin.md"
  plan="$w/migrate-plan.json"
  out="$("$OKFCTL" migrate "$w/v01" --plan "$plan")"
  assert_contains "$out" "Wrote $plan" "migrate: plan write message"
  [ -f "$plan" ] || fail "migrate: plan file was not written to $plan"
  assert_contains "$(cat "$plan")" '"target_version": "0.2"' "migrate: plan target_version"
  rm -rf "$w"
  echo "  migrate-plan workflow OK"
}

workflow_semantic_search() {
  info "okf-semantic-search: config list (exit 0); lint --semantic WITHOUT an index -> exit 1 naming 'okfctl-search index build'"
  local out rc
  # config list is read-only and exits 0 (empty config is fine).
  "$OKFCTL" config list >/dev/null 2>&1 || fail "semantic: config list exited non-zero"
  "$OKFCTL" bundle init mykb >/dev/null 2>&1 || fail "semantic: bundle init"
  "$OKFCTL" node new concepts/tannin.md --type Concept --title "Tannin" --bundle mykb >/dev/null 2>&1 \
    || fail "semantic: node new"
  # lint --semantic with no index must ERROR (exit 1) with the actionable message.
  set +e
  out="$("$OKFCTL" lint mykb --semantic 2>&1)"
  rc=$?
  set -e
  [ "$rc" -ne 0 ] || fail "semantic: lint --semantic without an index unexpectedly exited 0"
  assert_contains "$out" "okfctl-search index build" "semantic: missing-index error names the fix"
  rm -rf mykb
  echo "  semantic-search workflow OK"
}

run_workflows() {
  local w
  w="$(mktemp -d)"
  ( cd "$w" && workflow_authoring )
  ( cd "$w" && workflow_curation_health )
  workflow_migrate_plan
  ( cd "$w" && workflow_semantic_search )
  rm -rf "$w"
}

# ---------------------------------------------------------------------------
# 3. Self-test (negative control)
# ---------------------------------------------------------------------------
# Prove the sweep CAN fail: plant a bogus command into a scratch copy of the
# skills, assert the sweep goes RED; then confirm the pristine copy sweeps
# GREEN. A green-only run proves nothing — a job that cannot fail is decoration.
self_test() {
  local repo_root="$1"
  local scratch
  scratch="$(mktemp -d)"
  cp -R "$repo_root/skills" "$scratch/skills"

  # Green baseline: the pristine copy sweeps clean against the installed binary.
  if ! sweep_skills "$scratch/skills"; then
    rm -rf "$scratch"
    fail "self-test: pristine skills did not sweep green"
  fi

  # Plant a bogus command; the sweep MUST go red.
  # shellcheck disable=SC2016  # literal command text — no expansion intended
  printf '\n```sh\n$ okfctl node lst --bundle mykb\n```\n' \
    >> "$scratch/skills/okf-authoring/SKILL.md"
  if sweep_skills "$scratch/skills" >/dev/null 2>&1; then
    rm -rf "$scratch"
    fail "self-test: sweep did NOT go red on a bogus command (okfctl node lst)"
  fi

  rm -rf "$scratch"
  echo "self-test OK: the sweep catches a bogus documented command"
}

# ---------------------------------------------------------------------------
main() {
  local repo_root
  repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

  if [ "${1:-}" = "--self-test" ]; then
    self_test "$repo_root"
    return 0
  fi

  info "okfctl:        $("$OKFCTL" version 2>&1 || echo '(version failed)')"
  info "okfctl-search: $("$OKFCTL_SEARCH" --help >/dev/null 2>&1 && echo present || echo MISSING)"

  # The negative control runs first, every time, so the guard can never rot into
  # a silent no-op that green-lights real drift.
  info "negative control (self-test)"
  self_test "$repo_root"

  info "existence sweep — every documented subcommand resolves"
  if sweep_skills "$repo_root/skills"; then
    echo "  existence sweep OK"
  else
    fail "existence sweep found a documented command the shipped CLI does not have"
  fi

  info "workflow runs"
  run_workflows

  echo "ALL SKILL-COMMAND CONTRACT CHECKS PASSED"
}

main "$@"
