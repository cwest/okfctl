<!-- repo:managed -->
# Contributing

Thanks for your interest in contributing.

## The one rule that governs everything

`okfctl` is a **consumer of a specification it doesn't own**. The
[Open Knowledge Format spec](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md)
is the authority; this tool conforms to it. Where the spec defines behavior, the
spec wins — over convenience, over a maintainer's preference, over what the code
happens to do today.

Two failure directions matter equally:

- **Under-conformance** — emitting or accepting something the spec forbids.
- **Over-conformance** — rejecting something the spec *permits*. The spec leaves
  `type` values open (§7.4) and tolerates unknown keys (§11), so an unknown
  `type` or a future frontmatter key MUST pass `validate`. Inventing a closed
  vocabulary is a spec violation in the strict direction — and the easier one to
  ship by accident, because it looks like rigor. `okfctl` enforces the spec
  *floor* for everyone and keeps anything stricter behind an explicit opt-in
  overlay (`--templates`, §9.4).

When a behavior is spec-mandated, cite the section (`§6`, `§7.4`, `§9.4`, `§11`)
in the code comment, the test name, and the PR body. If a section is genuinely
ambiguous, open an issue rather than resolving it silently in code.

## Workflow

1. Open an issue to discuss substantial changes before coding.
2. Fork and create a topic branch (`topic/<short-name>`).
3. Write tests for your change first (the suites follow test-driven development).
4. Run the conformance gate below and confirm it's green.
5. Open a pull request that explains the *why* of the change and pins the gate
   output (see "What a good PR body contains").

## Running the conformance gate

Every change that touches OKF-defined behavior must be validated against the spec
*before* review — and the validation goes in the PR body as run output, not as a
claim. Run all three layers, cheapest first:

```sh
# 1. Spec-conformance suite — the closed generate -> validate loop.
go test ./internal/okf/ ./cmd/ -run Conformance -race -v

# 2. Full suite (conformance is necessary, not sufficient).
gofmt -l . && go vet ./... && go test ./... -race

# 3. Real corpus — the gate fixtures can't substitute for. Point the built
#    binary at a real OKF bundle (yours, or clone one) and confirm it stays clean:
go build -o /tmp/okfctl-check . && \
  /tmp/okfctl-check validate <path/to/a/real/bundle> && \
  /tmp/okfctl-check lint --strict <path/to/a/real/bundle>
```

Gate 3 isn't optional: a hand-built fixture only holds shapes the author
already had in mind. A real, sizable bundle surfaces the false positives and edge
cases fixtures never will. If your change touches a detector, prove **both**
controls: a *positive* control (the defect it targets still gets caught) and a
*negative* control (a case that legitimately looks similar stays silent). The
negative control is the one that gets skipped, and it's the one that proves your
change didn't just silence the detector.

If your change touches frontmatter, provenance, or freshness, also run it against
a v0.1 bundle — the documented legacy fallbacks must keep working, so "it passes"
on v0.2 alone isn't proof.

## What a good PR body contains

- **Why** the change exists — the problem, not just the diff.
- **Gate output.** Pin before/after counts, not "it passes." `validate 3→0,
  broken-link 0→0` is a result; a green checkmark is not. A count that didn't
  move matters as much as one that did — it's the control proving the change
  didn't silently start hiding findings.
- **Spec citations** for any spec-mandated behavior.
- **Both controls** for any detector change.

## Style

- Follow the conventions visible in the existing codebase. The shipped binary is
  pure Go and self-contained — it carries its own runtime, so it needs no C
  toolchain, interpreter, or model download, and `CGO_ENABLED=0 go build ./...`
  must pass.
- `gofmt -l .` must be empty and `go vet ./...` must be clean before you push.
- Keep commits focused and use [Conventional Commits](https://www.conventionalcommits.org/).
- Sign your commits.

## License

By contributing, you agree that your contributions will be licensed under the
project's [Apache-2.0 License](LICENSE).
