# AGENTS.md — how work flows in this repo (`cwest/okfctl`)

This file is the operating contract for any agent (or human) making changes to
this repo. It is auto-injected as project context whenever a session or job runs
with this directory as its working directory. Read it before you touch anything
here.

## The one fact that governs everything

**`okfctl` is a consumer of a spec it does not own. The Open Knowledge Format
specification is the authority; this tool conforms to it. Where the spec defines
behavior, the spec wins — over convenience, over a reviewer's preference, over
what the code happens to do today.**

### Where the spec lives — read the upstream, not a mirror

The authoritative source is:

```
https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md
raw: https://raw.githubusercontent.com/GoogleCloudPlatform/knowledge-catalog/main/okf/SPEC.md
```

**This trap is live and it has already caught us.** The obvious-looking source —
`openknowledge-sh/openknowledge`, `Wiki/SPEC.md` — is a *pinned mirror*, and it is
**stale at v0.1** while upstream is at **v0.2**. Its own frontmatter names the real
upstream in a `resource:` key. Anyone who greps for "the spec," finds that file,
and builds against it will conform to a superseded version and be confident about
it. Always fetch upstream and check the version line before citing a section
number — §-numbering *shifted between v0.1 and v0.2* (v0.1 had 11 sections, v0.2
has 13; index files moved from §6 to §8, conformance from §9 to §11, versioning
from §11 to §12).

Confirm the version before you cite anything:

```sh
curl -sL https://raw.githubusercontent.com/GoogleCloudPlatform/knowledge-catalog/main/okf/SPEC.md \
  | head -5    # expect: **Version 0.2**
```

### Version state — v0.1 pinned, v0.2 shipped, gap open

| Where | Version |
|---|---|
| Upstream spec | **0.2** |
| `SpecVersion` (`internal/okf/reserved.go`) | `0.1` |
| Real corpus (`knowledge-base` `.okf`) | `0.1` |

The tool and the corpus are both still v0.1. **Closing that gap is tracked work —
do not close it as a side effect of an unrelated change.** v0.2 is a minor bump
(§13) with two deliberate breaking renames: `timestamp` → `generated.at`, and the
body `# Citations` list → frontmatter `sources`. Consumers MAY fall back to the
legacy forms, so v0.1 bundles stay readable.

A bundle declares its own target via `okf_version` in the `.okf` sidecar; the tool
reads it rather than assuming. Per §12, a consumer that does not understand a
declared version SHOULD attempt best-effort consumption rather than refusing the
bundle — so **never make an unrecognized `okf_version` a hard failure.**

`okfctl` is **not** a spec-authoring tool (PRD "Non-goals"). It does not invent
rules, does not add a taxonomy the spec leaves open, and does not tighten a
permissive clause into a strict one. Two failure directions, both real:

- **Under-conformance** — emitting or accepting something the spec forbids.
- **Over-conformance** — rejecting something the spec permits. §7.4 leaves `type`
  values open, so an unknown `type` MUST pass the floor. Inventing a closed
  vocabulary is a spec violation in the strict direction, and it is the easier
  one to ship by accident because it *looks* like rigor.

The line the PRD draws: the tool enforces the **spec floor** for everyone, and
puts anything beyond it behind an explicit opt-in overlay (`--templates`, §9.4)
that never leaks into the floor.

## The conformance gate — run it, don't assume it

**Every feature or fix that touches OKF-defined behavior must be validated
against the spec before it goes to review, and the validation must appear in the
PR body as run output — not as a claim.**

Three layers, cheapest first. Run all three:

```sh
# 1. Spec-conformance suite — the closed generate→validate loop.
go test ./internal/okf/ ./cmd/ -run Conformance -race -v

# 2. Full suite (conformance is necessary, not sufficient).
gofmt -l . && go vet ./... && go test ./... -race

# 3. REAL CORPUS — the layer fixtures cannot substitute for.
go build -o /tmp/okfctl-check . && \
  /tmp/okfctl-check validate ~/src/knowledge-base/bundles/knowledge && \
  /tmp/okfctl-check lint --strict ~/src/knowledge-base/bundles/knowledge
```

**Layer 3 is not optional and fixtures do not replace it.** A hand-built fixture
holds a handful of nodes that the author already had in mind; the real corpus
(`~/src/knowledge-base`, bundle `bundles/knowledge`, ~239 nodes) holds the shapes
nobody anticipated. Increment 8's history is the argument: 319 ALLCAPS false hits
from frontmatter (`VERIFIED` ×219), 159 findings on reserved `index.md`/`log.md`,
129 of 137 `missing-xref` hits that were the bare words "index"/"log", and 1,333
sentence-initial-cap false positives — **every one invisible at fixture scale.**

Pin the before/after counts in the PR body. "It passes" is not a result; `validate
3→0, broken-link 0→0` is. A count that did not move is as load-bearing as one that
did — it is the control proving the change didn't silently start hiding findings.

### 4. Conform against the version the bundle declares

The corpus is v0.1 today, so a real-corpus run alone will never exercise v0.2.
Any change touching frontmatter, provenance, or freshness must ALSO be run against
a v0.2 fixture — otherwise "it passes" only means "v0.1 still passes."

Permissiveness makes this trap quiet: a v0.2 bundle currently **validates clean and
lints clean**, because §7.4's open `type` and unknown-key tolerance let every new
v0.2 key through untouched. That is correct floor behavior and it is *not* support.
**Passing is not the same as understanding** — no code path currently reads
`sources`, `generated`, `verified`, `status`, `stale_after`, or the
`Attested Computation` type. Never report "v0.2 works" off a green validate.

### Both controls, every time

A check that only fires less is indistinguishable from a check that broke. So a
change to any detector needs both directions proven:

- **Positive control** — the defect the change targets still gets caught.
- **Negative control** — a case that legitimately looks similar stays silent.

The negative control is the one that gets skipped, and it is the load-bearing one.

## Citing the spec in code and review

When a behavior is spec-mandated, **cite the section** (`§6`, `§7.4`, `§9.4`,
`§11`) in the code comment, the test name, and the PR body. The existing suites do
this — read `internal/okf/conformance_test.go` for the house pattern. A citation
turns "the reviewer disagreed" into "the spec says," which is the whole point: the
spec, not taste, arbitrates.

If a spec section is genuinely ambiguous, **stop and surface the ambiguity** — do
not resolve it silently in code. An interpretation baked into a merged commit is a
decision nobody reviewed.

## Repo conventions

- **Deploy checkout stays on `main`, permanently.** All in-flight work happens in
  `.worktrees/<card-id>` on a `wt/<card-id>` branch. A topic branch checked out on
  the deploy clone breaks the post-merge `git pull --ff-only`.
- **Apache headers read `Copyright <year> Google LLC`** — not Casey West. Match
  the existing files exactly.
- **Commits are SSH-signed** as `Casey West <casey@geeknest.com>`, Conventional
  format, no AI attribution and no agent/teammate names in any commit message, PR
  title/body, issue, review comment, or code comment.
- **No CGO, no Python, no ONNX.** Pure Go; `CGO_ENABLED=0` must build.
- **Two products stay separate** — the shipped generic skills and the migration
  skills. The leak-grep gate exists to keep them that way; run it.

## Definition of done

A change is done when all of the following hold — not when the code works:

1. Spec-conformance suite green (`-run Conformance -race`).
2. Full suite green under `-race`; `gofmt -l` empty; `go vet` clean.
3. Real-corpus run executed, with before/after counts pinned in the PR body.
4. Both controls proven for any detector change.
5. Spec sections cited for any spec-mandated behavior.
6. Commit signed, Conventional, team-invisible.
